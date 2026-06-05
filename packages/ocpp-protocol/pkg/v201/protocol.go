// Package v201 is the OCPP 2.0.1 protocol implementation. It is
// the only place that knows 2.0.1-specific wire field names,
// casing, enums, and the string transactionId / TransactionEvent
// flow. It produces raw [2, uid, action, payload] CALL frames via
// the codec and accepts matching CALL/CALLRESULT/CALLERROR frames
// back. v201 implements the 2.0.1-only capability set:
// BaseProtocol (Boot/Heartbeat/Status/Authorize/MeterValues with
// 2.0.1 fields), TransactionEventProtocol (the unified
// transaction flow with explicit SeqNo), VariableProtocol
// (Get/SetVariables on the device model), RemoteControlProtocol
// (Reset/UnlockConnector/TriggerMessage/ChangeAvailability), and
// RemoteTxProtocol (RequestStart/StopTransaction). v201 does NOT
// implement the legacy 1.6J capability interfaces — there is no
// StartTransaction/StopTransaction, no Get/ChangeConfiguration,
// no RemoteStart/StopTransaction. See plan.md §2.
package v201

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
)

type Protocol struct {
	codec *codec.Codec
}

func NewProtocol() *Protocol {
	return &Protocol{codec: codec.New()}
}

func (p *Protocol) Version() string {
	return protocol.Version2_0_1
}

// numericSampledValue is the 2.0.1-shaped SampledValue. The
// cross-version input struct carries Value as a string (matches
// 1.6J where it is a string) and Unit as a string (1.6J's flat
// "unit" field). 2.0.1 requires value to be a JSON number AND
// uses unitOfMeasure {unit, multiplier} instead of a flat
// "unit". A malformed non-numeric value is an error — the
// schema would reject the payload anyway.
type unitOfMeasure struct {
	Unit      string `json:"unit,omitempty"`
	Multiplier int   `json:"multiplier,omitempty"`
}

type numericSampledValue struct {
	Value         float64       `json:"value"`
	Measurand     string        `json:"measurand,omitempty"`
	UnitOfMeasure *unitOfMeasure `json:"unitOfMeasure,omitempty"`
}

type numericMeterValue struct {
	Timestamp    string                `json:"timestamp"`
	SampledValue []numericSampledValue `json:"sampledValue"`
}

func convertMeterValues(in []protocol.MeterValue) ([]numericMeterValue, error) {
	out := make([]numericMeterValue, 0, len(in))
	for _, m := range in {
		svs := make([]numericSampledValue, len(m.SampledValue))
		for j, s := range m.SampledValue {
			v, err := strconv.ParseFloat(s.Value, 64)
			if err != nil {
				return nil, fmt.Errorf("meter value %q is not numeric: %w", s.Value, err)
			}
			sv := numericSampledValue{Value: v, Measurand: s.Measurand}
			if s.Unit != "" {
				sv.UnitOfMeasure = &unitOfMeasure{Unit: s.Unit}
			}
			svs[j] = sv
		}
		out = append(out, numericMeterValue{
			Timestamp:    m.Timestamp.Format(time.RFC3339),
			SampledValue: svs,
		})
	}
	return out, nil
}

// Compile-time assertions: v201 implements only the 2.0.1
// capability set. v16 implements the legacy ones; v201 must NOT.
var (
	_ protocol.BaseProtocol           = (*Protocol)(nil)
	_ protocol.TransactionEventProtocol = (*Protocol)(nil)
	_ protocol.VariableProtocol       = (*Protocol)(nil)
	_ protocol.RemoteControlProtocol  = (*Protocol)(nil)
	_ protocol.RemoteTxProtocol       = (*Protocol)(nil)
)

// --- BaseProtocol: Boot, Heartbeat, Status, Authorize, MeterValues ---

// BuildBootNotification encodes a 2.0.1 BootNotification.req. The
// 1.6J flat chargePointVendor/chargePointModel fields are wrapped
// in a chargingStation object; a reason enum is required.
func (p *Protocol) BuildBootNotification(ctx context.Context, in protocol.BootNotificationInput) (message.Message, error) {
	station := struct {
		Model          string `json:"model"`
		VendorName     string `json:"vendorName"`
		FirmwareVersion string `json:"firmwareVersion,omitempty"`
	}{
		Model:          in.ChargePointModel,
		VendorName:     in.ChargePointVendor,
		FirmwareVersion: in.FirmwareVersion,
	}
	reason := in.Reason
	if reason == "" {
		reason = "PowerUp"
	}
	payload := struct {
		ChargingStation interface{} `json:"chargingStation"`
		Reason          string       `json:"reason"`
	}{ChargingStation: station, Reason: reason}
	return p.codec.BuildCall(message.GenerateUniqueID(), "BootNotification", payload)
}

func (p *Protocol) BuildHeartbeat(ctx context.Context) (message.Message, error) {
	// 2.0.1 Heartbeat.req is an empty object {}.
	return p.codec.BuildCall(message.GenerateUniqueID(), "Heartbeat", struct{}{})
}

// BuildStatusNotification encodes a 2.0.1 StatusNotification.req.
// The 1.6J errorCode field does not exist; the connector is
// identified by evseId + connectorId. timestamp is required and
// the codec always writes the current time.
func (p *Protocol) BuildStatusNotification(ctx context.Context, in protocol.StatusNotificationInput) (message.Message, error) {
	evseID := in.EVSEID
	if evseID == 0 {
		evseID = 1
	}
	connectorID := in.ConnectorID
	if connectorID == 0 {
		connectorID = 1
	}
	payload := struct {
		Timestamp       string `json:"timestamp"`
		ConnectorStatus string `json:"connectorStatus"`
		EVSEID          int    `json:"evseId"`
		ConnectorID     int    `json:"connectorId"`
	}{
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		ConnectorStatus: in.Status,
		EVSEID:          evseID,
		ConnectorID:     connectorID,
	}
	return p.codec.BuildCall(message.GenerateUniqueID(), "StatusNotification", payload)
}

// BuildAuthorize encodes a 2.0.1 Authorize.req. The flat idTag
// field is replaced by an idToken object {idToken, type}.
func (p *Protocol) BuildAuthorize(ctx context.Context, in protocol.AuthorizeInput) (message.Message, error) {
	if in.IDToken == nil {
		// Fallback: synthesize a 2.0.1 idToken from the 1.6J-shaped
		// IDTag. v201 always wraps in idToken; callers that want
		// to control the type can pass IDToken directly.
		in.IDToken = &protocol.IDToken{IDToken: in.IDTag, Type: "ISO14443"}
	}
	payload := struct {
		IDToken *protocol.IDToken `json:"idToken"`
	}{IDToken: in.IDToken}
	return p.codec.BuildCall(message.GenerateUniqueID(), "Authorize", payload)
}

// BuildMeterValues encodes a 2.0.1 MeterValues.req. The
// 1.6J connectorId + transactionId pair is replaced by evseId;
// transaction context lives on TransactionEvent, not here.
func (p *Protocol) BuildMeterValues(ctx context.Context, in protocol.MeterValuesInput) (message.Message, error) {
	evseID := in.EVSEID
	if evseID == 0 {
		evseID = in.ConnectorID
		if evseID == 0 {
			evseID = 1
		}
	}
	mvs, err := convertMeterValues(in.MeterValues)
	if err != nil {
		return message.Message{}, fmt.Errorf("BuildMeterValues: %w", err)
	}
	payload := struct {
		EVSEID     int               `json:"evseId"`
		MeterValue []numericMeterValue `json:"meterValue"`
	}{
		EVSEID:     evseID,
		MeterValue: mvs,
	}
	return p.codec.BuildCall(message.GenerateUniqueID(), "MeterValues", payload)
}

// --- TransactionEventProtocol: the unified transaction flow -----

// BuildTransactionEvent encodes a 2.0.1 TransactionEvent.req.
// SeqNo is REQUIRED by the spec (a per-transaction counter
// starting at 0) and the codec refuses to omit it. The caller
// (simulator runtime) is the source of truth for SeqNo and must
// maintain it per transaction.
func (p *Protocol) BuildTransactionEvent(ctx context.Context, in protocol.TransactionEventInput) (message.Message, error) {
	if in.TransactionID == "" {
		return message.Message{}, fmt.Errorf("TransactionEvent: transactionInfo.transactionId is required")
	}
	if in.EventType == "" {
		return message.Message{}, fmt.Errorf("TransactionEvent: eventType is required")
	}
	if in.TriggerReason == "" {
		return message.Message{}, fmt.Errorf("TransactionEvent: triggerReason is required")
	}
	if in.Timestamp.IsZero() {
		in.Timestamp = time.Now().UTC()
	}

	txInfo := struct {
		TransactionID string `json:"transactionId"`
		ChargingState string `json:"chargingState,omitempty"`
		StoppedReason string `json:"stoppedReason,omitempty"`
		RemoteStartID *int   `json:"remoteStartId,omitempty"`
	}{
		TransactionID: in.TransactionID,
	}
	// The TransactionInfo object in the spec is
	// `transactionInfo` (the property on the request, not a
	// `type` wrapper). ChargingState and stoppedReason live on
	// that object directly. Use 1.6J-style union by populating
	// fields the spec's TransactionType declares as optional.
	// (Per OCPP 2.0.1 FINAL, the schema uses a TransactionType
	// with optional `chargingState` and `stoppedReason`. The
	// chargingState values mirror 1.6J: Charging/EVConnected/
	// SuspendedEV/SuspendedEVSE/Idle.)
	if in.EventType == "Updated" {
		// chargingState defaults to omitted unless caller
		// indicates it; we leave it as zero-value to avoid
		// emitting an empty string. Callers that need it should
		// set the ChargingState field in a future input
		// extension; for MVP, omit.
	} else if in.EventType == "Ended" && in.StoppedReason != "" {
		txInfo.StoppedReason = in.StoppedReason
	}

	// Encode MeterValue items: 2.0.1 requires numeric values
	// (the spec's SampledValueType declares value: { type: number }).
	// The cross-version input struct carries Value as a string
	// (matches 1.6J); we parse it here and emit a float. A
	// non-numeric value is a programming error and returns
	// immediately rather than emitting a payload the schema
	// would reject.
	mvs, err := convertMeterValues(in.MeterValue)
	if err != nil {
		return message.Message{}, fmt.Errorf("BuildTransactionEvent: %w", err)
	}

	evseID := in.EVSEID
	if evseID == 0 {
		evseID = in.ConnectorID
		if evseID == 0 {
			evseID = 1
		}
	}
	connectorID := in.ConnectorID
	if connectorID == 0 {
		connectorID = 1
	}
	evse := struct {
		ID         int  `json:"id"`
		ConnectorID *int `json:"connectorId,omitempty"`
	}{ID: evseID}
	// Omit connectorId when 0 — the spec marks it optional and
	// the schema rejects additionalProperties=0.
	evse.ConnectorID = &connectorID

	payload := struct {
		EventType       string                 `json:"eventType"`
		Timestamp       string                 `json:"timestamp"`
		TriggerReason   string                 `json:"triggerReason"`
		SeqNo           int                    `json:"seqNo"`
		Offline         bool                   `json:"offline,omitempty"`
		TransactionInfo interface{}            `json:"transactionInfo"`
		EVSE            interface{}            `json:"evse,omitempty"`
		IDToken         *protocol.IDToken      `json:"idToken,omitempty"`
		MeterValue      []numericMeterValue    `json:"meterValue,omitempty"`
	}{
		EventType:       in.EventType,
		Timestamp:       in.Timestamp.Format(time.RFC3339),
		TriggerReason:   in.TriggerReason,
		SeqNo:           in.SeqNo,
		Offline:         in.Offline,
		TransactionInfo: txInfo,
		EVSE:            evse,
		IDToken:         in.IDToken,
		MeterValue:      mvs,
	}
	return p.codec.BuildCall(message.GenerateUniqueID(), "TransactionEvent", payload)
}

// --- VariableProtocol: Get/SetVariables on the device model ----

func (p *Protocol) ParseGetVariablesRequest(payload json.RawMessage) (*protocol.GetVariablesRequest, error) {
	var req protocol.GetVariablesRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse GetVariables.req: %w", err)
	}
	if len(req.GetVariableDescriptor) == 0 {
		return nil, fmt.Errorf("GetVariables: getVariableData must be non-empty")
	}
	return &req, nil
}

func (p *Protocol) ParseSetVariablesRequest(payload json.RawMessage) (*protocol.SetVariablesRequest, error) {
	var req protocol.SetVariablesRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse SetVariables.req: %w", err)
	}
	if len(req.SetVariableData) == 0 {
		return nil, fmt.Errorf("SetVariables: setVariableData must be non-empty")
	}
	return &req, nil
}

// BuildGetVariablesResponse encodes a 2.0.1 GetVariables.conf
// payload. The schema requires getVariableResult to be an array
// of {attributeStatus, component, variable}; the spec calls the
// per-item attributeStatus an enum (Accepted, Rejected, ...).
func (p *Protocol) BuildGetVariablesResponse(items []protocol.GetVariablesResultItem) (json.RawMessage, error) {
	resp := struct {
		GetVariableResult []protocol.GetVariablesResultItem `json:"getVariableResult"`
	}{GetVariableResult: items}
	return json.Marshal(resp)
}

func (p *Protocol) BuildSetVariablesResponse(items []protocol.SetVariablesResultItem) (json.RawMessage, error) {
	resp := struct {
		SetVariableResult []protocol.SetVariablesResultItem `json:"setVariableResult"`
	}{SetVariableResult: items}
	return json.Marshal(resp)
}

// --- RemoteControlProtocol (shared shape with 1.6J) ---------

// 2.0.1 Reset.type enum is {Immediate, OnIdle} — NOT the 1.6J
// Hard/Soft. v201 does not validate; it forwards the value the
// caller set. The codec does not enforce this either; the
// validator is the source of truth.
func (p *Protocol) ParseResetRequest(payload json.RawMessage) (*protocol.ResetRequest, error) {
	var req protocol.ResetRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse Reset.req: %w", err)
	}
	if req.Type != "Immediate" && req.Type != "OnIdle" {
		return nil, fmt.Errorf("Reset: invalid type %q (want Immediate|OnIdle)", req.Type)
	}
	return &req, nil
}

// 2.0.1 UnlockConnector.req is {evseId, connectorId} (no
// implicit station-wide unlock for MVP).
func (p *Protocol) ParseUnlockConnectorRequest(payload json.RawMessage) (*protocol.UnlockConnectorRequest, error) {
	// We use a local struct because 2.0.1's UnlockConnector
	// doesn't fit the 1.6J-shaped {connectorId} request. The
	// 1.6J-shaped struct is still useful for the response
	// builder; the parse path decodes the 2.0.1 shape on top.
	var raw struct {
		EVSEID      int `json:"evseId"`
		ConnectorID int `json:"connectorId"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse UnlockConnector.req: %w", err)
	}
	if raw.EVSEID <= 0 || raw.ConnectorID <= 0 {
		return nil, fmt.Errorf("UnlockConnector: evseId and connectorId must be > 0")
	}
	// Re-map into the 1.6J-shaped struct used by the response
	// builder path (ConnectorID is the only field the response
	// builder uses).
	return &protocol.UnlockConnectorRequest{ConnectorID: raw.ConnectorID}, nil
}

// 2.0.1 TriggerMessage.req is {evse {id}, requestedMessage};
// requestedMessage enum includes TransactionEvent. 1.6J's
// flat connectorId is gone.
func (p *Protocol) ParseTriggerMessageRequest(payload json.RawMessage) (*protocol.TriggerMessageRequest, error) {
	var raw struct {
		EVSE struct {
			ID int `json:"id"`
		} `json:"evse"`
		RequestedMessage string `json:"requestedMessage"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse TriggerMessage.req: %w", err)
	}
	if raw.RequestedMessage == "" {
		return nil, fmt.Errorf("TriggerMessage: requestedMessage is required")
	}
	// Re-map for response builder: 1.6J-shaped struct carries an
	// optional connectorId; v201's evse.id is the equivalent
	// hint.
	out := &protocol.TriggerMessageRequest{RequestedMessage: raw.RequestedMessage}
	if raw.EVSE.ID > 0 {
		cid := raw.EVSE.ID
		out.ConnectorID = &cid
	}
	return out, nil
}

// 2.0.1 ChangeAvailability.req is {evse {id}, operationalStatus};
// the 1.6J pair {connectorId, type} is gone (operationalStatus
// replaces type; evse replaces the station-wide 0 hack).
func (p *Protocol) ParseChangeAvailabilityRequest(payload json.RawMessage) (*protocol.ChangeAvailabilityRequest, error) {
	var raw struct {
		EVSE struct {
			ID int `json:"id"`
		} `json:"evse"`
		OperationalStatus string `json:"operationalStatus"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse ChangeAvailability.req: %w", err)
	}
	if raw.OperationalStatus != "Operative" && raw.OperationalStatus != "Inoperative" {
		return nil, fmt.Errorf("ChangeAvailability: invalid operationalStatus %q", raw.OperationalStatus)
	}
	// 1.6J-shaped struct uses {connectorId, type}. v201's
	// station-wide request uses evse.id=0 (but the v201 schema
	// says evse.id is required). For MVP we route the parsed
	// 1.6J-shaped struct: connectorId=evse.id (≥1), type carries
	// the operational status verbatim.
	return &protocol.ChangeAvailabilityRequest{ConnectorID: raw.EVSE.ID, Type: raw.OperationalStatus}, nil
}

func (p *Protocol) BuildResetResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}

func (p *Protocol) BuildUnlockConnectorResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}

func (p *Protocol) BuildTriggerMessageResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}

func (p *Protocol) BuildChangeAvailabilityResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}

// --- RemoteTxProtocol: 2.0.1 RequestStart/StopTransaction ---

func (p *Protocol) ParseRequestStartTransactionRequest(payload json.RawMessage) (*protocol.RequestStartTransactionRequest, error) {
	var raw struct {
		EVSEID        *int `json:"evseId,omitempty"`
		IDToken       protocol.IDToken `json:"idToken"`
		RemoteStartID int  `json:"remoteStartId"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse RequestStartTransaction.req: %w", err)
	}
	if raw.IDToken.IDToken == "" || raw.IDToken.Type == "" {
		return nil, fmt.Errorf("RequestStartTransaction: idToken.idToken and idToken.type are required")
	}
	return &protocol.RequestStartTransactionRequest{
		EVSEID:        raw.EVSEID,
		IDToken:       raw.IDToken,
		RemoteStartID: raw.RemoteStartID,
	}, nil
}

func (p *Protocol) ParseRequestStopTransactionRequest(payload json.RawMessage) (*protocol.RequestStopTransactionRequest, error) {
	var req protocol.RequestStopTransactionRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("parse RequestStopTransaction.req: %w", err)
	}
	if req.TransactionID == "" {
		return nil, fmt.Errorf("RequestStopTransaction: transactionId is required")
	}
	return &req, nil
}

func (p *Protocol) BuildRequestStartTransactionResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}

func (p *Protocol) BuildRequestStopTransactionResponse(status string) (json.RawMessage, error) {
	resp := struct {
		Status string `json:"status"`
	}{Status: status}
	return json.Marshal(resp)
}
