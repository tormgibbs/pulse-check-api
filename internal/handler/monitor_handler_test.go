package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tormgibbs/pulse-check-api/internal/domain"
)

func newTestHandler(store *mockMonitorStore, watcher *mockWatcher) *MonitorHandler {
	return &MonitorHandler{store: store, watcher: watcher}
}

func makeRequest(t *testing.T, method, url string, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("failed to encode request body: %v", err)
		}
	}
	r, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	r.Header.Set("Content-Type", "application/json")
	return r
}

func setURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestHandleCreate_Success(t *testing.T) {
	store := &mockMonitorStore{
		createFn: func(ctx context.Context, m *domain.Monitor) error {
			return nil
		},
	}
	watcher := &mockWatcher{}
	h := newTestHandler(store, watcher)

	body := map[string]any{"id": "device-01", "timeout": 30}
	r := makeRequest(t, http.MethodPost, "/monitors", body)
	w := httptest.NewRecorder()

	h.HandleCreate(w, r)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201 got %d", w.Code)
	}
}

func TestHandleCreate_InvalidBody(t *testing.T) {
	h := newTestHandler(&mockMonitorStore{}, &mockWatcher{})

	r, _ := http.NewRequest(http.MethodPost, "/monitors", bytes.NewBufferString("not-json"))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 got %d", w.Code)
	}
}

func TestHandleCreate_ValidationError_MissingID(t *testing.T) {
	h := newTestHandler(&mockMonitorStore{}, &mockWatcher{})

	body := map[string]any{"timeout": 30}
	r := makeRequest(t, http.MethodPost, "/monitors", body)
	w := httptest.NewRecorder()

	h.HandleCreate(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 got %d", w.Code)
	}
}

func TestHandleCreate_ValidationError_TimeoutTooLow(t *testing.T) {
	h := newTestHandler(&mockMonitorStore{}, &mockWatcher{})

	body := map[string]any{"id": "device-01", "timeout": 5}
	r := makeRequest(t, http.MethodPost, "/monitors", body)
	w := httptest.NewRecorder()

	h.HandleCreate(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 got %d", w.Code)
	}
}

func TestHandleCreate_ValidationError_InvalidIDCharacters(t *testing.T) {
	h := newTestHandler(&mockMonitorStore{}, &mockWatcher{})

	body := map[string]any{"id": "invalid id!", "timeout": 30}
	r := makeRequest(t, http.MethodPost, "/monitors", body)
	w := httptest.NewRecorder()

	h.HandleCreate(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 got %d", w.Code)
	}
}

func TestHandleCreate_Duplicate(t *testing.T) {
	store := &mockMonitorStore{
		createFn: func(ctx context.Context, m *domain.Monitor) error {
			return domain.ErrDuplicateMonitor
		},
	}
	h := newTestHandler(store, &mockWatcher{})

	body := map[string]any{"id": "device-01", "timeout": 30}
	r := makeRequest(t, http.MethodPost, "/monitors", body)
	w := httptest.NewRecorder()

	h.HandleCreate(w, r)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 got %d", w.Code)
	}
}

func TestHandleHeartbeat_ActiveMonitor(t *testing.T) {
	heartbeatCalled := false
	store := &mockMonitorStore{
		getByIDFn: func(ctx context.Context, id string) (*domain.Monitor, error) {
			return &domain.Monitor{ID: id, Status: domain.StatusActive}, nil
		},
		updateHeartbeatFn: func(ctx context.Context, id string, now time.Time) error {
			heartbeatCalled = true
			return nil
		},
	}
	watcher := &mockWatcher{}
	h := newTestHandler(store, watcher)

	r := makeRequest(t, http.MethodPost, "/monitors/device-01/heartbeat", nil)
	r = setURLParam(r, "id", "device-01")
	w := httptest.NewRecorder()

	h.HandleHeartbeat(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 got %d", w.Code)
	}
	if !heartbeatCalled {
		t.Error("expected UpdateHeartbeat to be called")
	}
}

func TestHandleHeartbeat_NotFound(t *testing.T) {
	store := &mockMonitorStore{
		getByIDFn: func(ctx context.Context, id string) (*domain.Monitor, error) {
			return nil, domain.ErrMonitorNotFound
		},
	}
	h := newTestHandler(store, &mockWatcher{})

	r := makeRequest(t, http.MethodPost, "/monitors/missing/heartbeat", nil)
	r = setURLParam(r, "id", "missing")
	w := httptest.NewRecorder()

	h.HandleHeartbeat(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 got %d", w.Code)
	}
}

func TestHandleHeartbeat_DownMonitor_TransitionsToRecovering(t *testing.T) {
	spawnCalled := false
	store := &mockMonitorStore{
		getByIDFn: func(ctx context.Context, id string) (*domain.Monitor, error) {
			return &domain.Monitor{
				ID:             id,
				Status:         domain.StatusDown,
				RecoveryWindow: 90,
				TimeoutSeconds: 30,
			}, nil
		},
		updateRecoveryFn: func(ctx context.Context, id string, consecutiveHB int, status domain.Status, recoveryDeadline *time.Time) error {
			return nil
		},
	}
	watcher := &mockWatcher{
		spawnFn: func(m *domain.Monitor) {
			spawnCalled = true
		},
	}
	h := newTestHandler(store, watcher)

	r := makeRequest(t, http.MethodPost, "/monitors/device-01/heartbeat", nil)
	r = setURLParam(r, "id", "device-01")
	w := httptest.NewRecorder()

	h.HandleHeartbeat(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 got %d", w.Code)
	}
	if !spawnCalled {
		t.Error("expected Spawn to be called for down monitor")
	}
}

func TestHandleHeartbeat_PausedMonitor_TransitionsToActive(t *testing.T) {
	spawnCalled := false
	store := &mockMonitorStore{
		getByIDFn: func(ctx context.Context, id string) (*domain.Monitor, error) {
			return &domain.Monitor{ID: id, Status: domain.StatusPaused}, nil
		},
		updateHeartbeatFn: func(ctx context.Context, id string, now time.Time) error {
			return nil
		},
		updateStatusFn: func(ctx context.Context, id string, status domain.Status, alertedAt *time.Time) error {
			return nil
		},
	}
	watcher := &mockWatcher{
		spawnFn: func(m *domain.Monitor) {
			spawnCalled = true
		},
	}
	h := newTestHandler(store, watcher)

	r := makeRequest(t, http.MethodPost, "/monitors/device-01/heartbeat", nil)
	r = setURLParam(r, "id", "device-01")
	w := httptest.NewRecorder()

	h.HandleHeartbeat(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 got %d", w.Code)
	}
	if !spawnCalled {
		t.Error("expected Spawn to be called for paused monitor")
	}
}

func TestHandlePause_Success(t *testing.T) {
	store := &mockMonitorStore{
		getByIDFn: func(ctx context.Context, id string) (*domain.Monitor, error) {
			return &domain.Monitor{ID: id, Status: domain.StatusActive}, nil
		},
	}
	h := newTestHandler(store, &mockWatcher{})

	r := makeRequest(t, http.MethodPost, "/monitors/device-01/pause", nil)
	r = setURLParam(r, "id", "device-01")
	w := httptest.NewRecorder()

	h.HandlePause(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 got %d", w.Code)
	}
}

func TestHandlePause_AlreadyPaused(t *testing.T) {
	store := &mockMonitorStore{
		getByIDFn: func(ctx context.Context, id string) (*domain.Monitor, error) {
			return &domain.Monitor{ID: id, Status: domain.StatusPaused}, nil
		},
	}
	h := newTestHandler(store, &mockWatcher{})

	r := makeRequest(t, http.MethodPost, "/monitors/device-01/pause", nil)
	r = setURLParam(r, "id", "device-01")
	w := httptest.NewRecorder()

	h.HandlePause(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 got %d", w.Code)
	}
}

func TestHandlePause_DownMonitor_Rejected(t *testing.T) {
	store := &mockMonitorStore{
		getByIDFn: func(ctx context.Context, id string) (*domain.Monitor, error) {
			return &domain.Monitor{ID: id, Status: domain.StatusDown}, nil
		},
	}
	h := newTestHandler(store, &mockWatcher{})

	r := makeRequest(t, http.MethodPost, "/monitors/device-01/pause", nil)
	r = setURLParam(r, "id", "device-01")
	w := httptest.NewRecorder()

	h.HandlePause(w, r)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 got %d", w.Code)
	}
}

func TestHandleDelete_Success(t *testing.T) {
	deleteCalled := false
	store := &mockMonitorStore{
		getByIDFn: func(ctx context.Context, id string) (*domain.Monitor, error) {
			return &domain.Monitor{ID: id, Status: domain.StatusActive}, nil
		},
		deleteFn: func(ctx context.Context, id string) error {
			deleteCalled = true
			return nil
		},
	}
	h := newTestHandler(store, &mockWatcher{})

	r := makeRequest(t, http.MethodDelete, "/monitors/device-01", nil)
	r = setURLParam(r, "id", "device-01")
	w := httptest.NewRecorder()

	h.HandleDelete(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 got %d", w.Code)
	}
	if !deleteCalled {
		t.Error("expected Delete to be called")
	}
}

func TestHandleDelete_NotFound(t *testing.T) {
	store := &mockMonitorStore{
		getByIDFn: func(ctx context.Context, id string) (*domain.Monitor, error) {
			return nil, domain.ErrMonitorNotFound
		},
	}
	h := newTestHandler(store, &mockWatcher{})

	r := makeRequest(t, http.MethodDelete, "/monitors/missing", nil)
	r = setURLParam(r, "id", "missing")
	w := httptest.NewRecorder()

	h.HandleDelete(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 got %d", w.Code)
	}
}
