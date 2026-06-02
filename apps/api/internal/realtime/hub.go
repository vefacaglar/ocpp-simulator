package realtime

import (
	"encoding/json"
	"sync"
	"time"
)

type Event struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	ChargePointID string          `json:"chargePointId"`
	ConnectorID   *int            `json:"connectorId,omitempty"`
	Direction     *string         `json:"direction,omitempty"`
	Action        *string         `json:"action,omitempty"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	Message       string          `json:"message,omitempty"`
	Timestamp     time.Time       `json:"timestamp"`
}

type EventHandler func(Event)

type EventBus struct {
	mu       sync.RWMutex
	handlers []EventHandler
}

func NewEventBus() *EventBus {
	return &EventBus{}
}

func (b *EventBus) Subscribe(handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, handler)
}

func (b *EventBus) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, h := range b.handlers {
		h(event)
	}
}

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan Event]struct{}
	bus         *EventBus
}

func NewHub(bus *EventBus) *Hub {
	h := &Hub{
		subscribers: make(map[string]map[chan Event]struct{}),
		bus:         bus,
	}
	bus.Subscribe(h.onEvent)
	return h
}

func (h *Hub) onEvent(event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers[event.ChargePointID] {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *Hub) SubscribeChargePoint(chargePointID string) chan Event {
	ch := make(chan Event, 64)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subscribers[chargePointID] == nil {
		h.subscribers[chargePointID] = make(map[chan Event]struct{})
	}
	h.subscribers[chargePointID][ch] = struct{}{}
	return ch
}

func (h *Hub) UnsubscribeChargePoint(chargePointID string, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.subscribers[chargePointID], ch)
}
