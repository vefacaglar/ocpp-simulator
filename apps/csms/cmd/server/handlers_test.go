package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBootNotificationResponse_Valid(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := map[string]interface{}{
		"status":      "Accepted",
		"currentTime": now,
		"interval":    300,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty payload")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["status"] != "Accepted" {
		t.Errorf("expected Accepted, got %v", parsed["status"])
	}
}

func TestHeartbeatResponse_Valid(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := map[string]interface{}{
		"currentTime": now,
	}
	data, _ := json.Marshal(payload)
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)
	if parsed["currentTime"] == nil {
		t.Error("missing currentTime")
	}
}

func TestStartTransactionResponse_Valid(t *testing.T) {
	payload := map[string]interface{}{
		"transactionId": 1,
		"idTagInfo":     map[string]interface{}{"status": "Accepted"},
	}
	data, _ := json.Marshal(payload)
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)
	if parsed["transactionId"] == nil {
		t.Error("missing transactionId")
	}
	idTagInfo, ok := parsed["idTagInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("missing idTagInfo")
	}
	if idTagInfo["status"] != "Accepted" {
		t.Errorf("expected Accepted, got %v", idTagInfo["status"])
	}
}

func TestHandleFrame_BootNotification(t *testing.T) {
	frame := `[2,"test-1","BootNotification",{"chargePointVendor":"Test","chargePointModel":"Model"}]`
	resp := handleFrame([]byte(frame))
	if resp == "" {
		t.Fatal("empty response")
	}

	var arr []json.RawMessage
	json.Unmarshal([]byte(resp), &arr)
	if len(arr) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(arr))
	}

	var typeID int
	json.Unmarshal(arr[0], &typeID)
	if typeID != 3 {
		t.Errorf("expected CALLRESULT (3), got %d", typeID)
	}

	var uniqueID string
	json.Unmarshal(arr[1], &uniqueID)
	if uniqueID != "test-1" {
		t.Errorf("expected test-1, got %s", uniqueID)
	}
}

func TestHandleFrame_UnknownAction(t *testing.T) {
	frame := `[2,"test-2","UnknownAction",{}]`
	resp := handleFrame([]byte(frame))
	if resp == "" {
		t.Fatal("empty response")
	}

	var arr []json.RawMessage
	json.Unmarshal([]byte(resp), &arr)

	var typeID int
	json.Unmarshal(arr[0], &typeID)
	if typeID != 4 {
		t.Errorf("expected CALLERROR (4), got %d", typeID)
	}
}
