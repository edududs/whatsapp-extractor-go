package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"sync/atomic"

	"github.com/edududs/whatsapp-extractor-go/domain"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var ErrOverflow = errors.New("message buffer full; extraction stopped to make data loss visible")

type Source struct {
	client            eventClient
	bufferSize        int
	log               *slog.Logger
	contactName       func(context.Context, domain.Chat) (string, error)
	received, skipped atomic.Uint64
}

type eventClient interface {
	AddEventHandler(whatsmeow.EventHandler) uint32
	RemoveEventHandler(uint32) bool
	ConnectContext(context.Context) error
	Disconnect()
}

func NewSource(client *whatsmeow.Client, bufferSize int, log *slog.Logger) *Source {
	return &Source{client: client, bufferSize: bufferSize, log: log, contactName: func(ctx context.Context, chat domain.Chat) (string, error) {
		candidates := []types.JID{{User: chat.CounterpartPhone, Server: types.DefaultUserServer}, {User: chat.JID.User, Server: chat.JID.Server}}
		for _, j := range candidates {
			if j.User == "" {
				continue
			}
			contact, err := client.Store.Contacts.GetContact(ctx, j)
			if err != nil {
				return "", err
			}
			if name := first(contact.FullName, contact.BusinessName, contact.PushName); name != "" {
				return name, nil
			}
		}
		return "", nil
	}}
}

func (s *Source) Stats() (received, skipped uint64) { return s.received.Load(), s.skipped.Load() }

// Messages owns the connection for a single iterator. Overflow is fatal rather than silently dropping.
// Cancellation stops promptly; queued messages are not guaranteed to be drained.
func (s *Source) Messages(ctx context.Context) iter.Seq2[domain.Message, error] {
	return func(yield func(domain.Message, error) bool) {
		if s.bufferSize < 1 {
			yield(domain.Message{}, errors.New("buffer_size must be positive"))
			return
		}
		log := s.log
		if log == nil {
			log = slog.Default()
		}
		queue := make(chan domain.Message, s.bufferSize)
		failure := make(chan error, 1)
		fail := func(err error) {
			select {
			case failure <- err:
			default:
			}
		}
		id := s.client.AddEventHandler(func(event any) {
			switch e := event.(type) {
			case *events.Message:
				m, ok, err := Map(e)
				if err != nil {
					fail(fmt.Errorf("map message: %w", err))
					return
				}
				if !ok {
					s.skipped.Add(1)
					return
				}
				select {
				case <-ctx.Done():
					return
				default:
				}
				select {
				case queue <- m:
					s.received.Add(1)
				default:
					fail(ErrOverflow)
				}
			case events.PermanentDisconnect:
				fail(fmt.Errorf("session ended: %s", e.PermanentDisconnectDescription()))
			case *events.Disconnected:
				log.WarnContext(ctx, "WhatsApp disconnected; waiting for automatic reconnect")
			}
		})
		defer s.client.RemoveEventHandler(id)
		defer s.client.Disconnect()
		if err := Connect(ctx, s.client); err != nil {
			yield(domain.Message{}, err)
			return
		}
		log.InfoContext(ctx, "WhatsApp connected")
		for {
			select {
			case err := <-failure:
				yield(domain.Message{}, err)
				return
			default:
			}
			select {
			case <-ctx.Done():
				yield(domain.Message{}, ctx.Err())
				return
			case err := <-failure:
				yield(domain.Message{}, err)
				return
			case m := <-queue:
				if m.Chat.Kind == "direct" && s.contactName != nil {
					name, err := s.contactName(ctx, m.Chat)
					if err != nil {
						log.WarnContext(ctx, "contact lookup failed")
					} else {
						m.Chat.Name = name
					}
				}
				if !yield(m, nil) {
					return
				}
			}
		}
	}
}
