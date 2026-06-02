package simulator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/user/ocpp-simulator/apps/api/internal/common"
	"github.com/user/ocpp-simulator/apps/api/internal/ocpp"
	"github.com/user/ocpp-simulator/apps/api/internal/realtime"
)

type ChargePointInstance struct {
	Config     common.ChargePoint
	Connectors map[int]common.Connector
	Connected  bool
	Client     *OcppWebSocketClient
	mu         sync.RWMutex
}

type Runtime struct {
	mu           sync.RWMutex
	chargePoints map[string]*ChargePointInstance
	eventBus     *realtime.EventBus
	factory      *ocpp.Factory
}

func NewRuntime(eventBus *realtime.EventBus, factory *ocpp.Factory) *Runtime {
	return &Runtime{
		chargePoints: make(map[string]*ChargePointInstance),
		eventBus:     eventBus,
		factory:      factory,
	}
}

func (r *Runtime) AddChargePoint(cp common.ChargePoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chargePoints[cp.ID] = &ChargePointInstance{
		Config:     cp,
		Connectors: make(map[int]common.Connector),
	}
	r.publishEvent("charge_point.created", cp.ID, nil, fmt.Sprintf("Charge point %s created", cp.ID))
}

func (r *Runtime) RemoveChargePoint(id string) {
	r.mu.Lock()
	cp, exists := r.chargePoints[id]
	if exists {
		cp.mu.Lock()
		if cp.Client != nil {
			cp.Client.Disconnect()
		}
		cp.Connected = false
		cp.mu.Unlock()
		delete(r.chargePoints, id)
	}
	r.mu.Unlock()
	if exists {
		r.publishEvent("charge_point.deleted", id, nil, fmt.Sprintf("Charge point %s deleted", id))
	}
}

func (r *Runtime) Connect(id string) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", id)
	}

	protocol, ok := r.factory.Create(cp.Config.OCPPVersion)
	if !ok {
		return fmt.Errorf("unsupported OCPP version: %s", cp.Config.OCPPVersion)
	}

	client := NewOcppWebSocketClient(id, cp.Config.CentralSystemURL, protocol, r.eventBus)

	cp.mu.Lock()
	cp.Client = client
	cp.mu.Unlock()

	if err := client.Connect(r.eventBusCtx()); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	cp.mu.Lock()
	cp.Connected = true
	cp.Config.Status = common.StatusConnected
	cp.mu.Unlock()

	return nil
}

func (r *Runtime) Disconnect(id string) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", id)
	}

	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.Client != nil {
		if err := cp.Client.Disconnect(); err != nil {
			return err
		}
	}

	cp.Connected = false
	cp.Config.Status = common.StatusDisconnected
	return nil
}

func (r *Runtime) SendBootNotification(id string) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", id)
	}

	cp.mu.Lock()
	client := cp.Client
	cp.mu.Unlock()

	if client == nil {
		return fmt.Errorf("charge point %s not connected", id)
	}

	return client.SendBootNotification(r.eventBusCtx(), "Simulator", "OCPP-Sim")
}

func (r *Runtime) SendHeartbeat(id string) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", id)
	}

	cp.mu.Lock()
	client := cp.Client
	cp.mu.Unlock()

	if client == nil {
		return fmt.Errorf("charge point %s not connected", id)
	}

	return client.SendHeartbeat(r.eventBusCtx())
}

func (r *Runtime) eventBusCtx() context.Context {
	return context.Background()
}

func (r *Runtime) GetChargePoint(id string) (*ChargePointInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp, ok := r.chargePoints[id]
	return cp, ok
}

func (r *Runtime) ListChargePoints() []*ChargePointInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*ChargePointInstance, 0, len(r.chargePoints))
	for _, cp := range r.chargePoints {
		list = append(list, cp)
	}
	return list
}

func (r *Runtime) SetConnectorStatus(cpID string, connectorID int, status common.ConnectorStatus) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", cpID)
	}

	cp.mu.Lock()
	c, exists := cp.Connectors[connectorID]
	if !exists {
		cp.mu.Unlock()
		return fmt.Errorf("connector %d not found", connectorID)
	}
	c.Status = status
	c.UpdatedAt = time.Now().UTC()
	cp.Connectors[connectorID] = c
	cp.mu.Unlock()

	r.publishEvent("connector.status_changed", cpID, &connectorID,
		fmt.Sprintf("Connector %d status changed to %s", connectorID, status))
	return nil
}

func (r *Runtime) publishEvent(eventType, cpID string, connectorID *int, message string) {
	r.eventBus.Publish(realtime.Event{
		ID:            fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:          eventType,
		ChargePointID: cpID,
		ConnectorID:   connectorID,
		Message:       message,
		Timestamp:     time.Now().UTC(),
	})
}
