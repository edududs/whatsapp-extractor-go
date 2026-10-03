// Package cli is the composition root. No adapter is selected inside the core.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/edududs/whatsapp-extractor-go/adapters/storage"
	"github.com/edududs/whatsapp-extractor-go/adapters/whatsapp"
	"github.com/edududs/whatsapp-extractor-go/application"
	"github.com/edududs/whatsapp-extractor-go/domain"
	"github.com/edududs/whatsapp-extractor-go/internal/config"
)

const help = `whatsapp-extractor-go — read-only WhatsApp extraction

Usage: whatsapp-extractor-go <command> [flags]

Commands:
  pair       Link a new device using a terminal QR code
  accounts   List paired account phone numbers
  groups     List groups as JSON (requires a paired account)
  run        Persist messages selected by the TOML watchlist
  check      Validate configuration without connecting
  version    Print build version

Flags: -config path/to/extractor.toml  -account 5511900000001
No config flag uses defaults and EXTRACTOR_* environment overrides.
Watchlists are edited directly in TOML. See extractor.example.toml.
`

func Run(ctx context.Context, args []string, out, errOut io.Writer, version string, lookup func(string) (string, bool)) (result error) {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := io.WriteString(out, help)
		return err
	}
	if args[0] == "version" {
		_, err := fmt.Fprintln(out, version)
		return err
	}
	command := args[0]
	switch command {
	case "pair", "accounts", "groups", "run", "check":
	default:
		return fmt.Errorf("unknown command %q", command)
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(errOut)
	path := flags.String("config", "", "TOML configuration path")
	account := flags.String("account", "", "paired phone number")
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	overrideAccount := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "account" {
			overrideAccount = true
		}
	})
	c, err := config.Load(*path, func(key string) (string, bool) {
		if key == "EXTRACTOR_ACCOUNT" && overrideAccount {
			return *account, true
		}
		if lookup != nil {
			return lookup(key)
		}
		return "", false
	})
	if err != nil {
		return err
	}
	if command == "check" {
		_, err = fmt.Fprintln(out, "configuration valid")
		return err
	}
	log := slog.New(slog.NewJSONHandler(errOut, nil))
	sessions, err := whatsapp.OpenSessions(ctx, c.Database)
	if err != nil {
		return errors.New("open session database failed; check path, permissions and database connectivity")
	}
	defer func() { result = errors.Join(result, sessions.Close()) }()
	switch command {
	case "pair":
		return sessions.Pair(ctx, out)
	case "accounts":
		accounts, err := sessions.Accounts(ctx)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(accounts)
	}
	client, err := sessions.Client(ctx, c.Account)
	if err != nil {
		return err
	}
	selected := client.Store.ID.User
	if command == "groups" {
		defer client.Disconnect()
		if err = whatsapp.Connect(ctx, client); err != nil {
			return err
		}
		groups, err := client.GetJoinedGroups(ctx)
		if err != nil {
			return err
		}
		type group struct {
			JID  string `json:"jid"`
			Name string `json:"name"`
			Size int    `json:"size"`
		}
		result := make([]group, 0, len(groups))
		for _, g := range groups {
			result = append(result, group{g.JID.String(), g.Name, len(g.Participants)})
		}
		return json.NewEncoder(out).Encode(result)
	}
	var writer application.Writer
	switch c.Store {
	case "memory":
		writer = storage.NewMemory()
	case "jsonl":
		path := strings.ReplaceAll(c.JSONLPath, "{account}", selected)
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		store, err := storage.OpenJSONL(path)
		if err != nil {
			return err
		}
		writer = store
		defer func() { result = errors.Join(result, store.Close()) }()
	case "sql":
		store, err := storage.OpenSQL(ctx, c.MessageAddress(selected), selected)
		if err != nil {
			return errors.New("open message database failed; check path, permissions and database connectivity")
		}
		writer = store
		defer func() { result = errors.Join(result, store.Close()) }()
	}
	var handlers []application.Handler
	switch c.View {
	case "json":
		enc := json.NewEncoder(out)
		handlers = append(handlers, func(_ context.Context, e domain.MessageExtracted) error { return enc.Encode(e.Message) })
	case "log":
		handlers = append(handlers, func(ctx context.Context, e domain.MessageExtracted) error {
			log.InfoContext(ctx, "message persisted", "kind", e.Message.Kind, "timestamp", e.Message.Timestamp)
			return nil
		})
	}
	source := whatsapp.NewSource(client, c.BufferSize, log)
	defer func() {
		received, skipped := source.Stats()
		log.Info("extraction stopped", "received", received, "skipped_protocol_events", skipped)
	}()
	return application.Extract(ctx, source, c.Filter(selected).Matches, writer, application.NewBus(log, handlers...))
}
