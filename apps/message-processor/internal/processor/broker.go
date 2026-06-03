package processor

import (
	"context"
	"errors"
)

// Broker is the minimal MQTT surface the processor needs. The
// production implementation wraps an eclipse/paho.mqtt.golang
// client; tests provide a fake. The processor publishes raw bytes
// only; it never wraps the payload.
//
// The Subscribe handler signature includes the originating topic
// because the processor must derive the charge point id from the
// ocpp/{cpId}/in topic, and a payload-only handler cannot do that
// for a wildcard subscription.
type Broker interface {
	Publish(topic string, payload []byte) error
	Subscribe(topic string, handler func(topic string, payload []byte)) (unsubscribe func(), err error)
	IsConnected() bool
	Connect(ctx context.Context) error
	Disconnect() error
}

// ErrUnknownAction is the decision recorded when a CALL arrives with
// an action the processor does not implement. The handler turns this
// into a NotImplemented CALLERROR.
var ErrUnknownAction = errors.New("unsupported OCPP action")

// ErrCoreUnavailable is returned by the core callback client when
// the HTTP call to ocpp-core fails or returns an error response. The
// handler turns this into a GenericError CALLERROR for the specific
// action that needed a business decision.
var ErrCoreUnavailable = errors.New("ocpp-core unavailable")
