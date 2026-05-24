// internal/handler/monitor_handler.go
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tormgibbs/pulse-check-api/internal/domain"
	"github.com/tormgibbs/pulse-check-api/internal/store"
	"github.com/tormgibbs/pulse-check-api/internal/watcher"
)

var validID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type MonitorHandler struct {
	store   store.Monitor
	watcher watcher.Monitor
}

func NewMonitorHandler(store store.Monitor, watcher watcher.Monitor) *MonitorHandler {
	return &MonitorHandler{store: store, watcher: watcher}
}

type createRequest struct {
	ID                string `json:"id"`
	TimeoutSeconds    int    `json:"timeout"`
	FailureThreshold  int    `json:"failure_threshold"`
	RecoveryThreshold int    `json:"recovery_threshold"`
	RecoveryWindow    int    `json:"recovery_window"`
	AlertOnRecovery   *bool  `json:"alert_on_recovery"`
}

func (req *createRequest) validate() map[string]string {
	errs := make(map[string]string)

	if req.ID == "" {
		errs["id"] = "required"
	} else if len(req.ID) > 64 {
		errs["id"] = "must be 64 characters or less"
	} else if !validID.MatchString(req.ID) {
		errs["id"] = "must contain only letters, numbers, hyphens, and underscores"
	}

	if req.TimeoutSeconds < 10 || req.TimeoutSeconds > 86400 {
		errs["timeout"] = "must be between 10 and 86400"
	}

	if req.FailureThreshold < 0 {
		errs["failure_threshold"] = "must be a positive integer"
	}

	if req.RecoveryThreshold < 0 {
		errs["recovery_threshold"] = "must be a positive integer"
	}

	if req.RecoveryWindow < 0 {
		errs["recovery_window"] = "must be a positive integer"
	}

	return errs
}

func (h *MonitorHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	if req.FailureThreshold == 0 {
		req.FailureThreshold = 1
	}
	if req.RecoveryThreshold == 0 {
		req.RecoveryThreshold = 3
	}
	if req.RecoveryWindow == 0 {
		req.RecoveryWindow = req.RecoveryThreshold * req.TimeoutSeconds
	}

	alertOnRecovery := true
	if req.AlertOnRecovery != nil {
		alertOnRecovery = *req.AlertOnRecovery
	}

	if errs := req.validate(); len(errs) > 0 {
		writeValidationError(w, errs)
		return
	}

	m := &domain.Monitor{
		ID:                req.ID,
		TimeoutSeconds:    req.TimeoutSeconds,
		FailureThreshold:  req.FailureThreshold,
		RecoveryThreshold: req.RecoveryThreshold,
		RecoveryWindow:    req.RecoveryWindow,
		AlertOnRecovery:   alertOnRecovery,
		Status:            domain.StatusActive,
		LastHeartbeatAt:   time.Now(),
	}

	if err := h.store.Create(r.Context(), m); err != nil {
		if errors.Is(err, domain.ErrDuplicateMonitor) {
			writeError(w, http.StatusConflict, "monitor already exists", codeDuplicate)
			return
		}
		writeInternalError(w)
		return
	}

	h.watcher.Spawn(m)

	writeJSON(w, http.StatusCreated, map[string]string{
		"message": "monitor created",
		"id":      m.ID,
	})
}

func (h *MonitorHandler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	m, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrMonitorNotFound) {
			writeNotFound(w)
			return
		}
		writeInternalError(w)
		return
	}

	now := time.Now()

	switch m.Status {
	case domain.StatusActive:
		if err := h.store.UpdateHeartbeat(r.Context(), id, now); err != nil {
			writeInternalError(w)
			return
		}
		h.watcher.SendHeartbeat(id)

	case domain.StatusPaused:
		if err := h.store.UpdateHeartbeat(r.Context(), id, now); err != nil {
			writeInternalError(w)
			return
		}
		if err := h.store.UpdateStatus(r.Context(), id, domain.StatusActive, nil); err != nil {
			writeInternalError(w)
			return
		}
		m.Status = domain.StatusActive
		m.LastHeartbeatAt = now
		h.watcher.Spawn(m)

	case domain.StatusDown:
		recoveryDeadline := now.Add(time.Duration(m.RecoveryWindow) * time.Second)
		if err := h.store.UpdateRecovery(r.Context(), id, 1, domain.StatusRecovering, &recoveryDeadline); err != nil {
			writeInternalError(w)
			return
		}
		m.Status = domain.StatusRecovering
		m.ConsecutiveHB = 1
		m.RecoveryDeadline = &recoveryDeadline
		m.LastHeartbeatAt = now
		h.watcher.Spawn(m)

	case domain.StatusRecovering:
		if err := h.store.UpdateHeartbeat(r.Context(), id, now); err != nil {
			writeInternalError(w)
			return
		}
		h.watcher.SendHeartbeat(id)
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "heartbeat received"})
}

func (h *MonitorHandler) HandlePause(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	m, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrMonitorNotFound) {
			writeNotFound(w)
			return
		}
		writeInternalError(w)
		return
	}

	if m.Status == domain.StatusPaused {
		writeJSON(w, http.StatusOK, map[string]string{"message": "monitor already paused"})
		return
	}

	if m.Status == domain.StatusDown {
		writeError(w, http.StatusConflict, "cannot pause a down monitor", codeInvalidTransition)
		return
	}

	h.watcher.SendPause(id)
	writeJSON(w, http.StatusOK, map[string]string{"message": "monitor paused"})
}

func (h *MonitorHandler) HandleGetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	m, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrMonitorNotFound) {
			writeNotFound(w)
			return
		}
		writeInternalError(w)
		return
	}

	writeJSON(w, http.StatusOK, m)
}

func (h *MonitorHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	monitors, err := h.store.List(r.Context())
	if err != nil {
		writeInternalError(w)
		return
	}

	if monitors == nil {
		monitors = []*domain.Monitor{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"monitors": monitors,
		"total":    len(monitors),
	})
}

func (h *MonitorHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	_, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrMonitorNotFound) {
			writeNotFound(w)
			return
		}
		writeInternalError(w)
		return
	}

	h.watcher.SendStop(id)

	if err := h.store.Delete(r.Context(), id); err != nil {
		writeInternalError(w)
		return
	}

	writeJSON(w, http.StatusNoContent, nil)
}
