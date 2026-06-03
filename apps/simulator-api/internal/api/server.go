package api

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/user/ocpp-simulator/apps/simulator-api/internal/common"
	"github.com/user/ocpp-simulator/apps/simulator-api/internal/db"
	"github.com/user/ocpp-simulator/apps/simulator-api/internal/realtime"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	mux             *http.ServeMux
	db              *sql.DB
	hub             *realtime.Hub
	chargePointRepo *db.ChargePointRepo
	connectorRepo   *db.ConnectorRepo
	settingsRepo    *db.SettingsRepo
}

func NewServer(database *sql.DB, hub *realtime.Hub) *Server {
	s := &Server{
		mux:             http.NewServeMux(),
		db:              database,
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
	s.mux.HandleFunc("GET /api/versions", s.handleListVersions)

	s.mux.HandleFunc("GET /api/charge-points", s.handleListChargePoints)
	s.mux.HandleFunc("POST /api/charge-points", s.handleCreateChargePoint)
	s.mux.HandleFunc("GET /api/charge-points/{id}", s.handleGetChargePoint)
	s.mux.HandleFunc("DELETE /api/charge-points/{id}", s.handleDeleteChargePoint)

	s.mux.HandleFunc("POST /api/charge-points/{id}/connectors", s.handleAddConnector)
	s.mux.HandleFunc("DELETE /api/charge-points/{id}/connectors/{connectorId}", s.handleDeleteConnector)

	s.mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	s.mux.HandleFunc("PUT /api/settings", s.handleUpdateSettings)

	s.mux.HandleFunc("GET /api/realtime", s.handleRealtime)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	versions := []map[string]string{
		{"version": "1.6J", "label": "OCPP 1.6J", "status": "supported"},
		{"version": "2.0.1", "label": "OCPP 2.0.1", "status": "planned"},
	}
	writeJSON(w, http.StatusOK, versions)
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
		connectors, err := s.connectorRepo.ListByChargePoint(r.Context(), list[i].ID)
		if err == nil {
			list[i].ConnectorCount = len(connectors)
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

	s.hub.PublishChargePointEvent(cp.ID, "created")
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

	connectors, err := s.connectorRepo.ListByChargePoint(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list connectors")
		return
	}
	if connectors == nil {
		connectors = []common.Connector{}
	}
	cp.ConnectorCount = len(connectors)

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
	s.hub.PublishChargePointEvent(id, "deleted")
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

	writeJSON(w, http.StatusCreated, connector)
}

func (s *Server) handleDeleteConnector(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
