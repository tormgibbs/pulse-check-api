package domain

import "time"

type Status string

const (
	StatusActive     Status = "active"
	StatusPaused     Status = "paused"
	StatusDown       Status = "down"
	StatusRecovering Status = "recovering"
)

type Monitor struct {
	ID                string
	TimeoutSeconds    int
	FailureThreshold  int
	FailureCount      int
	RecoveryThreshold int
	RecoveryWindow    int
	ConsecutiveHB     int
	AlertOnRecovery   bool
	Status            Status
	LastHeartbeatAt   time.Time
	RecoveryDeadline  *time.Time
	AlertedAt         *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
