package processor

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// Processor subscribes once to ocpp/+/+/in and dispatches every
// inbound frame to the Handler. There is no per-CP state in the
// processor; per nextplan §2b, the processor is stateless across
// calls (aside from its broker link, codec, and protocol
// implementation). The subscription pattern is version-scoped
// (multi-version plan §3) so the same processor instance can
// serve both 1.6J and 2.0.1 charge points concurrently.
type Processor struct {
	Broker  Broker
	Handler *Handler

	// inboundTopic is the MQTT topic the processor subscribes to.
	// It defaults to "ocpp/+/+/in" and is a field so tests can
	// override it.
	inboundTopic string
	unsubscribe  func()
}

func New(broker Broker, handler *Handler) *Processor {
	return &Processor{
		Broker:       broker,
		Handler:      handler,
		inboundTopic: "ocpp/+/+/in",
	}
}

// Start subscribes to the inbound topic. The handler closure
// receives both the originating topic and the payload so the
// version and charge point id can both be parsed from the topic
// (multi-version plan §3: ocpp/{version}/{cpId}/in).
func (p *Processor) Start(ctx context.Context) error {
	if p.Broker == nil {
		return fmt.Errorf("processor: nil broker")
	}
	if p.Handler == nil {
		return fmt.Errorf("processor: nil handler")
	}
	unsub, err := p.Broker.Subscribe(p.inboundTopic, func(topic string, payload []byte) {
		version, cpID, ok := SplitVersionAndCPIDFromTopic(topic)
		if !ok {
			log.Printf("[message-processor] inbound message on unexpected topic %q; dropping", topic)
			return
		}
		p.Handler.HandleWithVersion(ctx, cpID, version, payload)
	})
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", p.inboundTopic, err)
	}
	p.unsubscribe = unsub
	log.Printf("[message-processor] subscribed to %s", p.inboundTopic)
	return nil
}

func (p *Processor) Stop() {
	if p.unsubscribe != nil {
		p.unsubscribe()
		p.unsubscribe = nil
	}
}

// ChargePointIDFromTopic parses a versioned topic
// "ocpp/{version}/{cpId}/in" or "ocpp/{version}/{cpId}/out" and
// returns the {cpId} component, or "" if the topic doesn't match.
// Kept for backward compatibility — see also SplitVersionAndCPIDFromTopic
// which returns both fields.
func ChargePointIDFromTopic(topic string) string {
	_, cpID, ok := SplitVersionAndCPIDFromTopic(topic)
	if !ok {
		return ""
	}
	return cpID
}

// SplitVersionAndCPIDFromTopic parses a "ocpp/{version}/{cpId}/in"
// or "ocpp/{version}/{cpId}/out" topic and returns the version and
// the charge point id, plus a bool indicating whether the topic
// was well-formed. Used by the Start subscription closure to
// route messages to the right per-version handler path.
func SplitVersionAndCPIDFromTopic(topic string) (version, cpID string, ok bool) {
	parts := strings.Split(topic, "/")
	if len(parts) != 4 || parts[0] != "ocpp" || (parts[3] != "in" && parts[3] != "out") {
		return "", "", false
	}
	if parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}
