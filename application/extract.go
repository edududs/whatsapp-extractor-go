// Package application owns the extraction use case and the interfaces it consumes.
package application

import (
	"context"
	"fmt"
	"iter"
	"log/slog"

	"github.com/edududs/whatsapp-extractor-go/domain"
)

type Source interface {
	Messages(context.Context) iter.Seq2[domain.Message, error]
}
type Writer interface {
	Save(context.Context, domain.Message) error
}
type Store interface {
	Writer
	Load(context.Context, string, string) (domain.Message, error)
}
type Publisher interface {
	Publish(context.Context, domain.MessageExtracted)
}
type Handler func(context.Context, domain.MessageExtracted) error

// Extract preserves source order. A failed write terminates ingestion before publishing.
func Extract(ctx context.Context, source Source, accepts func(domain.Message) bool, writer Writer, publisher Publisher) error {
	for message, err := range source.Messages(ctx) {
		if err != nil {
			return fmt.Errorf("read source: %w", err)
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = message.Validate(); err != nil {
			return fmt.Errorf("validate message: %w", err)
		}
		if accepts != nil && !accepts(message) {
			continue
		}
		if err = writer.Save(ctx, message.Clone()); err != nil {
			return fmt.Errorf("save message: %w", err)
		}
		if publisher != nil {
			publisher.Publish(ctx, domain.MessageExtracted{Message: message.Clone()})
		}
	}
	return ctx.Err()
}

// Bus is configured once, before extraction. Handlers run sequentially and are best effort.
// Panics are deliberately not recovered: programming errors should fail visibly.
type Bus struct {
	log      *slog.Logger
	handlers []Handler
}

func NewBus(log *slog.Logger, handlers ...Handler) *Bus {
	if log == nil {
		log = slog.Default()
	}
	return &Bus{log: log, handlers: append([]Handler(nil), handlers...)}
}
func (b *Bus) Publish(ctx context.Context, event domain.MessageExtracted) {
	for i, handler := range b.handlers {
		copyEvent := domain.MessageExtracted{Message: event.Message.Clone()}
		if err := handler(ctx, copyEvent); err != nil {
			b.log.ErrorContext(ctx, "message handler failed", "handler", i, "error", err)
		}
	}
}
