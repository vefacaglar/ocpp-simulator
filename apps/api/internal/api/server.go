package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/user/ocpp-simulator/apps/api/internal/common"
	"github.com/user/ocpp-simulator/apps/api/internal/db"
	"github.com/user/ocpp-simulator/apps/api/internal/realtime"
	"github.com/user/ocpp-simulator/apps/api/internal/simulator"
	"github.com/user/ocpp-simulator/packages/ocpp-protocol/pkg/protocol"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	mux             *http.ServeMux
	db              *sql.DB
	runtime         *simulator.Runtime
	hub             *realtime.Hub
	chargePointRepo *db.ChargePointRepo
	connectorRepo   *db.ConnectorRepo
	settingsRepo    *db.SettingsRepo
}

func NewServer(database *sql.DB, runtime *simulator.Runtime, hub *realtime.Hub) *Server {
	s := &Server{
		mux:             http.NewServeMux(),
		db:              database,
		runtime:         runtime,
		hub:             hub,
		chargePointRepo: db.NewChargePointRepo(database),
		connectorRepo:   db.NewConnectorRepo(database),
		settingsRepo:    db.NewSettingsRepo(database),
	}
	s.registerRoutes()
	return s
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	s.mux.HandleFunc("GET /api/charge-points", s.handleListChargePoints)
	s.mux.HandleFunc("POST /api/charge-points", s.handleCreateChargePoint)
	s.mux.HandleFunc("GET /api/charge-points/{id}", s.handleGetChargePoint)
	s.mux.HandleFunc("DELETE /api/charge-points/{id}", s.handleDeleteChargePoint)

	s.mux.HandleFunc("POST /api/charge-points/{id}/connectors", s.handleAddConnector)
	s.mux.HandleFunc("DELETE /api/charge-points/{id}/connectors/{connectorId}", s.handleDeleteConnector)

	s.mux.HandleFunc("POST /api/charge-points/{id}/connect", s.handleConnect)
	s.mux.HandleFunc("POST /api/charge-points/{id}/disconnect", s.handleDisconnect)
	s.mux.HandleFunc("POST /api/charge-points/{id}/boot", s.handleBoot)
	s.mux.HandleFunc("POST /api/charge-points/{id}/heartbeat", s.handleHeartbeat)

	s.mux.HandleFunc("POST /api/charge-points/{id}/connectors/{connectorId}/start-transaction", s.handleStartTransaction)
	s.mux.HandleFunc("POST /api/charge-points/{id}/connectors/{connectorId}/stop-transaction", s.handleStopTransaction)
	s.mux.HandleFunc("POST /api/charge-points/{id}/connectors/{connectorId}/meter-values", s.handleMeterValues)
	s.mux.HandleFunc("POST /api/charge-points/{id}/connectors/{connectorId}/status", s.handleSetConnectorStatus)

	s.mux.HandleFunc("POST /api/charge-points/{id}/remote-start", s.handleRemoteStart)
	s.mux.HandleFunc("POST /api/charge-points/{id}/remote-stop", s.handleRemoteStop)

	s.mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/settings", s.handleUpdateSettings)
	s.mux.HandleFunc("GET /api/versions", s.handleListVersions)

	s.mux.HandleFunc("GET /api/realtime", s.handleRealtime)
	s.mux.HandleFunc("GET /api/ws/{id}", s.handleChargePointWS)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListChargePoints(w http.ResponseWriter, r *http.Request) {
	list, err := s.chargePointRepo.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list charge points")
		return
	}
	if list == nil {
		list = []common.ChargePoint{}
	}
	for i := range list {
		if rt, ok := s.runtime.GetChargePoint(list[i].ID); ok {
			rt.Mu.RLock()
			if rt.Connected {
				list[i].Status = common.StatusConnected
			}
			list[i].ConnectorCount = len(rt.Connectors)
			rt.Mu.RUnlock()
		}
	}
	writeJSON(w, http.StatusOK, list)
}

type createChargePointRequest struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	OCPPVersion      string `json:"ocppVersion"`
	CentralSystemURL string `json:"centralSystemUrl"`
	AutoConnect      bool   `json:"autoConnect"`
}

func (s *Server) handleCreateChargePoint(w http.ResponseWriter, r *http.Request) {
	var req createChargePointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}
	if req.Name == "" {
		req.Name = req.ID
	}
	if req.OCPPVersion == "" {
		req.OCPPVersion = "1.6J"
	}
	if req.CentralSystemURL == "" {
		req.CentralSystemURL = "ws://localhost:8080/ocpp"
	}

	cp := common.ChargePoint{
		ID:               req.ID,
		Name:             req.Name,
		OCPPVersion:      req.OCPPVersion,
		CentralSystemURL: req.CentralSystemURL,
		AutoConnect:      req.AutoConnect,
		Status:           common.StatusDisconnected,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if err := s.chargePointRepo.Create(r.Context(), cp); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create charge point")
		return
	}

	s.runtime.AddChargePoint(cp)
	writeJSON(w, http.StatusCreated, cp)
}

func (s *Server) handleGetChargePoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cp, err := s.chargePointRepo.GetByID(r.Context(), id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "charge point not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get charge point")
		return
	}

	if rt, ok := s.runtime.GetChargePoint(id); ok {
		rt.Mu.RLock()
		if rt.Connected {
			cp.Status = common.StatusConnected
		}
		cp.ConnectorCount = len(rt.Connectors)
		rt.Mu.RUnlock()
	}

	connectors, err := s.connectorRepo.ListByChargePoint(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list connectors")
		return
	}
	if connectors == nil {
		connectors = []common.Connector{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chargePoint": cp,
		"connectors":  connectors,
	})
}

func (s *Server) handleDeleteChargePoint(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := s.chargePointRepo.GetByID(r.Context(), id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "charge point not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get charge point")
		return
	}
	if err := s.chargePointRepo.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete charge point")
		return
	}
	s.runtime.RemoveChargePoint(id)
	w.WriteHeader(http.StatusNoContent)
}

type addConnectorRequest struct {
	EVSEID          int `json:"evseId"`
	ConnectorNumber int `json:"connectorNumber"`
}

func (s *Server) handleAddConnector(w http.ResponseWriter, r *http.Request) {
	cpID := r.PathValue("id")
	_, err := s.chargePointRepo.GetByID(r.Context(), cpID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "charge point not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get charge point")
		return
	}

	var req addConnectorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ConnectorNumber == 0 {
		num, err := s.connectorRepo.NextConnectorNumber(r.Context(), cpID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to get next connector number")
			return
		}
		req.ConnectorNumber = num
	}
	if req.EVSEID == 0 {
		req.EVSEID = 1
	}

	connector := common.Connector{
		ChargePointID:   cpID,
		EVSEID:          req.EVSEID,
		ConnectorNumber: req.ConnectorNumber,
		Status:          common.ConnectorAvailable,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	if err := s.connectorRepo.Create(r.Context(), connector); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create connector")
		return
	}

	s.runtime.AddConnector(cpID, connector)

	writeJSON(w, http.StatusCreated, connector)
}

func (s *Server) handleDeleteConnector(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.runtime.Connect(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "connected"})
}

func (s *Server) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.runtime.Disconnect(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

func (s *Server) handleBoot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.runtime.SendBootNotification(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "boot_sent"})
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.runtime.SendHeartbeat(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "heartbeat_sent"})
}

type startTransactionRequest struct {
	IDTag string `json:"idTag"`
}

func (s *Server) handleStartTransaction(w http.ResponseWriter, r *http.Request) {
	cpID := r.PathValue("id")
	connectorID := r.PathValue("connectorId")

	var req startTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.IDTag == "" {
		req.IDTag = "DEADBEEF"
	}

	connID := parseIntOr(connectorID, 1)
	if err := s.runtime.StartTransaction(cpID, connID, req.IDTag); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "transaction_started"})
}

type stopTransactionRequest struct {
	Reason string `json:"reason"`
}

func (s *Server) handleStopTransaction(w http.ResponseWriter, r *http.Request) {
	cpID := r.PathValue("id")
	connectorID := r.PathValue("connectorId")

	var req stopTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Reason = "Local"
	}
	if req.Reason == "" {
		req.Reason = "Local"
	}

	connID := parseIntOr(connectorID, 1)
	if err := s.runtime.StopTransaction(cpID, connID, req.Reason); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "transaction_stopped"})
}

func (s *Server) handleMeterValues(w http.ResponseWriter, r *http.Request) {
	cpID := r.PathValue("id")
	connectorID := r.PathValue("connectorId")

	connID := parseIntOr(connectorID, 1)
	if err := s.runtime.SendMeterValues(cpID, connID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "meter_values_sent"})
}

type setConnectorStatusRequest struct {
	Status string `json:"status"`
}

func (s *Server) handleSetConnectorStatus(w http.ResponseWriter, r *http.Request) {
	cpID := r.PathValue("id")
	connectorID := r.PathValue("connectorId")

	var req setConnectorStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}

	connID := parseIntOr(connectorID, 1)
	if err := s.runtime.SetConnectorStatus(cpID, connID, common.ConnectorStatus(req.Status)); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "connector_status_set"})
}

type remoteStartRequest struct {
	IDTag       string `json:"idTag"`
	ConnectorID *int   `json:"connectorId,omitempty"`
}

func (s *Server) handleRemoteStart(w http.ResponseWriter, r *http.Request) {
	cpID := r.PathValue("id")

	var req remoteStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.IDTag == "" {
		req.IDTag = "DEADBEEF"
	}

	log.Printf("[API] RemoteStart cpID=%s idTag=%s connectorID=%v", cpID, req.IDTag, req.ConnectorID)

	status, err := s.runtime.HandleRemoteStartTransaction(cpID, &protocol.RemoteStartTransactionRequest{
		IDTag:       req.IDTag,
		ConnectorID: req.ConnectorID,
	})
	if err != nil {
		log.Printf("[API] RemoteStart error: %v", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	log.Printf("[API] RemoteStart result: %s", status)
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

type remoteStopRequest struct {
	TransactionID int `json:"transactionId"`
}

func (s *Server) handleRemoteStop(w http.ResponseWriter, r *http.Request) {
	cpID := r.PathValue("id")

	var req remoteStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	status, err := s.runtime.HandleRemoteStopTransaction(cpID, &protocol.RemoteStopTransactionRequest{
		TransactionID: req.TransactionID,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.settingsRepo.GetAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get settings")
		return
	}
	if settings == nil {
		settings = []db.Setting{}
	}
	writeJSON(w, http.StatusOK, settings)
}

type updateSettingsRequest map[string]string

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req updateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	for key, value := range req {
		if err := s.settingsRepo.Set(r.Context(), key, value); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update settings")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	versions := []map[string]string{
		{"version": "1.6J", "label": "OCPP 1.6J", "status": "supported"},
		{"version": "2.0.1", "label": "OCPP 2.0.1", "status": "planned"},
	}
	writeJSON(w, http.StatusOK, versions)
}

func parseIntOr(s string, defaultVal int) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return defaultVal
		}
	}
	if n == 0 {
		return defaultVal
	}
	return n
}

func (s *Server) handleRealtime(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	var subscribedCP string
	ch := make(chan realtime.Event, 64)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			if subscribedCP != "" {
				s.hub.UnsubscribeChargePoint(subscribedCP, ch)
			}
			return
		}

		var cmd struct {
			Type          string `json:"type"`
			ChargePointID string `json:"chargePointId"`
		}
		if err := json.Unmarshal(msg, &cmd); err != nil {
			continue
		}

		switch cmd.Type {
		case "subscribe":
			if subscribedCP != "" {
				s.hub.UnsubscribeChargePoint(subscribedCP, ch)
			}
			subscribedCP = cmd.ChargePointID
			ch = s.hub.SubscribeChargePoint(subscribedCP)
			go func() {
				for event := range ch {
					if len(event.RawFrame) > 0 {
						conn.WriteMessage(1, event.RawFrame)
					} else {
						data, _ := json.Marshal(event)
						conn.WriteMessage(1, data)
					}
				}
			}()
		case "unsubscribe":
			if subscribedCP != "" {
				s.hub.UnsubscribeChargePoint(subscribedCP, ch)
				subscribedCP = ""
			}
		}
	}
}

func (s *Server) handleChargePointWS(w http.ResponseWriter, r *http.Request) {
	cpID := r.PathValue("id")

	cp, ok := s.runtime.GetChargePoint(cpID)
	if !ok {
		writeError(w, http.StatusNotFound, "charge point not found")
		return
	}

	uiConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer uiConn.Close()

	cp.Mu.RLock()
	csmsURL := cp.Config.CentralSystemURL
	cp.Mu.RUnlock()

	csmsConnURL := fmt.Sprintf("%s/%s", csmsURL, cpID)
	dialer := websocket.Dialer{
		Subprotocols:    []string{"ocpp1.6"},
		HandshakeTimeout: 10 * time.Second,
	}

	csmsConn, _, err := dialer.DialContext(r.Context(), csmsConnURL, nil)
	if err != nil {
		log.Printf("[%s] CSMS connect error: %v", cpID, err)
		uiConn.WriteMessage(1, []byte(fmt.Sprintf(`[4,"","InternalError","Failed to connect to CSMS: %s",{}]`, err.Error())))
		return
	}
	defer csmsConn.Close()

	cp.Mu.Lock()
	cp.Connected = true
	cp.Config.Status = common.StatusConnected
	cp.Mu.Unlock()

	s.hub.PublishChargePointEvent(cpID, "connected")
	log.Printf("[%s] UI WebSocket connected, proxied to CSMS", cpID)

	errCh := make(chan error, 2)

	go func() {
		for {
			_, msg, err := uiConn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if err := csmsConn.WriteMessage(websocket.TextMessage, msg); err != nil {
				errCh <- err
				return
			}
			s.hub.PublishOCPPFrame(cpID, "outbound", msg)
		}
	}()

	go func() {
		for {
			_, msg, err := csmsConn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if err := uiConn.WriteMessage(websocket.TextMessage, msg); err != nil {
				errCh <- err
				return
			}
			s.hub.PublishOCPPFrame(cpID, "inbound", msg)
		}
	}()

	<-errCh

	cp.Mu.Lock()
	cp.Connected = false
	cp.Config.Status = common.StatusDisconnected
	cp.Mu.Unlock()

	s.hub.PublishChargePointEvent(cpID, "disconnected")
	log.Printf("[%s] UI WebSocket disconnected", cpID)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
