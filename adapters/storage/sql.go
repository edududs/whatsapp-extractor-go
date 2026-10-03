package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/edududs/whatsapp-extractor-go/domain"
	"github.com/edududs/whatsapp-extractor-go/internal/database"
)

var accountPattern = regexp.MustCompile(`^[0-9]{5,15}$`)

func ValidateAccount(account string) error {
	if !accountPattern.MatchString(account) {
		return errors.New("account must contain 5 to 15 digits")
	}
	return nil
}

// SQL stores full JSON with indexed projections, namespaced by account and chat.
type SQL struct {
	db    *sql.DB
	table string
}

func OpenSQL(ctx context.Context, address, account string) (*SQL, error) {
	if err := ValidateAccount(account); err != nil {
		return nil, err
	}
	db, dialect, err := database.Open(ctx, address)
	if err != nil {
		return nil, err
	}
	prefix := ""
	if dialect == "postgres" {
		prefix = "wa_" + account + "."
	}
	// SQLite uses a separate table namespace too, so explicit shared paths remain isolated.
	if dialect == "sqlite3" {
		prefix = "wa_" + account + "_"
	}
	s := &SQL{db: db, table: prefix + "messages"}
	if err = s.migrate(ctx, dialect, prefix); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}
func (s *SQL) migrate(ctx context.Context, dialect, prefix string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if dialect == "postgres" {
		// Serialize concurrent migrations of this account on different processes.
		if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", prefix); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "CREATE SCHEMA IF NOT EXISTS "+prefix[:len(prefix)-1]); err != nil {
			return err
		}
	}
	versionTable := prefix + "schema_migrations"
	if _, err = tx.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS "+versionTable+" (version INTEGER PRIMARY KEY)"); err != nil {
		return err
	}
	var version int
	if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM "+versionTable).Scan(&version); err != nil {
		return err
	}
	if version > 1 {
		return fmt.Errorf("database schema %d is newer than supported version 1", version)
	}
	if version == 0 {
		statements := []string{
			"CREATE TABLE " + s.table + " (chat_jid TEXT NOT NULL, id TEXT NOT NULL, timestamp BIGINT NOT NULL, payload TEXT NOT NULL, PRIMARY KEY(chat_jid,id))",
			"CREATE INDEX wa_messages_chat_time ON " + s.table + " (chat_jid,timestamp)",
			"INSERT INTO " + versionTable + " (version) VALUES (1)",
		}
		if dialect == "sqlite3" {
			statements[1] = "CREATE INDEX " + prefix + "chat_time ON " + s.table + " (chat_jid,timestamp)"
		}
		for _, stmt := range statements {
			if _, err = tx.ExecContext(ctx, stmt); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
func (s *SQL) Save(ctx context.Context, m domain.Message) error {
	if err := m.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "INSERT INTO "+s.table+" (chat_jid,id,timestamp,payload) VALUES ($1,$2,$3,$4) ON CONFLICT(chat_jid,id) DO UPDATE SET timestamp=excluded.timestamp,payload=excluded.payload", m.Chat.JID.String(), m.ID, m.Timestamp.UnixMilli(), string(b))
	return err
}
func (s *SQL) Load(ctx context.Context, chat, id string) (domain.Message, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM "+s.table+" WHERE chat_jid=$1 AND id=$2", chat, id).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Message{}, ErrNotFound
	}
	if err != nil {
		return domain.Message{}, err
	}
	var m domain.Message
	if err = json.Unmarshal([]byte(payload), &m); err != nil {
		return m, err
	}
	return m, m.Validate()
}
func (s *SQL) Close() error { return s.db.Close() }
