package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tormgibbs/pulse-check-api/internal/config"
	"github.com/tormgibbs/pulse-check-api/internal/db"
	"github.com/tormgibbs/pulse-check-api/internal/handler"
	"github.com/tormgibbs/pulse-check-api/internal/store"
	"github.com/tormgibbs/pulse-check-api/internal/watcher"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	pool, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.RunMigrations(cfg.DatabaseURL, "internal/db/migrations"); err != nil {
		slog.Error("failed to run migrations", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations applied successfully")

	monitorStore := store.NewMonitorStore(pool)
	registry := watcher.NewRegistry()
	w := watcher.NewWatcher(monitorStore, registry)

	if err := w.StartAll(context.Background()); err != nil {
		slog.Error("failed to rebuild watchers on startup", "err", err)
		os.Exit(1)
	}

	router := handler.NewRouter(monitorStore, w)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "err", err)
	}

	registry.StopAllAndWait()

	slog.Info("server exited cleanly")
}
