package csms

import (
	"fmt"
	"sync"
	"time"
)

// PendingCall represents a CSMS-initiated CALL awaiting a CALLRESULT.
type PendingCall struct {
	UniqueID  string
	Action    string
	SentAt    time.Time
	TimeoutAt time.Time
	Response  chan CallResult
}

// CallResult holds the response payload or error from a CALLRESULT/CALLERROR.
type CallResult struct {
	Payload      []byte
	ErrorCode    string
	ErrorDesc    string
	IsError      bool
}

// PendingCallRegistry tracks CSMS-initiated CALLs awaiting responses.
type PendingCallRegistry struct {
	mu    sync.RWMutex
	calls map[string]*PendingCall
}

func NewPendingCallRegistry() *PendingCallRegistry {
	return &PendingCallRegistry{
		calls: make(map[string]*PendingCall),
	}
}

// Register adds a pending call and returns it.
func (r *PendingCallRegistry) Register(uniqueID, action string, timeout time.Duration) *PendingCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	call := &PendingCall{
		UniqueID:  uniqueID,
		Action:    action,
		SentAt:    time.Now(),
		TimeoutAt: time.Now().Add(timeout),
		Response:  make(chan CallResult, 1),
	}
	r.calls[uniqueID] = call
	return call
}

// Resolve completes a pending call with a CALLRESULT payload.
func (r *PendingCallRegistry) Resolve(uniqueID string, payload []byte) bool {
	r.mu.Lock()
	call, ok := r.calls[uniqueID]
	if ok {
		delete(r.calls, uniqueID)
	}
	r.mu.Unlock()

	if ok {
		call.Response <- CallResult{Payload: payload}
		return true
	}
	return false
}

// ResolveError completes a pending call with a CALLERROR.
func (r *PendingCallRegistry) ResolveError(uniqueID, errorCode, errorDesc string) bool {
	r.mu.Lock()
	call, ok := r.calls[uniqueID]
	if ok {
		delete(r.calls, uniqueID)
	}
	r.mu.Unlock()

	if ok {
		call.Response <- CallResult{
			ErrorCode: errorCode,
			ErrorDesc: errorDesc,
			IsError:   true,
		}
		return true
	}
	return false
}

// Cancel removes a pending call without resolving it.
func (r *PendingCallRegistry) Cancel(uniqueID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.calls, uniqueID)
}

// GenerateUniqueID creates a unique ID for a CSMS-initiated CALL.
func GenerateUniqueID() string {
	return fmt.Sprintf("csms-%d", time.Now().UnixNano())
}
