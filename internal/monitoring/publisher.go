package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

// StatusPublisher publishes cost instance status events.
type StatusPublisher interface {
	Publish(ctx context.Context, instanceID, status, message string) error
	Close() error
}

// NATSPublisher implements StatusPublisher using NATS.
type NATSPublisher struct {
	conn         *nats.Conn
	providerName string
	subject      string
}

func NewNATSPublisher(natsURL, providerName string, logger *slog.Logger) (*NATSPublisher, error) {
	conn, err := nats.Connect(natsURL,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			logger.Error("NATS disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("connecting to NATS at %s: %w", natsURL, err)
	}
	return &NATSPublisher{
		conn:         conn,
		providerName: providerName,
		subject:      "dcm.cost",
	}, nil
}

func (p *NATSPublisher) Publish(_ context.Context, instanceID, status, message string) error {
	ce := NewStatusCloudEvent(p.providerName, instanceID, status, message)
	data, err := json.Marshal(ce)
	if err != nil {
		return fmt.Errorf("marshaling cloud event: %w", err)
	}
	return p.conn.Publish(p.subject, data)
}

func (p *NATSPublisher) Close() error {
	p.conn.Close()
	return nil
}

// NoopPublisher is a StatusPublisher that does nothing (for testing).
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, string, string, string) error { return nil }
func (NoopPublisher) Close() error                                          { return nil }
