package storage_test

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/edududs/whatsapp-extractor-go/adapters/storage"
	"github.com/edududs/whatsapp-extractor-go/application"
	"github.com/edududs/whatsapp-extractor-go/domain"
	"github.com/edududs/whatsapp-extractor-go/storetest"
)

func TestMemoryContract(t *testing.T) {
	storetest.Contract(t, func(*testing.T) application.Store { return storage.NewMemory() })
}
func TestSQLiteContract(t *testing.T) {
	storetest.Contract(t, func(t *testing.T) application.Store {
		s, err := storage.OpenSQL(t.Context(), filepath.Join(t.TempDir(), "messages.db"), "5511999999999")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := s.Close(); err != nil {
				t.Error(err)
			}
		})
		return s
	})
}
func TestPostgresContract(t *testing.T) {
	address := os.Getenv("TEST_POSTGRES_DSN")
	if address == "" {
		t.Skip("TEST_POSTGRES_DSN not set; use a disposable database")
	}
	// Each contract gets a separate namespace; CI's database is disposable.
	storetest.Contract(t, func(t *testing.T) application.Store {
		account := fmt.Sprintf("9%014d", rand.Uint64()%100000000000000)
		s, err := storage.OpenSQL(t.Context(), address, account)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Close() })
		return s
	})
}
func TestSQLiteReopenAndAccountIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.db")
	m := storetest.Message()
	s, err := storage.OpenSQL(t.Context(), path, "5511111111111")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(t.Context(), m); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = storage.OpenSQL(t.Context(), path, "5511111111111")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Load(t.Context(), m.Chat.JID.String(), m.ID); err != nil {
		t.Fatal(err)
	}
	other, err := storage.OpenSQL(t.Context(), path, "5522222222222")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if _, err = other.Load(t.Context(), m.Chat.JID.String(), m.ID); err == nil {
		t.Fatal("cross-account leak")
	}
}
func TestJSONLConcurrentWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "messages.jsonl")
	s, err := storage.OpenJSONL(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if err := s.Save(t.Context(), storetest.Message()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for range 20 {
		var m domain.Message
		if err = dec.Decode(&m); err != nil {
			t.Fatal(err)
		}
		if m.ID != storetest.Message().ID {
			t.Fatal("corrupted output")
		}
	}
}
func TestAccountValidation(t *testing.T) {
	for _, s := range []string{"", "../escape", "1;DROP SCHEMA public", "1234", "1234567890123456"} {
		if storage.ValidateAccount(s) == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
