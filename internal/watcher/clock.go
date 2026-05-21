package watcher

import "time"

type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(d time.Duration)
}

type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

func (systemClock) NewTimer(d time.Duration) Timer {
	return &systemTimer{t: time.NewTimer(d)}
}

type systemTimer struct {
	t *time.Timer
}

func (s *systemTimer) C() <-chan time.Time {
	return s.t.C
}

func (s *systemTimer) Stop() bool {
	return s.t.Stop()
}

func (s *systemTimer) Reset(d time.Duration) {
	s.t.Reset(d)
}
