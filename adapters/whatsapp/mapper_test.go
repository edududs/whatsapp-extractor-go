package whatsapp_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/edududs/whatsapp-extractor-go/adapters/whatsapp"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func event(body *waE2E.Message) *events.Message {
	return &events.Message{Info: types.MessageInfo{ID: "m1", Timestamp: time.Now(), MessageSource: types.MessageSource{Chat: types.NewJID("123", types.HiddenUserServer), Sender: types.NewJID("123", types.HiddenUserServer), SenderAlt: types.NewJID("5511999999999", types.DefaultUserServer)}}, Message: body}
}
func TestMapContent(t *testing.T) {
	for _, tc := range []struct {
		name              string
		body              *waE2E.Message
		kind, text, media string
	}{
		{"plain", &waE2E.Message{Conversation: proto.String("hello")}, "text", "hello", ""},
		{"extended", &waE2E.Message{ExtendedTextMessage: &waE2E.ExtendedTextMessage{Text: proto.String("long")}}, "text", "long", ""},
		{"image", &waE2E.Message{ImageMessage: &waE2E.ImageMessage{Caption: proto.String("caption"), Width: proto.Uint32(640)}}, "media", "caption", "image"},
		{"video", &waE2E.Message{VideoMessage: &waE2E.VideoMessage{Caption: proto.String("video")}}, "media", "video", "video"},
		{"audio", &waE2E.Message{AudioMessage: &waE2E.AudioMessage{PTT: proto.Bool(true)}}, "media", "", "audio"},
		{"document", &waE2E.Message{DocumentMessage: &waE2E.DocumentMessage{Caption: proto.String("doc")}}, "media", "doc", "document"},
		{"sticker", &waE2E.Message{StickerMessage: &waE2E.StickerMessage{}}, "media", "", "sticker"},
		{"reaction", &waE2E.Message{ReactionMessage: &waE2E.ReactionMessage{Text: proto.String("👍")}}, "reaction", "👍", ""},
		{"poll", &waE2E.Message{PollCreationMessage: &waE2E.PollCreationMessage{Name: proto.String("vote")}}, "poll", "vote", ""},
		{"unknown", &waE2E.Message{ContactMessage: &waE2E.ContactMessage{}}, "unknown", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, ok, err := whatsapp.Map(event(tc.body))
			if err != nil || !ok {
				t.Fatalf("ok=%v err=%v", ok, err)
			}
			if m.Kind != tc.kind || m.Content.Text != tc.text {
				t.Fatalf("mapped %#v", m)
			}
			if tc.media != "" && (m.Content.Media == nil || m.Content.Media.Kind != tc.media) {
				t.Fatal("wrong media")
			}
			if m.Sender.Phone != "5511999999999" || m.Chat.CounterpartPhone != m.Sender.Phone {
				t.Fatal("phone alias missing")
			}
		})
	}
}
func TestMapSkipAndOutgoing(t *testing.T) {
	for _, e := range []*events.Message{nil, {}, event(&waE2E.Message{}), event(&waE2E.Message{SenderKeyDistributionMessage: &waE2E.SenderKeyDistributionMessage{}, MessageContextInfo: &waE2E.MessageContextInfo{}})} {
		if _, ok, err := whatsapp.Map(e); ok || err != nil {
			t.Fatalf("protocol event accepted: %v", err)
		}
	}
	e := event(&waE2E.Message{Conversation: proto.String("out")})
	e.Info.IsFromMe = true
	e.Info.RecipientAlt = types.NewJID("5522222222222", types.DefaultUserServer)
	e.IsEdit = true
	e.IsViewOnceV2 = true
	e.IsEphemeral = true
	m, _, err := whatsapp.Map(e)
	if err != nil {
		t.Fatal(err)
	}
	if m.Chat.CounterpartPhone != "5522222222222" || !m.IsEdit || !m.IsViewOnce || !m.IsEphemeral {
		t.Fatalf("lost outgoing data: %#v", m)
	}
	e.Info.ID = ""
	if _, _, err = whatsapp.Map(e); err == nil {
		t.Fatal("invalid message accepted")
	}
}
func TestSessionDatabaseLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.db")
	for range 2 {
		s, err := whatsapp.OpenSessions(t.Context(), path)
		if err != nil {
			t.Fatal(err)
		}
		accounts, err := s.Accounts(t.Context())
		if err != nil || len(accounts) != 0 {
			t.Fatalf("accounts=%v err=%v", accounts, err)
		}
		if _, err = s.Client(t.Context(), ""); err == nil {
			t.Fatal("unpaired session accepted")
		}
		if err = s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
func FuzzMap(f *testing.F) {
	for _, b := range [][]byte{{}, {10, 1, 65}, {255, 255}} {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var body waE2E.Message
		if proto.Unmarshal(data, &body) != nil {
			return
		}
		m, ok, err := whatsapp.Map(event(&body))
		if ok && err == nil {
			if err = m.Validate(); err != nil {
				t.Fatal(err)
			}
		}
	})
}
