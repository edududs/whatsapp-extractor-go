package whatsapp

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

type fakeClient struct {
	handlers     map[uint32]whatsmeow.EventHandler
	next         uint32
	events       []any
	disconnected bool
	connectErr   error
	ctx          context.Context
}

func (f *fakeClient) AddEventHandler(h whatsmeow.EventHandler) uint32 {
	if f.handlers == nil {
		f.handlers = map[uint32]whatsmeow.EventHandler{}
	}
	f.next++
	f.handlers[f.next] = h
	return f.next
}
func (f *fakeClient) RemoveEventHandler(id uint32) bool { delete(f.handlers, id); return true }
func (f *fakeClient) ConnectContext(ctx context.Context) error {
	f.ctx = ctx
	if f.connectErr != nil {
		return f.connectErr
	}
	for _, h := range f.handlers {
		h(&events.Connected{})
	}
	for _, e := range f.events {
		for _, h := range f.handlers {
			h(e)
		}
	}
	return nil
}
func (f *fakeClient) Disconnect() { f.disconnected = true }
func messageEvent() *events.Message {
	return &events.Message{Info: types.MessageInfo{ID: "test", Timestamp: time.Now(), MessageSource: types.MessageSource{Chat: types.NewJID("123", types.GroupServer), Sender: types.NewJID("5511111111111", types.DefaultUserServer)}}, Message: &waE2E.Message{Conversation: proto.String("hello")}}
}
func TestSourceLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name   string
		events []any
		buffer int
		want   error
	}{
		{"consumer-break", []any{messageEvent()}, 2, nil},
		{"overflow", []any{messageEvent(), messageEvent()}, 1, ErrOverflow},
		{"permanent-disconnect", []any{&events.LoggedOut{}}, 1, errors.New("disconnect")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeClient{events: tc.events}
			s := &Source{client: f, bufferSize: tc.buffer, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
			count := 0
			for m, err := range s.Messages(t.Context()) {
				count++
				if tc.want != nil {
					if err == nil {
						t.Fatal("missing failure")
					}
					if tc.want == ErrOverflow && !errors.Is(err, ErrOverflow) {
						t.Fatal(err)
					}
				} else if err != nil || m.ID != "test" {
					t.Fatalf("%v %v", m, err)
				}
				break
			}
			if count != 1 || !f.disconnected || len(f.handlers) != 0 {
				t.Fatalf("resource leak: %#v", f)
			}
		})
	}
}
func TestConnectKeepsParentContextAlive(t *testing.T) {
	f := &fakeClient{}
	if err := Connect(t.Context(), f); err != nil {
		t.Fatal(err)
	}
	if f.ctx.Err() != nil {
		t.Fatal("authentication cancelled the live connection")
	}
}
func TestSourceCancellationAndConnectionError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, tc := range []struct {
		ctx context.Context
		err error
	}{{ctx, nil}, {t.Context(), errors.New("dial failed")}} {
		f := &fakeClient{connectErr: tc.err}
		s := &Source{client: f, bufferSize: 1, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
		for _, err := range s.Messages(tc.ctx) {
			if err == nil {
				t.Fatal("missing failure")
			}
		}
		if !f.disconnected || len(f.handlers) != 0 {
			t.Fatal("resources not cleaned")
		}
	}
}
