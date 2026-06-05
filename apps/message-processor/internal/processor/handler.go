package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	ocppschemas "github.com/vefacaglar/ocpp-simulator/packages/ocpp-schemas"
)

// Handler is the response-producing logic of the processor. It
// owns the core callback client, the audit emitter, and the
// per-CP version used to pick the right schema for response
// validation. The capability-segregated Protocol interface lives
// in packages/ocpp-protocol/pkg/protocol; the message-processor
// does not need direct access to it (it dispatches on the action
// string and forwards to ocpp-core). All it needs is the broker
// to publish.
type Handler struct {
	Codec   *codec.Codec
	Core    CoreClient
	Broker  Broker
	Emitter AuditEmitter
	Version string // ocpp version string this handler validates responses for (e.g. "1.6J")
}

// NewHandler builds a Handler for the given version. core may be
// nil for development setups that don't yet route business
// decisions to ocpp-core; in that case Authorize/Start/Stop CALLs
// will fail with ErrCoreUnavailable.
func NewHandler(version string, core CoreClient, broker Broker, emitter AuditEmitter) *Handler {
	if emitter == nil {
		emitter = StdoutEmitter{IncludeFrame: false}
	}
	return &Handler{
		Codec:   codec.New(),
		Core:    core,
		Broker:  broker,
		Emitter: emitter,
		Version: version,
	}
}

// Handle is the per-message entry point invoked by the MQTT
// subscriber. It owns the full lifecycle: parse envelope, audit
// "consumed", decide on response (or no-op for inbound
// CALLRESULT/CALLERROR), build, validate, publish, audit
// "published". Any error inside the response path is converted into
// a CALLERROR before publication.
//
// For backward compatibility with the existing test surface (which
// constructs handlers pinned to a single version) this method uses
// h.Version. The per-CP version coming off the MQTT topic is
// preferred when available — see HandleWithVersion.
func (h *Handler) Handle(ctx context.Context, chargePointID string, rawFrame []byte) {
	h.HandleWithVersion(ctx, chargePointID, h.Version, rawFrame)
}

// HandleWithVersion is the routing seam called by the MQTT
// subscriber after parsing the per-CP version out of the topic
// (multi-version plan §3: ocpp/{version}/{cpId}/in). It uses the
// version from the topic — not the handler's pinned Version —
// so the action dispatch and response schema validation pick the
// right set for the actual CP that produced the frame.
func (h *Handler) HandleWithVersion(ctx context.Context, chargePointID, version string, rawFrame []byte) {
	msg, err := h.Codec.Decode(rawFrame)
	if err != nil {
		h.Emitter.Emit(AuditEvent{
			Kind:          "consumed",
			ChargePointID: chargePointID,
			OCPPVersion:   version,
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
		OCPPVersion:   version,
		Direction:     "inbound",
		MessageType:   msgTypeName(msg.MessageTypeID),
		Action:        msg.Action,
		UniqueID:      msg.UniqueID,
		Frame:         string(rawFrame),
	})

	switch msg.MessageTypeID {
	case message.CALL:
		h.respondToCall(ctx, chargePointID, version, msg)
	case message.CALLRESULT, message.CALLERROR:
		// CP→server result/error. The processor never produces a
		// response for these. Audit only.
		h.Emitter.Emit(AuditEvent{
			Kind:          "consumed",
			ChargePointID: chargePointID,
			OCPPVersion:   version,
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
// schema, publishes it to ocpp/{version}/{cpId}/out, and audits the
// publication. The version is read off the topic (HandleWithVersion)
// and is the source of truth for both the action dispatch (1.6J
// StartTransaction vs 2.0.1 TransactionEvent) and the response
// schema validation set.
func (h *Handler) respondToCall(ctx context.Context, chargePointID, version string, msg message.Message) {
	var (
		respPayload json.RawMessage
		err         error
	)

	switch msg.Action {
	// ---- core-independent responses ----
	case "BootNotification":
		respPayload, err = h.handleBootNotification(version)
	case "Heartbeat":
		respPayload, err = h.handleHeartbeat(version)
	case "StatusNotification":
		respPayload, err = h.handleStatusNotification(version)
	case "MeterValues":
		respPayload, err = h.handleMeterValues(version)

	// ---- core-callback responses ----
	case "Authorize":
		respPayload, err = h.handleAuthorize(ctx, chargePointID, version, msg.Payload)
	case "StartTransaction", "StopTransaction":
		// 1.6J-only transaction split. 2.0.1 never sends
		// either of these — it uses TransactionEvent for the
		// unified flow. If a 2.0.1 CP somehow sends one, the
		// action dispatch still handles it (ocpp-core's
		// start/stop endpoints accept a 1.6J-shaped payload and
		// return a 1.6J-shaped response). The response is
		// validated against V16 schema even for 2.0.1 CPs
		// because the spec only defines these action names in
		// 1.6J.
		if msg.Action == "StartTransaction" {
			respPayload, err = h.handleStartTransaction(ctx, chargePointID, version, msg.Payload)
		} else {
			respPayload, err = h.handleStopTransaction(ctx, chargePointID, version, msg.Payload)
		}
	case "TransactionEvent":
		// 2.0.1 unified transaction flow (Started/Updated/Ended).
		respPayload, err = h.handleTransactionEvent(ctx, chargePointID, version, msg.Payload)

	default:
		h.publishError(chargePointID, version, msg.UniqueID, message.ErrorCodeNotImplemented,
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
		h.publishError(chargePointID, version, msg.UniqueID, code, note, nil, msg.Action)
		return
	}

	h.publishResult(chargePointID, version, msg.UniqueID, respPayload, msg.Action)
}

// publishResult encodes, validates, and publishes a CALLRESULT to
// the CP's /out topic.
func (h *Handler) publishResult(chargePointID, version, uniqueID string, payload json.RawMessage, action string) {
	if err := h.validateResponseSchema(version, action, payload); err != nil {
		h.publishError(chargePointID, version, uniqueID, message.ErrorCodeGenericError,
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
	outTopic := "ocpp/" + version + "/" + chargePointID + "/out"
	if pubErr := h.Broker.Publish(outTopic, raw); pubErr != nil {
		h.Emitter.Emit(AuditEvent{
			Kind:          "published",
			ChargePointID: chargePointID,
			OCPPVersion:   version,
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
		OCPPVersion:   version,
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
func (h *Handler) publishError(chargePointID, version, uniqueID string, code message.ErrorCode, description string, details interface{}, action string) {
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
	outTopic := "ocpp/" + version + "/" + chargePointID + "/out"
	if pubErr := h.Broker.Publish(outTopic, raw); pubErr != nil {
		h.Emitter.Emit(AuditEvent{
			Kind:          "published",
			ChargePointID: chargePointID,
			OCPPVersion:   version,
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
		OCPPVersion:   version,
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
// official schema for the negotiated version. The version argument
// (typically read off the MQTT topic segment) selects which schema
// set is consulted: 1.6J uses V16, 2.0.1 uses V201. Unknown
// versions skip validation rather than silently accepting any
// payload.
func (h *Handler) validateResponseSchema(version, action string, payload json.RawMessage) error {
	switch version {
	case "1.6J":
		return ocppschemas.Validate(ocppschemas.V16, action, ocppschemas.Response, payload)
	case "2.0.1":
		return ocppschemas.Validate(ocppschemas.V201, action, ocppschemas.Response, payload)
	default:
		return nil
	}
}

// --- core-independent response builders ----

// handleBootNotification returns the OCPP 1.6J BootNotification.conf
// payload (Accepted, currentTime, interval=300). The processor owns
// the time-of-response value to keep wire formatting deterministic.
func (h *Handler) handleBootNotification(version string) (json.RawMessage, error) {
	if version == "2.0.1" {
		// BootNotificationResponse for 2.0.1: { status, currentTime, interval }.
		// 1.6J shape is identical and also validates against v16.
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

func (h *Handler) handleHeartbeat(version string) (json.RawMessage, error) {
	// HeartbeatResponse is identical across 1.6J and 2.0.1: { currentTime }.
	_ = version
	return json.Marshal(struct {
		CurrentTime string `json:"currentTime"`
	}{
		CurrentTime: time.Now().UTC().Format(time.RFC3339),
	})
}

// handleStatusNotification always returns an empty {} per OCPP
// spec. 1.6J StatusNotification.conf is {}; 2.0.1 is {} as well.
func (h *Handler) handleStatusNotification(version string) (json.RawMessage, error) {
	_ = version
	return json.RawMessage(`{}`), nil
}

func (h *Handler) handleMeterValues(version string) (json.RawMessage, error) {
	_ = version
	return json.RawMessage(`{}`), nil
}

// --- core-callback response builders ----

func (h *Handler) handleAuthorize(ctx context.Context, chargePointID, version string, payload json.RawMessage) (json.RawMessage, error) {
	if h.Core == nil {
		return nil, ErrCoreUnavailable
	}
	resp, err := h.Core.Authorize(ctx, chargePointID, version, payload)
	if err != nil {
		return nil, err
	}
	return corePayload(resp)
}

func (h *Handler) handleStartTransaction(ctx context.Context, chargePointID, version string, payload json.RawMessage) (json.RawMessage, error) {
	if h.Core == nil {
		return nil, ErrCoreUnavailable
	}
	resp, err := h.Core.StartTransaction(ctx, chargePointID, version, payload)
	if err != nil {
		return nil, err
	}
	return corePayload(resp)
}

func (h *Handler) handleStopTransaction(ctx context.Context, chargePointID, version string, payload json.RawMessage) (json.RawMessage, error) {
	if h.Core == nil {
		return nil, ErrCoreUnavailable
	}
	resp, err := h.Core.StopTransaction(ctx, chargePointID, version, payload)
	if err != nil {
		return nil, err
	}
	return corePayload(resp)
}

// handleTransactionEvent forwards a 2.0.1 TransactionEvent.req to
// ocpp-core. The CSMS response shape (per TransactionEvent.conf) is
// { idTokenInfo, ... } — ocpp-core builds it; we forward the bytes
// verbatim. The action name is only meaningful for 2.0.1; the
// dispatch above routes it to this handler only for that version.
func (h *Handler) handleTransactionEvent(ctx context.Context, chargePointID, version string, payload json.RawMessage) (json.RawMessage, error) {
	if h.Core == nil {
		return nil, ErrCoreUnavailable
	}
	resp, err := h.Core.TransactionEvent(ctx, chargePointID, version, payload)
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
