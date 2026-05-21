package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tormgibbs/pulse-check-api/internal/store"
	"github.com/tormgibbs/pulse-check-api/internal/watcher"
)

func NewRouter(store store.Monitor, watcher watcher.Monitor, pool *pgxpool.Pool) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	mh := NewMonitorHandler(store, watcher)
	hh := NewHealthHandler(pool)

	r.Get("/", HomeHandler)
	r.Get("/health", hh.HandleHealthCheck)
	r.Post("/monitors", mh.HandleCreate)
	r.Post("/monitors/{id}/heartbeat", mh.HandleHeartbeat)
	r.Post("/monitors/{id}/pause", mh.HandlePause)
	r.Get("/monitors/{id}", mh.HandleGetByID)
	r.Get("/monitors", mh.HandleList)
	r.Delete("/monitors/{id}", mh.HandleDelete)

	return r
}
