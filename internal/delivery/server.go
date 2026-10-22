package delivery

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/refreshTechRusuh/event-service/internal/config"
	eventhttp "github.com/refreshTechRusuh/event-service/internal/event/infrastructure/http"
	"github.com/refreshTechRusuh/event-service/pkg/middleware"
)

func New(_ context.Context, cfg *config.Config, pool *pgxpool.Pool) (*http.ServeMux, func(), error) {
	mux := http.NewServeMux()

	handler := eventhttp.NewWiredHandler(pool)
	// OptionalAuth dijalankan di level mux agar role tersedia di semua route.
	wrappedMux := middleware.Chain(mux, middleware.OptionalAuth(cfg.JWT.AccessTokenSecret))
	eventhttp.RegisterRoutes(mux, handler, cfg.JWT.AccessTokenSecret)
	_ = wrappedMux

	closeFn := func() { pool.Close() }
	return mux, closeFn, nil
}
