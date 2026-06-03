package processor

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// Processor subscribes once to ocpp/+/in and dispatches every
// inbound frame to the Handler. There is no per-CP state in the
// processor; per nextplan §2b, the processor is stateless across
// calls (aside from its broker link, codec, and protocol
// implementation).
type Processor struct {
	Broker  Broker
	Handler *Handler

	// inboundTopic is the MQTT topic the processor subscribes to.
	// It defaults to "ocpp/+/in" and is a field so tests can
	// override it.
	inboundTopic string
	unsubscribe  func()
}

func New(broker Broker, handler *Handler) *Processor {
	return &Processor{
		Broker:       broker,
		Handler:      handler,
		inboundTopic: "ocpp/+/in",
	}
}

// Start subscribes to the inbound topic. The handler closure
// receives both the originating topic and the payload so the
// charge point id can be parsed from the topic.
func (p *Processor) Start(ctx context.Context) error {
	if p.Broker == nil {
		return fmt.Errorf("processor: nil broker")
	}
	if p.Handler == nil {
		return fmt.Errorf("processor: nil handler")
	}
	unsub, err := p.Broker.Subscribe(p.inboundTopic, func(topic string, payload []byte) {
		cpID := ChargePointIDFromTopic(topic)
		if cpID == "" {
			log.Printf("[message-processor] inbound message on unexpected topic %q; dropping", topic)
			return
		}
		p.Handler.Handle(ctx, cpID, payload)
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

// ChargePointIDFromTopic parses a "ocpp/{cpId}/in" or
// "ocpp/{cpId}/out" topic and returns the {cpId} component, or ""
// if the topic doesn't match. Used by the Start subscription
// closure to extract the charge point id from the originating
// topic of an inbound message.
func ChargePointIDFromTopic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != "ocpp" || (parts[2] != "in" && parts[2] != "out") {
		return ""
	}
	if parts[1] == "" {
		return ""
	}
	return parts[1]
}
