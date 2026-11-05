package event_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/refreshTechRusuh/event-service/internal/event/infrastructure/event"
	"github.com/stretchr/testify/require"
	vk "github.com/valkey-io/valkey-go"
)

// newMiniredisClient membuat vk.Client yang terhubung ke miniredis (in-memory).
func newMiniredisClient(t *testing.T) vk.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	c, err := vk.NewClient(vk.ClientOption{
		InitAddress:  []string{mr.Addr()},
		DisableCache: true,
	})
	require.NoError(t, err)
	t.Cleanup(c.Close)
	return c
}

// ── BroadcastSoldOut ──────────────────────────────────────────────────────────

func TestRedisStockBroadcaster_BroadcastSoldOut_success(t *testing.T) {
	client := newMiniredisClient(t)
	broadcaster := event.NewRedisStockBroadcaster(client, discardLog())

	err := broadcaster.BroadcastSoldOut(context.Background(), "event-1")
	require.NoError(t, err)
}

func TestRedisStockBroadcaster_BroadcastSoldOut_multipleEvents(t *testing.T) {
	client := newMiniredisClient(t)
	broadcaster := event.NewRedisStockBroadcaster(client, discardLog())

	ctx := context.Background()
	require.NoError(t, broadcaster.BroadcastSoldOut(ctx, "event-1"))
	require.NoError(t, broadcaster.BroadcastSoldOut(ctx, "event-2"))
	require.NoError(t, broadcaster.BroadcastSoldOut(ctx, "event-3"))
}

func TestRedisStockBroadcaster_BroadcastSoldOut_idempotent(t *testing.T) {
	// Broadcast sold-out dua kali untuk event yang sama harus tetap berhasil
	client := newMiniredisClient(t)
	broadcaster := event.NewRedisStockBroadcaster(client, discardLog())

	ctx := context.Background()
	require.NoError(t, broadcaster.BroadcastSoldOut(ctx, "event-repeat"))
	require.NoError(t, broadcaster.BroadcastSoldOut(ctx, "event-repeat"))
}
