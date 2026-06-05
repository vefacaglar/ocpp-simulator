// Package csms owns CSMS-initiated OCPP CALL production. The CSMS
// (which in this simulator is the back-end side represented by
// ocpp-core) initiates commands like RemoteStartTransaction, Reset,
// ChangeConfiguration, etc. The CSMS service generates the raw
// [2, uniqueId, action, payload] frame, validates it against the
// official OCPP schema for the CP's negotiated version, and
// publishes it on ocpp/{version}/{cpId}/out.
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

	"github.com/vefacaglar/ocpp-simulator/apps/ocpp-core/internal/broker"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	ocppschemas "github.com/vefacaglar/ocpp-simulator/packages/ocpp-schemas"
)

// Service produces CSMS-initiated CALL frames and publishes them to
// the charge point's /out topic. The version is supplied per
// call (multi-version plan §3): ocpp-core's CSMS endpoint knows
// the version because it knows the CP.
type Service struct {
	broker broker.Broker
	codec  *codec.Codec

	mu       sync.Mutex
	uniqueID int64
}

func New(b broker.Broker) *Service {
	return &Service{broker: b, codec: codec.New()}
}

// remoteStart publishes a 1.6J RemoteStartTransaction.req. The
// 2.0.1 equivalent is requestStartTransaction (carries idToken
// and remoteStartId).
func (s *Service) remoteStart(ctx context.Context, cpID, version, idTag string, connectorID *int) error {
	req := struct {
		IDTag       string `json:"idTag"`
		ConnectorID *int   `json:"connectorId,omitempty"`
	}{IDTag: idTag, ConnectorID: connectorID}
	return s.publish(ctx, cpID, version, "RemoteStartTransaction", req)
}

func (s *Service) remoteStop(ctx context.Context, cpID, version string, transactionID int) error {
	req := struct {
		TransactionID int `json:"transactionId"`
	}{TransactionID: transactionID}
	return s.publish(ctx, cpID, version, "RemoteStopTransaction", req)
}

func (s *Service) reset(ctx context.Context, cpID, version, resetType string) error {
	if version == "2.0.1" {
		if resetType != "Immediate" && resetType != "OnIdle" {
			return fmt.Errorf("reset: invalid type %q (2.0.1 wants Immediate|OnIdle)", resetType)
		}
	} else if resetType != "Hard" && resetType != "Soft" {
		return fmt.Errorf("reset: invalid type %q (1.6J wants Hard|Soft)", resetType)
	}
	req := struct {
		Type string `json:"type"`
	}{Type: resetType}
	return s.publish(ctx, cpID, version, "Reset", req)
}

func (s *Service) unlockConnector(ctx context.Context, cpID, version string, evseID, connectorID int) error {
	if version == "2.0.1" {
		if evseID <= 0 || connectorID <= 0 {
			return fmt.Errorf("unlockConnector: 2.0.1 requires evseId>0 and connectorId>0")
		}
		req := struct {
			EVSEID      int `json:"evseId"`
			ConnectorID int `json:"connectorId"`
		}{EVSEID: evseID, ConnectorID: connectorID}
		return s.publish(ctx, cpID, version, "UnlockConnector", req)
	}
	if connectorID <= 0 {
		return fmt.Errorf("unlockConnector: connectorId must be > 0")
	}
	req := struct {
		ConnectorID int `json:"connectorId"`
	}{ConnectorID: connectorID}
	return s.publish(ctx, cpID, version, "UnlockConnector", req)
}

func (s *Service) changeConfiguration(ctx context.Context, cpID, version, key, value string) error {
	// 1.6J-only. v201 uses SetVariables on the device model.
	if version == "2.0.1" {
		return fmt.Errorf("changeConfiguration: not supported in 2.0.1; use SetVariables")
	}
	if key == "" {
		return fmt.Errorf("changeConfiguration: key is required")
	}
	req := struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}{Key: key, Value: value}
	return s.publish(ctx, cpID, version, "ChangeConfiguration", req)
}

func (s *Service) getConfiguration(ctx context.Context, cpID, version string, keys []string) error {
	if version == "2.0.1" {
		return fmt.Errorf("getConfiguration: not supported in 2.0.1; use GetVariables")
	}
	req := struct {
		Key []string `json:"key,omitempty"`
	}{Key: keys}
	return s.publish(ctx, cpID, version, "GetConfiguration", req)
}

func (s *Service) triggerMessage(ctx context.Context, cpID, version, requestedMessage string, evseID *int) error {
	if requestedMessage == "" {
		return fmt.Errorf("triggerMessage: requestedMessage is required")
	}
	if version == "2.0.1" {
		// 2.0.1 uses evse {id}; 1.6J uses flat connectorId.
		req := struct {
			EVSE            *struct {
				ID int `json:"id"`
			} `json:"evse,omitempty"`
			RequestedMessage string `json:"requestedMessage"`
		}{RequestedMessage: requestedMessage}
		if evseID != nil {
			req.EVSE = &struct {
				ID int `json:"id"`
			}{ID: *evseID}
		}
		return s.publish(ctx, cpID, version, "TriggerMessage", req)
	}
	req := struct {
		RequestedMessage string `json:"requestedMessage"`
		ConnectorID      *int   `json:"connectorId,omitempty"`
	}{RequestedMessage: requestedMessage, ConnectorID: evseID}
	return s.publish(ctx, cpID, version, "TriggerMessage", req)
}

func (s *Service) changeAvailability(ctx context.Context, cpID, version string, evseID int, availType string) error {
	if availType != "Operative" && availType != "Inoperative" {
		return fmt.Errorf("changeAvailability: invalid type %q", availType)
	}
	if version == "2.0.1" {
		if evseID <= 0 {
			return fmt.Errorf("changeAvailability: 2.0.1 requires evseId>0")
		}
		req := struct {
			EVSE              *struct {
				ID int `json:"id"`
			} `json:"evse"`
			OperationalStatus string `json:"operationalStatus"`
		}{
			EVSE:              &struct{ ID int `json:"id"` }{ID: evseID},
			OperationalStatus: availType,
		}
		return s.publish(ctx, cpID, version, "ChangeAvailability", req)
	}
	req := struct {
		ConnectorID int    `json:"connectorId"`
		Type        string `json:"type"`
	}{ConnectorID: evseID, Type: availType}
	return s.publish(ctx, cpID, version, "ChangeAvailability", req)
}

// --- 2.0.1-only CSMS methods ----

// requestStartTransaction publishes a 2.0.1
// RequestStartTransaction.req. The idToken is a structured
// {idToken, type} object (1.6J uses a flat idTag). remoteStartID
// is the CSMS-supplied correlation id that the CP echoes back in
// the TransactionEvent.
func (s *Service) requestStartTransaction(ctx context.Context, cpID string, idTokenValue, idTokenType string, remoteStartID int, evseID *int) error {
	if idTokenValue == "" || idTokenType == "" {
		return fmt.Errorf("requestStartTransaction: idToken and type are required")
	}
	req := struct {
		EVSEID        *int   `json:"evseId,omitempty"`
		IDToken       struct {
			IDToken string `json:"idToken"`
			Type    string `json:"type"`
		} `json:"idToken"`
		RemoteStartID int `json:"remoteStartId"`
	}{RemoteStartID: remoteStartID}
	req.IDToken.IDToken = idTokenValue
	req.IDToken.Type = idTokenType
	req.EVSEID = evseID
	return s.publish(ctx, cpID, "2.0.1", "RequestStartTransaction", req)
}

// requestStopTransaction publishes a 2.0.1
// RequestStopTransaction.req. The transactionId is the CP-chosen
// string GUID (NOT a 1.6J integer).
func (s *Service) requestStopTransaction(ctx context.Context, cpID, transactionID string) error {
	if transactionID == "" {
		return fmt.Errorf("requestStopTransaction: transactionId is required")
	}
	req := struct {
		TransactionID string `json:"transactionId"`
	}{TransactionID: transactionID}
	return s.publish(ctx, cpID, "2.0.1", "RequestStopTransaction", req)
}

// publish builds a CALL frame for the given action, validates the
// payload against the official OCPP schema for the negotiated
// version, and publishes the raw frame on
// ocpp/{version}/{cpId}/out. The version is mandatory: the v201
// schemas are not interchangeable with v16, and the topic
// segment is the routing key the gateway uses to deliver the
// frame to the right CP session.
func (s *Service) publish(ctx context.Context, cpID, version, action string, payload interface{}) error {
	if version == "" {
		return fmt.Errorf("publish: version is required")
	}
	var schemaVersion ocppschemas.Version
	switch version {
	case "1.6J":
		schemaVersion = ocppschemas.V16
	case "2.0.1":
		schemaVersion = ocppschemas.V201
	default:
		return fmt.Errorf("publish: unknown version %q", version)
	}
	// Spec-valid request payload check before wire write. The
	// canonical log service subscribes to /out and persists
	// whatever we publish here, so invalid payloads would
	// otherwise show up in the log and confuse downstream
	// consumers.
	probe, _ := json.Marshal(payload)
	if err := ocppschemas.Validate(schemaVersion, action, ocppschemas.Request, probe); err != nil {
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
	topic := "ocpp/" + version + "/" + cpID + "/out"
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

// RemoteStart is the public 1.6J entry point. Returns nil on
// successful publication; errors are validation/encode/publish
// failures. For 2.0.1, use RequestStartTransaction.
func (s *Service) RemoteStart(ctx context.Context, cpID, idTag string, connectorID *int) error {
	return s.remoteStart(ctx, cpID, "1.6J", idTag, connectorID)
}

func (s *Service) RemoteStop(ctx context.Context, cpID string, transactionID int) error {
	return s.remoteStop(ctx, cpID, "1.6J", transactionID)
}

// Reset dispatches to the right enum set for the negotiated
// version. 1.6J: Hard|Soft. 2.0.1: Immediate|OnIdle. The caller
// supplies the version explicitly.
func (s *Service) Reset(ctx context.Context, cpID, version, resetType string) error {
	return s.reset(ctx, cpID, version, resetType)
}

func (s *Service) UnlockConnector(ctx context.Context, cpID, version string, evseOrConnectorID int) error {
	if version == "2.0.1" {
		return s.unlockConnector(ctx, cpID, version, evseOrConnectorID, 1)
	}
	return s.unlockConnector(ctx, cpID, version, 0, evseOrConnectorID)
}

func (s *Service) ChangeConfiguration(ctx context.Context, cpID, key, value string) error {
	return s.changeConfiguration(ctx, cpID, "1.6J", key, value)
}

func (s *Service) GetConfiguration(ctx context.Context, cpID string, keys []string) error {
	return s.getConfiguration(ctx, cpID, "1.6J", keys)
}

func (s *Service) TriggerMessage(ctx context.Context, cpID, version, requestedMessage string, evseOrConnectorID *int) error {
	return s.triggerMessage(ctx, cpID, version, requestedMessage, evseOrConnectorID)
}

func (s *Service) ChangeAvailability(ctx context.Context, cpID, version string, evseOrConnectorID int, availType string) error {
	return s.changeAvailability(ctx, cpID, version, evseOrConnectorID, availType)
}

// RequestStartTransaction is the 2.0.1 entry point. idTokenValue
// is the raw token (e.g. RFID card id); idTokenType is the spec
// enum (ISO14443, ISO15693, eMAID, ...).
func (s *Service) RequestStartTransaction(ctx context.Context, cpID, idTokenValue, idTokenType string, remoteStartID int, evseID *int) error {
	return s.requestStartTransaction(ctx, cpID, idTokenValue, idTokenType, remoteStartID, evseID)
}

// RequestStopTransaction is the 2.0.1 entry point. transactionID
// is the string GUID the CP chose when the transaction started.
func (s *Service) RequestStopTransaction(ctx context.Context, cpID, transactionID string) error {
	return s.requestStopTransaction(ctx, cpID, transactionID)
}

// Compile-time interface check (no-op; keeps the import explicit
// in case future helpers reference the package).
var _ message.MessageTypeID
