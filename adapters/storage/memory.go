// Package storage provides account-scoped message stores.
package storage

import (
	"context"
	"errors"
	"sync"

	"github.com/edududs/whatsapp-extractor-go/domain"
)

var ErrNotFound = errors.New("message not found")

type key struct{ chat, id string }
type Memory struct {
	mu       sync.RWMutex
	messages map[key]domain.Message
}

func NewMemory() *Memory { return &Memory{messages: make(map[key]domain.Message)} }
func (s *Memory) Save(ctx context.Context, m domain.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[key{m.Chat.JID.String(), m.ID}] = m.Clone()
	return nil
}
func (s *Memory) Load(ctx context.Context, chat, id string) (domain.Message, error) {
	if err := ctx.Err(); err != nil {
		return domain.Message{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.messages[key{chat, id}]
	if !ok {
		return domain.Message{}, ErrNotFound
	}
	return m.Clone(), nil
}
