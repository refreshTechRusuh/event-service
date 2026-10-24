package delivery

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/refreshTechRusuh/event-service/internal/config"
	eventhttp "github.com/refreshTechRusuh/event-service/internal/event/infrastructure/http"
	"github.com/refreshTechRusuh/event-service/pkg/kafka"
	"github.com/refreshTechRusuh/event-service/pkg/middleware"
)

func New(_ context.Context, cfg *config.Config, pool *pgxpool.Pool, producer *kafka.Producer) (http.Handler, func(), error) {
	mux := http.NewServeMux()

	handler := eventhttp.NewWiredHandler(pool, producer)
	eventhttp.RegisterRoutes(mux, handler, cfg.JWT.AccessTokenSecret)

	// Wrap seluruh mux dengan request logger — catat semua inbound request.
	logged := middleware.RequestLogger(slog.Default())(mux)

	closeFn := func() {
		pool.Close()
		if producer != nil {
			producer.Close()
		}
	}
	return logged, closeFn, nil
}
