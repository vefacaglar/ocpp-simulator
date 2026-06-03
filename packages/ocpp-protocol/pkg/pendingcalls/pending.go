// Package pendingcalls tracks in-flight CALL messages so an inbound
// CALLRESULT or CALLERROR can be correlated back to the originating action.
package pendingcalls

import (
	"sync"
	"time"
)

type PendingCall struct {
	UniqueID  string
	Action    string
	SentAt    time.Time
	TimeoutAt time.Time
}

type Registry struct {
	mu    sync.RWMutex
	calls map[string]PendingCall
}

func New() *Registry {
	return &Registry{
		calls: make(map[string]PendingCall),
	}
}

func (c *PendingCall) IsExpired() bool {
	return time.Now().After(c.TimeoutAt)
}

func (r *Registry) Register(call PendingCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls[call.UniqueID] = call
}

func (r *Registry) Resolve(uniqueID string) (PendingCall, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	call, ok := r.calls[uniqueID]
	if ok {
		delete(r.calls, uniqueID)
	}
	return call, ok
}

func (r *Registry) Expired() []PendingCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	var expired []PendingCall
	for id, call := range r.calls {
		if call.IsExpired() {
			expired = append(expired, call)
			delete(r.calls, id)
		}
	}
	return expired
}
