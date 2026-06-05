package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/vefacaglar/ocpp-simulator/apps/ocpp-core/internal/csms"
	"github.com/vefacaglar/ocpp-simulator/apps/ocpp-core/internal/db"
	"github.com/vefacaglar/ocpp-simulator/apps/ocpp-core/internal/transaction"
)

// fakeBroker captures Publish for assertions.
type fakeBroker struct {
	mu        sync.Mutex
	published map[string][][]byte
}

func newFakeBroker() *fakeBroker { return &fakeBroker{published: make(map[string][][]byte)} }
func (b *fakeBroker) Publish(topic string, payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	cp := make([]byte, len(payload))
	copy(cp, payload)
	b.published[topic] = append(b.published[topic], cp)
	return nil
}
func (b *fakeBroker) Subscribe(string, func(string, []byte)) (func(), error) {
	return func() {}, nil
}
func (b *fakeBroker) IsConnected() bool               { return true }
func (b *fakeBroker) Connect(_ context.Context) error { return nil }
func (b *fakeBroker) Disconnect() error               { return nil }
func (b *fakeBroker) snapshot(topic string) [][]byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([][]byte, len(b.published[topic]))
	copy(out, b.published[topic])
	return out
}

// makeServer wires a Server backed by the shared postgres test DB and a
// fake broker. Tests don't need to start a real HTTP listener;
// they use httptest.NewRecorder.
func makeServer(t *testing.T) (*Server, *fakeBroker, *transaction.Service, func()) {
	t.Helper()
	d, err := db.Open(db.TestDBURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	repo := db.NewTransactionRepo(d)
	logs := db.NewMessageLogRepo(d)
	rt := db.NewRuntimeEventRepo(d)
	_ = logs

	// Clean up state before test
	for _, tbl := range []string{"transactions", "ocpp_message_logs", "runtime_events"} {
		_, _ = d.ExecContext(context.Background(), "TRUNCATE TABLE "+tbl+" RESTART IDENTITY CASCADE")
	}

	br := newFakeBroker()
	txSvc := transaction.NewService(repo)
	if err := txSvc.InitCounter(context.Background()); err != nil {
		t.Fatal(err)
	}
	csmsSvc := csms.New(br)
	srv := NewServer(txSvc, csmsSvc, rt, repo)
	return srv, br, txSvc, func() { d.Close() }
}

func postEnvelope(t *testing.T, srv *Server, path string, env envelope) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(env)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestAPI_Authorize_AcceptedEnvelope(t *testing.T) {
	srv, _, _, cleanup := makeServer(t)
	defer cleanup()

	env := envelope{
		ChargePointID: "CP-001",
		Payload:       json.RawMessage(`{"idTag":"TAG-1"}`),
	}
	w := postEnvelope(t, srv, "/internal/transactions/authorize", env)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var got envelope
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ChargePointID != "CP-001" {
		t.Errorf("chargePointId = %q", got.ChargePointID)
	}
	var payload struct {
		IDTagInfo struct {
			Status string `json:"status"`
		} `json:"idTagInfo"`
	}
	if err := json.Unmarshal(got.Payload, &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if payload.IDTagInfo.Status != "Accepted" {
		t.Errorf("idTagInfo.status = %q, want Accepted", payload.IDTagInfo.Status)
	}
}

func TestAPI_Start_CreatesTransactionWithDualIdentity(t *testing.T) {
	srv, _, _, cleanup := makeServer(t)
	defer cleanup()

	env := envelope{
		ChargePointID: "CP-001",
		Payload: json.RawMessage(`{
			"connectorId": 1, "idTag": "TAG-1", "meterStart": 0, "timestamp": "2025-01-01T00:00:00Z"
		}`),
	}
	w := postEnvelope(t, srv, "/internal/transactions/start", env)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var got envelope
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	var payload struct {
		TransactionID int `json:"transactionId"`
		IDTagInfo     struct {
			Status string `json:"status"`
		} `json:"idTagInfo"`
	}
	if err := json.Unmarshal(got.Payload, &payload); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if payload.TransactionID != 1 {
		t.Errorf("transactionId = %d, want 1 (counter was empty)", payload.TransactionID)
	}
	if payload.IDTagInfo.Status != "Accepted" {
		t.Errorf("idTagInfo.status = %q, want Accepted", payload.IDTagInfo.Status)
	}
}

func TestAPI_Start_ReusesCounterFromDB(t *testing.T) {
	// First start populates the counter; a second server
	// initialized from the same DB must continue from 2, proving
	// restart-safety.
	d, err := db.Open(db.TestDBURL)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	// Clean up before test
	for _, tbl := range []string{"transactions", "ocpp_message_logs", "runtime_events"} {
		_, _ = d.ExecContext(context.Background(), "TRUNCATE TABLE "+tbl+" RESTART IDENTITY CASCADE")
	}
	repo := db.NewTransactionRepo(d)
	logs := db.NewMessageLogRepo(d)
	_ = logs
	rt := db.NewRuntimeEventRepo(d)
	br := newFakeBroker()
	csmsSvc := csms.New(br)

	// First instance: counter empty, first start gives 1.
	svc1 := transaction.NewService(repo)
	if err := svc1.InitCounter(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv1 := NewServer(svc1, csmsSvc, rt, repo)
	w := postEnvelope(t, srv1, "/internal/transactions/start", envelope{
		ChargePointID: "CP",
		Payload:       json.RawMessage(`{"connectorId":1,"idTag":"T","meterStart":0,"timestamp":"2025-01-01T00:00:00Z"}`),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("first start: %d %s", w.Code, w.Body.String())
	}
	var r1 envelope
	_ = json.Unmarshal(w.Body.Bytes(), &r1)
	var p1 struct {
		TransactionID int `json:"transactionId"`
	}
	_ = json.Unmarshal(r1.Payload, &p1)
	if p1.TransactionID != 1 {
		t.Errorf("first transactionId = %d, want 1", p1.TransactionID)
	}

	// Second instance: same DB. InitCounter must read MAX=1
	// and the next Start must yield 2.
	svc2 := transaction.NewService(repo)
	if err := svc2.InitCounter(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv2 := NewServer(svc2, csmsSvc, rt, repo)
	w = postEnvelope(t, srv2, "/internal/transactions/start", envelope{
		ChargePointID: "CP",
		Payload:       json.RawMessage(`{"connectorId":1,"idTag":"T","meterStart":0,"timestamp":"2025-01-01T00:00:00Z"}`),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("second start: %d %s", w.Code, w.Body.String())
	}
	var r2 envelope
	_ = json.Unmarshal(w.Body.Bytes(), &r2)
	var p2 struct {
		TransactionID int `json:"transactionId"`
	}
	_ = json.Unmarshal(r2.Payload, &p2)
	if p2.TransactionID != 2 {
		t.Errorf("second transactionId = %d, want 2 (restart-safe)", p2.TransactionID)
	}
}

func TestAPI_Stop_FinalizesByNumericID(t *testing.T) {
	srv, _, _, cleanup := makeServer(t)
	defer cleanup()

	// Start a transaction so we have a numeric id to stop.
	w := postEnvelope(t, srv, "/internal/transactions/start", envelope{
		ChargePointID: "CP-S",
		Payload:       json.RawMessage(`{"connectorId":1,"idTag":"T","meterStart":0,"timestamp":"2025-01-01T00:00:00Z"}`),
	})
	if w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}

	// Stop by numeric id 1.
	w = postEnvelope(t, srv, "/internal/transactions/stop", envelope{
		ChargePointID: "CP-S",
		Payload:       json.RawMessage(`{"transactionId":1,"meterStop":1234,"timestamp":"2025-01-01T01:00:00Z","reason":"Local"}`),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var got envelope
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	var payload struct {
		IDTagInfo *struct {
			Status string `json:"status"`
		} `json:"idTagInfo"`
	}
	_ = json.Unmarshal(got.Payload, &payload)
	if payload.IDTagInfo == nil || payload.IDTagInfo.Status != "Accepted" {
		t.Errorf("idTagInfo.status missing or wrong: %+v", payload.IDTagInfo)
	}
}

func TestAPI_RemoteStart_PublishesCALL(t *testing.T) {
	srv, br, _, cleanup := makeServer(t)
	defer cleanup()

	connID := 1
	w := postEnvelope(t, srv, "/internal/csms/remote-start", envelope{
		ChargePointID: "CP-CSMS",
		Payload:       json.RawMessage(`{"idTag":"T","connectorId":1}`),
		// Note: connectorId is an int but we want a *int in the
		// typed struct; the JSON decoder is permissive about
		// numeric values, so this works.
	})
	_ = connID
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	out := br.snapshot("ocpp/1.6J/CP-CSMS/out")
	if len(out) != 1 {
		t.Fatalf("expected 1 outbound frame, got %d", len(out))
	}
	if out[0][0] != '[' {
		t.Errorf("outbound not a JSON array: %q", out[0])
	}
}

func TestAPI_RemoteStart_RejectsMissingIDTag(t *testing.T) {
	srv, br, _, cleanup := makeServer(t)
	defer cleanup()
	w := postEnvelope(t, srv, "/internal/csms/remote-start", envelope{
		ChargePointID: "CP-CSMS",
		Payload:       json.RawMessage(`{}`),
	})
	if w.Code == http.StatusAccepted {
		t.Errorf("expected 4xx, got 202")
	}
	if out := br.snapshot("ocpp/CP-CSMS/out"); len(out) != 0 {
		t.Errorf("must not publish on validation failure")
	}
}

func TestAPI_Health(t *testing.T) {
	srv, _, _, cleanup := makeServer(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

// --- 2.0.1 TransactionEvent (Started / Ended) ---

// TestAPI_TransactionEvent_Started_CreatesTransaction: a 2.0.1
// Started event POSTed through /internal/transactions/event must
// create a row with the CP-chosen GUID and no numeric_id. The
// response payload is the spec-exact TransactionEvent.conf body
// { idTokenInfo: { status: "Accepted" } }.
func TestAPI_TransactionEvent_Started_CreatesTransaction(t *testing.T) {
	srv, _, _, cleanup := makeServer(t)
	defer cleanup()

	payload := []byte(`{
		"eventType":"Started",
		"timestamp":"2025-01-01T00:00:00Z",
		"triggerReason":"CablePluggedIn",
		"seqNo":0,
		"transactionInfo":{"transactionId":"tx-guid-1"},
		"evse":{"id":1,"connectorId":1},
		"idToken":{"idToken":"TOKEN-1","type":"ISO14443"}
	}`)
	w := postEnvelope(t, srv, "/internal/transactions/event", envelope{
		ChargePointID: "CP-TE",
		Version:       "2.0.1",
		Payload:       json.RawMessage(payload),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	var got envelope
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var conf struct {
		IDTokenInfo struct {
			Status string `json:"status"`
		} `json:"idTokenInfo"`
	}
	if err := json.Unmarshal(got.Payload, &conf); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if conf.IDTokenInfo.Status != "Accepted" {
		t.Errorf("idTokenInfo.status = %q, want Accepted", conf.IDTokenInfo.Status)
	}
}

// TestAPI_TransactionEvent_Ended_Finalizes: a Started + Ended
// sequence on the same GUID must succeed; the Ended response is
// also { idTokenInfo: { status: "Accepted" } }.
func TestAPI_TransactionEvent_Ended_Finalizes(t *testing.T) {
	srv, _, _, cleanup := makeServer(t)
	defer cleanup()

	guid := "tx-guid-ended"
	startPayload := []byte(`{
		"eventType":"Started",
		"timestamp":"2025-01-01T00:00:00Z",
		"triggerReason":"CablePluggedIn",
		"seqNo":0,
		"transactionInfo":{"transactionId":"` + guid + `"},
		"evse":{"id":1}
	}`)
	if w := postEnvelope(t, srv, "/internal/transactions/event", envelope{
		ChargePointID: "CP-TE2",
		Version:       "2.0.1",
		Payload:       json.RawMessage(startPayload),
	}); w.Code != http.StatusOK {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}

	endPayload := []byte(`{
		"eventType":"Ended",
		"timestamp":"2025-01-01T01:00:00Z",
		"triggerReason":"EVDeparted",
		"seqNo":1,
		"transactionInfo":{"transactionId":"` + guid + `"},
		"evse":{"id":1}
	}`)
	w := postEnvelope(t, srv, "/internal/transactions/event", envelope{
		ChargePointID: "CP-TE2",
		Version:       "2.0.1",
		Payload:       json.RawMessage(endPayload),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("end: %d %s", w.Code, w.Body.String())
	}
	var got envelope
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var conf struct {
		IDTokenInfo struct {
			Status string `json:"status"`
		} `json:"idTokenInfo"`
	}
	_ = json.Unmarshal(got.Payload, &conf)
	if conf.IDTokenInfo.Status != "Accepted" {
		t.Errorf("idTokenInfo.status = %q, want Accepted", conf.IDTokenInfo.Status)
	}
}

// TestAPI_TransactionEvent_RejectsMissingTransactionId: the spec
// requires transactionInfo.transactionId; a missing id is a 400.
func TestAPI_TransactionEvent_RejectsMissingTransactionId(t *testing.T) {
	srv, _, _, cleanup := makeServer(t)
	defer cleanup()

	payload := []byte(`{
		"eventType":"Started",
		"timestamp":"2025-01-01T00:00:00Z",
		"triggerReason":"CablePluggedIn",
		"seqNo":0,
		"evse":{"id":1}
	}`)
	w := postEnvelope(t, srv, "/internal/transactions/event", envelope{
		ChargePointID: "CP-TE3",
		Version:       "2.0.1",
		Payload:       json.RawMessage(payload),
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// --- 2.0.1 CSMS-initiated: RequestStartTransaction / RequestStopTransaction ---

// TestAPI_RequestStartTransaction_PublishesSpecExactCall: a
// /internal/csms/request-start-transaction POST must publish a
// spec-exact RequestStartTransaction CALL on
// ocpp/2.0.1/{cpId}/out. The payload uses the structured
// {idToken, type} and the mandatory remoteStartId.
func TestAPI_RequestStartTransaction_PublishesSpecExactCall(t *testing.T) {
	srv, br, _, cleanup := makeServer(t)
	defer cleanup()

	payload := []byte(`{
		"idToken":{"idToken":"TOKEN-7","type":"ISO14443"},
		"remoteStartId":42
	}`)
	w := postEnvelope(t, srv, "/internal/csms/request-start-transaction", envelope{
		ChargePointID: "CP-RS201",
		Version:       "2.0.1",
		Payload:       json.RawMessage(payload),
	})
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	out := br.snapshot("ocpp/2.0.1/CP-RS201/out")
	if len(out) != 1 {
		t.Fatalf("expected 1 outbound frame, got %d", len(out))
	}
	raw := out[0]
	var frame []json.RawMessage
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(frame) != 4 {
		t.Fatalf("frame arity = %d, want 4", len(frame))
	}
	var typeID int
	_ = json.Unmarshal(frame[0], &typeID)
	if typeID != 2 {
		t.Errorf("typeID = %d, want 2 (CALL)", typeID)
	}
	var action string
	_ = json.Unmarshal(frame[2], &action)
	if action != "RequestStartTransaction" {
		t.Errorf("action = %q", action)
	}
	var body struct {
		IDToken       struct{ IDToken, Type string } `json:"idToken"`
		RemoteStartID int                            `json:"remoteStartId"`
	}
	_ = json.Unmarshal(frame[3], &body)
	if body.IDToken.IDToken != "TOKEN-7" || body.IDToken.Type != "ISO14443" {
		t.Errorf("idToken = %+v", body.IDToken)
	}
	if body.RemoteStartID != 42 {
		t.Errorf("remoteStartId = %d", body.RemoteStartID)
	}
}

// TestAPI_RequestStartTransaction_RejectsMissingIDToken: a
// missing idToken is a 400, and nothing is published.
func TestAPI_RequestStartTransaction_RejectsMissingIDToken(t *testing.T) {
	srv, br, _, cleanup := makeServer(t)
	defer cleanup()

	w := postEnvelope(t, srv, "/internal/csms/request-start-transaction", envelope{
		ChargePointID: "CP-RS201",
		Version:       "2.0.1",
		Payload:       json.RawMessage(`{"remoteStartId":1}`),
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if out := br.snapshot("ocpp/2.0.1/CP-RS201/out"); len(out) != 0 {
		t.Errorf("must not publish on validation failure, got %d frames", len(out))
	}
}

// TestAPI_RequestStopTransaction_PublishesSpecExactCall: a
// /internal/csms/request-stop-transaction POST must publish a
// RequestStopTransaction CALL with the string transactionId in
// the payload.
func TestAPI_RequestStopTransaction_PublishesSpecExactCall(t *testing.T) {
	srv, br, _, cleanup := makeServer(t)
	defer cleanup()

	payload := []byte(`{"transactionId":"tx-stop-9"}`)
	w := postEnvelope(t, srv, "/internal/csms/request-stop-transaction", envelope{
		ChargePointID: "CP-ST201",
		Version:       "2.0.1",
		Payload:       json.RawMessage(payload),
	})
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	out := br.snapshot("ocpp/2.0.1/CP-ST201/out")
	if len(out) != 1 {
		t.Fatalf("expected 1 outbound frame, got %d", len(out))
	}
	var frame []json.RawMessage
	_ = json.Unmarshal(out[0], &frame)
	var action string
	_ = json.Unmarshal(frame[2], &action)
	if action != "RequestStopTransaction" {
		t.Errorf("action = %q", action)
	}
	var body struct {
		TransactionID string `json:"transactionId"`
	}
	_ = json.Unmarshal(frame[3], &body)
	if body.TransactionID != "tx-stop-9" {
		t.Errorf("transactionId = %q, want tx-stop-9", body.TransactionID)
	}
}

// TestAPI_RequestStopTransaction_RejectsMissingTransactionId: a
// missing transactionId is a 400.
func TestAPI_RequestStopTransaction_RejectsMissingTransactionId(t *testing.T) {
	srv, br, _, cleanup := makeServer(t)
	defer cleanup()

	w := postEnvelope(t, srv, "/internal/csms/request-stop-transaction", envelope{
		ChargePointID: "CP-ST201",
		Version:       "2.0.1",
		Payload:       json.RawMessage(`{}`),
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if out := br.snapshot("ocpp/2.0.1/CP-ST201/out"); len(out) != 0 {
		t.Errorf("must not publish on validation failure, got %d frames", len(out))
	}
}

// --- envelope version defaulting (1.6J when absent) ---

// TestAPI_EnvelopeVersion_DefaultsTo1_6J: a 1.6J-shaped
// RemoteStart with no "version" field in the envelope must still
// be accepted (backward compatibility) and routed to the 1.6J
// /out topic.
func TestAPI_EnvelopeVersion_DefaultsTo1_6J(t *testing.T) {
	srv, br, _, cleanup := makeServer(t)
	defer cleanup()

	// Note: envelope has no Version field set.
	w := postEnvelope(t, srv, "/internal/csms/remote-start", envelope{
		ChargePointID: "CP-DEFAULT",
		Payload:       json.RawMessage(`{"idTag":"TAG-X"}`),
	})
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if out := br.snapshot("ocpp/1.6J/CP-DEFAULT/out"); len(out) != 1 {
		t.Errorf("expected 1 outbound on 1.6J /out, got %d", len(out))
	}
	if out := br.snapshot("ocpp/2.0.1/CP-DEFAULT/out"); len(out) != 0 {
		t.Errorf("must not publish on 2.0.1 /out when version is default")
	}
}

// TestAPI_EnvelopeVersion_2_0_1_Reset_PublishesOnV201Topic: a
// 2.0.1 Reset with the new Immediate enum must route to
// ocpp/2.0.1/.../out (proves the version flows through).
func TestAPI_EnvelopeVersion_2_0_1_Reset_PublishesOnV201Topic(t *testing.T) {
	srv, br, _, cleanup := makeServer(t)
	defer cleanup()

	w := postEnvelope(t, srv, "/internal/csms/reset", envelope{
		ChargePointID: "CP-R201",
		Version:       "2.0.1",
		Payload:       json.RawMessage(`{"type":"Immediate"}`),
	})
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body = %s", w.Code, w.Body.String())
	}
	if out := br.snapshot("ocpp/2.0.1/CP-R201/out"); len(out) != 1 {
		t.Errorf("expected 1 outbound on 2.0.1 /out, got %d", len(out))
	}
	if out := br.snapshot("ocpp/1.6J/CP-R201/out"); len(out) != 0 {
		t.Errorf("must not publish on 1.6J /out")
	}
}
