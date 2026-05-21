// internal/watcher/watcher.go
package watcher

import (
	"context"
	"log/slog"
	"time"

	"github.com/tormgibbs/pulse-check-api/internal/domain"
	"github.com/tormgibbs/pulse-check-api/internal/store"
)

type Monitor interface {
	Spawn(m *domain.Monitor)
	SendHeartbeat(id string) bool
	SendPause(id string) bool
	SendStop(id string)
	StartAll(ctx context.Context) error
}

type Watcher struct {
	store    store.Monitor
	registry *Registry
}

func NewWatcher(store store.Monitor, registry *Registry) *Watcher {
	return &Watcher{store: store, registry: registry}
}

func (w *Watcher) Spawn(m *domain.Monitor) {
	handle := NewMonitorHandle()
	w.registry.Register(m.ID, handle)
	go w.run(m, handle)
}

func (w *Watcher) run(m *domain.Monitor, handle *MonitorHandle) {
	defer func() {
		w.registry.Remove(m.ID)
		close(handle.doneCh)
	}()

	remaining := w.remainingTime(m)
	if remaining <= 0 {
		w.handleExpiry(m)
		return
	}

	timer := time.NewTimer(remaining)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			m.FailureCount++
			if m.FailureCount < m.FailureThreshold {
				if err := w.store.UpdateFailureCount(context.Background(), m.ID, m.FailureCount); err != nil {
					slog.Error("failed to update failure count", "id", m.ID, "err", err)
					return
				}
				slog.Info("heartbeat missed", "id", m.ID, "failure_count", m.FailureCount, "threshold", m.FailureThreshold)
				timer.Reset(time.Duration(m.TimeoutSeconds) * time.Second)
				continue
			}
			w.handleExpiry(m)
			return

		case <-handle.resetCh:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			now := time.Now()
			if m.Status == domain.StatusRecovering {
				m.ConsecutiveHB++
				if m.ConsecutiveHB >= m.RecoveryThreshold {
					if err := w.store.UpdateRecovery(context.Background(), m.ID, m.ConsecutiveHB, domain.StatusActive, nil); err != nil {
						slog.Error("failed to update recovery to active", "id", m.ID, "err", err)
						return
					}
					m.Status = domain.StatusActive
					m.FailureCount = 0
					m.ConsecutiveHB = 0
					if m.AlertOnRecovery {
						slog.Warn("RECOVERY ALERT", "id", m.ID, "event", "monitor.recovered", "time", now)
					}
					timer.Reset(time.Duration(m.TimeoutSeconds) * time.Second)
				} else {
					if err := w.store.UpdateRecovery(context.Background(), m.ID, m.ConsecutiveHB, domain.StatusRecovering, m.RecoveryDeadline); err != nil {
						slog.Error("failed to update consecutive heartbeats", "id", m.ID, "err", err)
						return
					}
					remaining := time.Until(*m.RecoveryDeadline)
					if remaining <= 0 {
						w.handleExpiry(m)
						return
					}
					timer.Reset(remaining)
				}
			} else {
				m.FailureCount = 0
				if err := w.store.UpdateHeartbeat(context.Background(), m.ID, now); err != nil {
					slog.Error("failed to update heartbeat", "id", m.ID, "err", err)
					return
				}
				slog.Info("heartbeat received", "id", m.ID, "time", now)
				timer.Reset(time.Duration(m.TimeoutSeconds) * time.Second)
			}

		case <-handle.pauseCh:
			if err := w.store.UpdateStatus(context.Background(), m.ID, domain.StatusPaused, nil); err != nil {
				slog.Error("failed to pause monitor", "id", m.ID, "err", err)
			}
			slog.Info("monitor paused", "id", m.ID)
			return

		case <-handle.stopCh:
			slog.Info("monitor stopped", "id", m.ID)
			return
		}
	}
}

func (w *Watcher) remainingTime(m *domain.Monitor) time.Duration {
	if m.Status == domain.StatusRecovering && m.RecoveryDeadline != nil {
		return time.Until(*m.RecoveryDeadline)
	}
	deadline := m.LastHeartbeatAt.Add(time.Duration(m.TimeoutSeconds) * time.Second)
	return time.Until(deadline)
}

func (w *Watcher) handleExpiry(m *domain.Monitor) {
	now := time.Now()
	maxRetries := 3
	backoff := time.Second

	for attempt := range maxRetries {
		err := w.store.UpdateStatus(context.Background(), m.ID, domain.StatusDown, &now)
		if err == nil {
			break
		}
		slog.Error("failed to persist down status", "id", m.ID, "attempt", attempt+1, "err", err)
		if attempt < maxRetries-1 {
			time.Sleep(backoff)
			backoff *= 2
		} else {
			slog.Error("CRITICAL: failed to persist down status after retries, state is inconsistent", "id", m.ID)
		}
	}

	slog.Warn("ALERT", "event", "monitor.down", "id", m.ID, "time", now)
}

func (w *Watcher) SendHeartbeat(id string) bool {
	handle, ok := w.registry.Get(id)
	if !ok {
		return false
	}
	select {
	case handle.resetCh <- struct{}{}:
	default:
	}
	return true
}

func (w *Watcher) SendPause(id string) bool {
	handle, ok := w.registry.Get(id)
	if !ok {
		return false
	}
	select {
	case handle.pauseCh <- struct{}{}:
	default:
	}
	return true
}

func (w *Watcher) SendStop(id string) {
	handle, ok := w.registry.Get(id)
	if !ok {
		return
	}
	close(handle.stopCh)
}

func (w *Watcher) StartAll(ctx context.Context) error {
	monitors, err := w.store.ListRecoverable(ctx)
	if err != nil {
		return err
	}

	count := 0
	for _, m := range monitors {
		if m.Status == domain.StatusPaused {
			continue
		}
		remaining := w.remainingTime(m)
		if remaining <= 0 {
			w.handleExpiry(m)
			continue
		}
		w.Spawn(m)
		count++
	}

	slog.Info("startup rebuild complete", "goroutines_spawned", count)
	return nil
}
