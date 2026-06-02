package ocppschemas

import (
	"testing"
)

func TestValidate_RemoteStartTransaction_Request(t *testing.T) {
	payload := []byte(`{"idTag":"ABC12345","connectorId":1}`)
	if err := Validate(V16, "RemoteStartTransaction", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_RemoteStartTransaction_Minimal(t *testing.T) {
	payload := []byte(`{"idTag":"ABC12345"}`)
	if err := Validate(V16, "RemoteStartTransaction", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_RemoteStartTransaction_MissingIDTag(t *testing.T) {
	payload := []byte(`{"connectorId":1}`)
	if err := Validate(V16, "RemoteStartTransaction", Request, payload); err == nil {
		t.Error("expected validation error for missing idTag")
	}
}

func TestValidate_RemoteStartTransactionResponse(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := Validate(V16, "RemoteStartTransaction", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_RemoteStartTransactionResponse_InvalidStatus(t *testing.T) {
	payload := []byte(`{"status":"Invalid"}`)
	if err := Validate(V16, "RemoteStartTransaction", Response, payload); err == nil {
		t.Error("expected validation error for invalid status")
	}
}

func TestValidate_RemoteStopTransaction_Request(t *testing.T) {
	payload := []byte(`{"transactionId":42}`)
	if err := Validate(V16, "RemoteStopTransaction", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_RemoteStopTransaction_MissingTransactionID(t *testing.T) {
	payload := []byte(`{}`)
	if err := Validate(V16, "RemoteStopTransaction", Request, payload); err == nil {
		t.Error("expected validation error for missing transactionId")
	}
}

func TestValidate_Reset_Request(t *testing.T) {
	payload := []byte(`{"type":"Soft"}`)
	if err := Validate(V16, "Reset", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_Reset_Hard(t *testing.T) {
	payload := []byte(`{"type":"Hard"}`)
	if err := Validate(V16, "Reset", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_Reset_InvalidType(t *testing.T) {
	payload := []byte(`{"type":"Invalid"}`)
	if err := Validate(V16, "Reset", Request, payload); err == nil {
		t.Error("expected validation error for invalid type")
	}
}

func TestValidate_UnlockConnector_Request(t *testing.T) {
	payload := []byte(`{"connectorId":1}`)
	if err := Validate(V16, "UnlockConnector", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_UnlockConnectorResponse(t *testing.T) {
	payload := []byte(`{"status":"Unlocked"}`)
	if err := Validate(V16, "UnlockConnector", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_UnlockConnectorResponse_InvalidStatus(t *testing.T) {
	payload := []byte(`{"status":"Invalid"}`)
	if err := Validate(V16, "UnlockConnector", Response, payload); err == nil {
		t.Error("expected validation error for invalid status")
	}
}

func TestValidate_ChangeConfiguration_Request(t *testing.T) {
	payload := []byte(`{"key":"HeartbeatInterval","value":"300"}`)
	if err := Validate(V16, "ChangeConfiguration", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_ChangeConfigurationResponse(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := Validate(V16, "ChangeConfiguration", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_GetConfiguration_Request(t *testing.T) {
	payload := []byte(`{"key":["HeartbeatInterval","MeterValueSampleInterval"]}`)
	if err := Validate(V16, "GetConfiguration", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_GetConfiguration_EmptyRequest(t *testing.T) {
	payload := []byte(`{}`)
	if err := Validate(V16, "GetConfiguration", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_GetConfigurationResponse(t *testing.T) {
	payload := []byte(`{"configurationKey":[{"key":"HeartbeatInterval","readonly":false,"value":"300"}],"unknownKey":[]}`)
	if err := Validate(V16, "GetConfiguration", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_TriggerMessage_Request(t *testing.T) {
	payload := []byte(`{"requestedMessage":"StatusNotification","connectorId":1}`)
	if err := Validate(V16, "TriggerMessage", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_TriggerMessage_InvalidMessage(t *testing.T) {
	payload := []byte(`{"requestedMessage":"InvalidMessage"}`)
	if err := Validate(V16, "TriggerMessage", Request, payload); err == nil {
		t.Error("expected validation error for invalid requestedMessage")
	}
}

func TestValidate_TriggerMessageResponse(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := Validate(V16, "TriggerMessage", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_ChangeAvailability_Request(t *testing.T) {
	payload := []byte(`{"connectorId":1,"type":"Inoperative"}`)
	if err := Validate(V16, "ChangeAvailability", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_ChangeAvailability_StationWide(t *testing.T) {
	payload := []byte(`{"connectorId":0,"type":"Operative"}`)
	if err := Validate(V16, "ChangeAvailability", Request, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_ChangeAvailabilityResponse(t *testing.T) {
	payload := []byte(`{"status":"Accepted"}`)
	if err := Validate(V16, "ChangeAvailability", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

func TestValidate_ChangeAvailabilityResponse_Scheduled(t *testing.T) {
	payload := []byte(`{"status":"Scheduled"}`)
	if err := Validate(V16, "ChangeAvailability", Response, payload); err != nil {
		t.Fatalf("validation failed: %v", err)
	}
}

// Test that wrong casing is rejected for new schemas
func TestValidate_RemoteStartTransaction_WrongCasing(t *testing.T) {
	payload := []byte(`{"IdTag":"ABC12345"}`)
	if err := Validate(V16, "RemoteStartTransaction", Request, payload); err == nil {
		t.Error("expected validation error for wrong casing IdTag")
	}
}

func TestValidate_ChangeConfiguration_WrongCasing(t *testing.T) {
	payload := []byte(`{"Key":"HeartbeatInterval","Value":"300"}`)
	if err := Validate(V16, "ChangeConfiguration", Request, payload); err == nil {
		t.Error("expected validation error for wrong casing Key/Value")
	}
}

// Test unknown action returns error
func TestValidate_UnknownAction(t *testing.T) {
	payload := []byte(`{}`)
	if err := Validate(V16, "NonExistentAction", Request, payload); err == nil {
		t.Error("expected error for unknown action")
	}
}
