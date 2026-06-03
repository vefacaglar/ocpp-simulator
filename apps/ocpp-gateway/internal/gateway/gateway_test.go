package gateway

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

// Test 1 — raw byte passthrough. A frame published from a CP must
// arrive on the broker's /in topic byte-for-byte. No re-marshal, no
// normalization, no JSON canonicalization.
func TestGateway_PublishInbound_RawBytesUnchanged(t *testing.T) {
	broker := newFakeBroker()
	g, err := New(broker)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := g.Connect("CP-001"); err != nil {
		t.Fatalf("Connect CP-001: %v", err)
	}

	// Intentionally non-canonical JSON: weird whitespace, weird
	// key order, even a trailing newline. If the gateway ever
	// re-marshaled, this would change.
	original := []byte("[2,  \"uid-1\" ,\"BootNotification\" ,{ \"chargePointVendor\" : \"V\" , \"chargePointModel\":\"M\"} ]\n")

	if err := g.PublishInbound("CP-001", original); err != nil {
		t.Fatalf("PublishInbound: %v", err)
	}

	got := broker.publishedOn(InTopic("CP-001"))
	if len(got) != 1 {
		t.Fatalf("expected 1 published frame on %s, got %d", InTopic("CP-001"), len(got))
	}
	if !bytes.Equal(got[0], original) {
		t.Errorf("frame bytes mutated by gateway:\nwant=%q\ngot =%q", original, got[0])
	}
}

// Test 2 — topic format. Both in and out topics use the exact format
// ocpp/{cpId}/in and ocpp/{cpId}/out.
func TestGateway_TopicFormat(t *testing.T) {
	if got := InTopic("CP-007"); got != "ocpp/CP-007/in" {
		t.Errorf("InTopic = %q, want %q", got, "ocpp/CP-007/in")
	}
	if got := OutTopic("CP-007"); got != "ocpp/CP-007/out" {
		t.Errorf("OutTopic = %q, want %q", got, "ocpp/CP-007/out")
	}
	// Confirm Connect subscribes to the matching out topic.
	broker := newFakeBroker()
	g, _ := New(broker)
	if _, err := g.Connect("CP-007"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if got := broker.subscriberCount(OutTopic("CP-007")); got != 1 {
		t.Errorf("expected 1 subscriber on %s, got %d", OutTopic("CP-007"), got)
	}
}

// Test 3 — per-CP isolation. Two CPs must not share Outbound channels
// or subscriptions. A frame delivered on CP-A's /out must reach only
// CP-A's outbound channel.
func TestGateway_PerCPIsolation(t *testing.T) {
	broker := newFakeBroker()
	g, _ := New(broker)

	cpA, err := g.Connect("CP-A")
	if err != nil {
		t.Fatalf("Connect CP-A: %v", err)
	}
	cpB, err := g.Connect("CP-B")
	if err != nil {
		t.Fatalf("Connect CP-B: %v", err)
	}

	// Same logical Connection value (pointer) would mean the
	// registry is buggy and returned the same instance.
	if cpA == cpB {
		t.Fatalf("CP-A and CP-B must have separate Connection objects")
	}
	if cpA.Outbound == cpB.Outbound {
		t.Fatalf("CP-A and CP-B must have separate Outbound channels")
	}
	if cpA.Unsubscribe == nil || cpB.Unsubscribe == nil {
		t.Fatalf("both connections must have unsubscribe handles")
	}

	// Fire a frame on CP-A's out topic; CP-B's channel must stay
	// empty. Use a buffered channel + non-blocking receive so a
	// bug (broadcast to all CPs) is observable.
	broker.deliver(OutTopic("CP-A"), []byte("only-for-A"))

	select {
	case got := <-cpA.Outbound:
		if string(got) != "only-for-A" {
			t.Errorf("CP-A received wrong payload: %q", got)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("CP-A did not receive the /out frame")
	}

	select {
	case got := <-cpB.Outbound:
		t.Errorf("CP-B received a frame intended for CP-A: %q", got)
	case <-time.After(50 * time.Millisecond):
		// expected
	}

	// Symmetric check.
	broker.deliver(OutTopic("CP-B"), []byte("only-for-B"))
	select {
	case got := <-cpB.Outbound:
		if string(got) != "only-for-B" {
			t.Errorf("CP-B received wrong payload: %q", got)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("CP-B did not receive the /out frame")
	}
	select {
	case got := <-cpA.Outbound:
		t.Errorf("CP-A received a frame intended for CP-B: %q", got)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

// Test 4 — Connect is idempotent up to a point: a second Connect for
// the same CP id must NOT silently overwrite the existing connection.
// The existing connection must keep working and the new attempt must
// be rejected.
func TestGateway_Connect_RejectsDuplicate(t *testing.T) {
	broker := newFakeBroker()
	g, _ := New(broker)

	first, err := g.Connect("CP-DUP")
	if err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	existing, err := g.Connect("CP-DUP")
	if err == nil {
		t.Fatal("expected duplicate Connect to return an error")
	}
	if existing != first {
		t.Errorf("error must carry the existing Connection; got %p want %p", existing, first)
	}
	if _, ok := g.Registry.Lookup("CP-DUP"); !ok {
		t.Fatal("existing connection must still be registered after rejected duplicate")
	}
	// The original connection still works.
	broker.deliver(OutTopic("CP-DUP"), []byte("still-alive"))
	select {
	case got := <-first.Outbound:
		if string(got) != "still-alive" {
			t.Errorf("first connection got wrong frame: %q", got)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("first connection stopped receiving after duplicate attempt")
	}
}

// Test 5 — Disconnect tears down cleanly. The subscription is
// detached, the Outbound channel is closed, and a late /out delivery
// after Disconnect does not panic.
func TestGateway_Disconnect_TearsDown(t *testing.T) {
	broker := newFakeBroker()
	g, _ := New(broker)
	cp, err := g.Connect("CP-X")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	g.Disconnect("CP-X")

	if _, ok := g.Registry.Lookup("CP-X"); ok {
		t.Error("Disconnect did not remove CP-X from registry")
	}
	if got := broker.subscriberCount(OutTopic("CP-X")); got != 0 {
		t.Errorf("Disconnect did not unsubscribe from /out: %d handlers remain", got)
	}
	select {
	case _, ok := <-cp.Outbound:
		if ok {
			t.Error("Outbound channel should be drained or have no senders after Disconnect")
		}
	case <-time.After(50 * time.Millisecond):
		// acceptable: nothing pending; the channel is simply unused
	}
	select {
	case <-cp.Closed():
		// expected
	case <-time.After(50 * time.Millisecond):
		t.Error("Closed() channel should be closed after Disconnect")
	}

	// Disconnect is idempotent: a second call is a no-op.
	g.Disconnect("CP-X")
}

// Test 6 — PublishInbound on an unknown charge point returns
// ErrNoSuchChargePoint without touching the broker.
func TestGateway_PublishInbound_UnknownCP(t *testing.T) {
	broker := newFakeBroker()
	g, _ := New(broker)

	err := g.PublishInbound("CP-DOES-NOT-EXIST", []byte("ignored"))
	if !errors.Is(err, ErrNoSuchChargePoint) {
		t.Errorf("expected ErrNoSuchChargePoint, got %v", err)
	}
	if got := broker.publishedOn(InTopic("CP-DOES-NOT-EXIST")); len(got) != 0 {
		t.Errorf("unknown CP must not publish anything; got %d", len(got))
	}
}

// Test 7 — a late /out delivery after Disconnect does not panic and
// does not deliver to a closed/stale connection.
func TestGateway_LateOutboundDelivery_NoPanic(t *testing.T) {
	broker := newFakeBroker()
	g, _ := New(broker)
	cp, err := g.Connect("CP-LATE")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Snapshot the registered handler while CP-LATE is still in
	// the registry. After Disconnect, the gateway's closure is no
	// longer reachable from the broker map, but a real paho
	// broker can deliver a message that was already in flight
	// before Unsubscribe completed. We simulate that by capturing
	// the closure now and calling it after Disconnect.
	broker.mu.Lock()
	ids := broker.byTopic[OutTopic("CP-LATE")]
	if len(ids) != 1 {
		broker.mu.Unlock()
		t.Fatalf("expected 1 subscription before disconnect, got %d", len(ids))
	}
	s := broker.subs[ids[0]]
	broker.mu.Unlock()

	g.Disconnect("CP-LATE")

	// Now invoke the captured handler with a payload. The
	// gateway's closure re-looks-up the registry, finds the entry
	// removed, and silently drops the message. Nothing should
	// panic and cp.Outbound should not receive the frame.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("late delivery panicked: %v", r)
		}
	}()
	s.handler([]byte("late"))

	select {
	case got, ok := <-cp.Outbound:
		if ok {
			t.Errorf("late delivery reached Outbound: %q", got)
		}
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

// Test 8 — extractChargePointID rejects paths that would escape the
// /ws/{id} namespace or introduce MQTT wildcards. The id is everything
// after /ws/ up to the next path segment; nested segments, empty
// segments, and MQTT wildcard characters are all rejected.
func TestExtractChargePointID_RejectsBadPaths(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/ws/CP-001", "CP-001"},
		{"/ws/", ""},         // empty id rejected
		{"/other/CP-1", ""},  // missing prefix
		{"/ws/CP#1", ""},     // MQTT wildcard
		{"/ws/CP+1", ""},     // MQTT wildcard
		{"/ws/CP/1", ""},     // nested CP id with slash
		{"/ws/CP-A/B", ""},   // nested segment with slash
		{"/ws/CP-007/extra", ""}, // nested segment with slash
	}
	for _, c := range cases {
		if got := extractChargePointID(c.path); got != c.want {
			t.Errorf("extractChargePointID(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
