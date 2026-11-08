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
	vk "github.com/valkey-io/valkey-go"
)

func New(_ context.Context, cfg *config.Config, pool *pgxpool.Pool, producer *kafka.Producer, valkeyClient vk.Client) (http.Handler, func(), error) {
	mux := http.NewServeMux()

	handler := eventhttp.NewWiredHandler(pool, producer, valkeyClient)
	eventhttp.RegisterRoutes(mux, handler, cfg.JWT.AccessTokenSecret, cfg.JWT.UserJWTSecret)

	// Wrap seluruh mux dengan request logger — catat semua inbound request.
	// OtelMiddleware di-apply paling luar agar span mencakup seluruh request lifecycle
	// termasuk logging middleware.
	logged := middleware.RequestLogger(slog.Default())(mux)
	traced := middleware.OtelMiddleware("event-service")(logged)

	closeFn := func() {
		pool.Close()
		if producer != nil {
			producer.Close() // #nosec G104 -- shutdown, error irrelevant
		}
		if valkeyClient != nil {
			valkeyClient.Close()
		}
	}
	return traced, closeFn, nil
}
