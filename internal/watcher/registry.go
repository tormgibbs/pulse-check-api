// internal/watcher/registry.go
package watcher

import "sync"

type MonitorHandle struct {
	resetCh chan struct{}
	pauseCh chan struct{}
	stopCh  chan struct{}
	doneCh  chan struct{}
}

type Registry struct {
	mu       sync.RWMutex
	monitors map[string]*MonitorHandle
}

func NewRegistry() *Registry {
	return &Registry{
		monitors: make(map[string]*MonitorHandle),
	}
}

func NewMonitorHandle() *MonitorHandle {
	return &MonitorHandle{
		resetCh: make(chan struct{}, 1),
		pauseCh: make(chan struct{}, 1),
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

func (r *Registry) Register(id string, handle *MonitorHandle) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.monitors[id] = handle
}

func (r *Registry) Get(id string) (*MonitorHandle, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handle, ok := r.monitors[id]
	return handle, ok
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.monitors, id)
}

func (r *Registry) StopAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, handle := range r.monitors {
		close(handle.stopCh)
	}
	r.monitors = make(map[string]*MonitorHandle)
}

func (r *Registry) WaitAll() {
	r.mu.RLock()
	handles := make([]*MonitorHandle, 0, len(r.monitors))
	for _, h := range r.monitors {
		handles = append(handles, h)
	}
	r.mu.RUnlock()

	for _, h := range handles {
		<-h.doneCh
	}
}
