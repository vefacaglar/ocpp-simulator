package gateway

import (
	"sync"
)

// Connection is one charge point's bridge binding: a reference to the
// outbound unsubscribe function returned by the broker, plus a per-CP
// write channel for outbound frames. The WebSocket writer goroutine
// drains outbound and writes to the socket. We hold the unsubscribe
// handle so Disconnect can detach from the broker cleanly even if the
// WS is already gone.
type Connection struct {
	ChargePointID string
	Unsubscribe   func()
	Outbound      chan []byte
	CloseOnce     sync.Once
	closed        chan struct{}
}

// Registry tracks all active CP connections. Access is guarded by mu
// so concurrent Register/Disconnect/lookup are safe.
type Registry struct {
	mu    sync.Mutex
	conns map[string]*Connection
}

func NewRegistry() *Registry {
	return &Registry{conns: make(map[string]*Connection)}
}

// Register stores a new Connection. If a connection for the same CP
// id already exists, the existing one is returned along with
// errAlreadyRegistered (defined in broker.go) so the caller can
// decide what to do (typically: close the new WS, log, reject the
// upgrade). The registry never silently overwrites.
func (r *Registry) Register(c *Connection) (*Connection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.conns[c.ChargePointID]; ok {
		return existing, errAlreadyRegistered
	}
	if c.Outbound == nil {
		c.Outbound = make(chan []byte, 64)
	}
	if c.closed == nil {
		c.closed = make(chan struct{})
	}
	r.conns[c.ChargePointID] = c
	return nil, nil
}

// Lookup returns the Connection for chargePointID, or (nil, false).
func (r *Registry) Lookup(chargePointID string) (*Connection, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.conns[chargePointID]
	return c, ok
}

// Disconnect removes the connection from the registry, invokes its
// Unsubscribe handle exactly once, and signals the outbound channel's
// reader goroutine to stop. Idempotent: calling on a missing or
// already disconnected charge point is a no-op.
func (r *Registry) Disconnect(chargePointID string) {
	r.mu.Lock()
	c, ok := r.conns[chargePointID]
	if ok {
		delete(r.conns, chargePointID)
	}
	r.mu.Unlock()
	if !ok {
		return
	}
	c.CloseOnce.Do(func() {
		if c.Unsubscribe != nil {
			c.Unsubscribe()
		}
		close(c.closed)
	})
}

// Closed returns a channel that is closed when Disconnect has been
// called for this connection. WebSocket reader/writer goroutines
// select on it to abort.
func (c *Connection) Closed() <-chan struct{} { return c.closed }

// ChargePointIDs returns a snapshot of registered ids. Used by tests
// and by debug surfaces.
func (r *Registry) ChargePointIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.conns))
	for id := range r.conns {
		out = append(out, id)
	}
	return out
}
