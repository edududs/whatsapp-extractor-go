package database_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/edududs/whatsapp-extractor-go/internal/database"
)

func TestSQLitePathWithReservedCharacters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested directory", "messages #1.db")
	db, dialect, err := database.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if dialect != "sqlite3" {
		t.Fatalf("unexpected dialect %q", dialect)
	}
	if _, err = db.ExecContext(t.Context(), "CREATE TABLE example (id INTEGER)"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(path); err != nil {
		t.Fatalf("database was not created at requested path: %v", err)
	}
	var foreignKeys int
	if err = db.QueryRowContext(t.Context(), "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys=%d err=%v", foreignKeys, err)
	}
}
