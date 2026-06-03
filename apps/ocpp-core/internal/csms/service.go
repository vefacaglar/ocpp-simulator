// Package csms owns CSMS-initiated OCPP CALL production. The CSMS
// (which in this simulator is the back-end side represented by
// ocpp-core) initiates commands like RemoteStartTransaction, Reset,
// ChangeConfiguration, etc. The CSMS service generates the raw
// [2, uniqueId, action, payload] frame, validates it against the
// official OCPP schema, and publishes it on ocpp/{cpId}/out.
//
// Per nextplan §2b: the payload bytes are spec-exact. Internal
// domain types (transactions, runtime state) never appear on the
// wire.
package csms

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/broker"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	ocppschemas "github.com/user/ocpp-simulator/packages/ocpp-schemas"
)

// Service produces CSMS-initiated CALL frames and publishes them to
// the charge point's /out topic.
type Service struct {
	broker broker.Broker
	codec  *codec.Codec

	mu       sync.Mutex
	uniqueID int64
}

func New(b broker.Broker) *Service {
	return &Service{broker: b, codec: codec.New()}
}

// remoteStart publishes a RemoteStartTransaction.req to cpId.
// The request payload is built directly to the spec's shape; the
// CP's CALLRESULT will arrive on /in and be logged canonically.
func (s *Service) remoteStart(ctx context.Context, cpID string, idTag string, connectorID *int) error {
	req := struct {
		IDTag       string `json:"idTag"`
		ConnectorID *int   `json:"connectorId,omitempty"`
	}{IDTag: idTag, ConnectorID: connectorID}
	return s.publish(ctx, cpID, "RemoteStartTransaction", req)
}

func (s *Service) remoteStop(ctx context.Context, cpID string, transactionID int) error {
	req := struct {
		TransactionID int `json:"transactionId"`
	}{TransactionID: transactionID}
	return s.publish(ctx, cpID, "RemoteStopTransaction", req)
}

func (s *Service) reset(ctx context.Context, cpID, resetType string) error {
	if resetType != "Hard" && resetType != "Soft" {
		return fmt.Errorf("reset: invalid type %q", resetType)
	}
	req := struct {
		Type string `json:"type"`
	}{Type: resetType}
	return s.publish(ctx, cpID, "Reset", req)
}

func (s *Service) unlockConnector(ctx context.Context, cpID string, connectorID int) error {
	if connectorID <= 0 {
		return fmt.Errorf("unlockConnector: connectorId must be > 0")
	}
	req := struct {
		ConnectorID int `json:"connectorId"`
	}{ConnectorID: connectorID}
	return s.publish(ctx, cpID, "UnlockConnector", req)
}

func (s *Service) changeConfiguration(ctx context.Context, cpID, key, value string) error {
	if key == "" {
		return fmt.Errorf("changeConfiguration: key is required")
	}
	req := struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}{Key: key, Value: value}
	return s.publish(ctx, cpID, "ChangeConfiguration", req)
}

func (s *Service) getConfiguration(ctx context.Context, cpID string, keys []string) error {
	req := struct {
		Key []string `json:"key,omitempty"`
	}{Key: keys}
	return s.publish(ctx, cpID, "GetConfiguration", req)
}

func (s *Service) triggerMessage(ctx context.Context, cpID, requestedMessage string, connectorID *int) error {
	if requestedMessage == "" {
		return fmt.Errorf("triggerMessage: requestedMessage is required")
	}
	req := struct {
		RequestedMessage string `json:"requestedMessage"`
		ConnectorID      *int   `json:"connectorId,omitempty"`
	}{RequestedMessage: requestedMessage, ConnectorID: connectorID}
	return s.publish(ctx, cpID, "TriggerMessage", req)
}

func (s *Service) changeAvailability(ctx context.Context, cpID string, connectorID int, availType string) error {
	if availType != "Operative" && availType != "Inoperative" {
		return fmt.Errorf("changeAvailability: invalid type %q", availType)
	}
	req := struct {
		ConnectorID int    `json:"connectorId"`
		Type        string `json:"type"`
	}{ConnectorID: connectorID, Type: availType}
	return s.publish(ctx, cpID, "ChangeAvailability", req)
}

// publish builds a CALL frame for the given action, validates the
// payload against the OCPP 1.6J request schema, and publishes the
// raw frame on ocpp/{cpId}/out.
func (s *Service) publish(ctx context.Context, cpID, action string, payload interface{}) error {
	// Spec-valid request payload check before wire write. This is
	// the same package the v16 codec uses elsewhere; the
	// canonical log service subscribes to /out and persists
	// whatever we publish here, so invalid payloads would
	// otherwise show up in the log and confuse downstream
	// consumers.
	probe, _ := json.Marshal(payload)
	if err := ocppschemas.Validate(ocppschemas.V16, action, ocppschemas.Request, probe); err != nil {
		return fmt.Errorf("schema validation failed for %s: %w", action, err)
	}
	uid := s.nextUniqueID()
	msg, err := s.codec.BuildCall(uid, action, payload)
	if err != nil {
		return fmt.Errorf("build CALL %s: %w", action, err)
	}
	raw, err := s.codec.Encode(msg)
	if err != nil {
		return fmt.Errorf("encode CALL %s: %w", action, err)
	}
	topic := "ocpp/" + cpID + "/out"
	if err := s.broker.Publish(topic, raw); err != nil {
		return fmt.Errorf("publish %s: %w", topic, err)
	}
	log.Printf("[ocpp-core/csms] published %s to %s (uid=%s)", action, topic, uid)
	return nil
}

func (s *Service) nextUniqueID() string {
	s.mu.Lock()
	s.uniqueID++
	n := s.uniqueID
	s.mu.Unlock()
	// Mix a process-local counter and 8 random bytes so two
	// concurrent processes can produce non-colliding ids even if
	// their counters are reset to 0. The OCPP wire contract
	// only requires the id be a non-empty string; uniqueness
	// across processes is best-effort. The first 8 hex chars
	// are the per-process counter; the rest is randomness.
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-derived randomness; the OCPP wire
		// contract only requires that the id be a non-empty
		// string.
		b = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
	}
	return fmt.Sprintf("%08x-%s", n, hex.EncodeToString(b))
}

// --- Public action surface ---

// RemoteStart is the public entry point. Returns nil on successful
// publication; errors are validation/encode/publish failures.
func (s *Service) RemoteStart(ctx context.Context, cpID, idTag string, connectorID *int) error {
	return s.remoteStart(ctx, cpID, idTag, connectorID)
}

func (s *Service) RemoteStop(ctx context.Context, cpID string, transactionID int) error {
	return s.remoteStop(ctx, cpID, transactionID)
}

func (s *Service) Reset(ctx context.Context, cpID, resetType string) error {
	return s.reset(ctx, cpID, resetType)
}

func (s *Service) UnlockConnector(ctx context.Context, cpID string, connectorID int) error {
	return s.unlockConnector(ctx, cpID, connectorID)
}

func (s *Service) ChangeConfiguration(ctx context.Context, cpID, key, value string) error {
	return s.changeConfiguration(ctx, cpID, key, value)
}

func (s *Service) GetConfiguration(ctx context.Context, cpID string, keys []string) error {
	return s.getConfiguration(ctx, cpID, keys)
}

func (s *Service) TriggerMessage(ctx context.Context, cpID, requestedMessage string, connectorID *int) error {
	return s.triggerMessage(ctx, cpID, requestedMessage, connectorID)
}

func (s *Service) ChangeAvailability(ctx context.Context, cpID string, connectorID int, availType string) error {
	return s.changeAvailability(ctx, cpID, connectorID, availType)
}

// Compile-time interface check (no-op; keeps the import explicit
// in case future helpers reference the package).
var _ message.MessageTypeID
