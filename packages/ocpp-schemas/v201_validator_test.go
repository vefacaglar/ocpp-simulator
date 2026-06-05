package ocppschemas

import (
	"strings"
	"testing"
)

// All 14 v201 actions the multi-version plan promotes to the
// validator. Each has a minimal known-good request payload and a
// minimal known-good response payload (where applicable). The
// payloads are hand-written to match the official OCPP 2.0.1 FINAL
// JSON schemas, then validated by the same validator that
// production code uses. A non-spec payload MUST be rejected.

func TestValidateV201_BootNotification_Minimal(t *testing.T) {
	payload := []byte(`{"reason":"PowerUp","chargingStation":{"model":"M","vendorName":"V"}}`)
	if err := Validate(V201, "BootNotification", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_BootNotification_FullChargingStation(t *testing.T) {
	payload := []byte(`{"reason":"RemoteReset","chargingStation":{"model":"M","vendorName":"V","serialNumber":"S","firmwareVersion":"1.2.3"}}`)
	if err := Validate(V201, "BootNotification", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_BootNotification_MissingVendorName(t *testing.T) {
	payload := []byte(`{"reason":"PowerUp","chargingStation":{"model":"M"}}`)
	if err := Validate(V201, "BootNotification", Request, payload); err == nil {
		t.Error("expected validation error for missing vendorName")
	}
}

func TestValidateV201_BootNotification_BadReason(t *testing.T) {
	payload := []byte(`{"reason":"NotAReason","chargingStation":{"model":"M","vendorName":"V"}}`)
	if err := Validate(V201, "BootNotification", Request, payload); err == nil {
		t.Error("expected validation error for invalid reason enum")
	}
}

func TestValidateV201_BootNotificationResponse(t *testing.T) {
	payload := []byte(`{"status":"Accepted","currentTime":"2025-01-01T00:00:00Z","interval":300}`)
	if err := Validate(V201, "BootNotification", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_Heartbeat(t *testing.T) {
	payload := []byte(`{}`)
	if err := Validate(V201, "Heartbeat", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_HeartbeatResponse(t *testing.T) {
	payload := []byte(`{"currentTime":"2025-01-01T00:00:00Z"}`)
	if err := Validate(V201, "Heartbeat", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_StatusNotification(t *testing.T) {
	payload := []byte(`{"timestamp":"2025-01-01T00:00:00Z","connectorStatus":"Available","evseId":1,"connectorId":1}`)
	if err := Validate(V201, "StatusNotification", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_StatusNotification_MissingEvseId(t *testing.T) {
	payload := []byte(`{"timestamp":"2025-01-01T00:00:00Z","connectorStatus":"Available","connectorId":1}`)
	if err := Validate(V201, "StatusNotification", Request, payload); err == nil {
		t.Error("expected validation error for missing evseId")
	}
}

func TestValidateV201_StatusNotification_BadConnectorStatus(t *testing.T) {
	payload := []byte(`{"timestamp":"2025-01-01T00:00:00Z","connectorStatus":"Broken","evseId":1,"connectorId":1}`)
	if err := Validate(V201, "StatusNotification", Request, payload); err == nil {
		t.Error("expected validation error for invalid connectorStatus enum")
	}
}

func TestValidateV201_Authorize(t *testing.T) {
	payload := []byte(`{"idToken":{"idToken":"ABC12345","type":"ISO14443"}}`)
	if err := Validate(V201, "Authorize", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_Authorize_MissingIdTokenType(t *testing.T) {
	payload := []byte(`{"idToken":{"idToken":"ABC12345"}}`)
	if err := Validate(V201, "Authorize", Request, payload); err == nil {
		t.Error("expected validation error for missing type")
	}
}

func TestValidateV201_Authorize_BadType(t *testing.T) {
	payload := []byte(`{"idToken":{"idToken":"X","type":"MagStripe"}}`)
	if err := Validate(V201, "Authorize", Request, payload); err == nil {
		t.Error("expected validation error for invalid idToken type")
	}
}

func TestValidateV201_AuthorizeResponse(t *testing.T) {
	payload := []byte(`{"idTokenInfo":{"status":"Accepted"}}`)
	if err := Validate(V201, "Authorize", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_TransactionEvent_Started(t *testing.T) {
	payload := []byte(`{
		"eventType":"Started",
		"timestamp":"2025-01-01T00:00:00Z",
		"triggerReason":"CablePluggedIn",
		"seqNo":0,
		"transactionInfo":{"transactionId":"tx-1"},
		"evse":{"id":1,"connectorId":1}
	}`)
	if err := Validate(V201, "TransactionEvent", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_TransactionEvent_MissingSeqNo(t *testing.T) {
	// seqNo is REQUIRED by the spec; the validator must reject a
	// payload that omits it. This is the central reason
	// TransactionEventInput carries SeqNo explicitly.
	payload := []byte(`{
		"eventType":"Started",
		"timestamp":"2025-01-01T00:00:00Z",
		"triggerReason":"CablePluggedIn",
		"transactionInfo":{"transactionId":"tx-1"}
	}`)
	err := Validate(V201, "TransactionEvent", Request, payload)
	if err == nil {
		t.Fatal("expected validation error for missing seqNo")
	}
	if !strings.Contains(err.Error(), "seqNo") {
		t.Errorf("error should mention seqNo, got: %v", err)
	}
}

func TestValidateV201_TransactionEvent_MissingTransactionInfo(t *testing.T) {
	payload := []byte(`{
		"eventType":"Started",
		"timestamp":"2025-01-01T00:00:00Z",
		"triggerReason":"CablePluggedIn",
		"seqNo":0
	}`)
	if err := Validate(V201, "TransactionEvent", Request, payload); err == nil {
		t.Error("expected validation error for missing transactionInfo")
	}
}

func TestValidateV201_TransactionEvent_BadEventType(t *testing.T) {
	payload := []byte(`{
		"eventType":"Commenced",
		"timestamp":"2025-01-01T00:00:00Z",
		"triggerReason":"CablePluggedIn",
		"seqNo":0,
		"transactionInfo":{"transactionId":"tx-1"}
	}`)
	if err := Validate(V201, "TransactionEvent", Request, payload); err == nil {
		t.Error("expected validation error for invalid eventType enum")
	}
}

func TestValidateV201_TransactionEvent_UpdatedWithMeterValues(t *testing.T) {
	payload := []byte(`{
		"eventType":"Updated",
		"timestamp":"2025-01-01T00:01:00Z",
		"triggerReason":"MeterValuePeriodic",
		"seqNo":1,
		"transactionInfo":{"transactionId":"tx-1","chargingState":"Charging"},
		"evse":{"id":1,"connectorId":1},
		"meterValue":[{
			"timestamp":"2025-01-01T00:01:00Z",
			"sampledValue":[{"value":1234.5,"unitOfMeasure":{"unit":"Wh","multiplier":0}}]
		}]
	}`)
	if err := Validate(V201, "TransactionEvent", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_TransactionEvent_Ended(t *testing.T) {
	payload := []byte(`{
		"eventType":"Ended",
		"timestamp":"2025-01-01T01:00:00Z",
		"triggerReason":"EVDeparted",
		"seqNo":2,
		"transactionInfo":{"transactionId":"tx-1","stoppedReason":"EVDisconnected"}
	}`)
	if err := Validate(V201, "TransactionEvent", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_TransactionEventResponse(t *testing.T) {
	payload := []byte(`{"idTokenInfo":{"status":"Accepted"}}`)
	if err := Validate(V201, "TransactionEvent", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_MeterValues(t *testing.T) {
	payload := []byte(`{
		"evseId":1,
		"meterValue":[{
			"timestamp":"2025-01-01T00:00:00Z",
			"sampledValue":[{"value":42.0}]
		}]
	}`)
	if err := Validate(V201, "MeterValues", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_MeterValues_MissingEvseId(t *testing.T) {
	// 2.0.1 requires evseId; a 1.6J-shaped payload (connectorId
	// only) MUST be rejected.
	payload := []byte(`{
		"connectorId":1,
		"meterValue":[{
			"timestamp":"2025-01-01T00:00:00Z",
			"sampledValue":[{"value":42.0}]
		}]
	}`)
	if err := Validate(V201, "MeterValues", Request, payload); err == nil {
		t.Error("expected validation error: 2.0.1 requires evseId, not connectorId")
	}
}

func TestValidateV201_Reset_Immediate(t *testing.T) {
	payload := []byte(`{"type":"Immediate"}`)
	if err := Validate(V201, "Reset", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_Reset_OnIdle(t *testing.T) {
	payload := []byte(`{"type":"OnIdle"}`)
	if err := Validate(V201, "Reset", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_Reset_BadType(t *testing.T) {
	// 2.0.1 Reset.type enum is Immediate|OnIdle — NOT the 1.6J
	// Hard|Soft. A 1.6J-shaped payload must be rejected.
	payload := []byte(`{"type":"Soft"}`)
	if err := Validate(V201, "Reset", Request, payload); err == nil {
		t.Error("expected validation error: 2.0.1 does not have Soft/Hard reset enum")
	}
}

func TestValidateV201_ResetResponse(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := Validate(V201, "Reset", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_UnlockConnector(t *testing.T) {
	payload := []byte(`{"evseId":1,"connectorId":1}`)
	if err := Validate(V201, "UnlockConnector", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_TriggerMessage_TransactionEvent(t *testing.T) {
	// 2.0.1's MessageTriggerEnum includes TransactionEvent. This
	// is a 2.0.1-only value; a 1.6J TriggerMessage would reject
	// it. 2.0.1 also uses an `evse {id}` object, not a flat
	// `evseId` integer.
	payload := []byte(`{"requestedMessage":"TransactionEvent","evse":{"id":1}}`)
	if err := Validate(V201, "TriggerMessage", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_ChangeAvailability(t *testing.T) {
	payload := []byte(`{"operationalStatus":"Inoperative","evse":{"id":1}}`)
	if err := Validate(V201, "ChangeAvailability", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_GetVariables(t *testing.T) {
	payload := []byte(`{"getVariableData":[{"component":{"name":"ChargingStation"},"variable":{"name":"Model"}}]}`)
	if err := Validate(V201, "GetVariables", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_SetVariables(t *testing.T) {
	// 2.0.1 SetVariables.attributeValue is a string, not a
	// number. The device-model variables are typeless in JSON
	// (the CSMS interprets the string according to the
	// component/variable metadata).
	payload := []byte(`{"setVariableData":[{"component":{"name":"ChargingStation"},"variable":{"name":"HeartbeatInterval"},"attributeValue":"300"}]}`)
	if err := Validate(V201, "SetVariables", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_RequestStartTransaction(t *testing.T) {
	payload := []byte(`{"idToken":{"idToken":"X","type":"ISO14443"},"remoteStartId":1}`)
	if err := Validate(V201, "RequestStartTransaction", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_RequestStartTransactionResponse(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := Validate(V201, "RequestStartTransaction", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_RequestStopTransaction(t *testing.T) {
	payload := []byte(`{"transactionId":"tx-1"}`)
	if err := Validate(V201, "RequestStopTransaction", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidateV201_RequestStopTransactionResponse(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := Validate(V201, "RequestStopTransaction", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

// Compile-time canary that the v201 embed is actually wired up.
func TestValidateV201_AllSchemasCompilable(t *testing.T) {
	for _, action := range []string{
		"BootNotification", "Heartbeat", "StatusNotification", "Authorize",
		"TransactionEvent", "MeterValues",
		"RequestStartTransaction", "RequestStopTransaction",
		"Reset", "UnlockConnector", "TriggerMessage", "ChangeAvailability",
		"GetVariables", "SetVariables",
	} {
		for _, dir := range []Direction{Request, Response} {
			if _, err := loadSchema(V201, action, dir); err != nil {
				t.Errorf("loadSchema(V201, %s, %s): %v", action, dir, err)
			}
		}
	}
}
