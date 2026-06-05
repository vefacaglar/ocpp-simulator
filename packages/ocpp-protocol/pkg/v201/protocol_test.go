package v201

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
	ocppschemas "github.com/vefacaglar/ocpp-simulator/packages/ocpp-schemas"
)

// All builders MUST produce payloads that validate against the
// official OCPP 2.0.1 FINAL schemas. If a builder produces a
// non-spec payload, this test catches it.

func TestBuildBootNotification_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildBootNotification(context.Background(), protocol.BootNotificationInput{
		ChargePointVendor: "V",
		ChargePointModel:  "M",
		Reason:            "PowerUp",
		FirmwareVersion:   "1.2.3",
	})
	if err != nil {
		t.Fatalf("BuildBootNotification: %v", err)
	}
	if msg.MessageTypeID != message.CALL {
		t.Fatalf("expected CALL, got %d", msg.MessageTypeID)
	}
	if msg.Action != "BootNotification" {
		t.Fatalf("expected BootNotification, got %s", msg.Action)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "BootNotification", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildBootNotification_NoFirmware_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildBootNotification(context.Background(), protocol.BootNotificationInput{
		ChargePointVendor: "V",
		ChargePointModel:  "M",
		Reason:            "PowerUp",
	})
	if err != nil {
		t.Fatalf("BuildBootNotification: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "BootNotification", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildHeartbeat_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildHeartbeat(context.Background())
	if err != nil {
		t.Fatalf("BuildHeartbeat: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "Heartbeat", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildStatusNotification_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildStatusNotification(context.Background(), protocol.StatusNotificationInput{
		EVSEID:      1,
		ConnectorID: 1,
		Status:      "Available",
	})
	if err != nil {
		t.Fatalf("BuildStatusNotification: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "StatusNotification", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildStatusNotification_AllConnectorStatuses(t *testing.T) {
	// Every ConnectorStatusEnum value the spec defines must be
	// emitted cleanly.
	for _, status := range []string{"Available", "Occupied", "Reserved", "Unavailable", "Faulted"} {
		t.Run(status, func(t *testing.T) {
			p := NewProtocol()
			msg, err := p.BuildStatusNotification(context.Background(), protocol.StatusNotificationInput{
				EVSEID: 1, ConnectorID: 1, Status: status,
			})
			if err != nil {
				t.Fatalf("BuildStatusNotification: %v", err)
			}
			if err := ocppschemas.Validate(ocppschemas.V201, "StatusNotification", ocppschemas.Request, msg.Payload); err != nil {
				t.Fatalf("schema validation failed for status=%s: %v", status, err)
			}
		})
	}
}

func TestBuildAuthorize_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildAuthorize(context.Background(), protocol.AuthorizeInput{
		IDToken: &protocol.IDToken{IDToken: "ABC12345", Type: "ISO14443"},
	})
	if err != nil {
		t.Fatalf("BuildAuthorize: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "Authorize", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildAuthorize_FallbackFromIDTag(t *testing.T) {
	// When the caller passes only IDTag (1.6J-shaped), v201
	// must wrap it in idToken{...} with a default type so
	// the payload still validates.
	p := NewProtocol()
	msg, err := p.BuildAuthorize(context.Background(), protocol.AuthorizeInput{
		IDTag: "ABC12345",
	})
	if err != nil {
		t.Fatalf("BuildAuthorize: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "Authorize", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildAuthorize_AllTokenTypes(t *testing.T) {
	for _, tokType := range []string{
		"Central", "eMAID", "ISO14443", "ISO15693", "KeyCode", "Local", "MacAddress", "NoAuthorization",
	} {
		t.Run(tokType, func(t *testing.T) {
			p := NewProtocol()
			msg, err := p.BuildAuthorize(context.Background(), protocol.AuthorizeInput{
				IDToken: &protocol.IDToken{IDToken: "X", Type: tokType},
			})
			if err != nil {
				t.Fatalf("BuildAuthorize: %v", err)
			}
			if err := ocppschemas.Validate(ocppschemas.V201, "Authorize", ocppschemas.Request, msg.Payload); err != nil {
				t.Fatalf("schema validation failed: %v", err)
			}
		})
	}
}

func TestBuildTransactionEvent_Started_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildTransactionEvent(context.Background(), protocol.TransactionEventInput{
		EventType:     "Started",
		Timestamp:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		TriggerReason: "CablePluggedIn",
		SeqNo:         0,
		TransactionID: "tx-1",
		EVSEID:        1,
		ConnectorID:   1,
		IDToken:       &protocol.IDToken{IDToken: "T1", Type: "ISO14443"},
	})
	if err != nil {
		t.Fatalf("BuildTransactionEvent: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "TransactionEvent", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildTransactionEvent_UpdatedWithMeter_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildTransactionEvent(context.Background(), protocol.TransactionEventInput{
		EventType:     "Updated",
		Timestamp:     time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC),
		TriggerReason: "MeterValuePeriodic",
		SeqNo:         1,
		TransactionID: "tx-1",
		EVSEID:        1,
		ConnectorID:   1,
		MeterValue: []protocol.MeterValue{{
			Timestamp: time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC),
			SampledValue: []protocol.SampledValue{
				{Value: "1234.5", Measurand: "Energy.Active.Import.Register", Unit: "Wh"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("BuildTransactionEvent: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "TransactionEvent", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildTransactionEvent_Ended_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildTransactionEvent(context.Background(), protocol.TransactionEventInput{
		EventType:     "Ended",
		Timestamp:     time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC),
		TriggerReason: "EVDeparted",
		SeqNo:         2,
		TransactionID: "tx-1",
		EVSEID:        1,
		ConnectorID:   1,
		StoppedReason: "EVDisconnected",
	})
	if err != nil {
		t.Fatalf("BuildTransactionEvent: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "TransactionEvent", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildTransactionEvent_MissingTransactionID_Rejected(t *testing.T) {
	// transactionInfo.transactionId is REQUIRED.
	p := NewProtocol()
	_, err := p.BuildTransactionEvent(context.Background(), protocol.TransactionEventInput{
		EventType:     "Started",
		TriggerReason: "CablePluggedIn",
		SeqNo:         0,
	})
	if err == nil {
		t.Error("expected error for missing transactionId")
	}
}

func TestBuildTransactionEvent_MissingEventType_Rejected(t *testing.T) {
	p := NewProtocol()
	_, err := p.BuildTransactionEvent(context.Background(), protocol.TransactionEventInput{
		TransactionID: "tx-1",
		TriggerReason: "CablePluggedIn",
		SeqNo:         0,
	})
	if err == nil {
		t.Error("expected error for missing eventType")
	}
}

func TestBuildTransactionEvent_MissingTriggerReason_Rejected(t *testing.T) {
	p := NewProtocol()
	_, err := p.BuildTransactionEvent(context.Background(), protocol.TransactionEventInput{
		TransactionID: "tx-1",
		EventType:     "Started",
		SeqNo:         0,
	})
	if err == nil {
		t.Error("expected error for missing triggerReason")
	}
}

func TestBuildTransactionEvent_BadEventType_Rejected(t *testing.T) {
	// "Started"/"Updated"/"Ended" are the only valid eventType
	// values. The codec forwards the value; the schema validator
	// catches it.
	p := NewProtocol()
	msg, err := p.BuildTransactionEvent(context.Background(), protocol.TransactionEventInput{
		EventType:     "Cancelled",
		TriggerReason: "CablePluggedIn",
		TransactionID: "tx-1",
		SeqNo:         0,
	})
	if err != nil {
		t.Fatalf("BuildTransactionEvent: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "TransactionEvent", ocppschemas.Request, msg.Payload); err == nil {
		t.Error("expected schema validation error for invalid eventType enum")
	}
}

func TestBuildMeterValues_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildMeterValues(context.Background(), protocol.MeterValuesInput{
		EVSEID: 1,
		MeterValues: []protocol.MeterValue{{
			Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			SampledValue: []protocol.SampledValue{
				{Value: "100", Measurand: "Energy.Active.Import.Register", Unit: "Wh"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("BuildMeterValues: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "MeterValues", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

// --- RemoteControlProtocol (2.0.1-shaped parse) ----

func TestParseResetRequest_Immediate(t *testing.T) {
	p := NewProtocol()
	req, err := p.ParseResetRequest(json.RawMessage(`{"type":"Immediate","evseId":1}`))
	if err != nil {
		t.Fatalf("ParseResetRequest: %v", err)
	}
	if req.Type != "Immediate" {
		t.Errorf("Type = %q, want Immediate", req.Type)
	}
}

func TestParseResetRequest_OnIdle(t *testing.T) {
	p := NewProtocol()
	req, err := p.ParseResetRequest(json.RawMessage(`{"type":"OnIdle"}`))
	if err != nil {
		t.Fatalf("ParseResetRequest: %v", err)
	}
	if req.Type != "OnIdle" {
		t.Errorf("Type = %q, want OnIdle", req.Type)
	}
}

func TestParseResetRequest_BadType(t *testing.T) {
	// 2.0.1 must NOT accept the 1.6J "Soft"/"Hard" enum.
	p := NewProtocol()
	if _, err := p.ParseResetRequest(json.RawMessage(`{"type":"Soft"}`)); err == nil {
		t.Error("expected error: v201 does not accept Soft reset type")
	}
}

func TestParseUnlockConnectorRequest_SchemaShape(t *testing.T) {
	p := NewProtocol()
	req, err := p.ParseUnlockConnectorRequest(json.RawMessage(`{"evseId":2,"connectorId":1}`))
	if err != nil {
		t.Fatalf("ParseUnlockConnectorRequest: %v", err)
	}
	if req.ConnectorID != 1 {
		t.Errorf("ConnectorID = %d, want 1", req.ConnectorID)
	}
}

func TestParseUnlockConnectorRequest_RejectsZero(t *testing.T) {
	p := NewProtocol()
	if _, err := p.ParseUnlockConnectorRequest(json.RawMessage(`{"evseId":0,"connectorId":0}`)); err == nil {
		t.Error("expected error for zero evseId/connectorId")
	}
}

func TestParseTriggerMessageRequest_TransactionEvent(t *testing.T) {
	// 2.0.1's requestedMessage enum includes "TransactionEvent".
	p := NewProtocol()
	req, err := p.ParseTriggerMessageRequest(json.RawMessage(`{"requestedMessage":"TransactionEvent","evse":{"id":1}}`))
	if err != nil {
		t.Fatalf("ParseTriggerMessageRequest: %v", err)
	}
	if req.RequestedMessage != "TransactionEvent" {
		t.Errorf("RequestedMessage = %q", req.RequestedMessage)
	}
	if req.ConnectorID == nil || *req.ConnectorID != 1 {
		t.Errorf("ConnectorID = %v, want pointer to 1", req.ConnectorID)
	}
}

func TestParseChangeAvailabilityRequest_Operative(t *testing.T) {
	p := NewProtocol()
	req, err := p.ParseChangeAvailabilityRequest(json.RawMessage(`{"operationalStatus":"Operative","evse":{"id":1}}`))
	if err != nil {
		t.Fatalf("ParseChangeAvailabilityRequest: %v", err)
	}
	if req.Type != "Operative" {
		t.Errorf("Type = %q, want Operative", req.Type)
	}
	if req.ConnectorID != 1 {
		t.Errorf("ConnectorID = %d, want 1", req.ConnectorID)
	}
}

// --- RemoteTxProtocol ----

func TestParseRequestStartTransaction(t *testing.T) {
	p := NewProtocol()
	req, err := p.ParseRequestStartTransactionRequest(json.RawMessage(`{"idToken":{"idToken":"X","type":"ISO14443"},"remoteStartId":42}`))
	if err != nil {
		t.Fatalf("ParseRequestStartTransactionRequest: %v", err)
	}
	if req.IDToken.IDToken != "X" {
		t.Errorf("idToken = %q", req.IDToken.IDToken)
	}
	if req.RemoteStartID != 42 {
		t.Errorf("remoteStartId = %d, want 42", req.RemoteStartID)
	}
}

func TestParseRequestStartTransaction_MissingIDToken(t *testing.T) {
	p := NewProtocol()
	_, err := p.ParseRequestStartTransactionRequest(json.RawMessage(`{"remoteStartId":42}`))
	if err == nil {
		t.Error("expected error for missing idToken")
	}
}

func TestParseRequestStopTransaction(t *testing.T) {
	p := NewProtocol()
	req, err := p.ParseRequestStopTransactionRequest(json.RawMessage(`{"transactionId":"tx-1"}`))
	if err != nil {
		t.Fatalf("ParseRequestStopTransactionRequest: %v", err)
	}
	if req.TransactionID != "tx-1" {
		t.Errorf("transactionId = %q", req.TransactionID)
	}
}

func TestParseRequestStopTransaction_MissingID(t *testing.T) {
	p := NewProtocol()
	_, err := p.ParseRequestStopTransactionRequest(json.RawMessage(`{}`))
	if err == nil {
		t.Error("expected error for missing transactionId")
	}
}

// --- Response builders ----

func TestBuildGetVariablesResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	resp, err := p.BuildGetVariablesResponse([]protocol.GetVariablesResultItem{{
		AttributeStatus: "Accepted",
		Component:       protocol.GetVariableComponent{Name: "ChargingStation"},
		Variable:        protocol.Variable{Name: "Model"},
	}})
	if err != nil {
		t.Fatalf("BuildGetVariablesResponse: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "GetVariables", ocppschemas.Response, resp); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildSetVariablesResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	resp, err := p.BuildSetVariablesResponse([]protocol.SetVariablesResultItem{{
		AttributeStatus: "Accepted",
		Component:       protocol.GetVariableComponent{Name: "ChargingStation"},
		Variable:        protocol.Variable{Name: "HeartbeatInterval"},
	}})
	if err != nil {
		t.Fatalf("BuildSetVariablesResponse: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "SetVariables", ocppschemas.Response, resp); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildResetResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	resp, err := p.BuildResetResponse("Accepted")
	if err != nil {
		t.Fatalf("BuildResetResponse: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "Reset", ocppschemas.Response, resp); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildRequestStartTransactionResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	resp, err := p.BuildRequestStartTransactionResponse("Accepted")
	if err != nil {
		t.Fatalf("BuildRequestStartTransactionResponse: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "RequestStartTransaction", ocppschemas.Response, resp); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

// --- Golden fixture: 2.0.1 transaction lifecycle in one test ---

// TestGoldenTransactionLifecycle encodes a Started → Updated →
// Ended sequence and validates each wire frame against the
// official schema. The exact bytes are not asserted (Go map
// iteration order is non-deterministic and the JSON encoder is
// allowed to reorder fields); the schema compliance is.
func TestGoldenTransactionLifecycle(t *testing.T) {
	p := NewProtocol()
	txID := "9c1f8e2a-bb3d-4a4e-bf7a-3a2b3b9c8d11"

	started, err := p.BuildTransactionEvent(context.Background(), protocol.TransactionEventInput{
		EventType:     "Started",
		Timestamp:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		TriggerReason: "CablePluggedIn",
		SeqNo:         0,
		TransactionID: txID,
		EVSEID:        1,
		ConnectorID:   1,
		IDToken:       &protocol.IDToken{IDToken: "T1", Type: "ISO14443"},
	})
	if err != nil {
		t.Fatalf("Started: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "TransactionEvent", ocppschemas.Request, started.Payload); err != nil {
		t.Fatalf("Started schema: %v", err)
	}

	updated, err := p.BuildTransactionEvent(context.Background(), protocol.TransactionEventInput{
		EventType:     "Updated",
		Timestamp:     time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC),
		TriggerReason: "MeterValuePeriodic",
		SeqNo:         1,
		TransactionID: txID,
		EVSEID:        1,
		ConnectorID:   1,
		MeterValue: []protocol.MeterValue{{
			Timestamp: time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC),
			SampledValue: []protocol.SampledValue{
				{Value: "500", Measurand: "Energy.Active.Import.Register", Unit: "Wh"},
				{Value: "11000", Measurand: "Power.Active.Import", Unit: "W"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("Updated: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "TransactionEvent", ocppschemas.Request, updated.Payload); err != nil {
		t.Fatalf("Updated schema: %v", err)
	}

	ended, err := p.BuildTransactionEvent(context.Background(), protocol.TransactionEventInput{
		EventType:     "Ended",
		Timestamp:     time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC),
		TriggerReason: "EVDeparted",
		SeqNo:         2,
		TransactionID: txID,
		EVSEID:        1,
		ConnectorID:   1,
		StoppedReason: "EVDisconnected",
	})
	if err != nil {
		t.Fatalf("Ended: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V201, "TransactionEvent", ocppschemas.Request, ended.Payload); err != nil {
		t.Fatalf("Ended schema: %v", err)
	}

	// Each frame must be a CALL with the right action.
	for _, m := range []message.Message{started, updated, ended} {
		if m.MessageTypeID != message.CALL {
			t.Errorf("expected CALL, got %d", m.MessageTypeID)
		}
		if m.Action != "TransactionEvent" {
			t.Errorf("expected TransactionEvent, got %s", m.Action)
		}
		if m.UniqueID == "" {
			t.Error("UniqueID is empty")
		}
	}
}

// keep the strings import live
var _ = strings.TrimSpace
