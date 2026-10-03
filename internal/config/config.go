// Package config loads declarative TOML with explicit environment overrides.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/edududs/whatsapp-extractor-go/adapters/storage"
	"github.com/edududs/whatsapp-extractor-go/domain"
	"github.com/pelletier/go-toml/v2"
)

type Watchlist struct {
	Chats   []string `toml:"chats"`
	Senders []string `toml:"senders"`
}
type Account struct {
	Watchlist *Watchlist `toml:"watchlist"`
}
type Settings struct {
	Account          string             `toml:"account"`
	Database         string             `toml:"database"`
	MessagesDatabase string             `toml:"messages_database"`
	Store            string             `toml:"store"`
	View             string             `toml:"view"`
	BufferSize       int                `toml:"buffer_size"`
	JSONLPath        string             `toml:"jsonl_path"`
	Watchlist        Watchlist          `toml:"watchlist"`
	Accounts         map[string]Account `toml:"accounts"`
}

func Load(path string, lookup func(string) (string, bool)) (Settings, error) {
	c := Settings{Database: "data/session.db", Store: "sql", View: "log", BufferSize: 10000, JSONLPath: "data/messages_{account}.jsonl"}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return c, err
		}
		if err = toml.NewDecoder(strings.NewReader(string(b))).DisallowUnknownFields().Decode(&c); err != nil {
			return c, fmt.Errorf("config: %w", err)
		}
	}
	if lookup != nil {
		for name, dest := range map[string]*string{"ACCOUNT": &c.Account, "DATABASE": &c.Database, "MESSAGES_DATABASE": &c.MessagesDatabase, "STORE": &c.Store, "VIEW": &c.View, "JSONL_PATH": &c.JSONLPath} {
			if v, ok := lookup("EXTRACTOR_" + name); ok {
				*dest = v
			}
		}
		if v, ok := lookup("EXTRACTOR_BUFFER_SIZE"); ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				return c, errors.New("EXTRACTOR_BUFFER_SIZE must be an integer")
			}
			c.BufferSize = n
		}
	}
	return c, c.Validate()
}
func (c Settings) Validate() error {
	if c.Account != "" {
		if err := storage.ValidateAccount(c.Account); err != nil {
			return err
		}
	}
	if c.Database == "" {
		return errors.New("database must not be empty")
	}
	if c.BufferSize < 1 || c.BufferSize > 1000000 {
		return errors.New("buffer_size must be between 1 and 1000000")
	}
	switch c.Store {
	case "sql", "jsonl", "memory":
	default:
		return errors.New("store must be sql, jsonl or memory")
	}
	switch c.View {
	case "json", "log", "none":
	default:
		return errors.New("view must be json, log or none")
	}
	if c.Store == "jsonl" && !strings.Contains(c.JSONLPath, "{account}") {
		return errors.New("jsonl_path must include {account} for account isolation")
	}
	for account := range c.Accounts {
		if err := storage.ValidateAccount(account); err != nil {
			return err
		}
	}
	return nil
}
func (c Settings) Filter(account string) domain.Watchlist {
	w := c.Watchlist
	if a, ok := c.Accounts[account]; ok && a.Watchlist != nil {
		w = *a.Watchlist
	}
	return domain.NewWatchlist(w.Chats, w.Senders)
}
func (c Settings) MessageAddress(account string) string {
	if c.MessagesDatabase != "" {
		return c.MessagesDatabase
	}
	if strings.HasPrefix(c.Database, "postgres://") || strings.HasPrefix(c.Database, "postgresql://") {
		return c.Database
	}
	return filepath.Join(filepath.Dir(c.Database), "wa_"+account+".db")
}
