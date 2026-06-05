// Package api exposes the HTTP surface of ocpp-core. It serves
// two audiences:
//   1. message-processor, which calls /internal/transactions/{authorize,start,stop}
//      to get a business decision and the spec-exact response payload.
//   2. External/test callers (and T29 verification), which call
//      /internal/csms/* to trigger CSMS-initiated OCPP commands.
//
// The wire contract for /internal/* is ocpp-core-internal: requests
// and responses use { chargePointId, payload } envelopes so the
// processor can hand the inner payload bytes verbatim to the
// codec. This envelope is internal and never appears on the OCPP
// wire.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/vefacaglar/ocpp-simulator/apps/ocpp-core/internal/csms"
	"github.com/vefacaglar/ocpp-simulator/apps/ocpp-core/internal/db"
	"github.com/vefacaglar/ocpp-simulator/apps/ocpp-core/internal/transaction"
)

// envelope is the request/response shape used by every
// /internal/* endpoint. The Payload field is ocpp-core's internal
// container for the OCPP request/response bytes; on the wire they
// are passed through verbatim.
type envelope struct {
	ChargePointID string          `json:"chargePointId"`
	Payload       json.RawMessage `json:"payload"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

// Server wires the HTTP router.
type Server struct {
	mux              *http.ServeMux
	transactionSvc   *transaction.Service
	csmsSvc          *csms.Service
	runtimeEventRepo *db.RuntimeEventRepo
	transactionRepo  *db.TransactionRepo
}

func NewServer(transactionSvc *transaction.Service, csmsSvc *csms.Service, runtimeEventRepo *db.RuntimeEventRepo, transactionRepo *db.TransactionRepo) *Server {
	s := &Server{
		mux:              http.NewServeMux(),
		transactionSvc:   transactionSvc,
		csmsSvc:          csmsSvc,
		runtimeEventRepo: runtimeEventRepo,
		transactionRepo:  transactionRepo,
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("POST /internal/transactions/authorize", s.handleAuthorize)
	s.mux.HandleFunc("POST /internal/transactions/start", s.handleStartTransaction)
	s.mux.HandleFunc("POST /internal/transactions/stop", s.handleStopTransaction)
	s.mux.HandleFunc("POST /internal/transactions/event", s.handleTransactionEvent)
	s.mux.HandleFunc("POST /internal/csms/remote-start", s.handleRemoteStart)
	s.mux.HandleFunc("POST /internal/csms/remote-stop", s.handleRemoteStop)
	s.mux.HandleFunc("POST /internal/csms/reset", s.handleReset)
	s.mux.HandleFunc("POST /internal/csms/unlock-connector", s.handleUnlockConnector)
	s.mux.HandleFunc("POST /internal/csms/change-configuration", s.handleChangeConfiguration)
	s.mux.HandleFunc("POST /internal/csms/get-configuration", s.handleGetConfiguration)
	s.mux.HandleFunc("POST /internal/csms/trigger-message", s.handleTriggerMessage)
	s.mux.HandleFunc("POST /internal/csms/change-availability", s.handleChangeAvailability)
	// 2.0.1 CSMS surface.
	s.mux.HandleFunc("POST /internal/csms/request-start-transaction", s.handleRequestStartTransaction)
	s.mux.HandleFunc("POST /internal/csms/request-stop-transaction", s.handleRequestStopTransaction)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- business decisions: Authorize / StartTransaction / StopTransaction ---

type authorizeReq struct {
	IDTag string `json:"idTag"`
}

type authorizeResp struct {
	IDTagInfo struct {
		Status string `json:"status"`
	} `json:"idTagInfo"`
}

// handleAuthorize decides whether the idTag is accepted. For MVP
// the policy is hard-coded: any non-empty idTag is Accepted. The
// processor forwards the returned payload as the OCPP
// Authorize.conf body.
func (s *Server) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	var req envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var authReq authorizeReq
	if err := json.Unmarshal(req.Payload, &authReq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if authReq.IDTag == "" {
		writeError(w, http.StatusBadRequest, "idTag is required")
		return
	}
	resp := authorizeResp{}
	resp.IDTagInfo.Status = "Accepted"
	if err := s.recordRuntimeEvent(r.Context(), &req.ChargePointID, "transaction.authorized", "info", "idTag accepted"); err != nil {
		log.Printf("[ocpp-core/api] runtime event: %v", err)
	}
	writeJSON(w, http.StatusOK, envelope{ChargePointID: req.ChargePointID, Payload: mustJSON(resp)})
}

type startTxReq struct {
	ConnectorID int    `json:"connectorId"`
	IDTag       string `json:"idTag"`
	MeterStart  int    `json:"meterStart"`
	Timestamp   string `json:"timestamp"`
}

// handleStartTransaction creates a transaction row with a fresh
// UUID, assigns the next numeric id from the in-memory counter,
// and returns the spec-exact StartTransaction.conf payload.
func (s *Server) handleStartTransaction(w http.ResponseWriter, r *http.Request) {
	var req envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var st startTxReq
	if err := json.Unmarshal(req.Payload, &st); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if st.IDTag == "" {
		writeError(w, http.StatusBadRequest, "idTag is required")
		return
	}
	res, err := s.transactionSvc.Start(r.Context(), transaction.StartInput{
		ChargePointID:   req.ChargePointID,
		EVSEID:          1,
		ConnectorNumber: st.ConnectorID,
		IDTag:           st.IDTag,
		MeterStart:      st.MeterStart,
		OCPPVersion:     "1.6J",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// StartTransaction.conf shape per OCPP 1.6J:
	//   { "transactionId": <int>, "idTagInfo": { "status": "..." } }
	conf := struct {
		TransactionID int `json:"transactionId"`
		IDTagInfo     struct {
			Status string `json:"status"`
		} `json:"idTagInfo"`
	}{TransactionID: res.TransactionID}
	conf.IDTagInfo.Status = res.IDTagInfo.Status
	writeJSON(w, http.StatusOK, envelope{ChargePointID: req.ChargePointID, Payload: mustJSON(conf)})
}

type stopTxReq struct {
	TransactionID int    `json:"transactionId"`
	IDTag         string `json:"idTag,omitempty"`
	MeterStop     int    `json:"meterStop"`
	Timestamp     string `json:"timestamp"`
	Reason        string `json:"reason,omitempty"`
}

// handleStopTransaction accepts either a numeric 1.6J transactionId
// or a UUID (2.0.1) and finalizes the matching row. The response
// payload is the spec-exact StopTransaction.conf body.
func (s *Server) handleStopTransaction(w http.ResponseWriter, r *http.Request) {
	var req envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var st stopTxReq
	if err := json.Unmarshal(req.Payload, &st); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	// Resolve the transaction by numeric_id. A 2.0.1 path would
	// resolve by id (UUID) instead; only the 1.6J path is wired
	// in the MVP.
	tx, err := s.findTransactionByNumericID(r.Context(), req.ChargePointID, st.TransactionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "transaction not found")
		return
	}
	if err := s.transactionSvc.Stop(r.Context(), tx.ID, st.MeterStop, st.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	conf := struct {
		IDTagInfo *struct {
			Status string `json:"status"`
		} `json:"idTagInfo,omitempty"`
	}{}
	inner := struct {
		Status string `json:"status"`
	}{Status: "Accepted"}
	conf.IDTagInfo = &inner
	writeJSON(w, http.StatusOK, envelope{ChargePointID: req.ChargePointID, Payload: mustJSON(conf)})
}

func (s *Server) findTransactionByNumericID(ctx context.Context, chargePointID string, numericID int) (*db.Transaction, error) {
	list, err := s.transactionRepo.ListByChargePoint(ctx, chargePointID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].NumericID != nil && *list[i].NumericID == numericID {
			return &list[i], nil
		}
	}
	return nil, errors.New("not found")
}

// --- CSMS-initiated: produce raw OCPP CALLs to /out ---

type remoteStartReq struct {
	IDTag       string `json:"idTag"`
	ConnectorID *int   `json:"connectorId,omitempty"`
}

func (s *Server) handleRemoteStart(w http.ResponseWriter, r *http.Request) {
	var req envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var rs remoteStartReq
	if err := json.Unmarshal(req.Payload, &rs); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if rs.IDTag == "" {
		writeError(w, http.StatusBadRequest, "idTag is required")
		return
	}
	if err := s.csmsSvc.RemoteStart(r.Context(), req.ChargePointID, rs.IDTag, rs.ConnectorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

type remoteStopReq struct {
	TransactionID int `json:"transactionId"`
}

func (s *Server) handleRemoteStop(w http.ResponseWriter, r *http.Request) {
	var req envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var rs remoteStopReq
	if err := json.Unmarshal(req.Payload, &rs); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := s.csmsSvc.RemoteStop(r.Context(), req.ChargePointID, rs.TransactionID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

type resetReq struct {
	Type string `json:"type"`
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	var req envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var rs resetReq
	if err := json.Unmarshal(req.Payload, &rs); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := s.csmsSvc.Reset(r.Context(), req.ChargePointID, rs.Type); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

type unlockReq struct {
	ConnectorID int `json:"connectorId"`
}

func (s *Server) handleUnlockConnector(w http.ResponseWriter, r *http.Request) {
	var req envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var u unlockReq
	if err := json.Unmarshal(req.Payload, &u); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := s.csmsSvc.UnlockConnector(r.Context(), req.ChargePointID, u.ConnectorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

type changeConfigReq struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (s *Server) handleChangeConfiguration(w http.ResponseWriter, r *http.Request) {
	var req envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var cc changeConfigReq
	if err := json.Unmarshal(req.Payload, &cc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := s.csmsSvc.ChangeConfiguration(r.Context(), req.ChargePointID, cc.Key, cc.Value); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

type getConfigReq struct {
	Key []string `json:"key,omitempty"`
}

func (s *Server) handleGetConfiguration(w http.ResponseWriter, r *http.Request) {
	var req envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var gc getConfigReq
	if err := json.Unmarshal(req.Payload, &gc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := s.csmsSvc.GetConfiguration(r.Context(), req.ChargePointID, gc.Key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

type triggerMsgReq struct {
	RequestedMessage string `json:"requestedMessage"`
	ConnectorID      *int   `json:"connectorId,omitempty"`
}

func (s *Server) handleTriggerMessage(w http.ResponseWriter, r *http.Request) {
	var req envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var t triggerMsgReq
	if err := json.Unmarshal(req.Payload, &t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := s.csmsSvc.TriggerMessage(r.Context(), req.ChargePointID, t.RequestedMessage, t.ConnectorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

type changeAvailReq struct {
	ConnectorID int    `json:"connectorId"`
	Type        string `json:"type"`
}

func (s *Server) handleChangeAvailability(w http.ResponseWriter, r *http.Request) {
	var req envelope
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	var ca changeAvailReq
	if err := json.Unmarshal(req.Payload, &ca); err != nil {
		writeError(w, http.StatusBadRequest, "invalid payload")
		return
	}
	if err := s.csmsSvc.ChangeAvailability(r.Context(), req.ChargePointID, ca.ConnectorID, ca.Type); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func mustJSON(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		// This only happens on un-marshalable types we control
		// (structs of strings, ints, and pointers). Falling back
		// to an explicit null keeps the response shape stable
		// rather than 500ing the caller.
		return json.RawMessage(`null`)
	}
	return b
}

func (s *Server) recordRuntimeEvent(ctx context.Context, chargePointID *string, eventType, severity, message string) error {
	_, err := s.runtimeEventRepo.Create(ctx, db.RuntimeEvent{
		ChargePointID: chargePointID,
		EventType:     eventType,
		Severity:      severity,
		Message:       message,
	})
	return err
}

// keep uuid/time in use even if helpers above are trimmed.
var _ = uuid.NewString
var _ = time.Now
