package delivery_test

import (
	"context"
	"testing"

	"github.com/refreshTechRusuh/event-service/internal/config"
	"github.com/refreshTechRusuh/event-service/internal/delivery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDelivery_New_nilDeps(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}
	cfg.JWT.AccessTokenSecret = "test-access-secret"
	cfg.JWT.UserJWTSecret = "test-user-secret"

	handler, closeFn, err := delivery.New(ctx, cfg, nil, nil, nil)
	require.NoError(t, err)
	assert.NotNil(t, handler)
	assert.NotNil(t, closeFn)
	// Tidak memanggil closeFn() karena pool/producer/valkeyClient = nil → panic
}
