// Package canonlog persists every OCPP wire frame observed on
// ocpp/+/+/in and ocpp/+/+/out to ocpp_message_logs. The raw
// payload bytes from the broker are kept verbatim, and the
// envelope is parsed lazily to extract message_type / action /
// unique_id for filtering. The log is the source of truth for
// OCPP traffic in the system; message-processor's stdout audit
// and the realtime event stream are auxiliary.
package canonlog

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/vefacaglar/ocpp-simulator/apps/ocpp-core/internal/broker"
	"github.com/vefacaglar/ocpp-simulator/apps/ocpp-core/internal/db"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/message"
)

const (
	InboundTopicPattern  = "ocpp/+/+/in"
	OutboundTopicPattern = "ocpp/+/+/out"
)

// Service owns the two wildcard subscriptions that make up the
// canonical log. It decodes each frame's envelope just enough to
// populate the log row's structured columns; the payload is stored
// verbatim. The OCPP version is read off the topic segment and
// persisted in ocpp_message_logs.ocpp_version (multi-version plan
// §3: the topic carries routing metadata, the payload stays
// raw OCPP-J).
type Service struct {
	broker broker.Broker
	repo   *db.MessageLogRepo
	codec  *codec.Codec
}

func New(b broker.Broker, repo *db.MessageLogRepo) *Service {
	return &Service{broker: b, repo: repo, codec: codec.New()}
}

// Start subscribes to the inbound and outbound wildcard topics.
// Returns the unsubscribe function for graceful shutdown.
func (s *Service) Start(ctx context.Context) (func(), error) {
	unsubIn, err := s.broker.Subscribe(InboundTopicPattern, func(topic string, payload []byte) {
		s.persist(ctx, topic, "inbound", payload)
	})
	if err != nil {
		return nil, fmt.Errorf("subscribe %s: %w", InboundTopicPattern, err)
	}
	unsubOut, err := s.broker.Subscribe(OutboundTopicPattern, func(topic string, payload []byte) {
		s.persist(ctx, topic, "outbound", payload)
	})
	if err != nil {
		unsubIn()
		return nil, fmt.Errorf("subscribe %s: %w", OutboundTopicPattern, err)
	}
	return func() {
		unsubIn()
		unsubOut()
	}, nil
}

// persist decodes the envelope and writes a log row. Decode errors
// are logged but do not stop the wire — the row is still written
// with status="parse_error" so the log is the canonical record
// even for malformed traffic.
func (s *Service) persist(ctx context.Context, topic, direction string, payload []byte) {
	version, cpID, ok := splitVersionedTopic(topic)
	if !ok {
		// Defensive: subscription should never deliver a topic
		// outside the versioned shape, but the parser protects
		// against broker misconfiguration.
		log.Printf("[ocpp-core/canonlog] unexpected topic %q", topic)
		return
	}
	row := db.MessageLog{
		ChargePointID: cpID,
		Direction:     direction,
		OCPPVersion:   strPtr(version),
		MessageType:   "UNKNOWN",
		PayloadJSON:   string(payload),
		Status:        "ok",
		Topic:         topic,
	}
	if msg, err := s.codec.Decode(payload); err == nil {
		row.MessageType = msgTypeName(msg.MessageTypeID)
		id := int(msg.MessageTypeID)
		row.MessageTypeID = &id
		row.UniqueID = strPtr(msg.UniqueID)
		if msg.Action != "" {
			a := msg.Action
			row.Action = &a
		}
		switch msg.MessageTypeID {
		case message.CALLERROR:
			row.Status = "call_error"
			c := msg.ErrorCode
			row.ErrorCode = &c
			d := msg.ErrorDescription
			row.ErrorDescription = &d
		}
	} else {
		row.Status = "parse_error"
	}
	if _, err := s.repo.Create(ctx, row); err != nil {
		// We log the persistence failure but do not propagate:
		// the wire is already in flight and we cannot undo it.
		// A monitoring system can alert on this log line.
		log.Printf("[ocpp-core/canonlog] persist %s %s: %v", direction, cpID, err)
	}
}

// splitVersionedTopic parses "ocpp/{version}/{cpId}/in" or
// "ocpp/{version}/{cpId}/out" and returns the version, charge
// point id, and a bool indicating whether the topic was
// well-formed. Returns ok=false on any other shape.
func splitVersionedTopic(topic string) (version, cpID string, ok bool) {
	parts := strings.Split(topic, "/")
	if len(parts) != 4 || parts[0] != "ocpp" || (parts[3] != "in" && parts[3] != "out") {
		return "", "", false
	}
	if parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func msgTypeName(id message.MessageTypeID) string {
	switch id {
	case message.CALL:
		return "CALL"
	case message.CALLRESULT:
		return "CALLRESULT"
	case message.CALLERROR:
		return "CALLERROR"
	default:
		return "UNKNOWN"
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
