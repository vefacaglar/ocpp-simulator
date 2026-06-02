package csms

import (
	"sync"

	"nhooyr.io/websocket"
)

// ConnectionRegistry tracks live WebSocket connections to charge points.
type ConnectionRegistry struct {
	mu          sync.RWMutex
	connections map[string]*websocket.Conn
}

func NewConnectionRegistry() *ConnectionRegistry {
	return &ConnectionRegistry{
		connections: make(map[string]*websocket.Conn),
	}
}

func (r *ConnectionRegistry) Register(chargePointID string, conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.connections[chargePointID] = conn
}

func (r *ConnectionRegistry) Unregister(chargePointID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.connections, chargePointID)
}

func (r *ConnectionRegistry) Get(chargePointID string) (*websocket.Conn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	conn, ok := r.connections[chargePointID]
	return conn, ok
}

func (r *ConnectionRegistry) IsConnected(chargePointID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.connections[chargePointID]
	return ok
}
