package event_test

import (
	"testing"

	"github.com/refreshTechRusuh/event-service/internal/event/infrastructure/event"
	"github.com/stretchr/testify/assert"
)

// TestRedisStockBroadcaster_Construction verifikasi konstruksi broadcaster.
// BroadcastSoldOut memanggil vk.Client.Do() yang membutuhkan koneksi Redis nyata.
// Test publish-level dilakukan via integration test dengan Redis real atau miniredis.
func TestRedisStockBroadcaster_Construction(t *testing.T) {
	// nil client; tidak ada operasi Redis yang dipanggil di sini.
	// Verifikasi bahwa tipe dibuat tanpa panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NewRedisStockBroadcaster panicked: %v", r)
		}
	}()

	broadcaster := event.NewRedisStockBroadcaster(nil, discardLog())
	assert.NotNil(t, broadcaster)
}
