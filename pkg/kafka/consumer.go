package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/refreshTechRusuh/event-service/internal/config"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
)

// SoldOutHandler dipanggil saat pesan event.sold_out diterima.
type SoldOutHandler interface {
	MarkSoldOut(ctx context.Context, eventID string) error
}

// SoldOutConsumer consume topik event.sold_out dan update status event.
// event.sold_out di-publish oleh booking-service dan payment-service saat semua stok habis.
// Menggantikan HTTP internal call POST /internal/events/{id}/sold-out yang sebelumnya
// dipanggil booking-service secara synchronous.
type SoldOutConsumer struct {
	reader  *kafkago.Reader
	handler SoldOutHandler
	log     *slog.Logger
}

func NewSoldOutConsumer(cfg config.KafkaConfig, handler SoldOutHandler, log *slog.Logger) *SoldOutConsumer {
	dialer := &kafkago.Dialer{Timeout: 10 * time.Second}
	if cfg.TLS || cfg.Username != "" {
		dialer.TLS = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if cfg.Username != "" {
		dialer.SASLMechanism = plain.Mechanism{Username: cfg.Username, Password: cfg.Password}
	}

	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     strings.Split(cfg.Brokers, ","),
		Topic:       TopicEventSoldOut,
		GroupID:     cfg.ConsumerGroup + "-event-soldout",
		MinBytes:    1,
		MaxBytes:    1e6,
		StartOffset: kafkago.LastOffset,
		Dialer:      dialer,
	})

	return &SoldOutConsumer{reader: r, handler: handler, log: log}
}

// Start menjalankan consumer loop dalam goroutine.
// Berhenti ketika ctx di-cancel (graceful shutdown).
func (c *SoldOutConsumer) Start(ctx context.Context) {
	go c.run(ctx)
	c.log.InfoContext(ctx, "event.sold_out consumer started", slog.String("topic", TopicEventSoldOut))
}

// Close menutup reader.
func (c *SoldOutConsumer) Close() {
	c.reader.Close()
}

func (c *SoldOutConsumer) run(ctx context.Context) {
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				c.log.InfoContext(ctx, "event.sold_out consumer stopped")
				return
			}
			c.log.ErrorContext(ctx, "kafka read error",
				slog.String("topic", TopicEventSoldOut),
				slog.String("error", err.Error()))
			time.Sleep(5 * time.Second)
			continue
		}

		c.log.InfoContext(ctx, "kafka message received",
			slog.String("topic", TopicEventSoldOut),
			slog.Int64("offset", msg.Offset),
			slog.Int("bytes", len(msg.Value)))

		var ev EventSoldOutEvent
		if err := json.Unmarshal(msg.Value, &ev); err != nil {
			c.log.ErrorContext(ctx, "unmarshal event.sold_out failed",
				slog.String("error", err.Error()))
			continue
		}

		start := time.Now()
		if err := c.handler.MarkSoldOut(ctx, ev.EventID); err != nil {
			c.log.ErrorContext(ctx, "mark sold-out failed",
				slog.String("eventId", ev.EventID),
				slog.String("error", err.Error()),
				slog.String("duration", time.Since(start).String()))
		} else {
			c.log.InfoContext(ctx, "event marked sold-out via kafka",
				slog.String("eventId", ev.EventID),
				slog.String("duration", time.Since(start).String()))
		}
	}
}
