package csms

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"nhooyr.io/websocket"
)

const callTimeout = 30 * time.Second

// API provides REST endpoints that send CSMS-initiated OCPP CALLs to charge points.
type API struct {
	connections *ConnectionRegistry
	pending     *PendingCallRegistry
}

func NewAPI(connections *ConnectionRegistry, pending *PendingCallRegistry) *API {
	return &API{
		connections: connections,
		pending:     pending,
	}
}

// RegisterRoutes registers all CSMS-initiated command REST endpoints.
func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/chargepoints/{id}/remote-start", a.handleRemoteStart)
	mux.HandleFunc("POST /api/chargepoints/{id}/remote-stop", a.handleRemoteStop)
	mux.HandleFunc("POST /api/chargepoints/{id}/reset", a.handleReset)
	mux.HandleFunc("POST /api/chargepoints/{id}/unlock", a.handleUnlock)
	mux.HandleFunc("PUT /api/chargepoints/{id}/config", a.handleChangeConfig)
	mux.HandleFunc("GET /api/chargepoints/{id}/config", a.handleGetConfig)
	mux.HandleFunc("POST /api/chargepoints/{id}/trigger", a.handleTrigger)
	mux.HandleFunc("POST /api/chargepoints/{id}/availability", a.handleChangeAvailability)
}

// sendAndWait sends an OCPP CALL and waits for the CALLRESULT.
func (a *API) sendAndWait(w http.ResponseWriter, r *http.Request, chargePointID, action string, payload interface{}) {
	conn, ok := a.connections.Get(chargePointID)
	if !ok {
		writeError(w, http.StatusConflict, "charge_point_offline")
		return
	}

	uniqueID := GenerateUniqueID()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal payload")
		return
	}

	// Build OCPP CALL frame: [2, uniqueId, action, payload]
	frame := fmt.Sprintf(`[2,"%s","%s",%s]`, uniqueID, action, string(payloadBytes))

	// Register pending call before sending
	pending := a.pending.Register(uniqueID, action, callTimeout)

	// Send the CALL
	ctx := r.Context()
	if err := conn.Write(ctx, websocket.MessageText, []byte(frame)); err != nil {
		a.pending.Cancel(uniqueID)
		writeError(w, http.StatusInternalServerError, "failed to send OCPP call")
		return
	}

	// Await response with timeout
	select {
	case result := <-pending.Response:
		if result.IsError {
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":           "ocpp_call_error",
				"errorCode":       result.ErrorCode,
				"errorDescription": result.ErrorDesc,
			})
			return
		}
		// Parse the response payload to extract status
		var resp struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(result.Payload, &resp); err != nil {
			// Some responses don't have a status field (e.g., GetConfiguration)
			writeJSON(w, http.StatusOK, json.RawMessage(result.Payload))
			return
		}
		writeJSON(w, statusFromOCPP(resp.Status), map[string]interface{}{
			"status": resp.Status,
		})
	case <-time.After(callTimeout):
		a.pending.Cancel(uniqueID)
		writeError(w, http.StatusGatewayTimeout, "ocpp_response_timeout")
	}
}

// sendAndWaitRaw sends an OCPP CALL and returns the raw response payload.
func (a *API) sendAndWaitRaw(w http.ResponseWriter, r *http.Request, chargePointID, action string, payload interface{}) {
	conn, ok := a.connections.Get(chargePointID)
	if !ok {
		writeError(w, http.StatusConflict, "charge_point_offline")
		return
	}

	uniqueID := GenerateUniqueID()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal payload")
		return
	}

	frame := fmt.Sprintf(`[2,"%s","%s",%s]`, uniqueID, action, string(payloadBytes))

	pending := a.pending.Register(uniqueID, action, callTimeout)

	ctx := r.Context()
	if err := conn.Write(ctx, websocket.MessageText, []byte(frame)); err != nil {
		a.pending.Cancel(uniqueID)
		writeError(w, http.StatusInternalServerError, "failed to send OCPP call")
		return
	}

	select {
	case result := <-pending.Response:
		if result.IsError {
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error":           "ocpp_call_error",
				"errorCode":       result.ErrorCode,
				"errorDescription": result.ErrorDesc,
			})
			return
		}
		writeJSON(w, http.StatusOK, json.RawMessage(result.Payload))
	case <-time.After(callTimeout):
		a.pending.Cancel(uniqueID)
		writeError(w, http.StatusGatewayTimeout, "ocpp_response_timeout")
	}
}

// --- REST Handlers ---

type remoteStartRequest struct {
	IDTag       string `json:"idTag"`
	ConnectorID *int   `json:"connectorId,omitempty"`
}

func (a *API) handleRemoteStart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req remoteStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.IDTag == "" {
		writeError(w, http.StatusBadRequest, "idTag is required")
		return
	}

	payload := struct {
		IDTag       string `json:"idTag"`
		ConnectorID *int   `json:"connectorId,omitempty"`
	}{IDTag: req.IDTag, ConnectorID: req.ConnectorID}

	a.sendAndWait(w, r, id, "RemoteStartTransaction", payload)
}

type remoteStopRequest struct {
	TransactionID int `json:"transactionId"`
}

func (a *API) handleRemoteStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req remoteStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	payload := struct {
		TransactionID int `json:"transactionId"`
	}{TransactionID: req.TransactionID}

	a.sendAndWait(w, r, id, "RemoteStopTransaction", payload)
}

type resetRequest struct {
	Type string `json:"type"`
}

func (a *API) handleReset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req resetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Type != "Soft" && req.Type != "Hard" {
		writeError(w, http.StatusBadRequest, "type must be Soft or Hard")
		return
	}

	payload := struct {
		Type string `json:"type"`
	}{Type: req.Type}

	a.sendAndWait(w, r, id, "Reset", payload)
}

func (a *API) handleUnlock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	connectorID := r.PathValue("connectorId")

	var connID int
	if _, err := fmt.Sscanf(connectorID, "%d", &connID); err != nil || connID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid connectorId")
		return
	}

	payload := struct {
		ConnectorID int `json:"connectorId"`
	}{ConnectorID: connID}

	a.sendAndWait(w, r, id, "UnlockConnector", payload)
}

type changeConfigRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (a *API) handleChangeConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req changeConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Key == "" {
		writeError(w, http.StatusBadRequest, "key is required")
		return
	}

	payload := struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}{Key: req.Key, Value: req.Value}

	a.sendAndWait(w, r, id, "ChangeConfiguration", payload)
}

func (a *API) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	keysParam := r.URL.Query().Get("keys")
	var keys []string
	if keysParam != "" {
		for _, k := range splitKeys(keysParam) {
			if k != "" {
				keys = append(keys, k)
			}
		}
	}

	payload := struct {
		Key []string `json:"key,omitempty"`
	}{Key: keys}

	a.sendAndWaitRaw(w, r, id, "GetConfiguration", payload)
}

type triggerRequest struct {
	RequestedMessage string `json:"requestedMessage"`
	ConnectorID      *int   `json:"connectorId,omitempty"`
}

func (a *API) handleTrigger(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req triggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RequestedMessage == "" {
		writeError(w, http.StatusBadRequest, "requestedMessage is required")
		return
	}

	payload := struct {
		RequestedMessage string `json:"requestedMessage"`
		ConnectorID      *int   `json:"connectorId,omitempty"`
	}{RequestedMessage: req.RequestedMessage, ConnectorID: req.ConnectorID}

	a.sendAndWait(w, r, id, "TriggerMessage", payload)
}

type changeAvailabilityRequest struct {
	ConnectorID int    `json:"connectorId"`
	Type        string `json:"type"`
}

func (a *API) handleChangeAvailability(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req changeAvailabilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Type != "Operative" && req.Type != "Inoperative" {
		writeError(w, http.StatusBadRequest, "type must be Operative or Inoperative")
		return
	}

	payload := struct {
		ConnectorID int    `json:"connectorId"`
		Type        string `json:"type"`
	}{ConnectorID: req.ConnectorID, Type: req.Type}

	a.sendAndWait(w, r, id, "ChangeAvailability", payload)
}

// --- Helpers ---

func statusFromOCPP(ocppStatus string) int {
	switch ocppStatus {
	case "Accepted":
		return http.StatusAccepted
	case "Rejected":
		return http.StatusConflict
	case "Scheduled":
		return http.StatusAccepted
	case "Unlocked":
		return http.StatusOK
	case "UnlockFailed", "NotSupported":
		return http.StatusConflict
	case "RebootRequired":
		return http.StatusOK
	default:
		return http.StatusOK
	}
}

func splitKeys(s string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if c == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
