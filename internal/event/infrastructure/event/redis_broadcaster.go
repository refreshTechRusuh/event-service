package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	vk "github.com/valkey-io/valkey-go"
)

// RedisStockBroadcaster mengimplementasikan outbound.IStockBroadcaster:
// mem-publish frame stok ke channel Redis/Valkey "stock:{eventId}" — channel yang
// sama di-subscribe booking-service untuk SSE. Dengan ini MarkSoldOut di
// event-service langsung terlihat oleh klien SSE tanpa menunggu TTL sweeper.
type RedisStockBroadcaster struct {
	client vk.Client
	log    *slog.Logger
}

func NewRedisStockBroadcaster(client vk.Client, log *slog.Logger) *RedisStockBroadcaster {
	return &RedisStockBroadcaster{client: client, log: log}
}

// stockEvent harus cocok dengan dto.StockEvent di booking-service (field JSON sama).
type stockEvent struct {
	EventID    string `json:"eventId"`
	CategoryID string `json:"categoryId,omitempty"`
	Available  int    `json:"available,omitempty"`
	SoldOut    bool   `json:"soldOut,omitempty"`
}

// BroadcastSoldOut mengirim frame {eventId, soldOut:true} ke channel "stock:{eventId}".
func (b *RedisStockBroadcaster) BroadcastSoldOut(ctx context.Context, eventID string) error {
	channel := fmt.Sprintf("stock:%s", eventID)
	payload, err := json.Marshal(stockEvent{EventID: eventID, SoldOut: true})
	if err != nil {
		return fmt.Errorf("marshal sold-out frame: %w", err)
	}
	start := time.Now()
	err = b.client.Do(ctx,
		b.client.B().Publish().Channel(channel).Message(string(payload)).Build(),
	).Error()
	if err != nil {
		b.log.ErrorContext(ctx, "redis publish failed",
			slog.String("channel", channel),
			slog.String("duration", time.Since(start).String()),
			slog.String("error", err.Error()))
		return err
	}
	b.log.InfoContext(ctx, "redis publish ok",
		slog.String("channel", channel),
		slog.Int("bytes", len(payload)),
		slog.String("duration", time.Since(start).String()))
	return nil
}
