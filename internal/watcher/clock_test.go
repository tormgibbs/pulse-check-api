package watcher

import (
	"sync"
	"testing"
	"time"
)

type fakeTimer struct {
	ch      chan time.Time
	stopped bool
	mu      sync.Mutex
}

func newFakeTimer() *fakeTimer {
	return &fakeTimer{
		ch: make(chan time.Time, 1),
	}
}

func (f *fakeTimer) C() <-chan time.Time {
	return f.ch
}

func (f *fakeTimer) Stop() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped {
		return false
	}
	f.stopped = true
	return true
}

func (f *fakeTimer) Reset(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = false

	select {
	case <-f.ch:
	default:
	}
}

func (f *fakeTimer) Fire(t time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.stopped {
		f.ch <- t
	}
}

type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	timers  []*fakeTimer
	timerCh chan *fakeTimer
}

func newFakeClock(now time.Time) *fakeClock {
	return &fakeClock{
		now:     now,
		timerCh: make(chan *fakeTimer, 10),
	}
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

func (f *fakeClock) NewTimer(d time.Duration) Timer {
	f.mu.Lock()
	t := newFakeTimer()
	f.timers = append(f.timers, t)
	f.mu.Unlock()
	f.timerCh <- t
	return t
}

func (f *fakeClock) WaitTimer(t *testing.T) *fakeTimer {
	t.Helper()
	select {
	case timer := <-f.timerCh:
		return timer
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for timer to be created")
		return nil
	}
}

func (f *fakeClock) Advance(now time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = now
	for _, t := range f.timers {
		t.Fire(now)
	}
}
