// Package domain defines protocol-independent WhatsApp messages and selection rules.
package domain

import (
	"errors"
	"strings"
	"time"
)

// JID is a normalized address without a linked-device suffix.
type JID struct {
	User   string `json:"user"`
	Server string `json:"server"`
}

func ParseJID(value string) (JID, error) {
	user, server, ok := strings.Cut(value, "@")
	if !ok || user == "" || server == "" || strings.ContainsAny(value, " \t\r\n") || strings.Contains(server, "@") {
		return JID{}, errors.New("JID must be user@server")
	}
	user, _, _ = strings.Cut(user, ":")
	if user == "" {
		return JID{}, errors.New("empty JID user")
	}
	return JID{User: user, Server: server}, nil
}
func (j JID) String() string { return j.User + "@" + j.Server }
func (j JID) Phone() string {
	if j.Server == "s.whatsapp.net" {
		return j.User
	}
	return ""
}
func (j JID) LID() string {
	if j.Server == "lid" {
		return j.User
	}
	return ""
}
func (j JID) Kind() string {
	switch j.Server {
	case "s.whatsapp.net", "lid":
		return "direct"
	case "g.us":
		return "group"
	case "newsletter":
		return "newsletter"
	case "broadcast":
		if j.User == "status" {
			return "status"
		}
		return "broadcast"
	default:
		return "unknown"
	}
}

type Chat struct {
	JID              JID    `json:"jid"`
	Kind             string `json:"kind"`
	CounterpartPhone string `json:"counterpart_phone,omitempty"`
	Name             string `json:"name,omitempty"`
}
type Sender struct {
	JID      JID    `json:"jid"`
	LID      string `json:"lid,omitempty"`
	Phone    string `json:"phone,omitempty"`
	PushName string `json:"pushname"`
}
type Media struct {
	Kind            string `json:"kind"`
	MIMEType        string `json:"mimetype"`
	Caption         string `json:"caption,omitempty"`
	DurationSeconds uint32 `json:"duration_seconds,omitempty"`
	Width           uint32 `json:"width,omitempty"`
	Height          uint32 `json:"height,omitempty"`
	FileLength      uint64 `json:"file_length,omitempty"`
	FileName        string `json:"file_name,omitempty"`
	IsVoiceNote     bool   `json:"is_voice_note,omitempty"`
}
type Content struct {
	Text  string `json:"text"`
	Media *Media `json:"media,omitempty"`
}
type Message struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Kind        string    `json:"kind"`
	FromMe      bool      `json:"from_me"`
	Chat        Chat      `json:"chat"`
	Sender      Sender    `json:"sender"`
	Recipient   *JID      `json:"recipient,omitempty"`
	Content     Content   `json:"content"`
	IsEphemeral bool      `json:"is_ephemeral"`
	IsViewOnce  bool      `json:"is_view_once"`
	IsEdit      bool      `json:"is_edit"`
}

func (m Message) Validate() error {
	if strings.TrimSpace(m.ID) == "" || m.Timestamp.IsZero() {
		return errors.New("message requires id and timestamp")
	}
	if _, err := ParseJID(m.Chat.JID.String()); err != nil {
		return err
	}
	if _, err := ParseJID(m.Sender.JID.String()); err != nil {
		return err
	}
	switch m.Kind {
	case "text", "media", "reaction", "poll", "unknown":
	default:
		return errors.New("invalid message kind")
	}
	return nil
}

// Clone prevents mutable pointer fields from being shared with stores and handlers.
func (m Message) Clone() Message {
	if m.Content.Media != nil {
		v := *m.Content.Media
		m.Content.Media = &v
	}
	if m.Recipient != nil {
		v := *m.Recipient
		m.Recipient = &v
	}
	return m
}

type MessageExtracted struct {
	Message Message `json:"message"`
}

// Watchlist matches a chat OR a sender. An empty list accepts all messages.
type Watchlist struct{ chats, senders map[string]struct{} }

func NewWatchlist(chats, senders []string) Watchlist {
	w := Watchlist{chats: make(map[string]struct{}, len(chats)), senders: make(map[string]struct{}, len(senders))}
	for _, v := range chats {
		if v != "" {
			w.chats[v] = struct{}{}
		}
	}
	for _, v := range senders {
		if v != "" {
			w.senders[v] = struct{}{}
		}
	}
	return w
}
func (w Watchlist) Matches(m Message) bool {
	if len(w.chats)+len(w.senders) == 0 {
		return true
	}
	for _, v := range []string{m.Chat.JID.String(), m.Chat.CounterpartPhone} {
		if _, ok := w.chats[v]; ok {
			return true
		}
	}
	for _, v := range []string{m.Sender.JID.String(), m.Sender.Phone, m.Sender.LID} {
		if _, ok := w.senders[v]; ok {
			return true
		}
	}
	return false
}
