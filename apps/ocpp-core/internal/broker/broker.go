// Package broker defines the minimal MQTT surface ocpp-core needs.
// Production: PahoBroker. Tests: an in-memory fake. The core
// service never wraps MQTT payloads: whatever bytes it publishes on
// ocpp/{cpId}/out or realtime/{cpId}.event are forwarded as-is, so
// the OCPP wire contract is preserved end-to-end.
package broker

import (
	"context"
	"errors"
)

type Broker interface {
	Publish(topic string, payload []byte) error
	Subscribe(topic string, handler func(topic string, payload []byte)) (unsubscribe func(), err error)
	IsConnected() bool
	Connect(ctx context.Context) error
	Disconnect() error
}

var ErrBrokerDown = errors.New("mqtt broker not connected")
