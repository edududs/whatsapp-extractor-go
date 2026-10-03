package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/edududs/whatsapp-extractor-go/internal/config"
	"github.com/edududs/whatsapp-extractor-go/storetest"
)

func TestPrecedenceAndAccountWatchlist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := `account = "5511111111111"
buffer_size = 12
[watchlist]
chats = ["no-match"]
[accounts."5522222222222".watchlist]
chats = []
`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(path, func(k string) (string, bool) {
		v, ok := map[string]string{"EXTRACTOR_ACCOUNT": "5522222222222", "EXTRACTOR_BUFFER_SIZE": "42"}[k]
		return v, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Account != "5522222222222" || c.BufferSize != 42 {
		t.Fatalf("precedence: %#v", c)
	}
	if !c.Filter(c.Account).Matches(storetest.Message()) || c.Filter("5511111111111").Matches(storetest.Message()) {
		t.Fatal("account watchlist did not replace global")
	}
}
func TestInvalidSettings(t *testing.T) {
	for key, value := range map[string]string{"EXTRACTOR_BUFFER_SIZE": "0", "EXTRACTOR_STORE": "typo", "EXTRACTOR_VIEW": "panel", "EXTRACTOR_ACCOUNT": "../../x", "EXTRACTOR_DATABASE": ""} {
		t.Run(key, func(t *testing.T) {
			_, err := config.Load("", func(k string) (string, bool) { return value, k == key })
			if err == nil {
				t.Fatal("invalid override accepted")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("bufffer_size = 5"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(path, nil); err == nil {
		t.Fatal("unknown config key accepted")
	}
}
