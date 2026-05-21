package watcher

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tormgibbs/pulse-check-api/internal/domain"
)

type mockStore struct {
	mu                   sync.Mutex
	updateFailureCountFn func(ctx context.Context, id string, count int) error
	updateStatusFn       func(ctx context.Context, id string, status domain.Status, alertedAt *time.Time) error
	updateHeartbeatFn    func(ctx context.Context, id string, now time.Time) error
	updateRecoveryFn     func(ctx context.Context, id string, consecutiveHB int, status domain.Status, recoveryDeadline *time.Time) error
}

func (m *mockStore) Create(ctx context.Context, mon *domain.Monitor) error { return nil }
func (m *mockStore) GetByID(ctx context.Context, id string) (*domain.Monitor, error) {
	return nil, nil
}
func (m *mockStore) List(ctx context.Context) ([]*domain.Monitor, error)            { return nil, nil }
func (m *mockStore) ListRecoverable(ctx context.Context) ([]*domain.Monitor, error) { return nil, nil }
func (m *mockStore) Delete(ctx context.Context, id string) error                    { return nil }

func (m *mockStore) UpdateHeartbeat(ctx context.Context, id string, now time.Time) error {
	if m.updateHeartbeatFn != nil {
		return m.updateHeartbeatFn(ctx, id, now)
	}
	return nil
}
func (m *mockStore) UpdateStatus(ctx context.Context, id string, status domain.Status, alertedAt *time.Time) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status, alertedAt)
	}
	return nil
}
func (m *mockStore) UpdateFailureCount(ctx context.Context, id string, count int) error {
	if m.updateFailureCountFn != nil {
		return m.updateFailureCountFn(ctx, id, count)
	}
	return nil
}
func (m *mockStore) UpdateRecovery(ctx context.Context, id string, consecutiveHB int, status domain.Status, recoveryDeadline *time.Time) error {
	if m.updateRecoveryFn != nil {
		return m.updateRecoveryFn(ctx, id, consecutiveHB, status, recoveryDeadline)
	}
	return nil
}

func newTestWatcher(store *mockStore, clock Clock) *Watcher {
	registry := NewRegistry()
	return &Watcher{store: store, registry: registry, clock: clock}
}

func activeMonitor(id string) *domain.Monitor {
	now := time.Now()
	return &domain.Monitor{
		ID:                id,
		TimeoutSeconds:    30,
		FailureThreshold:  2,
		RecoveryThreshold: 3,
		RecoveryWindow:    90,
		Status:            domain.StatusActive,
		LastHeartbeatAt:   now,
		FailureCount:      0,
		ConsecutiveHB:     0,
	}
}

func TestWatcher_TimerFires_BelowThreshold_IncrementsFailureCount(t *testing.T) {
	clock := newFakeClock(time.Now())
	failureCountUpdated := 0

	store := &mockStore{
		updateFailureCountFn: func(ctx context.Context, id string, count int) error {
			failureCountUpdated = count
			return nil
		},
	}

	w := newTestWatcher(store, clock)
	m := activeMonitor("device-01")
	m.FailureThreshold = 2

	w.Spawn(m)

	timer := clock.WaitTimer(t)
	timer.Fire(clock.Now())

	time.Sleep(50 * time.Millisecond)

	if failureCountUpdated != 1 {
		t.Errorf("expected failure_count 1, got %d", failureCountUpdated)
	}
}

func TestWatcher_TimerFires_AtThreshold_TransitionsToDown(t *testing.T) {
	clock := newFakeClock(time.Now())
	var downdStatus domain.Status

	store := &mockStore{
		updateStatusFn: func(ctx context.Context, id string, status domain.Status, alertedAt *time.Time) error {
			downdStatus = status
			return nil
		},
	}

	w := newTestWatcher(store, clock)
	m := activeMonitor("device-02")
	m.FailureThreshold = 1

	handle := NewMonitorHandle()
	w.registry.Register(m.ID, handle)
	go w.run(m, handle)

	timer := clock.WaitTimer(t)
	timer.Fire(clock.Now())

	<-handle.doneCh

	if downdStatus != domain.StatusDown {
		t.Errorf("expected status down, got %s", downdStatus)
	}
}

func TestWatcher_Heartbeat_ActiveMonitor_ResetsTimer(t *testing.T) {
	clock := newFakeClock(time.Now())
	heartbeatCalled := false

	store := &mockStore{
		updateHeartbeatFn: func(ctx context.Context, id string, now time.Time) error {
			heartbeatCalled = true
			return nil
		},
	}

	w := newTestWatcher(store, clock)
	m := activeMonitor("device-03")
	w.Spawn(m)

	w.SendHeartbeat(m.ID)

	time.Sleep(50 * time.Millisecond)

	if !heartbeatCalled {
		t.Error("expected UpdateHeartbeat to be called on heartbeat signal")
	}
}

func TestWatcher_PauseSignal_ExitsGoroutine(t *testing.T) {
	clock := newFakeClock(time.Now())
	pausedStatus := domain.Status("")

	store := &mockStore{
		updateStatusFn: func(ctx context.Context, id string, status domain.Status, alertedAt *time.Time) error {
			pausedStatus = status
			return nil
		},
	}

	w := newTestWatcher(store, clock)
	m := activeMonitor("device-04")

	handle := NewMonitorHandle()
	w.registry.Register(m.ID, handle)
	go w.run(m, handle)

	w.SendPause(m.ID)

	<-handle.doneCh

	if pausedStatus != domain.StatusPaused {
		t.Errorf("expected status paused, got %s", pausedStatus)
	}
}

func TestWatcher_StopSignal_ExitsCleanly(t *testing.T) {
	clock := newFakeClock(time.Now())
	w := newTestWatcher(&mockStore{}, clock)
	m := activeMonitor("device-05")

	handle := NewMonitorHandle()
	w.registry.Register(m.ID, handle)
	go w.run(m, handle)

	w.SendStop(m.ID)

	select {
	case <-handle.doneCh:

	case <-time.After(time.Second):
		t.Error("goroutine did not exit after stop signal")
	}
}

func TestWatcher_NegativeRemainingTime_TransitionsToDownImmediately(t *testing.T) {
	clock := newFakeClock(time.Now())
	var downdStatus domain.Status

	store := &mockStore{
		updateStatusFn: func(ctx context.Context, id string, status domain.Status, alertedAt *time.Time) error {
			downdStatus = status
			return nil
		},
	}

	w := newTestWatcher(store, clock)
	m := activeMonitor("device-06")

	m.LastHeartbeatAt = time.Now().Add(-2 * time.Minute)

	handle := NewMonitorHandle()
	w.registry.Register(m.ID, handle)
	go w.run(m, handle)

	<-handle.doneCh

	if downdStatus != domain.StatusDown {
		t.Errorf("expected status down, got %s", downdStatus)
	}
}
