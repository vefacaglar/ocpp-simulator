package v16

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
	ocppschemas "github.com/vefacaglar/ocpp-simulator/packages/ocpp-schemas"
)

func TestBuildBootNotification_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildBootNotification(context.Background(), protocol.BootNotificationInput{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
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
	if msg.UniqueID == "" {
		t.Fatal("uniqueID is empty")
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "BootNotification", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildHeartbeat_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildHeartbeat(context.Background())
	if err != nil {
		t.Fatalf("BuildHeartbeat: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "Heartbeat", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildStatusNotification_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildStatusNotification(context.Background(), protocol.StatusNotificationInput{
		ConnectorID: 1,
		Status:      "Available",
		ErrorCode:   "NoError",
	})
	if err != nil {
		t.Fatalf("BuildStatusNotification: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "StatusNotification", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildAuthorize_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildAuthorize(context.Background(), protocol.AuthorizeInput{IDTag: "ABCDEF12"})
	if err != nil {
		t.Fatalf("BuildAuthorize: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "Authorize", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildStartTransaction_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildStartTransaction(context.Background(), protocol.StartTransactionInput{
		ConnectorID: 1,
		IDTag:       "ABCDEF12",
		MeterStart:  0,
		Timestamp:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildStartTransaction: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "StartTransaction", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildMeterValues_SchemaValid(t *testing.T) {
	p := NewProtocol()
	txID := 42
	msg, err := p.BuildMeterValues(context.Background(), protocol.MeterValuesInput{
		ConnectorID:   1,
		TransactionID: &txID,
		MeterValues: []protocol.MeterValue{{
			Timestamp: time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC),
			SampledValue: []protocol.SampledValue{
				{Value: "1234", Measurand: "Energy.Active.Import.Register", Unit: "Wh"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("BuildMeterValues: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "MeterValues", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildStopTransaction_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildStopTransaction(context.Background(), protocol.StopTransactionInput{
		TransactionID: 42,
		IDTag:         "ABCDEF12",
		MeterStop:     5000,
		Timestamp:     time.Date(2025, 1, 1, 1, 0, 0, 0, time.UTC),
		Reason:        "Local",
	})
	if err != nil {
		t.Fatalf("BuildStopTransaction: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "StopTransaction", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestEncodeDecode_CALL(t *testing.T) {
	c := codec.New()
	msg := message.Message{
		MessageTypeID: message.CALL,
		UniqueID:      "test-123",
		Action:        "Heartbeat",
		Payload:       json.RawMessage(`{}`),
	}
	raw, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := c.Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.MessageTypeID != msg.MessageTypeID {
		t.Errorf("MessageTypeID mismatch: %d != %d", decoded.MessageTypeID, msg.MessageTypeID)
	}
	if decoded.UniqueID != msg.UniqueID {
		t.Errorf("UniqueID mismatch: %s != %s", decoded.UniqueID, msg.UniqueID)
	}
	if decoded.Action != msg.Action {
		t.Errorf("Action mismatch: %s != %s", decoded.Action, msg.Action)
	}
}

func TestEncodeDecode_CALLRESULT(t *testing.T) {
	c := codec.New()
	msg := message.Message{
		MessageTypeID: message.CALLRESULT,
		UniqueID:      "test-456",
		Payload:       json.RawMessage(`{"currentTime":"2025-01-01T00:00:00Z"}`),
	}
	raw, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := c.Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.MessageTypeID != message.CALLRESULT {
		t.Errorf("MessageTypeID mismatch: %d", decoded.MessageTypeID)
	}
	if decoded.UniqueID != "test-456" {
		t.Errorf("UniqueID mismatch: %s", decoded.UniqueID)
	}
}

func TestEncodeDecode_CALLERROR(t *testing.T) {
	c := codec.New()
	msg, err := c.BuildError("test-789", message.ErrorCodeNotImplemented, "not supported", nil)
	if err != nil {
		t.Fatalf("BuildError: %v", err)
	}
	raw, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := c.Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.MessageTypeID != message.CALLERROR {
		t.Errorf("MessageTypeID mismatch: %d", decoded.MessageTypeID)
	}
	if decoded.ErrorCode != string(message.ErrorCodeNotImplemented) {
		t.Errorf("ErrorCode mismatch: %s", decoded.ErrorCode)
	}
}

func TestDecode_Malformed(t *testing.T) {
	c := codec.New()
	_, err := c.Decode([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	_, err = c.Decode([]byte(`[]`))
	if err == nil {
		t.Error("expected error for empty array")
	}
	_, err = c.Decode([]byte(`[99,"id","action",{}]`))
	if err == nil {
		t.Error("expected error for unknown message type")
	}
}

func TestUniqueID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := message.GenerateUniqueID()
		if id == "" {
			t.Fatal("empty unique ID")
		}
		if seen[id] {
			t.Fatalf("duplicate unique ID: %s", id)
		}
		seen[id] = true
	}
}

func TestNonSpecPayload_Rejected(t *testing.T) {
	badPayload := []byte(`{"ChargePointVendor":"test","ChargePointModel":"test"}`)
	err := ocppschemas.Validate(ocppschemas.V16, "BootNotification", ocppschemas.Request, badPayload)
	if err == nil {
		t.Error("expected validation error for wrong casing")
	}
}

func TestMissingRequiredField_Rejected(t *testing.T) {
	badPayload := []byte(`{"chargePointVendor":"test"}`)
	err := ocppschemas.Validate(ocppschemas.V16, "BootNotification", ocppschemas.Request, badPayload)
	if err == nil {
		t.Error("expected validation error for missing required field chargePointModel")
	}
}

func TestInvalidEnum_Rejected(t *testing.T) {
	badPayload := []byte(`{"connectorId":1,"errorCode":"NoError","status":"Busy"}`)
	err := ocppschemas.Validate(ocppschemas.V16, "StatusNotification", ocppschemas.Request, badPayload)
	if err == nil {
		t.Error("expected validation error for invalid enum value Busy")
	}
}

// CSMS-initiated message schema validation tests

func TestRemoteStartTransaction_SchemaValid(t *testing.T) {
	payload := []byte(`{"idTag":"ABC12345","connectorId":1}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "RemoteStartTransaction", ocppschemas.Request, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestRemoteStartTransaction_Minimal_SchemaValid(t *testing.T) {
	payload := []byte(`{"idTag":"ABC12345"}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "RemoteStartTransaction", ocppschemas.Request, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestRemoteStartTransactionResponse_SchemaValid(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "RemoteStartTransaction", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestRemoteStopTransaction_SchemaValid(t *testing.T) {
	payload := []byte(`{"transactionId":42}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "RemoteStopTransaction", ocppschemas.Request, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestRemoteStopTransactionResponse_SchemaValid(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "RemoteStopTransaction", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestReset_SchemaValid(t *testing.T) {
	payload := []byte(`{"type":"Soft"}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "Reset", ocppschemas.Request, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestResetResponse_SchemaValid(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "Reset", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestUnlockConnector_SchemaValid(t *testing.T) {
	payload := []byte(`{"connectorId":1}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "UnlockConnector", ocppschemas.Request, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestUnlockConnectorResponse_SchemaValid(t *testing.T) {
	payload := []byte(`{"status":"Unlocked"}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "UnlockConnector", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestChangeConfiguration_SchemaValid(t *testing.T) {
	payload := []byte(`{"key":"HeartbeatInterval","value":"300"}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "ChangeConfiguration", ocppschemas.Request, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestChangeConfigurationResponse_SchemaValid(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "ChangeConfiguration", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestGetConfiguration_SchemaValid(t *testing.T) {
	payload := []byte(`{"key":["HeartbeatInterval","MeterValueSampleInterval"]}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "GetConfiguration", ocppschemas.Request, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestGetConfiguration_EmptyKeys_SchemaValid(t *testing.T) {
	payload := []byte(`{}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "GetConfiguration", ocppschemas.Request, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestGetConfigurationResponse_SchemaValid(t *testing.T) {
	payload := []byte(`{"configurationKey":[{"key":"HeartbeatInterval","readonly":false,"value":"300"}],"unknownKey":[]}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "GetConfiguration", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestTriggerMessage_SchemaValid(t *testing.T) {
	payload := []byte(`{"requestedMessage":"StatusNotification","connectorId":1}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "TriggerMessage", ocppschemas.Request, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestTriggerMessageResponse_SchemaValid(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "TriggerMessage", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestChangeAvailability_SchemaValid(t *testing.T) {
	payload := []byte(`{"connectorId":1,"type":"Inoperative"}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "ChangeAvailability", ocppschemas.Request, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestChangeAvailabilityResponse_SchemaValid(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := ocppschemas.Validate(ocppschemas.V16, "ChangeAvailability", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

// CSMS-initiated request parsing tests

func TestParseRemoteStartTransactionRequest(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{"idTag":"ABC12345","connectorId":1}`)
	req, err := p.ParseRemoteStartTransactionRequest(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if req.IDTag != "ABC12345" {
		t.Errorf("expected idTag ABC12345, got %s", req.IDTag)
	}
	if req.ConnectorID == nil || *req.ConnectorID != 1 {
		t.Errorf("expected connectorId 1")
	}
}

func TestParseRemoteStartTransactionRequest_MissingIDTag(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{"connectorId":1}`)
	_, err := p.ParseRemoteStartTransactionRequest(payload)
	if err == nil {
		t.Error("expected error for missing idTag")
	}
}

func TestParseRemoteStopTransactionRequest(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{"transactionId":42}`)
	req, err := p.ParseRemoteStopTransactionRequest(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if req.TransactionID != 42 {
		t.Errorf("expected transactionId 42, got %d", req.TransactionID)
	}
}

func TestParseResetRequest(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{"type":"Soft"}`)
	req, err := p.ParseResetRequest(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if req.Type != "Soft" {
		t.Errorf("expected type Soft, got %s", req.Type)
	}
}

func TestParseResetRequest_InvalidType(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{"type":"Invalid"}`)
	_, err := p.ParseResetRequest(payload)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestParseUnlockConnectorRequest(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{"connectorId":1}`)
	req, err := p.ParseUnlockConnectorRequest(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if req.ConnectorID != 1 {
		t.Errorf("expected connectorId 1, got %d", req.ConnectorID)
	}
}

func TestParseUnlockConnectorRequest_InvalidID(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{"connectorId":0}`)
	_, err := p.ParseUnlockConnectorRequest(payload)
	if err == nil {
		t.Error("expected error for connectorId 0")
	}
}

func TestParseChangeConfigurationRequest(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{"key":"HeartbeatInterval","value":"300"}`)
	req, err := p.ParseChangeConfigurationRequest(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if req.Key != "HeartbeatInterval" {
		t.Errorf("expected key HeartbeatInterval, got %s", req.Key)
	}
	if req.Value != "300" {
		t.Errorf("expected value 300, got %s", req.Value)
	}
}

func TestParseGetConfigurationRequest(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{"key":["HeartbeatInterval"]}`)
	req, err := p.ParseGetConfigurationRequest(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(req.Key) != 1 || req.Key[0] != "HeartbeatInterval" {
		t.Errorf("expected [HeartbeatInterval], got %v", req.Key)
	}
}

func TestParseGetConfigurationRequest_EmptyKeys(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{}`)
	req, err := p.ParseGetConfigurationRequest(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(req.Key) != 0 {
		t.Errorf("expected empty keys, got %v", req.Key)
	}
}

func TestParseTriggerMessageRequest(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{"requestedMessage":"StatusNotification","connectorId":1}`)
	req, err := p.ParseTriggerMessageRequest(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if req.RequestedMessage != "StatusNotification" {
		t.Errorf("expected StatusNotification, got %s", req.RequestedMessage)
	}
}

func TestParseChangeAvailabilityRequest(t *testing.T) {
	p := NewProtocol()
	payload := json.RawMessage(`{"connectorId":1,"type":"Inoperative"}`)
	req, err := p.ParseChangeAvailabilityRequest(payload)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if req.ConnectorID != 1 {
		t.Errorf("expected connectorId 1, got %d", req.ConnectorID)
	}
	if req.Type != "Inoperative" {
		t.Errorf("expected type Inoperative, got %s", req.Type)
	}
}

// CSMS-initiated response builder tests

func TestBuildRemoteStartTransactionResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	payload, err := p.BuildRemoteStartTransactionResponse("Accepted")
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "RemoteStartTransaction", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildRemoteStopTransactionResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	payload, err := p.BuildRemoteStopTransactionResponse("Accepted")
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "RemoteStopTransaction", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildResetResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	payload, err := p.BuildResetResponse("Accepted")
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "Reset", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildUnlockConnectorResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	payload, err := p.BuildUnlockConnectorResponse("Unlocked")
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "UnlockConnector", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildChangeConfigurationResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	payload, err := p.BuildChangeConfigurationResponse("Accepted")
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "ChangeConfiguration", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildGetConfigurationResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	val := "300"
	payload, err := p.BuildGetConfigurationResponse([]protocol.ConfigurationKey{
		{Key: "HeartbeatInterval", Readonly: false, Value: &val},
	}, nil)
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "GetConfiguration", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildTriggerMessageResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	payload, err := p.BuildTriggerMessageResponse("Accepted")
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "TriggerMessage", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildChangeAvailabilityResponse_SchemaValid(t *testing.T) {
	p := NewProtocol()
	payload, err := p.BuildChangeAvailabilityResponse("Accepted")
	if err != nil {
		t.Fatalf("build error: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "ChangeAvailability", ocppschemas.Response, payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}
