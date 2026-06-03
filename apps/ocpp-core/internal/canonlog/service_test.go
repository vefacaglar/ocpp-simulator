package canonlog

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/db"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/message"
)

func newTestService(t *testing.T) (*Service, *fakeBroker, *db.MessageLogRepo, func()) {
	t.Helper()
	d, err := db.Open(db.TestDBURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	// Clean state
	for _, tbl := range []string{"transactions", "ocpp_message_logs", "runtime_events"} {
		_, _ = d.ExecContext(context.Background(), "TRUNCATE TABLE "+tbl+" RESTART IDENTITY CASCADE")
	}
	repo := db.NewMessageLogRepo(d)
	broker := newFakeBroker()
	svc := New(broker, repo)
	return svc, broker, repo, func() { d.Close() }
}

func callFrame(t *testing.T, action, uid string, payload interface{}) []byte {
	t.Helper()
	c := codec.New()
	msg, err := c.BuildCall(uid, action, payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := c.Encode(msg)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Test 1: every inbound/outbound frame becomes exactly one log
// row with the raw payload preserved verbatim.
func TestCanonicalLog_InboundAndOutbound_PersistedRaw(t *testing.T) {
	svc, broker, repo, cleanup := newTestService(t)
	defer cleanup()

	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Inject a CALL (CP→server) and a CALLRESULT (server→CP).
	boot := callFrame(t, "BootNotification", "uid-A", map[string]string{
		"chargePointVendor": "V", "chargePointModel": "M",
	})
	broker.deliver(InboundTopicPattern, "ocpp/CP-A/in", boot)

	c := codec.New()
	resp, _ := c.BuildResult("uid-A", map[string]interface{}{"status": "Accepted", "currentTime": "2025-01-01T00:00:00Z", "interval": 300})
	respRaw, _ := c.Encode(resp)
	broker.deliver(OutboundTopicPattern, "ocpp/CP-A/out", respRaw)

	// Wait briefly for the persistence goroutines.
	deadline := time.Now().Add(2 * time.Second)
	for {
		list, err := repo.ListByChargePoint(context.Background(), "CP-A", 100)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(list) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected 2 log rows, got %d", len(list))
		}
		time.Sleep(10 * time.Millisecond)
	}

	list, _ := repo.ListByChargePoint(context.Background(), "CP-A", 100)
	if len(list) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(list))
	}

	// Find one inbound and one outbound (order is newest-first;
	// they may arrive in any order).
	var in, out *db.MessageLog
	for i := range list {
		switch list[i].Direction {
		case "inbound":
			in = &list[i]
		case "outbound":
			out = &list[i]
		}
	}
	if in == nil || out == nil {
		t.Fatalf("missing inbound or outbound: in=%v out=%v", in, out)
	}
	if in.MessageType != "CALL" {
		t.Errorf("inbound messageType = %q, want CALL", in.MessageType)
	}
	if in.PayloadJSON != string(boot) {
		t.Errorf("inbound payload not verbatim: got %q want %q", in.PayloadJSON, boot)
	}
	if out.MessageType != "CALLRESULT" {
		t.Errorf("outbound messageType = %q, want CALLRESULT", out.MessageType)
	}
	if out.PayloadJSON != string(respRaw) {
		t.Errorf("outbound payload not verbatim: got %q want %q", out.PayloadJSON, respRaw)
	}
}

// Test 2: subscription covers both wildcard topics. Without both
// the canonical log would miss a direction.
func TestCanonicalLog_SubscribesBothTopics(t *testing.T) {
	svc, broker, _, cleanup := newTestService(t)
	defer cleanup()
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	broker.mu.Lock()
	in := len(broker.byTopic[InboundTopicPattern])
	out := len(broker.byTopic[OutboundTopicPattern])
	broker.mu.Unlock()
	if in != 1 || out != 1 {
		t.Errorf("expected 1 subscriber each on /in and /out, got in=%d out=%d", in, out)
	}
}

// Test 3: per-CP charge point id is extracted from the originating
// topic, not from the subscription filter.
func TestCanonicalLog_ChargePointIDFromTopic(t *testing.T) {
	svc, broker, repo, cleanup := newTestService(t)
	defer cleanup()
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Two CPs, two different frames. Each must log under its own
	// charge_point_id.
	for _, cp := range []string{"CP-X", "CP-Y"} {
		raw := callFrame(t, "Heartbeat", "uid-"+cp, struct{}{})
		broker.deliver(InboundTopicPattern, "ocpp/"+cp+"/in", raw)
	}

	// Wait until both rows are visible. Allow up to 2s because
	// writes are async through the broker callback.
	deadline := time.Now().Add(2 * time.Second)
	for {
		lx, _ := repo.ListByChargePoint(context.Background(), "CP-X", 100)
		ly, _ := repo.ListByChargePoint(context.Background(), "CP-Y", 100)
		if len(lx) == 1 && len(ly) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected 1 row per CP, got x=%d y=%d", len(lx), len(ly))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// Test 4: parse-error frames still get a log row (status
// "parse_error") so the log is complete even for malformed input.
func TestCanonicalLog_ParseError_StillLogged(t *testing.T) {
	svc, broker, repo, cleanup := newTestService(t)
	defer cleanup()
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	broker.deliver(InboundTopicPattern, "ocpp/CP-E/in", []byte("not a json array"))

	deadline := time.Now().Add(2 * time.Second)
	for {
		list, _ := repo.ListByChargePoint(context.Background(), "CP-E", 100)
		if len(list) == 1 {
			if list[0].Status != "parse_error" {
				t.Errorf("status = %q, want parse_error", list[0].Status)
			}
			if list[0].PayloadJSON != "not a json array" {
				t.Errorf("payload not preserved verbatim: %q", list[0].PayloadJSON)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("expected 1 row for parse error, got 0")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// silence the json import when helpers above are trimmed during
// future edits.
var _ = json.Marshal
var _ sync.Mutex
var _ = message.CALL
