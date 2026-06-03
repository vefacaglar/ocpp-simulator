// Package canonlog persists every OCPP wire frame observed on
// ocpp/+/in and ocpp/+/out to ocpp_message_logs. The raw payload
// bytes from the broker are kept verbatim, and the envelope is
// parsed lazily to extract message_type / action / unique_id for
// filtering. The log is the source of truth for OCPP traffic in
// the system; message-processor's stdout audit and the realtime
// event stream are auxiliary.
package canonlog

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/broker"
	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/db"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/message"
)

const (
	InboundTopicPattern  = "ocpp/+/in"
	OutboundTopicPattern = "ocpp/+/out"
)

// Service owns the two wildcard subscriptions that make up the
// canonical log. It decodes each frame's envelope just enough to
// populate the log row's structured columns; the payload is stored
// verbatim.
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
	cpID := chargePointIDFromTopic(topic)
	if cpID == "" {
		// Defensive: subscription should never deliver a topic
		// outside ocpp/{cpId}/in|out shape, but the parser
		// protects against broker misconfiguration.
		log.Printf("[ocpp-core/canonlog] unexpected topic %q", topic)
		return
	}
	row := db.MessageLog{
		ChargePointID: cpID,
		Direction:     direction,
		MessageType:   "UNKNOWN",
		PayloadJSON:   string(payload),
		Status:        "ok",
		Topic:         topic,
	}
	if msg, err := s.codec.Decode(payload); err == nil {
		row.MessageType = msgTypeName(msg.MessageTypeID)
		if row.MessageTypeID == nil {
			id := int(msg.MessageTypeID)
			row.MessageTypeID = &id
		} else {
			v := int(msg.MessageTypeID)
			row.MessageTypeID = &v
		}
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

func chargePointIDFromTopic(topic string) string {
	// ocpp/{cpId}/in or ocpp/{cpId}/out
	parts := strings.Split(topic, "/")
	if len(parts) != 3 || parts[0] != "ocpp" {
		return ""
	}
	return parts[1]
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

// Ensure imports used (avoid "imported and not used" when trimming
// during edits). These are referenced by helpers above; this
// comment block keeps the linter happy if the file is regenerated
// in isolation.
var (
	_ = sql.ErrNoRows
	_ = json.Marshal
)
