package csms

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

type fakeBroker struct {
	mu        sync.Mutex
	published map[string][][]byte
	connected bool
}

func newFakeBroker() *fakeBroker {
	return &fakeBroker{published: make(map[string][][]byte), connected: true}
}

func (b *fakeBroker) Publish(topic string, payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.connected {
		return errBrokerDown
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	b.published[topic] = append(b.published[topic], cp)
	return nil
}
func (b *fakeBroker) Subscribe(string, func(string, []byte)) (func(), error) {
	return func() {}, nil
}
func (b *fakeBroker) IsConnected() bool           { return b.connected }
func (b *fakeBroker) Connect(_ context.Context) error { return nil }
func (b *fakeBroker) Disconnect() error               { return nil }

func (b *fakeBroker) snapshot(topic string) [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([][]byte, len(b.published[topic]))
	copy(out, b.published[topic])
	return out
}

var errBrokerDown = errBrokerDownT{}

type errBrokerDownT struct{}

func (errBrokerDownT) Error() string { return "broker down" }

func newSvc(t *testing.T) (*Service, *fakeBroker) {
	t.Helper()
	b := newFakeBroker()
	return New(b), b
}

// Test: RemoteStart publishes a spec-exact CALL on
// ocpp/{cpId}/out. The frame is a JSON array starting with 2,
// with a fresh uniqueId each call.
func TestCSMS_RemoteStart_PublishesSpecExactCall(t *testing.T) {
	svc, b := newSvc(t)
	cpID := "CP-001"
	connID := 1
	if err := svc.RemoteStart(context.Background(), cpID, "TAG-1", &connID); err != nil {
		t.Fatalf("RemoteStart: %v", err)
	}
	out := b.snapshot("ocpp/" + cpID + "/out")
	if len(out) != 1 {
		t.Fatalf("expected 1 outbound frame, got %d", len(out))
	}
	raw := out[0]
	if raw[0] != '[' || raw[len(raw)-1] != ']' {
		t.Fatalf("outbound is not a JSON array: %q", raw)
	}
	// Decode and assert the spec-exact fields.
	var frame []json.RawMessage
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	if len(frame) != 4 {
		t.Fatalf("CALL frame must have 4 elements, got %d", len(frame))
	}
	var typeID int
	if err := json.Unmarshal(frame[0], &typeID); err != nil {
		t.Fatalf("type id: %v", err)
	}
	if typeID != 2 {
		t.Errorf("typeID = %d, want 2 (CALL)", typeID)
	}
	var action string
	if err := json.Unmarshal(frame[2], &action); err != nil {
		t.Fatalf("action: %v", err)
	}
	if action != "RemoteStartTransaction" {
		t.Errorf("action = %q, want RemoteStartTransaction", action)
	}
	// Payload must include idTag="TAG-1" and connectorId=1.
	var payload struct {
		IDTag       string `json:"idTag"`
		ConnectorID *int   `json:"connectorId"`
	}
	if err := json.Unmarshal(frame[3], &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if payload.IDTag != "TAG-1" {
		t.Errorf("idTag = %q", payload.IDTag)
	}
	if payload.ConnectorID == nil || *payload.ConnectorID != 1 {
		t.Errorf("connectorId = %v", payload.ConnectorID)
	}
}

// Test: Reset rejects an out-of-spec type so a typo cannot produce
// a non-spec wire frame.
func TestCSMS_Reset_RejectsInvalidType(t *testing.T) {
	svc, b := newSvc(t)
	if err := svc.Reset(context.Background(), "CP-1", "Bogus"); err == nil {
		t.Fatal("expected error for invalid reset type")
	}
	if out := b.snapshot("ocpp/CP-1/out"); len(out) != 0 {
		t.Errorf("must not publish on validation failure, got %d", len(out))
	}
}

// Test: unique ids are unique across calls. This is essential for
// the CP to correlate the eventual CALLRESULT.
func TestCSMS_UniqueIDs_AreUnique(t *testing.T) {
	svc, b := newSvc(t)
	for i := 0; i < 50; i++ {
		if err := svc.RemoteStart(context.Background(), "CP-U", "T", nil); err != nil {
			t.Fatal(err)
		}
	}
	out := b.snapshot("ocpp/CP-U/out")
	seen := make(map[string]bool)
	for _, raw := range out {
		var frame []json.RawMessage
		_ = json.Unmarshal(raw, &frame)
		var uid string
		_ = json.Unmarshal(frame[1], &uid)
		if uid == "" {
			t.Errorf("empty unique id")
		}
		if seen[uid] {
			t.Errorf("duplicate unique id: %s", uid)
		}
		seen[uid] = true
	}
}

// Test: RemoteStop and UnlockConnector also publish raw CALLs.
func TestCSMS_RemoteStopAndUnlock(t *testing.T) {
	svc, b := newSvc(t)
	if err := svc.RemoteStop(context.Background(), "CP-X", 42); err != nil {
		t.Fatal(err)
	}
	if err := svc.UnlockConnector(context.Background(), "CP-X", 1); err != nil {
		t.Fatal(err)
	}
	if out := b.snapshot("ocpp/CP-X/out"); len(out) != 2 {
		t.Fatalf("expected 2 outbound frames, got %d", len(out))
	}
}

// silence the import when helpers above are trimmed.
var _ = json.Marshal
