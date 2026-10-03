package application_test

import (
	"context"
	"errors"
	"io"
	"iter"
	"log/slog"
	"reflect"
	"testing"

	"github.com/edududs/whatsapp-extractor-go/application"
	"github.com/edududs/whatsapp-extractor-go/domain"
	"github.com/edududs/whatsapp-extractor-go/storetest"
)

type source struct {
	messages []domain.Message
	err      error
	closed   *bool
}

func (s source) Messages(context.Context) iter.Seq2[domain.Message, error] {
	return func(yield func(domain.Message, error) bool) {
		if s.closed != nil {
			defer func() { *s.closed = true }()
		}
		for _, m := range s.messages {
			if !yield(m, nil) {
				return
			}
		}
		if s.err != nil {
			yield(domain.Message{}, s.err)
		}
	}
}

type writer func(context.Context, domain.Message) error

func (w writer) Save(ctx context.Context, m domain.Message) error { return w(ctx, m) }

func TestExtractOrderingAndFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "failed-write"}[fail], func(t *testing.T) {
			var order []string
			closed := false
			boom := errors.New("storage failure")
			w := writer(func(context.Context, domain.Message) error {
				order = append(order, "save")
				if fail {
					return boom
				}
				return nil
			})
			bus := application.NewBus(slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context, domain.MessageExtracted) error {
				order = append(order, "handler1")
				return errors.New("handler failure")
			}, func(context.Context, domain.MessageExtracted) error { order = append(order, "handler2"); return nil })
			err := application.Extract(t.Context(), source{messages: []domain.Message{storetest.Message()}, closed: &closed}, nil, w, bus)
			want := []string{"save", "handler1", "handler2"}
			if fail {
				want = []string{"save"}
				if !errors.Is(err, boom) {
					t.Fatalf("lost error: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(order, want) || !closed {
				t.Fatalf("order=%v closed=%v", order, closed)
			}
		})
	}
}
func TestFilteringAndSourceError(t *testing.T) {
	boom := errors.New("source failed")
	calls := 0
	err := application.Extract(t.Context(), source{messages: []domain.Message{storetest.Message()}, err: boom}, func(domain.Message) bool { return false }, writer(func(context.Context, domain.Message) error { calls++; return nil }), nil)
	if calls != 0 || !errors.Is(err, boom) {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
func TestHandlerIsolation(t *testing.T) {
	m := storetest.Message()
	m.Content.Media = &domain.Media{Caption: "original"}
	bus := application.NewBus(nil, func(_ context.Context, e domain.MessageExtracted) error {
		e.Message.Content.Media.Caption = "mutated"
		return nil
	}, func(_ context.Context, e domain.MessageExtracted) error {
		if e.Message.Content.Media.Caption != "original" {
			t.Fatal("handler mutation leaked")
		}
		return nil
	})
	bus.Publish(t.Context(), domain.MessageExtracted{Message: m})
}
