package store

import (
	"context"
	"time"

	"github.com/tormgibbs/pulse-check-api/internal/domain"
)

type Monitor interface {
	Create(ctx context.Context, m *domain.Monitor) error
	GetByID(ctx context.Context, id string) (*domain.Monitor, error)
	List(ctx context.Context) ([]*domain.Monitor, error)
	ListRecoverable(ctx context.Context) ([]*domain.Monitor, error)
	UpdateHeartbeat(ctx context.Context, id string, now time.Time) error
	UpdateStatus(ctx context.Context, id string, status domain.Status, alertedAt *time.Time) error
	UpdateFailureCount(ctx context.Context, id string, count int) error
	UpdateRecovery(ctx context.Context, id string, consecutiveHB int, status domain.Status, recoveryDeadline *time.Time) error
	Delete(ctx context.Context, id string) error
}


