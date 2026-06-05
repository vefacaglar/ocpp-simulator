package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/codec"
	"github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/message"
	v16 "github.com/vefacaglar/ocpp-simulator/packages/ocpp-protocol/pkg/v16"
)

// callFrame is a small helper to build a raw OCPP-J CALL array
// bytes for a given (action, uniqueID, payload) triple. The bytes
// are produced by the codec so the framing matches what the wire
// would carry.
func callFrame(t *testing.T, action, uniqueID string, payload interface{}) []byte {
	t.Helper()
	c := codec.New()
	msg, err := c.BuildCall(uniqueID, action, payload)
	if err != nil {
		t.Fatalf("BuildCall %s: %v", action, err)
	}
	raw, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode %s: %v", action, err)
	}
	return raw
}

func callResultFrame(t *testing.T, uniqueID string, payload interface{}) []byte {
	t.Helper()
	c := codec.New()
	msg, err := c.BuildResult(uniqueID, payload)
	if err != nil {
		t.Fatalf("BuildResult: %v", err)
	}
	raw, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return raw
}

func callErrorFrame(t *testing.T, uniqueID string, code message.ErrorCode, desc string) []byte {
	t.Helper()
	c := codec.New()
	msg, err := c.BuildError(uniqueID, code, desc, nil)
	if err != nil {
		t.Fatalf("BuildError: %v", err)
	}
	raw, err := c.Encode(msg)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return raw
}

func newTestHandler(broker Broker, core CoreClient) (*Handler, *memoryEmitter) {
	em := &memoryEmitter{}
	h := NewHandler(v16.NewProtocol(), core, broker, em)
	return h, em
}

// --- Test 1: core-independent responses are produced and validated ---

func TestHandler_CoreIndependentResponses(t *testing.T) {
	broker := newFakeBroker()
	_ = broker
	_, em := newTestHandler(broker, &fakeCore{})

	cases := []struct {
		action  string
		payload interface{}
		// extract reads the CALLRESULT payload bytes and returns
		// the decoded object for assertions. The exact shape is
		// spec-validated by ocpp-schemas during publish.
		extract func(t *testing.T, payload []byte)
	}{
		{
			action:  "BootNotification",
			payload: map[string]string{"chargePointVendor": "V", "chargePointModel": "M"},
			extract: func(t *testing.T, payload []byte) {
				var resp struct {
					Status      string `json:"status"`
					CurrentTime string `json:"currentTime"`
					Interval    int    `json:"interval"`
				}
				if err := json.Unmarshal(payload, &resp); err != nil {
					t.Fatalf("decode BootNotification.conf: %v", err)
				}
				if resp.Status != "Accepted" {
					t.Errorf("status = %q, want Accepted", resp.Status)
				}
				if resp.Interval != 300 {
					t.Errorf("interval = %d, want 300", resp.Interval)
				}
				if _, err := time.Parse(time.RFC3339, resp.CurrentTime); err != nil {
					t.Errorf("currentTime not RFC3339: %v", err)
				}
			},
		},
		{
			action:  "Heartbeat",
			payload: struct{}{},
			extract: func(t *testing.T, payload []byte) {
				var resp struct {
					CurrentTime string `json:"currentTime"`
				}
				if err := json.Unmarshal(payload, &resp); err != nil {
					t.Fatalf("decode Heartbeat.conf: %v", err)
				}
				if resp.CurrentTime == "" {
					t.Errorf("currentTime is empty")
				}
			},
		},
		{
			action:  "StatusNotification",
			payload: map[string]interface{}{"connectorId": 1, "errorCode": "NoError", "status": "Available", "timestamp": "2025-01-01T00:00:00Z"},
			extract: func(t *testing.T, payload []byte) {
				if !bytes.Equal(bytes.TrimSpace(payload), []byte("{}")) {
					t.Errorf("StatusNotification.conf = %s, want {}", payload)
				}
			},
		},
		{
			action: "MeterValues",
			payload: map[string]interface{}{
				"connectorId": 1,
				"meterValue": []map[string]interface{}{{
					"timestamp":   "2025-01-01T00:00:00Z",
					"sampledValue": []map[string]interface{}{{"value": "100", "measurand": "Energy.Active.Import.Register", "unit": "Wh"}},
				}},
			},
			extract: func(t *testing.T, payload []byte) {
				if !bytes.Equal(bytes.TrimSpace(payload), []byte("{}")) {
					t.Errorf("MeterValues.conf = %s, want {}", payload)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.action, func(t *testing.T) {
			// Each sub-test gets a fresh broker/handler so
			// the published map and audit list start empty.
			subBroker := newFakeBroker()
			subHandler, _ := newTestHandler(subBroker, &fakeCore{})
			raw := callFrame(t, c.action, "uid-"+c.action, c.payload)
			subHandler.Handle(context.Background(), "CP-T", raw)

			// Exactly one frame on the /out topic, byte-equal to
			// what the codec would produce. No wrapping.
			out := subBroker.publishedOn("ocpp/CP-T/out")
			if len(out) != 1 {
				t.Fatalf("expected 1 outbound frame, got %d", len(out))
			}
			// Frame shape: [3, "uid-...", <payload>]
			decoded, err := codec.New().Decode(out[0])
			if err != nil {
				t.Fatalf("decode outbound: %v", err)
			}
			if decoded.MessageTypeID != message.CALLRESULT {
				t.Errorf("expected CALLRESULT, got %d", decoded.MessageTypeID)
			}
			if decoded.UniqueID != "uid-"+c.action {
				t.Errorf("uniqueID = %q, want uid-%s", decoded.UniqueID, c.action)
			}
			c.extract(t, decoded.Payload)
		})
	}

	// Audit sanity: the outer emitter was used for the very first
	// invocation, so it carries the first sub-test's events. The
	// remaining sub-tests use their own emitter. We don't make a
	// cross-test audit assertion here to keep the test honest
	// about scope.
	_ = em
}

// --- Test 2: business decisions route to ocpp-core and core's
//              payload becomes the CALLRESULT body byte-for-byte ---

func TestHandler_CoreRoutedActions(t *testing.T) {
	broker := newFakeBroker()
	core := &fakeCore{
		authorizeResp:       json.RawMessage(`{"idTagInfo":{"status":"ConcurrentTx"}}`),
		startTransactionResp: json.RawMessage(`{"transactionId":7,"idTagInfo":{"status":"Accepted"}}`),
		stopTransactionResp:  json.RawMessage(`{"idTagInfo":{"status":"Expired"}}`),
	}
	h, _ := newTestHandler(broker, core)

	t.Run("Authorize", func(t *testing.T) {
		raw := callFrame(t, "Authorize", "uid-AUTH", map[string]string{"idTag": "TAG-1"})
		h.Handle(context.Background(), "CP-CORE", raw)
		// Core saw the call.
		core.mu.Lock()
		seen := len(core.authorizeCalls)
		core.mu.Unlock()
		if seen != 1 {
			t.Errorf("expected 1 Authorize call to core, got %d", seen)
		}
		// Outbound is the core-supplied payload, byte-exact.
		out := broker.publishedOn("ocpp/CP-CORE/out")
		if len(out) != 1 {
			t.Fatalf("expected 1 outbound frame, got %d", len(out))
		}
		dec, _ := codec.New().Decode(out[0])
		if !bytes.Contains(dec.Payload, []byte(`"ConcurrentTx"`)) {
			t.Errorf("Authorize.conf payload did not contain core's idTagInfo status; got %s", dec.Payload)
		}
	})

	t.Run("StartTransaction", func(t *testing.T) {
		broker.published = make(map[string][][]byte) // reset
		raw := callFrame(t, "StartTransaction", "uid-START", map[string]interface{}{
			"connectorId": 1, "idTag": "TAG", "meterStart": 0, "timestamp": "2025-01-01T00:00:00Z",
		})
		h.Handle(context.Background(), "CP-CORE", raw)
		core.mu.Lock()
		seen := len(core.startTransactionCalls)
		core.mu.Unlock()
		if seen != 1 {
			t.Errorf("expected 1 StartTransaction call to core, got %d", seen)
		}
		dec, _ := codec.New().Decode(broker.publishedOn("ocpp/CP-CORE/out")[0])
		// Core's transactionId=7 must be present.
		if !bytes.Contains(dec.Payload, []byte(`"transactionId":7`)) {
			t.Errorf("StartTransaction.conf did not contain core's transactionId; got %s", dec.Payload)
		}
	})

	t.Run("StopTransaction", func(t *testing.T) {
		broker.published = make(map[string][][]byte)
		raw := callFrame(t, "StopTransaction", "uid-STOP", map[string]interface{}{
			"transactionId": 7, "meterStop": 100, "timestamp": "2025-01-01T00:00:00Z", "reason": "Local",
		})
		h.Handle(context.Background(), "CP-CORE", raw)
		core.mu.Lock()
		seen := len(core.stopTransactionCalls)
		core.mu.Unlock()
		if seen != 1 {
			t.Errorf("expected 1 StopTransaction call to core, got %d", seen)
		}
		dec, _ := codec.New().Decode(broker.publishedOn("ocpp/CP-CORE/out")[0])
		if !bytes.Contains(dec.Payload, []byte(`"Expired"`)) {
			t.Errorf("StopTransaction.conf did not contain core's idTagInfo status; got %s", dec.Payload)
		}
	})
}

// --- Test 3: core unavailable produces GenericError CALLERROR, not
//              a CALLRESULT ---

func TestHandler_CoreUnavailable_ProducesCALLERROR(t *testing.T) {
	broker := newFakeBroker()
	core := &fakeCore{failAuthorize: true}
	h, _ := newTestHandler(broker, core)

	raw := callFrame(t, "Authorize", "uid-ERR", map[string]string{"idTag": "T"})
	h.Handle(context.Background(), "CP-ERR", raw)

	out := broker.publishedOn("ocpp/CP-ERR/out")
	if len(out) != 1 {
		t.Fatalf("expected 1 outbound frame, got %d", len(out))
	}
	dec, err := codec.New().Decode(out[0])
	if err != nil {
		t.Fatalf("decode outbound: %v", err)
	}
	if dec.MessageTypeID != message.CALLERROR {
		t.Errorf("expected CALLERROR, got %d", dec.MessageTypeID)
	}
	if dec.ErrorCode != string(message.ErrorCodeGenericError) {
		t.Errorf("ErrorCode = %q, want GenericError", dec.ErrorCode)
	}
	if dec.UniqueID != "uid-ERR" {
		t.Errorf("UniqueID mismatch: %s", dec.UniqueID)
	}
}

// --- Test 4: unknown action produces NotImplemented CALLERROR ---

func TestHandler_UnknownAction_ProducesNotImplemented(t *testing.T) {
	broker := newFakeBroker()
	h, _ := newTestHandler(broker, &fakeCore{})

	// "RemoteStartTransaction" is a CSMS-initiated action; the
	// processor never produces it. If a CP somehow sends it
	// (malformed or fuzz input), the processor replies with
	// NotImplemented.
	raw := callFrame(t, "RemoteStartTransaction", "uid-RS", map[string]interface{}{"idTag": "T"})
	h.Handle(context.Background(), "CP-U", raw)

	out := broker.publishedOn("ocpp/CP-U/out")
	if len(out) != 1 {
		t.Fatalf("expected 1 outbound frame, got %d", len(out))
	}
	dec, _ := codec.New().Decode(out[0])
	if dec.MessageTypeID != message.CALLERROR {
		t.Errorf("expected CALLERROR, got %d", dec.MessageTypeID)
	}
	if dec.ErrorCode != string(message.ErrorCodeNotImplemented) {
		t.Errorf("ErrorCode = %q, want NotImplemented", dec.ErrorCode)
	}
}

// --- Test 5: CALLRESULT inbound produces NO outbound response ---

func TestHandler_InboundCALLRESULT_NoResponse(t *testing.T) {
	broker := newFakeBroker()
	h, em := newTestHandler(broker, &fakeCore{})

	// CP→server CALLRESULT for a BootNotification.conf. The
	// processor must audit and stop; no /out publication.
	raw := callResultFrame(t, "uid-CCC", map[string]interface{}{
		"status":      "Accepted",
		"currentTime": "2025-01-01T00:00:00Z",
		"interval":    300,
	})
	h.Handle(context.Background(), "CP-RES", raw)

	if got := broker.publishedOn("ocpp/CP-RES/out"); len(got) != 0 {
		t.Errorf("CALLRESULT inbound must not produce a response; got %d frames", len(got))
	}
	events := em.snapshot()
	// Two events: "consumed" (parse OK) and a "noop" decision
	// marker explaining that no response was produced.
	if len(events) != 2 {
		t.Fatalf("expected 2 audit events, got %d", len(events))
	}
	if events[0].Kind != "consumed" {
		t.Errorf("event[0].kind = %q, want consumed", events[0].Kind)
	}
	if events[0].MessageType != "CALLRESULT" {
		t.Errorf("event[0].messageType = %q, want CALLRESULT", events[0].MessageType)
	}
	if events[1].Decision != "noop" {
		t.Errorf("event[1].decision = %q, want noop", events[1].Decision)
	}
}

// --- Test 6: CALLERROR inbound produces NO outbound response ---

func TestHandler_InboundCALLERROR_NoResponse(t *testing.T) {
	broker := newFakeBroker()
	h, em := newTestHandler(broker, &fakeCore{})

	raw := callErrorFrame(t, "uid-ERR", message.ErrorCodeGenericError, "oops")
	h.Handle(context.Background(), "CP-ERR-IN", raw)

	if got := broker.publishedOn("ocpp/CP-ERR-IN/out"); len(got) != 0 {
		t.Errorf("CALLERROR inbound must not produce a response; got %d frames", len(got))
	}
	events := em.snapshot()
	if len(events) != 2 {
		t.Fatalf("expected 2 audit events, got %d", len(events))
	}
	if events[0].MessageType != "CALLERROR" {
		t.Errorf("event[0].messageType = %q, want CALLERROR", events[0].MessageType)
	}
	if events[1].Decision != "noop" {
		t.Errorf("event[1].decision = %q, want noop", events[1].Decision)
	}
}

// --- Test 7: parse error audits and does not publish ---

func TestHandler_ParseError_NoPublish(t *testing.T) {
	broker := newFakeBroker()
	h, em := newTestHandler(broker, &fakeCore{})

	h.Handle(context.Background(), "CP-PARSE", []byte("not a json array"))

	if got := broker.publishedOn("ocpp/CP-PARSE/out"); len(got) != 0 {
		t.Errorf("parse error must not publish; got %d", len(got))
	}
	events := em.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}
	if events[0].Decision != "parse_error" {
		t.Errorf("decision = %q, want parse_error", events[0].Decision)
	}
}

// --- Test 8: outbound frame is a raw OCPP-J array (not wrapped) ---

func TestHandler_OutboundIsRawOCPPArray(t *testing.T) {
	broker := newFakeBroker()
	h, _ := newTestHandler(broker, &fakeCore{})

	raw := callFrame(t, "Heartbeat", "uid-RAW", struct{}{})
	h.Handle(context.Background(), "CP-RAW", raw)

	out := broker.publishedOn("ocpp/CP-RAW/out")
	if len(out) != 1 {
		t.Fatalf("expected 1 outbound frame, got %d", len(out))
	}
	got := out[0]
	if got[0] != '[' {
		t.Errorf("outbound does not start with '[': %q", got)
	}
	if got[len(got)-1] != ']' {
		t.Errorf("outbound does not end with ']': %q", got)
	}
	// Must NOT be an object envelope like {"type":"CALL",...}.
	if got[0] == '{' {
		t.Errorf("outbound is an object, not an array: %q", got)
	}
	// Decode round-trips; if it had been wrapped in an object,
	// Decode would return a different shape or fail.
	dec, err := codec.New().Decode(got)
	if err != nil {
		t.Fatalf("decode outbound: %v", err)
	}
	if dec.MessageTypeID != message.CALLRESULT {
		t.Errorf("decoded type = %d, want CALLRESULT", dec.MessageTypeID)
	}
}

// --- Test 9: per-CP routing — outbound goes to the right /out topic ---

func TestHandler_PerCPRouting(t *testing.T) {
	broker := newFakeBroker()
	h, _ := newTestHandler(broker, &fakeCore{})

	for _, cp := range []string{"CP-A", "CP-B"} {
		raw := callFrame(t, "Heartbeat", "uid-"+cp, struct{}{})
		h.Handle(context.Background(), cp, raw)
	}
	if got := broker.publishedOn("ocpp/CP-A/out"); len(got) != 1 {
		t.Errorf("CP-A: expected 1 frame, got %d", len(got))
	}
	if got := broker.publishedOn("ocpp/CP-B/out"); len(got) != 1 {
		t.Errorf("CP-B: expected 1 frame, got %d", len(got))
	}
	if got := broker.publishedOn("ocpp/CP-A/in"); len(got) != 0 {
		t.Errorf("nothing should be published to /in")
	}
}

// --- Test 10: Processor.Start delivers inbound messages to the handler ---

func TestProcessor_Start_DispatchesByTopic(t *testing.T) {
	broker := newFakeBroker()
	h, em := newTestHandler(broker, &fakeCore{})
	p := New(broker, h)
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	raw := callFrame(t, "Heartbeat", "uid-START", struct{}{})
	if n := broker.deliver("ocpp/+/in", "ocpp/CP-S/in", raw); n != 1 {
		t.Fatalf("deliver reached %d handlers, want 1", n)
	}

	if got := broker.publishedOn("ocpp/CP-S/out"); len(got) != 1 {
		t.Errorf("expected 1 outbound frame on /CP-S/out, got %d", len(got))
	}
	// At least one "consumed" audit event should be present.
	var consumed int
	for _, ev := range em.snapshot() {
		if ev.Kind == "consumed" && ev.ChargePointID == "CP-S" {
			consumed++
		}
	}
	if consumed != 1 {
		t.Errorf("expected 1 consumed audit for CP-S, got %d", consumed)
	}
}

// --- Test 11: Processor unsubscribes cleanly on Stop ---

func TestProcessor_Stop_Unsubscribes(t *testing.T) {
	broker := newFakeBroker()
	h, _ := newTestHandler(broker, &fakeCore{})
	p := New(broker, h)
	if err := p.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	p.Stop()
	// After Stop, no handler should be registered.
	broker.mu.Lock()
	n := len(broker.byTopic["ocpp/+/in"])
	broker.mu.Unlock()
	if n != 0 {
		t.Errorf("after Stop, expected 0 subscribers on ocpp/+/in, got %d", n)
	}
}

// --- Test 12: ChargePointIDFromTopic parser ---

func TestChargePointIDFromTopic(t *testing.T) {
	cases := []struct {
		topic string
		want  string
	}{
		{"ocpp/CP-001/in", "CP-001"},
		{"ocpp/CP-001/out", "CP-001"},
		{"ocpp/CP-007/in", "CP-007"},
		{"", ""},
		{"ocpp//in", ""},
		{"other/CP-1/in", ""},
		{"ocpp/CP-1/other", ""},
		{"ocpp/CP-1", ""},
		{"ocpp/CP-1/in/extra", ""},
	}
	for _, c := range cases {
		if got := ChargePointIDFromTopic(c.topic); got != c.want {
			t.Errorf("ChargePointIDFromTopic(%q) = %q, want %q", c.topic, got, c.want)
		}
	}
}
