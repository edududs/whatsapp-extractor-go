// Package storetest ships reusable behavioral tests for third-party store adapters.
package storetest

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/edududs/whatsapp-extractor-go/application"
	"github.com/edududs/whatsapp-extractor-go/domain"
)

func Message() domain.Message {
	return domain.Message{ID: "message-1", Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Kind: "text", Chat: domain.Chat{JID: domain.JID{User: "123", Server: "g.us"}, Kind: "group"}, Sender: domain.Sender{JID: domain.JID{User: "5511999999999", Server: "s.whatsapp.net"}, Phone: "5511999999999"}, Content: domain.Content{Text: "hello"}}
}

// Contract requires a fresh store, round trips, upserts, chat isolation and cancellation.
// Missing records must return a non-nil error; adapters may expose their own sentinel.
func Contract(t *testing.T, factory func(*testing.T) application.Store) {
	t.Helper()
	t.Run("roundtrip-upsert-and-chat-isolation", func(t *testing.T) {
		s := factory(t)
		ctx := t.Context()
		m := Message()
		if _, err := s.Load(ctx, m.Chat.JID.String(), m.ID); err == nil {
			t.Fatal("missing record returned no error")
		}
		m.Content.Media = &domain.Media{Kind: "image", Caption: "caption", Width: 128}
		m.Kind = "media"
		if err := s.Save(ctx, m); err != nil {
			t.Fatal(err)
		}
		got, err := s.Load(ctx, m.Chat.JID.String(), m.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, m) {
			t.Fatalf("roundtrip: %#v != %#v", got, m)
		}
		m.Content.Text = "edited"
		m.IsEdit = true
		if err = s.Save(ctx, m); err != nil {
			t.Fatal(err)
		}
		other := m.Clone()
		other.Chat.JID.User = "456"
		other.Content.Text = "other chat"
		if err = s.Save(ctx, other); err != nil {
			t.Fatal(err)
		}
		got, err = s.Load(ctx, m.Chat.JID.String(), m.ID)
		if err != nil || !reflect.DeepEqual(got, m) {
			t.Fatalf("upsert/isolation: %#v %v", got, err)
		}
		// Caller mutation must not modify stored data.
		got.Content.Media.Caption = "mutated"
		got, err = s.Load(ctx, m.Chat.JID.String(), m.ID)
		if err != nil || got.Content.Media.Caption != "caption" {
			t.Fatalf("aliasing: %v", err)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		s := factory(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := s.Save(ctx, Message()); err == nil {
			t.Fatal("save ignored cancellation")
		}
		if _, err := s.Load(ctx, "chat", "id"); err == nil {
			t.Fatal("load ignored cancellation")
		}
	})
	t.Run("invalid-message", func(t *testing.T) {
		if err := factory(t).Save(t.Context(), domain.Message{}); err == nil {
			t.Fatal("accepted invalid message")
		}
	})
}
