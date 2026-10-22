package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/refreshTechRusuh/event-service/internal/config"
	"github.com/refreshTechRusuh/event-service/internal/delivery"
	pkgdb "github.com/refreshTechRusuh/event-service/pkg/db"
	"github.com/refreshTechRusuh/event-service/pkg/middleware"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}
	config.SetupLogger(cfg.Log)

	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()

	pool, err := pkgdb.NewPool(rootCtx, cfg.DB)
	if err != nil {
		slog.Error("failed to connect to postgres", "err", err)
		os.Exit(1)
	}

	mux, closeDeps, err := delivery.New(rootCtx, cfg, pool)
	if err != nil {
		slog.Error("failed to init server", "err", err)
		os.Exit(1)
	}
	defer closeDeps()

	handler := middleware.Chain(mux, middleware.TransactionID, middleware.OptionalAuth(cfg.JWT.AccessTokenSecret))

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.App.Port),
		ReadHeaderTimeout: 5 * time.Second,
		Handler:           handler,
	}

	go func() {
		slog.Info("event-service listening", "port", cfg.App.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err); os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancelRoot()
	shutCtx, cancelShut := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShut()
	srv.Shutdown(shutCtx)
	slog.Info("event-service stopped")
}
