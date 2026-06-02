package ocpp

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

type PendingCallRegistry struct {
	mu    sync.RWMutex
	calls map[string]PendingCall
}

func NewPendingCallRegistry() *PendingCallRegistry {
	return &PendingCallRegistry{
		calls: make(map[string]PendingCall),
	}
}

func (r *PendingCall) IsExpired() bool {
	return time.Now().After(r.TimeoutAt)
}

func (r *PendingCallRegistry) Register(call PendingCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls[call.UniqueID] = call
}

func (r *PendingCallRegistry) Resolve(uniqueID string) (PendingCall, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	call, ok := r.calls[uniqueID]
	if ok {
		delete(r.calls, uniqueID)
	}
	return call, ok
}

func (r *PendingCallRegistry) Expired() []PendingCall {
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
