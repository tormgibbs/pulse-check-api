package handler

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tormgibbs/pulse-check-api/internal/store"
	"github.com/tormgibbs/pulse-check-api/internal/watcher"
)

func NewRouter(store *store.MonitorStore, watcher *watcher.Watcher) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	mh := NewMonitorHandler(store, watcher)

	r.Get("/health", Health)
	r.Post("/monitors", mh.Create)
	r.Post("/monitors/{id}/heartbeat", mh.Heartbeat)
	r.Post("/monitors/{id}/pause", mh.Pause)
	r.Get("/monitors/{id}", mh.GetByID)
	r.Get("/monitors", mh.List)
	r.Delete("/monitors/{id}", mh.Delete)

	return r
}
