package storage

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/edududs/whatsapp-extractor-go/domain"
)

// JSONL is an append-only writer, including redeliveries and edits. It does not implement Load.
// Each successful Save is flushed with fsync before publication.
type JSONL struct {
	mu   sync.Mutex
	file *os.File
}

func OpenJSONL(path string) (*JSONL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	return &JSONL{file: f}, nil
}
func (s *JSONL) Save(ctx context.Context, m domain.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err = s.file.Write(append(b, '\n')); err != nil {
		return err
	}
	return s.file.Sync()
}
func (s *JSONL) Close() error { s.mu.Lock(); defer s.mu.Unlock(); return s.file.Close() }
