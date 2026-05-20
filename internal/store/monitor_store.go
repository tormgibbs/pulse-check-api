package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tormgibbs/pulse-check-api/internal/domain"
)

type MonitorStore struct {
	db *pgxpool.Pool
}

func NewMonitorStore(db *pgxpool.Pool) *MonitorStore {
	return &MonitorStore{db: db}
}

func (s *MonitorStore) Create(ctx context.Context, m *domain.Monitor) error {
	query := `
		INSERT INTO monitors (
			id, timeout_seconds, failure_threshold, recovery_threshold,
			recovery_window, alert_on_recovery, status, last_heartbeat_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := s.db.Exec(ctx, query,
		m.ID, m.TimeoutSeconds, m.FailureThreshold, m.RecoveryThreshold,
		m.RecoveryWindow, m.AlertOnRecovery, m.Status, m.LastHeartbeatAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateMonitor
		}
		return err
	}
	return nil
}

func (s *MonitorStore) GetByID(ctx context.Context, id string) (*domain.Monitor, error) {
	query := `
		SELECT id, timeout_seconds, failure_threshold, failure_count,
			recovery_threshold, recovery_window, consecutive_hb,
			alert_on_recovery, status, last_heartbeat_at,
			recovery_deadline, alerted_at, created_at, updated_at
		FROM monitors WHERE id = $1`

	m := &domain.Monitor{}
	err := s.db.QueryRow(ctx, query, id).Scan(
		&m.ID, &m.TimeoutSeconds, &m.FailureThreshold, &m.FailureCount,
		&m.RecoveryThreshold, &m.RecoveryWindow, &m.ConsecutiveHB,
		&m.AlertOnRecovery, &m.Status, &m.LastHeartbeatAt,
		&m.RecoveryDeadline, &m.AlertedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMonitorNotFound
		}
		return nil, err
	}
	return m, nil
}

func (s *MonitorStore) List(ctx context.Context) ([]*domain.Monitor, error) {
	query := `
		SELECT id, timeout_seconds, failure_threshold, failure_count,
			recovery_threshold, recovery_window, consecutive_hb,
			alert_on_recovery, status, last_heartbeat_at,
			recovery_deadline, alerted_at, created_at, updated_at
		FROM monitors ORDER BY created_at DESC`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []*domain.Monitor
	for rows.Next() {
		m := &domain.Monitor{}
		err := rows.Scan(
			&m.ID, &m.TimeoutSeconds, &m.FailureThreshold, &m.FailureCount,
			&m.RecoveryThreshold, &m.RecoveryWindow, &m.ConsecutiveHB,
			&m.AlertOnRecovery, &m.Status, &m.LastHeartbeatAt,
			&m.RecoveryDeadline, &m.AlertedAt, &m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, nil
}

func (s *MonitorStore) ListRecoverable(ctx context.Context) ([]*domain.Monitor, error) {
	query := `
		SELECT id, timeout_seconds, failure_threshold, failure_count,
			recovery_threshold, recovery_window, consecutive_hb,
			alert_on_recovery, status, last_heartbeat_at,
			recovery_deadline, alerted_at, created_at, updated_at
		FROM monitors WHERE status IN ('active', 'recovering', 'paused')`

	rows, err := s.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var monitors []*domain.Monitor
	for rows.Next() {
		m := &domain.Monitor{}
		err := rows.Scan(
			&m.ID, &m.TimeoutSeconds, &m.FailureThreshold, &m.FailureCount,
			&m.RecoveryThreshold, &m.RecoveryWindow, &m.ConsecutiveHB,
			&m.AlertOnRecovery, &m.Status, &m.LastHeartbeatAt,
			&m.RecoveryDeadline, &m.AlertedAt, &m.CreatedAt, &m.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		monitors = append(monitors, m)
	}
	return monitors, nil
}

func (s *MonitorStore) UpdateHeartbeat(ctx context.Context, id string, now time.Time) error {
	query := `
		UPDATE monitors
		SET last_heartbeat_at = $1, failure_count = 0, updated_at = NOW()
		WHERE id = $2`

	tag, err := s.db.Exec(ctx, query, now, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMonitorNotFound
	}
	return nil
}

func (s *MonitorStore) UpdateStatus(ctx context.Context, id string, status domain.Status, alertedAt *time.Time) error {
	query := `
		UPDATE monitors
		SET status = $1, alerted_at = $2, updated_at = NOW()
		WHERE id = $3`

	tag, err := s.db.Exec(ctx, query, status, alertedAt, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMonitorNotFound
	}
	return nil
}

func (s *MonitorStore) UpdateFailureCount(ctx context.Context, id string, count int) error {
	query := `
		UPDATE monitors SET failure_count = $1, updated_at = NOW()
		WHERE id = $2`

	tag, err := s.db.Exec(ctx, query, count, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMonitorNotFound
	}
	return nil
}

func (s *MonitorStore) UpdateRecovery(ctx context.Context, id string, consecutiveHB int, status domain.Status, recoveryDeadline *time.Time) error {
	query := `
		UPDATE monitors
		SET consecutive_hb = $1, status = $2, recovery_deadline = $3, updated_at = NOW()
		WHERE id = $4`

	tag, err := s.db.Exec(ctx, query, consecutiveHB, status, recoveryDeadline, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMonitorNotFound
	}
	return nil
}

func (s *MonitorStore) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM monitors WHERE id = $1`

	tag, err := s.db.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrMonitorNotFound
	}
	return nil
}
