package handler

import (
	"context"
	"time"

	"github.com/tormgibbs/pulse-check-api/internal/domain"
)

type mockMonitorStore struct {
	createFn             func(ctx context.Context, m *domain.Monitor) error
	getByIDFn            func(ctx context.Context, id string) (*domain.Monitor, error)
	listFn               func(ctx context.Context) ([]*domain.Monitor, error)
	listRecoverableFn    func(ctx context.Context) ([]*domain.Monitor, error)
	updateHeartbeatFn    func(ctx context.Context, id string, now time.Time) error
	updateStatusFn       func(ctx context.Context, id string, status domain.Status, alertedAt *time.Time) error
	updateFailureCountFn func(ctx context.Context, id string, count int) error
	updateRecoveryFn     func(ctx context.Context, id string, consecutiveHB int, status domain.Status, recoveryDeadline *time.Time) error
	deleteFn             func(ctx context.Context, id string) error
}

func (m *mockMonitorStore) Create(ctx context.Context, mon *domain.Monitor) error {
	return m.createFn(ctx, mon)
}

func (m *mockMonitorStore) GetByID(ctx context.Context, id string) (*domain.Monitor, error) {
	return m.getByIDFn(ctx, id)
}

func (m *mockMonitorStore) List(ctx context.Context) ([]*domain.Monitor, error) {
	return m.listFn(ctx)
}

func (m *mockMonitorStore) ListRecoverable(ctx context.Context) ([]*domain.Monitor, error) {
	return m.listRecoverableFn(ctx)
}

func (m *mockMonitorStore) UpdateHeartbeat(ctx context.Context, id string, now time.Time) error {
	return m.updateHeartbeatFn(ctx, id, now)
}


func (m *mockMonitorStore) UpdateStatus(ctx context.Context, id string, status domain.Status, alertedAt *time.Time) error {
	return m.updateStatusFn(ctx, id, status, alertedAt)
}


func (m *mockMonitorStore) UpdateFailureCount(ctx context.Context, id string, count int) error {
	return m.updateFailureCountFn(ctx, id, count)
}


func (m *mockMonitorStore) UpdateRecovery(ctx context.Context, id string, consecutiveHB int, status domain.Status, recoveryDeadline *time.Time) error {
	return m.updateRecoveryFn(ctx, id, consecutiveHB, status, recoveryDeadline)
}


func (m *mockMonitorStore) Delete(ctx context.Context, id string) error {
	return m.deleteFn(ctx, id)
}


type mockWatcher struct {
	spawnFn         func(m *domain.Monitor)
	sendHeartbeatFn func(id string) bool
	sendPauseFn     func(id string) bool
	sendStopFn      func(id string)
	startAllFn      func(ctx context.Context) error
}

func (w *mockWatcher) Spawn(m *domain.Monitor) {
	if w.spawnFn != nil {
		w.spawnFn(m)
	}
}

func (w *mockWatcher) SendHeartbeat(id string) bool {
	if w.sendHeartbeatFn != nil {
		return w.sendHeartbeatFn(id)
	}
	return true
}

func (w *mockWatcher) SendPause(id string) bool {
	if w.sendPauseFn != nil {
		return w.sendPauseFn(id)
	}
	return true
}

func (w *mockWatcher) SendStop(id string) {
	if w.sendStopFn != nil {
		w.sendStopFn(id)
	}
}

func (w *mockWatcher) StartAll(ctx context.Context) error {
	if w.startAllFn != nil {
		return w.startAllFn(ctx)
	}
	return nil
}
