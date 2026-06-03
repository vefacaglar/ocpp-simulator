package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
	ocppschemas "github.com/user/ocpp-simulator/packages/ocpp-schemas"
)

// Handler is the response-producing logic of the processor. It owns
// the OCPP version-aware protocol implementation, the core callback
// client, and the audit emitter. All it needs is the broker to
// publish.
type Handler struct {
	Codec    *codec.Codec
	Protocol protocol.Protocol
	Core     CoreClient
	Broker   Broker
	Emitter  AuditEmitter
	Version  string // ocpp version string registered with the codec (e.g. "1.6J")
}

// NewHandler builds a Handler for the given protocol implementation.
// core may be nil for development setups that don't yet route
// business decisions to ocpp-core; in that case Authorize/Start/
// Stop CALLs will fail with ErrCoreUnavailable.
func NewHandler(p protocol.Protocol, core CoreClient, broker Broker, emitter AuditEmitter) *Handler {
	if emitter == nil {
		emitter = StdoutEmitter{IncludeFrame: false}
	}
	return &Handler{
		Codec:    codec.New(),
		Protocol: p,
		Core:     core,
		Broker:   broker,
		Emitter:  emitter,
		Version:  p.Version(),
	}
}

// Handle is the per-message entry point invoked by the MQTT
// subscriber. It owns the full lifecycle: parse envelope, audit
// "consumed", decide on response (or no-op for inbound
// CALLRESULT/CALLERROR), build, validate, publish, audit
// "published". Any error inside the response path is converted into
// a CALLERROR before publication.
func (h *Handler) Handle(ctx context.Context, chargePointID string, rawFrame []byte) {
	msg, err := h.Codec.Decode(rawFrame)
	if err != nil {
		h.Emitter.Emit(AuditEvent{
			Kind:          "consumed",
			ChargePointID: chargePointID,
			Direction:     "inbound",
			Decision:      "parse_error",
			Note:          err.Error(),
			Frame:         string(rawFrame),
		})
		return
	}

	// Inbound audit for every successfully parsed frame.
	h.Emitter.Emit(AuditEvent{
		Kind:          "consumed",
		ChargePointID: chargePointID,
		Direction:     "inbound",
		MessageType:   msgTypeName(msg.MessageTypeID),
		Action:        msg.Action,
		UniqueID:      msg.UniqueID,
		Frame:         string(rawFrame),
	})

	switch msg.MessageTypeID {
	case message.CALL:
		h.respondToCall(ctx, chargePointID, msg)
	case message.CALLRESULT, message.CALLERROR:
		// CP→server result/error. The processor never produces a
		// response for these. Audit only.
		h.Emitter.Emit(AuditEvent{
			Kind:          "consumed",
			ChargePointID: chargePointID,
			Direction:     "inbound",
			MessageType:   msgTypeName(msg.MessageTypeID),
			Action:        msg.Action,
			UniqueID:      msg.UniqueID,
			Decision:      "noop",
		})
	default:
		// Defensive: codec.Decode already rejects unknown types.
		log.Printf("[message-processor] unexpected message type %d from %s", msg.MessageTypeID, chargePointID)
	}
}

// respondToCall dispatches a CP→server CALL to the right handler,
// builds the response message, validates it against the official
// schema, publishes it to ocpp/{cpId}/out, and audits the
// publication.
func (h *Handler) respondToCall(ctx context.Context, chargePointID string, msg message.Message) {
	var (
		respPayload json.RawMessage
		err         error
	)

	switch msg.Action {
	// ---- core-independent responses ----
	case "BootNotification":
		respPayload, err = h.handleBootNotification()
	case "Heartbeat":
		respPayload, err = h.handleHeartbeat()
	case "StatusNotification":
		respPayload, err = h.handleStatusNotification()
	case "MeterValues":
		respPayload, err = h.handleMeterValues()

	// ---- core-callback responses ----
	case "Authorize":
		respPayload, err = h.handleAuthorize(ctx, chargePointID, msg.Payload)
	case "StartTransaction":
		respPayload, err = h.handleStartTransaction(ctx, chargePointID, msg.Payload)
	case "StopTransaction":
		respPayload, err = h.handleStopTransaction(ctx, chargePointID, msg.Payload)

	default:
		h.publishError(chargePointID, msg.UniqueID, message.ErrorCodeNotImplemented,
			fmt.Sprintf("action %s is not supported by message-processor", msg.Action), nil, msg.Action)
		return
	}

	if err != nil {
		// Distinguish "this action is unknown" from "core is
		// down". The dispatch above already handled unknown
		// actions, so an error here means a real processing
		// failure: bad payload, core unavailable, schema
		// validation, etc.
		note := err.Error()
		code := message.ErrorCodeGenericError
		if errors.Is(err, ErrCoreUnavailable) {
			// Keep the same code; the note carries the cause.
		}
		h.publishError(chargePointID, msg.UniqueID, code, note, nil, msg.Action)
		return
	}

	h.publishResult(chargePointID, msg.UniqueID, respPayload, msg.Action)
}

// publishResult encodes, validates, and publishes a CALLRESULT to
// the CP's /out topic.
func (h *Handler) publishResult(chargePointID, uniqueID string, payload json.RawMessage, action string) {
	if err := h.validateResponseSchema(action, payload); err != nil {
		h.publishError(chargePointID, uniqueID, message.ErrorCodeGenericError,
			"response schema validation failed: "+err.Error(), nil, action)
		return
	}
	resp, err := h.Codec.BuildResult(uniqueID, payload)
	if err != nil {
		log.Printf("[message-processor] build CALLRESULT %s for %s: %v", action, chargePointID, err)
		return
	}
	raw, err := h.Codec.Encode(resp)
	if err != nil {
		log.Printf("[message-processor] encode CALLRESULT %s for %s: %v", action, chargePointID, err)
		return
	}
	outTopic := "ocpp/" + chargePointID + "/out"
	if pubErr := h.Broker.Publish(outTopic, raw); pubErr != nil {
		h.Emitter.Emit(AuditEvent{
			Kind:          "published",
			ChargePointID: chargePointID,
			Direction:     "outbound",
			MessageType:   "CALLRESULT",
			Action:        action,
			UniqueID:      uniqueID,
			Topic:         outTopic,
			Decision:      "publish_failed",
			Note:          pubErr.Error(),
		})
		return
	}
	h.Emitter.Emit(AuditEvent{
		Kind:          "published",
		ChargePointID: chargePointID,
		Direction:     "outbound",
		MessageType:   "CALLRESULT",
		Action:        action,
		UniqueID:      uniqueID,
		Topic:         outTopic,
		Decision:      "responded",
		Frame:         string(raw),
	})
}

// publishError encodes and publishes a CALLERROR to the CP's /out
// topic. If the publish itself fails, the failure is audited but the
// wire is unreachable so we cannot retry.
func (h *Handler) publishError(chargePointID, uniqueID string, code message.ErrorCode, description string, details interface{}, action string) {
	resp, err := h.Codec.BuildError(uniqueID, code, description, details)
	if err != nil {
		log.Printf("[message-processor] build CALLERROR for %s: %v", chargePointID, err)
		return
	}
	raw, err := h.Codec.Encode(resp)
	if err != nil {
		log.Printf("[message-processor] encode CALLERROR for %s: %v", chargePointID, err)
		return
	}
	outTopic := "ocpp/" + chargePointID + "/out"
	if pubErr := h.Broker.Publish(outTopic, raw); pubErr != nil {
		h.Emitter.Emit(AuditEvent{
			Kind:          "published",
			ChargePointID: chargePointID,
			Direction:     "outbound",
			MessageType:   "CALLERROR",
			Action:        action,
			UniqueID:      uniqueID,
			Topic:         outTopic,
			Decision:      "publish_failed",
			Note:          pubErr.Error(),
		})
		return
	}
	decision := "responded"
	if code == message.ErrorCodeNotImplemented {
		decision = "not_implemented"
	}
	h.Emitter.Emit(AuditEvent{
		Kind:          "published",
		ChargePointID: chargePointID,
		Direction:     "outbound",
		MessageType:   "CALLERROR",
		Action:        action,
		UniqueID:      uniqueID,
		Topic:         outTopic,
		Decision:      decision,
		Frame:         string(raw),
	})
}

// validateResponseSchema checks the response payload against the
// official schema for the negotiated version. Only OCPP 1.6J is
// currently implemented; other versions still build/parse but
// schema validation is skipped (the v201 placeholder is the only
// other registered version, and it returns ErrNotImplemented
// before reaching this point).
func (h *Handler) validateResponseSchema(action string, payload json.RawMessage) error {
	if h.Version != "1.6J" {
		return nil
	}
	return ocppschemas.Validate(ocppschemas.V16, action, ocppschemas.Response, payload)
}

// --- core-independent response builders ---

// handleBootNotification returns the OCPP 1.6J BootNotification.conf
// payload (Accepted, currentTime, interval=300). The processor owns
// the time-of-response value to keep wire formatting deterministic.
func (h *Handler) handleBootNotification() (json.RawMessage, error) {
	return json.Marshal(struct {
		Status      string `json:"status"`
		CurrentTime string `json:"currentTime"`
		Interval    int    `json:"interval"`
	}{
		Status:      "Accepted",
		CurrentTime: time.Now().UTC().Format(time.RFC3339),
		Interval:    300,
	})
}

func (h *Handler) handleHeartbeat() (json.RawMessage, error) {
	return json.Marshal(struct {
		CurrentTime string `json:"currentTime"`
	}{
		CurrentTime: time.Now().UTC().Format(time.RFC3339),
	})
}

// handleStatusNotification always returns an empty {} per OCPP 1.6J
// spec (§9.2). Any error in the request payload is reflected as a
// CALLERROR upstream; this builder does not validate the request
// (the codec's parser is permissive and the simulator can send any
// spec-legal status string).
func (h *Handler) handleStatusNotification() (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func (h *Handler) handleMeterValues() (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

// --- core-callback response builders ---

func (h *Handler) handleAuthorize(ctx context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error) {
	if h.Core == nil {
		return nil, ErrCoreUnavailable
	}
	resp, err := h.Core.Authorize(ctx, chargePointID, payload)
	if err != nil {
		return nil, err
	}
	return corePayload(resp)
}

func (h *Handler) handleStartTransaction(ctx context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error) {
	if h.Core == nil {
		return nil, ErrCoreUnavailable
	}
	resp, err := h.Core.StartTransaction(ctx, chargePointID, payload)
	if err != nil {
		return nil, err
	}
	return corePayload(resp)
}

func (h *Handler) handleStopTransaction(ctx context.Context, chargePointID string, payload json.RawMessage) (json.RawMessage, error) {
	if h.Core == nil {
		return nil, ErrCoreUnavailable
	}
	resp, err := h.Core.StopTransaction(ctx, chargePointID, payload)
	if err != nil {
		return nil, err
	}
	return corePayload(resp)
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
