package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/edududs/whatsapp-extractor-go/internal/database"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
)

type Sessions struct{ container *sqlstore.Container }

func OpenSessions(ctx context.Context, address string) (*Sessions, error) {
	db, dialect, err := database.Open(ctx, address)
	if err != nil {
		return nil, err
	}
	container := sqlstore.NewWithDB(db, dialect, nil)
	if err = container.Upgrade(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Sessions{container: container}, nil
}
func (s *Sessions) Close() error { return s.container.Close() }
func (s *Sessions) Accounts(ctx context.Context) ([]string, error) {
	devices, err := s.container.GetAllDevices(ctx)
	if err != nil {
		return nil, err
	}
	accounts := make([]string, 0, len(devices))
	for _, d := range devices {
		if d.ID != nil {
			accounts = append(accounts, d.ID.User)
		}
	}
	return accounts, nil
}
func (s *Sessions) Client(ctx context.Context, account string) (*whatsmeow.Client, error) {
	devices, err := s.container.GetAllDevices(ctx)
	if err != nil {
		return nil, err
	}
	var selected *store.Device
	for _, d := range devices {
		if d.ID != nil && (account == "" || d.ID.User == account) {
			if selected != nil {
				return nil, errors.New("multiple matching sessions; use a separate session database per linked device")
			}
			selected = d
		}
	}
	if selected == nil {
		return nil, errors.New("no paired account found; run pair first")
	}
	return whatsmeow.NewClient(selected, nil), nil
}
func (s *Sessions) Pair(ctx context.Context, out io.Writer) error {
	client := whatsmeow.NewClient(s.container.NewDevice(), nil)
	qr, err := client.GetQRChannel(ctx)
	if err != nil {
		return err
	}
	defer client.Disconnect()
	if err = client.ConnectContext(ctx); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-qr:
			if !ok {
				return errors.New("pairing ended before success")
			}
			switch item.Event {
			case "code":
				code, err := qrcode.New(item.Code, qrcode.Medium)
				if err != nil {
					return err
				}
				if _, err = fmt.Fprintln(out, code.ToSmallString(false)); err != nil {
					return err
				}
			case "success":
				return nil
			default:
				if item.Error != nil {
					return item.Error
				}
				return fmt.Errorf("pairing: %s", item.Event)
			}
		}
	}
}

// Connect waits for authentication, not merely a websocket connection.
func Connect(ctx context.Context, client eventClient) error {
	timer := time.NewTimer(45 * time.Second)
	defer timer.Stop()
	ready := make(chan error, 1)
	id := client.AddEventHandler(func(event any) {
		var err error
		switch e := event.(type) {
		case *events.Connected:
		case events.PermanentDisconnect:
			err = fmt.Errorf("session ended: %s", e.PermanentDisconnectDescription())
		default:
			return
		}
		select {
		case ready <- err:
		default:
		}
	})
	defer client.RemoveEventHandler(id)
	if err := client.ConnectContext(ctx); err != nil {
		return err
	}
	select {
	case err := <-ready:
		return err
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errors.New("timed out waiting for WhatsApp authentication")
	}
}
