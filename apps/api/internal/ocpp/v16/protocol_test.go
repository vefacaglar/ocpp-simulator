package v16

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	ocppschemas "github.com/user/ocpp-simulator/packages/ocpp-schemas"
	"github.com/user/ocpp-simulator/apps/api/internal/ocpp"
)

func TestBuildBootNotification_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildBootNotification(context.Background(), ocpp.BootNotificationInput{
		ChargePointVendor: "TestVendor",
		ChargePointModel:  "TestModel",
	})
	if err != nil {
		t.Fatalf("BuildBootNotification: %v", err)
	}
	if msg.MessageTypeID != ocpp.CALL {
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
	msg, err := p.BuildStatusNotification(context.Background(), ocpp.StatusNotificationInput{
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
	msg, err := p.BuildAuthorize(context.Background(), ocpp.AuthorizeInput{IDTag: "ABCDEF12"})
	if err != nil {
		t.Fatalf("BuildAuthorize: %v", err)
	}
	if err := ocppschemas.Validate(ocppschemas.V16, "Authorize", ocppschemas.Request, msg.Payload); err != nil {
		t.Fatalf("schema validation failed: %v", err)
	}
}

func TestBuildStartTransaction_SchemaValid(t *testing.T) {
	p := NewProtocol()
	msg, err := p.BuildStartTransaction(context.Background(), ocpp.StartTransactionInput{
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
	msg, err := p.BuildMeterValues(context.Background(), ocpp.MeterValuesInput{
		ConnectorID:   1,
		TransactionID: &txID,
		MeterValues: []ocpp.MeterValue{{
			Timestamp: time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC),
			SampledValue: []ocpp.SampledValue{
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
	msg, err := p.BuildStopTransaction(context.Background(), ocpp.StopTransactionInput{
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
	codec := ocpp.NewCodec()
	msg := ocpp.Message{
		MessageTypeID: ocpp.CALL,
		UniqueID:      "test-123",
		Action:        "Heartbeat",
		Payload:       json.RawMessage(`{}`),
	}
	raw, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := codec.Decode(raw)
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
	codec := ocpp.NewCodec()
	msg := ocpp.Message{
		MessageTypeID: ocpp.CALLRESULT,
		UniqueID:      "test-456",
		Payload:       json.RawMessage(`{"currentTime":"2025-01-01T00:00:00Z"}`),
	}
	raw, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := codec.Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.MessageTypeID != ocpp.CALLRESULT {
		t.Errorf("MessageTypeID mismatch: %d", decoded.MessageTypeID)
	}
	if decoded.UniqueID != "test-456" {
		t.Errorf("UniqueID mismatch: %s", decoded.UniqueID)
	}
}

func TestEncodeDecode_CALLERROR(t *testing.T) {
	codec := ocpp.NewCodec()
	msg, err := codec.BuildError("test-789", ocpp.ErrorCodeNotImplemented, "not supported", nil)
	if err != nil {
		t.Fatalf("BuildError: %v", err)
	}
	raw, err := codec.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := codec.Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.MessageTypeID != ocpp.CALLERROR {
		t.Errorf("MessageTypeID mismatch: %d", decoded.MessageTypeID)
	}
	if decoded.ErrorCode != string(ocpp.ErrorCodeNotImplemented) {
		t.Errorf("ErrorCode mismatch: %s", decoded.ErrorCode)
	}
}

func TestDecode_Malformed(t *testing.T) {
	codec := ocpp.NewCodec()
	_, err := codec.Decode([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
	_, err = codec.Decode([]byte(`[]`))
	if err == nil {
		t.Error("expected error for empty array")
	}
	_, err = codec.Decode([]byte(`[99,"id","action",{}]`))
	if err == nil {
		t.Error("expected error for unknown message type")
	}
}

func TestUniqueID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := ocpp.GenerateUniqueID()
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
