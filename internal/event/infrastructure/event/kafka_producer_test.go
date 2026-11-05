package event_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/refreshTechRusuh/event-service/internal/event/infrastructure/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discardLog() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{Level: slog.LevelError + 10}))
}

type mockPublisher struct {
	calls []struct{ topic, key string; payload any }
	err   error
}

func (m *mockPublisher) Publish(_ context.Context, topic, key string, payload any) error {
	m.calls = append(m.calls, struct{ topic, key string; payload any }{topic, key, payload})
	return m.err
}

func TestKafkaProducer_PublishEventSoldOut(t *testing.T) {
	mp := &mockPublisher{}
	p := event.NewKafkaProducerWithPublisher(mp, discardLog())

	err := p.PublishEventSoldOut(context.Background(), "event-1", map[string]string{"eventId": "event-1"})
	require.NoError(t, err)
	require.Len(t, mp.calls, 1)
	assert.Equal(t, "event.sold_out", mp.calls[0].topic)
	assert.Equal(t, "event-1", mp.calls[0].key)
}

func TestKafkaProducer_PublishError_propagated(t *testing.T) {
	mp := &mockPublisher{err: errors.New("kafka unavailable")}
	p := event.NewKafkaProducerWithPublisher(mp, discardLog())

	err := p.PublishEventSoldOut(context.Background(), "event-1", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kafka unavailable")
}
