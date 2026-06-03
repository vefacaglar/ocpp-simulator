package gateway

import (
	"context"
	"errors"
)

// Broker is the minimal MQTT surface the gateway needs. The production
// implementation wraps an eclipse/paho.mqtt.golang client; tests
// provide a fake. The gateway only cares about topic strings; payload
// bytes are forwarded unchanged.
type Broker interface {
	// Publish writes payload to topic with retain=false, qos=0.
	// Returns an error if the broker is not connected.
	Publish(topic string, payload []byte) error
	// Subscribe registers a per-topic handler. The handler is invoked
	// for every message the broker delivers on that topic. The
	// returned function unsubscribes and detaches the handler.
	Subscribe(topic string, handler func(payload []byte)) (unsubscribe func(), err error)
	// IsConnected reports whether the broker link is currently usable.
	IsConnected() bool
	// Connect dials or re-dials the broker; safe to call multiple
	// times.
	Connect(ctx context.Context) error
	// Disconnect closes the broker link cleanly.
	Disconnect() error
}

// ErrNoSuchChargePoint is returned by Gateway when an operation
// references a charge point that has no registered session.
var ErrNoSuchChargePoint = errors.New("charge point not registered")

// ErrNilBroker is returned by New if the broker is nil.
var ErrNilBroker = errors.New("broker is nil")

// errAlreadyRegistered is returned when Connect is called twice for the
// same charge point id. It is unexported because callers see it only
// via the error string; tests assert on errors.Is through the public
// surface.
var errAlreadyRegistered = errors.New("charge point already registered")
