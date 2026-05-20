package domain

import "errors"

var (
	ErrMonitorNotFound   = errors.New("monitor not found")
	ErrDuplicateMonitor  = errors.New("monitor already exists")
	ErrInvalidTransition = errors.New("invalid state transition")
)
