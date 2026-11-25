// Command server runs the geofencing API and real-time alert WebSocket.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/config"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/db"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/httpapi"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/realtime"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/store"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/tracking"
	"github.com/chandug3121/mapup_geo-fence/backend/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	// All timestamps read from Postgres come back in UTC so API responses
	// are consistent no matter where the container runs.
	time.Local = time.UTC

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// ctx is cancelled on Ctrl+C or SIGTERM (what Docker sends on stop).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, migrations.Files, log); err != nil {
		return err
	}

	hubCtx, stopHub := context.WithCancel(context.Background())
	defer stopHub()
	hub := realtime.NewHub(log)
	go hub.Run(hubCtx)

	st := store.New(pool)
	svc := tracking.NewService(st, hub, log)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(cfg, st, svc, hub, log),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: it would cut long-lived WebSocket connections.
		// Each WebSocket write sets its own deadline instead.
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", srv.Addr, "api_key_required", cfg.APIKey != "")
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Close WebSocket clients first (they are hijacked connections, which
	// Shutdown does not wait for), then drain in-flight HTTP requests.
	stopHub()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("server stopped")
	return nil
}
