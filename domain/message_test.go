package domain_test

import (
	"testing"

	"github.com/edududs/whatsapp-extractor-go/domain"
)

func TestWatchlist(t *testing.T) {
	m := domain.Message{Chat: domain.Chat{JID: domain.JID{User: "123", Server: "lid"}, CounterpartPhone: "5511111111111"}, Sender: domain.Sender{JID: domain.JID{User: "456", Server: "lid"}, LID: "456", Phone: "5522222222222"}}
	for _, tc := range []struct {
		name           string
		chats, senders []string
		want           bool
	}{
		{"empty", nil, nil, true}, {"chat JID", []string{"123@lid"}, nil, true}, {"counterpart", []string{"5511111111111"}, nil, true}, {"sender phone", nil, []string{"5522222222222"}, true}, {"sender LID", nil, []string{"456"}, true}, {"OR", []string{"no"}, []string{"456@lid"}, true}, {"reject", []string{"no"}, []string{"no"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.NewWatchlist(tc.chats, tc.senders).Matches(m); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}
func TestJIDKinds(t *testing.T) {
	for value, want := range map[string]string{"1@s.whatsapp.net": "direct", "1@lid": "direct", "1@g.us": "group", "status@broadcast": "status", "1@broadcast": "broadcast", "1@newsletter": "newsletter", "1@future": "unknown"} {
		j, err := domain.ParseJID(value)
		if err != nil || j.Kind() != want {
			t.Fatalf("%s: %v %s", value, err, j.Kind())
		}
	}
}
func FuzzParseJID(f *testing.F) {
	for _, s := range []string{"5511:4@s.whatsapp.net", "123@g.us", "status@broadcast", "", "@", "a@@b", "a\nb@c"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, value string) {
		j, err := domain.ParseJID(value)
		if err != nil {
			return
		}
		again, err := domain.ParseJID(j.String())
		if err != nil || again != j {
			t.Fatalf("unstable normalization: %q", value)
		}
	})
}
func BenchmarkWatchlist(b *testing.B) {
	w := domain.NewWatchlist([]string{"123@g.us"}, nil)
	m := domain.Message{Chat: domain.Chat{JID: domain.JID{User: "123", Server: "g.us"}}}
	b.ReportAllocs()
	for b.Loop() {
		w.Matches(m)
	}
}
