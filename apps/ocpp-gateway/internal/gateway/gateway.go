// Package gateway is a dumb WebSocket-to-MQTT bridge for OCPP charge
// point sessions. It does not parse OCPP frames, does not own
// business state, and does not persist anything. Inbound raw OCPP-J
// text frames from ws://gateway/ws/{chargePointId} are published
// unchanged to MQTT topic ocpp/{version}/{chargePointId}/in. MQTT
// messages received on ocpp/{version}/{chargePointId}/out are
// written unchanged back to that charge point's WebSocket. The
// version segment is added by the gateway at the WebSocket
// subprotocol handshake boundary (multi-version plan §3): the
// gateway is the only component that sees the handshake, so it is
// the correct place to discover the version. Per-CP connection
// isolation is enforced by a registry keyed by chargePointId.
package gateway

import (
	"fmt"
	"log"
)

// InTopic is the MQTT topic for frames sent by a charge point to the
// server. Outbound from the gateway's perspective, inbound from the
// simulated CP's perspective. The version segment is a topic
// separator (broker-agnostic, MQTT 3.1.1 compatible) so the
// message-processor and canonlog can pick a schema set by
// subscription pattern. The payload stays a raw OCPP-J array
// (multi-version plan: never wrap).
func InTopic(version, chargePointID string) string {
	return "ocpp/" + version + "/" + chargePointID + "/in"
}

// OutTopic is the MQTT topic for frames the server sends back to a
// charge point. Inbound from the gateway's perspective, outbound
// from the server's perspective.
func OutTopic(version, chargePointID string) string {
	return "ocpp/" + version + "/" + chargePointID + "/out"
}

// Gateway wires a Broker and a Registry together. It exposes the
// actions the HTTP layer needs: register/unregister a CP, publish a
// frame that arrived over WS, and consume frames that the broker
// delivered on the CP's /out topic.
type Gateway struct {
	Broker   Broker
	Registry *Registry
}

// New constructs a Gateway. broker must be non-nil.
func New(broker Broker) (*Gateway, error) {
	if broker == nil {
		return nil, ErrNilBroker
	}
	return &Gateway{Broker: broker, Registry: NewRegistry()}, nil
}

// Connect binds a charge point's connection: subscribes to its /out
// topic and stores the binding in the registry. The handler is
// invoked by the broker with raw payload bytes for every message on
// the /out topic. The handler pushes the bytes into the connection's
// Outbound channel; the WS writer goroutine drains it.
//
// If a previous connection for the same charge point id is still
// registered, the registry gracefully closes the old connection and
// replaces it with this new one, enforcing a single active session.
func (g *Gateway) Connect(version, chargePointID string) (*Connection, error) {
	conn := &Connection{
		ChargePointID: chargePointID,
		Version:       version,
	}

	unsubscribe, err := g.Broker.Subscribe(OutTopic(version, chargePointID), func(payload []byte) {
		// Re-lookup so a late delivery after Disconnect cannot
		// write into a closed channel.
		if c, ok := g.Registry.Lookup(chargePointID); ok {
			select {
			case c.Outbound <- payload:
			case <-c.Closed():
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("subscribe %s: %w", OutTopic(version, chargePointID), err)
	}
	conn.Unsubscribe = unsubscribe

	g.Registry.Register(conn)

	log.Printf("[ocpp-gateway] connected charge point %s (version=%s)", chargePointID, version)
	return conn, nil
}

// PublishInbound sends a frame received from the CP's WebSocket to
// the broker on the /in topic. Bytes are forwarded unchanged; this
// method does not parse OCPP.
func (g *Gateway) PublishInbound(version, chargePointID string, payload []byte) error {
	if _, ok := g.Registry.Lookup(chargePointID); !ok {
		return ErrNoSuchChargePoint
	}
	return g.Broker.Publish(InTopic(version, chargePointID), payload)
}

// Disconnect tears down a charge point: removes from registry,
// invokes the broker unsubscribe, signals the writer goroutine via
// Closed().
func (g *Gateway) Disconnect(chargePointID string) {
	g.Registry.Disconnect(chargePointID)
	log.Printf("[ocpp-gateway] disconnected charge point %s", chargePointID)
}
