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
	ID                string     `json:"id"`
	TimeoutSeconds    int        `json:"timeout_seconds"`
	FailureThreshold  int        `json:"failure_threshold"`
	FailureCount      int        `json:"failure_count"`
	RecoveryThreshold int        `json:"recovery_threshold"`
	RecoveryWindow    int        `json:"recovery_window"`
	ConsecutiveHB     int        `json:"consecutive_hb"`
	AlertOnRecovery   bool       `json:"alert_on_recovery"`
	Status            Status     `json:"status"`
	LastHeartbeatAt   time.Time  `json:"last_heartbeat_at"`
	RecoveryDeadline  *time.Time `json:"recovery_deadline"`
	AlertedAt         *time.Time `json:"alerted_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
