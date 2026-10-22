package delivery

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/refreshTechRusuh/event-service/internal/config"
	eventhttp "github.com/refreshTechRusuh/event-service/internal/event/infrastructure/http"
	"github.com/refreshTechRusuh/event-service/pkg/kafka"
	"github.com/refreshTechRusuh/event-service/pkg/middleware"
)

func New(_ context.Context, cfg *config.Config, pool *pgxpool.Pool, producer *kafka.Producer) (*http.ServeMux, func(), error) {
	mux := http.NewServeMux()

	handler := eventhttp.NewWiredHandler(pool, producer)
	eventhttp.RegisterRoutes(mux, handler, cfg.JWT.AccessTokenSecret)

	// OptionalAuth di level mux agar role tersedia di semua route.
	_ = middleware.OptionalAuth // dipakai di handler via Chain

	closeFn := func() {
		pool.Close()
		if producer != nil {
			producer.Close()
		}
	}
	return mux, closeFn, nil
}
