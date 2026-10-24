package delivery

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/refreshTechRusuh/event-service/internal/config"
	"github.com/refreshTechRusuh/event-service/internal/event/application/usecase"
	eventhttp "github.com/refreshTechRusuh/event-service/internal/event/infrastructure/http"
	"github.com/refreshTechRusuh/event-service/pkg/kafka"
	"github.com/refreshTechRusuh/event-service/pkg/middleware"
)

// New mewiring semua dependency event-service.
// Mengembalikan handler HTTP, usecase (untuk consumer), dan closeFn.
func New(_ context.Context, cfg *config.Config, pool *pgxpool.Pool, producer *kafka.Producer) (http.Handler, *usecase.EventUsecase, func(), error) {
	mux := http.NewServeMux()

	handler, uc := eventhttp.NewWiredHandlerWithUC(pool, producer)
	eventhttp.RegisterRoutes(mux, handler, cfg.JWT.AccessTokenSecret)

	// Wrap seluruh mux dengan request logger.
	logged := middleware.RequestLogger(slog.Default())(mux)

	closeFn := func() {
		pool.Close()
		if producer != nil {
			producer.Close()
		}
	}
	return logged, uc, closeFn, nil
}
