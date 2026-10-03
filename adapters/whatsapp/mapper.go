// Package whatsapp confines whatsmeow and protobuf details to an adapter boundary.
package whatsapp

import (
	"github.com/edududs/whatsapp-extractor-go/domain"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func jid(j types.JID) domain.JID { return domain.JID{User: j.User, Server: j.Server} }
func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// Map returns accepted=false for empty/key-exchange-only events. Unknown content survives.
func Map(event *events.Message) (message domain.Message, accepted bool, err error) {
	if event == nil || event.Message == nil {
		return message, false, nil
	}
	body := event.Message
	body.ProtoReflect().Range(func(field protoreflect.FieldDescriptor, _ protoreflect.Value) bool {
		if field.Name() != "senderKeyDistributionMessage" && field.Name() != "messageContextInfo" {
			accepted = true
		}
		return true
	})
	if !accepted && len(body.ProtoReflect().GetUnknown()) == 0 {
		return message, false, nil
	}
	info := event.Info
	chat, sender, alt, recipient := jid(info.Chat), jid(info.Sender), jid(info.SenderAlt), jid(info.RecipientAlt)
	message = domain.Message{ID: info.ID, Timestamp: info.Timestamp.UTC(), Kind: "unknown", FromMe: info.IsFromMe,
		Chat: domain.Chat{JID: chat, Kind: chat.Kind()}, Sender: domain.Sender{JID: sender, Phone: first(sender.Phone(), alt.Phone()), LID: first(sender.LID(), alt.LID()), PushName: info.PushName},
		IsEphemeral: event.IsEphemeral, IsViewOnce: event.IsViewOnce || event.IsViewOnceV2 || event.IsViewOnceV2Extension, IsEdit: event.IsEdit}
	if !info.RecipientAlt.IsEmpty() {
		message.Recipient = &recipient
	}
	if message.Chat.Kind == "direct" {
		message.Chat.CounterpartPhone = first(alt.Phone(), sender.Phone(), chat.Phone())
		if info.IsFromMe {
			message.Chat.CounterpartPhone = first(recipient.Phone(), chat.Phone())
		}
	}
	switch {
	case body.Conversation != nil:
		message.Kind = "text"
		message.Content.Text = body.GetConversation()
	case body.ExtendedTextMessage != nil:
		message.Kind = "text"
		message.Content.Text = body.GetExtendedTextMessage().GetText()
	case body.ImageMessage != nil:
		m := body.GetImageMessage()
		message.Content.Media = &domain.Media{Kind: "image", MIMEType: m.GetMimetype(), Caption: m.GetCaption(), Width: m.GetWidth(), Height: m.GetHeight(), FileLength: m.GetFileLength()}
	case body.VideoMessage != nil:
		m := body.GetVideoMessage()
		message.Content.Media = &domain.Media{Kind: "video", MIMEType: m.GetMimetype(), Caption: m.GetCaption(), Width: m.GetWidth(), Height: m.GetHeight(), FileLength: m.GetFileLength(), DurationSeconds: m.GetSeconds()}
	case body.AudioMessage != nil:
		m := body.GetAudioMessage()
		message.Content.Media = &domain.Media{Kind: "audio", MIMEType: m.GetMimetype(), FileLength: m.GetFileLength(), DurationSeconds: m.GetSeconds(), IsVoiceNote: m.GetPTT()}
	case body.DocumentMessage != nil:
		m := body.GetDocumentMessage()
		message.Content.Media = &domain.Media{Kind: "document", MIMEType: m.GetMimetype(), Caption: m.GetCaption(), FileLength: m.GetFileLength(), FileName: first(m.GetFileName(), m.GetTitle())}
	case body.StickerMessage != nil:
		m := body.GetStickerMessage()
		message.Content.Media = &domain.Media{Kind: "sticker", MIMEType: m.GetMimetype(), Width: m.GetWidth(), Height: m.GetHeight(), FileLength: m.GetFileLength()}
	case body.ReactionMessage != nil:
		message.Kind = "reaction"
		message.Content.Text = body.GetReactionMessage().GetText()
	case body.PollCreationMessage != nil:
		message.Kind = "poll"
		message.Content.Text = body.GetPollCreationMessage().GetName()
	case body.PollCreationMessageV2 != nil:
		message.Kind = "poll"
		message.Content.Text = body.GetPollCreationMessageV2().GetName()
	case body.PollCreationMessageV3 != nil:
		message.Kind = "poll"
		message.Content.Text = body.GetPollCreationMessageV3().GetName()
	}
	if message.Content.Media != nil {
		message.Kind = "media"
		message.Content.Text = message.Content.Media.Caption
	}
	return message, true, message.Validate()
}
