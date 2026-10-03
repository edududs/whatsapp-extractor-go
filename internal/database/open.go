// Package database centralizes driver configuration without leaking SQL into the core.
package database

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func IsPostgres(address string) bool {
	return strings.HasPrefix(address, "postgres://") || strings.HasPrefix(address, "postgresql://")
}
func Open(ctx context.Context, address string) (*sql.DB, string, error) {
	driver, dialect := "sqlite", "sqlite3"
	if IsPostgres(address) {
		driver, dialect = "pgx", "postgres"
	} else {
		if address == "" {
			return nil, "", errors.New("empty database path")
		}
		if address != ":memory:" {
			path, err := filepath.Abs(address)
			if err != nil {
				return nil, "", err
			}
			if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return nil, "", err
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
			if err != nil {
				return nil, "", err
			}
			if err = f.Close(); err != nil {
				return nil, "", err
			}
			address = (&url.URL{Scheme: "file", Path: path}).String()
		}
		address += "?_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	}
	db, err := sql.Open(driver, address)
	if err != nil {
		return nil, "", err
	}
	if driver == "sqlite" {
		db.SetMaxOpenConns(1)
	} else {
		db.SetMaxOpenConns(5)
	}
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, "", err
	}
	return db, dialect, nil
}
