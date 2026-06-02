package simulator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/user/ocpp-simulator/apps/api/internal/common"
	"github.com/user/ocpp-simulator/apps/api/internal/db"
	"github.com/user/ocpp-simulator/apps/api/internal/ocpp"
	"github.com/user/ocpp-simulator/apps/api/internal/ocpp/v16"
	"github.com/user/ocpp-simulator/apps/api/internal/realtime"
)

type ConnectorRuntime struct {
	Number       int
	StateMachine *ConnectorStateMachine
	MeterGen     *MeterValueGenerator
	TransactionID string
}

type ChargePointInstance struct {
	Config       common.ChargePoint
	Connectors   map[int]*ConnectorRuntime
	Connected    bool
	Client       *OcppWebSocketClient
	Mu           sync.RWMutex
}

type Runtime struct {
	mu              sync.RWMutex
	chargePoints    map[string]*ChargePointInstance
	eventBus        *realtime.EventBus
	factory         *ocpp.Factory
	transactionRepo *db.TransactionRepo
	messageLogRepo  *db.MessageLogRepo
	connectorRepo   *db.ConnectorRepo
}

func NewRuntime(eventBus *realtime.EventBus, factory *ocpp.Factory, txRepo *db.TransactionRepo, msgRepo *db.MessageLogRepo, connRepo *db.ConnectorRepo) *Runtime {
	return &Runtime{
		chargePoints:    make(map[string]*ChargePointInstance),
		eventBus:        eventBus,
		factory:         factory,
		transactionRepo: txRepo,
		messageLogRepo:  msgRepo,
		connectorRepo:   connRepo,
	}
}

func (r *Runtime) AddChargePoint(cp common.ChargePoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.chargePoints[cp.ID] = &ChargePointInstance{
		Config:     cp,
		Connectors: make(map[int]*ConnectorRuntime),
	}
	r.publishEvent("charge_point.created", cp.ID, nil, fmt.Sprintf("Charge point %s created", cp.ID))
}

func (r *Runtime) RemoveChargePoint(id string) {
	r.mu.Lock()
	cp, exists := r.chargePoints[id]
	if exists {
		cp.Mu.Lock()
		if cp.Client != nil {
			cp.Client.Disconnect()
		}
		cp.Connected = false
		cp.Mu.Unlock()
		delete(r.chargePoints, id)
	}
	r.mu.Unlock()
	if exists {
		r.publishEvent("charge_point.deleted", id, nil, fmt.Sprintf("Charge point %s deleted", id))
	}
}

func (r *Runtime) Connect(id string) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", id)
	}

	protocol, ok := r.factory.Create(cp.Config.OCPPVersion)
	if !ok {
		return fmt.Errorf("unsupported OCPP version: %s", cp.Config.OCPPVersion)
	}

	client := NewOcppWebSocketClient(id, cp.Config.CentralSystemURL, protocol, r.eventBus)
	client.SetResponseHandler(r.handleOCPPResponse)
	client.SetInboundHandler(NewInboundCallHandler(r, r.factory))

	cp.Mu.Lock()
	cp.Client = client
	cp.Mu.Unlock()

	if err := client.Connect(context.Background()); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	cp.Mu.Lock()
	cp.Connected = true
	cp.Config.Status = common.StatusConnected
	cp.Mu.Unlock()

	return nil
}

func (r *Runtime) Disconnect(id string) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", id)
	}

	cp.Mu.Lock()
	defer cp.Mu.Unlock()

	if cp.Client != nil {
		if err := cp.Client.Disconnect(); err != nil {
			return err
		}
	}

	cp.Connected = false
	cp.Config.Status = common.StatusDisconnected
	return nil
}

func (r *Runtime) SendBootNotification(id string) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", id)
	}
	cp.Mu.Lock()
	client := cp.Client
	cp.Mu.Unlock()
	if client == nil {
		return fmt.Errorf("charge point %s not connected", id)
	}
	return client.SendBootNotification(context.Background(), "Simulator", "OCPP-Sim")
}

func (r *Runtime) SendHeartbeat(id string) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", id)
	}
	cp.Mu.Lock()
	client := cp.Client
	cp.Mu.Unlock()
	if client == nil {
		return fmt.Errorf("charge point %s not connected", id)
	}
	return client.SendHeartbeat(context.Background())
}

func (r *Runtime) AddConnector(cpID string, connector common.Connector) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.Lock()
	defer cp.Mu.Unlock()
	cp.Connectors[connector.ConnectorNumber] = &ConnectorRuntime{
		Number:       connector.ConnectorNumber,
		StateMachine: NewConnectorStateMachine(StateAvailable),
		MeterGen:     NewMeterValueGenerator(11000),
	}
	return nil
}

func (r *Runtime) SetConnectorStatus(cpID string, connectorID int, status common.ConnectorStatus) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.Lock()
	cr, exists := cp.Connectors[connectorID]
	if !exists {
		cp.Mu.Unlock()
		return fmt.Errorf("connector %d not found", connectorID)
	}

	if err := cr.StateMachine.Transition(ConnectorState(status)); err != nil {
		cp.Mu.Unlock()
		return err
	}
	cp.Mu.Unlock()

	cp.Mu.RLock()
	client := cp.Client
	connected := cp.Connected
	cp.Mu.RUnlock()

	if connected && client != nil {
		client.SendStatusNotification(context.Background(), connectorID, string(status), "NoError")
	}

	r.publishEvent("connector.status_changed", cpID, &connectorID,
		fmt.Sprintf("Connector %d status changed to %s", connectorID, status))
	return nil
}

func (r *Runtime) StartTransaction(cpID string, connectorID int, idTag string) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.Lock()
	client := cp.Client
	connected := cp.Connected
	cr, connExists := cp.Connectors[connectorID]
	cp.Mu.Unlock()

	if !connected || client == nil {
		return fmt.Errorf("charge point %s not connected", cpID)
	}
	if !connExists {
		return fmt.Errorf("connector %d not found", connectorID)
	}

	cp.Mu.RLock()
	version := cp.Config.OCPPVersion
	cp.Mu.RUnlock()

	// Check if connector already has active transaction
	cp.Mu.RLock()
	hasActive := cr.TransactionID != ""
	cp.Mu.RUnlock()
	if hasActive {
		return fmt.Errorf("connector %d already has an active transaction", connectorID)
	}

	// Transition state: Available -> Preparing
	if err := r.SetConnectorStatus(cpID, connectorID, common.ConnectorPreparing); err != nil {
		return fmt.Errorf("transition to Preparing: %w", err)
	}

	// Create transaction in DB with pending_start status
	meterStart := cr.MeterGen.CurrentMeterWh()
	txID, err := r.transactionRepo.Create(context.Background(), db.Transaction{
		ChargePointID:   cpID,
		EVSEID:          1,
		ConnectorNumber: connectorID,
		IDTag:           idTag,
		OCPPVersion:     version,
		Status:          "pending_start",
		StartMeterWh:    meterStart,
	})
	if err != nil {
		return fmt.Errorf("create transaction: %w", err)
	}

	cp.Mu.Lock()
	cr.TransactionID = txID
	cp.Mu.Unlock()

	// Send StartTransaction via OCPP
	msg, err := client.protocol.BuildStartTransaction(context.Background(), ocpp.StartTransactionInput{
		ConnectorID: connectorID,
		IDTag:       idTag,
		MeterStart:  meterStart,
		Timestamp:   time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("build StartTransaction: %w", err)
	}

	if err := client.Send(msg); err != nil {
		return fmt.Errorf("send StartTransaction: %w", err)
	}

	r.publishEvent("transaction.started", cpID, &connectorID,
		fmt.Sprintf("Transaction %s started on connector %d", txID, connectorID))
	log.Printf("[%s] StartTransaction sent, pending txID=%s", cpID, txID)
	return nil
}

func (r *Runtime) HandleStartTransactionResponse(cpID string, msg ocpp.Message) {
	resp, err := v16.ParseStartTransactionResponse(msg.Payload)
	if err != nil {
		log.Printf("[%s] parse StartTransaction.conf: %v", cpID, err)
		return
	}

	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return
	}

	// Find the pending transaction by matching the pending call
	// For now, find the most recent pending_start transaction
	cp.Mu.RLock()
	var targetConnector int
	var txID string
	for num, cr := range cp.Connectors {
		if cr.TransactionID != "" {
			txID = cr.TransactionID
			targetConnector = num
			break
		}
	}
	cp.Mu.RUnlock()

	if txID == "" {
		log.Printf("[%s] no pending transaction found for StartTransaction.conf", cpID)
		return
	}

	if resp.IDTagInfo.Status == "Accepted" {
		// Update transaction with numeric_id
		if err := r.transactionRepo.UpdateNumericID(context.Background(), txID, resp.TransactionID); err != nil {
			log.Printf("[%s] update numeric_id: %v", cpID, err)
			return
		}

		// Transition to Charging
		r.SetConnectorStatus(cpID, targetConnector, common.ConnectorCharging)

		// Start meter value generator
		cp.Mu.Lock()
		cr := cp.Connectors[targetConnector]
		cr.MeterGen.Start(cr.MeterGen.CurrentMeterWh())
		cp.Mu.Unlock()

		r.publishEvent("transaction.confirmed", cpID, &targetConnector,
			fmt.Sprintf("Transaction %s confirmed, numericId=%d", txID, resp.TransactionID))
		log.Printf("[%s] StartTransaction accepted, txID=%s numericId=%d", cpID, txID, resp.TransactionID)
	} else {
		// Rejected
		r.transactionRepo.UpdateStatus(context.Background(), txID, "failed")
		r.SetConnectorStatus(cpID, targetConnector, common.ConnectorAvailable)

		cp.Mu.Lock()
		cp.Connectors[targetConnector].TransactionID = ""
		cp.Mu.Unlock()

		r.publishEvent("transaction.failed", cpID, &targetConnector,
			fmt.Sprintf("Transaction %s rejected: %s", txID, resp.IDTagInfo.Status))
	}
}

func (r *Runtime) StopTransaction(cpID string, connectorID int, reason string) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.Lock()
	client := cp.Client
	connected := cp.Connected
	cr, connExists := cp.Connectors[connectorID]
	cp.Mu.Unlock()

	if !connected || client == nil {
		return fmt.Errorf("charge point %s not connected", cpID)
	}
	if !connExists {
		return fmt.Errorf("connector %d not found", connectorID)
	}

	cp.Mu.RLock()
	txID := cr.TransactionID
	cp.Mu.RUnlock()

	if txID == "" {
		return fmt.Errorf("no active transaction on connector %d", connectorID)
	}

	// Stop meter generator and get final meter
	cp.Mu.Lock()
	meterStop := cr.MeterGen.Stop()
	cp.Mu.Unlock()

	// Get numeric_id from DB
	tx, err := r.transactionRepo.GetByID(context.Background(), txID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}

	numericID := 0
	if tx.NumericID != nil {
		numericID = *tx.NumericID
	}

	// Send StopTransaction
	msg, err := client.protocol.BuildStopTransaction(context.Background(), ocpp.StopTransactionInput{
		TransactionID: numericID,
		MeterStop:     meterStop,
		Timestamp:     time.Now().UTC(),
		Reason:        reason,
	})
	if err != nil {
		return fmt.Errorf("build StopTransaction: %w", err)
	}

	if err := client.Send(msg); err != nil {
		return fmt.Errorf("send StopTransaction: %w", err)
	}

	// Persist
	r.transactionRepo.Stop(context.Background(), txID, meterStop, reason)

	// Transition: Charging -> Finishing -> Available
	r.SetConnectorStatus(cpID, connectorID, common.ConnectorFinishing)

	cp.Mu.Lock()
	cr.TransactionID = ""
	cp.Mu.Unlock()

	r.SetConnectorStatus(cpID, connectorID, common.ConnectorAvailable)

	r.publishEvent("transaction.stopped", cpID, &connectorID,
		fmt.Sprintf("Transaction %s stopped, meter=%dWh reason=%s", txID, meterStop, reason))
	log.Printf("[%s] StopTransaction sent, txID=%s meter=%dWh", cpID, txID, meterStop)
	return nil
}

func (r *Runtime) SendMeterValues(cpID string, connectorID int) error {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.Lock()
	client := cp.Client
	connected := cp.Connected
	cr, connExists := cp.Connectors[connectorID]
	cp.Mu.Unlock()

	if !connected || client == nil {
		return fmt.Errorf("charge point %s not connected", cpID)
	}
	if !connExists {
		return fmt.Errorf("connector %d not found", connectorID)
	}

	cp.Mu.RLock()
	txID := cr.TransactionID
	cp.Mu.RUnlock()

	if txID == "" {
		return fmt.Errorf("no active transaction on connector %d", connectorID)
	}

	tx, err := r.transactionRepo.GetByID(context.Background(), txID)
	if err != nil {
		return fmt.Errorf("get transaction: %w", err)
	}

	numericID := 0
	if tx.NumericID != nil {
		numericID = *tx.NumericID
	}

	currentMeter := cr.MeterGen.CurrentMeterWh()

	msg, err := client.protocol.BuildMeterValues(context.Background(), ocpp.MeterValuesInput{
		ConnectorID:   connectorID,
		TransactionID: &numericID,
		MeterValues: []ocpp.MeterValue{{
			Timestamp: time.Now().UTC(),
			SampledValue: []ocpp.SampledValue{
				{Value: fmt.Sprintf("%d", currentMeter), Measurand: "Energy.Active.Import.Register", Unit: "Wh"},
			},
		}},
	})
	if err != nil {
		return fmt.Errorf("build MeterValues: %w", err)
	}

	if err := client.Send(msg); err != nil {
		return fmt.Errorf("send MeterValues: %w", err)
	}

	r.publishEvent("meter_value.sent", cpID, &connectorID,
		fmt.Sprintf("MeterValues sent: %dWh", currentMeter))
	return nil
}

func (r *Runtime) GetChargePoint(id string) (*ChargePointInstance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp, ok := r.chargePoints[id]
	return cp, ok
}

func (r *Runtime) ListChargePoints() []*ChargePointInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*ChargePointInstance, 0, len(r.chargePoints))
	for _, cp := range r.chargePoints {
		list = append(list, cp)
	}
	return list
}

func (r *Runtime) GetConnectorState(cpID string, connectorID int) (ConnectorState, error) {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("charge point %s not found", cpID)
	}
	cp.Mu.RLock()
	defer cp.Mu.RUnlock()
	cr, exists := cp.Connectors[connectorID]
	if !exists {
		return "", fmt.Errorf("connector %d not found", connectorID)
	}
	return cr.StateMachine.Current(), nil
}

func (r *Runtime) GetMeterValue(cpID string, connectorID int) (int, error) {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return 0, fmt.Errorf("charge point %s not found", cpID)
	}
	cp.Mu.RLock()
	defer cp.Mu.RUnlock()
	cr, exists := cp.Connectors[connectorID]
	if !exists {
		return 0, fmt.Errorf("connector %d not found", connectorID)
	}
	return cr.MeterGen.CurrentMeterWh(), nil
}

func (r *Runtime) publishEvent(eventType, cpID string, connectorID *int, message string) {
	r.eventBus.Publish(realtime.Event{
		ID:            fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:          eventType,
		ChargePointID: cpID,
		ConnectorID:   connectorID,
		Message:       message,
		Timestamp:     time.Now().UTC(),
	})
}

func (r *Runtime) handleOCPPResponse(cpID string, action string, msg ocpp.Message) {
	switch action {
	case "StartTransaction":
		r.HandleStartTransactionResponse(cpID, msg)
	}
}

// HandleRemoteStartTransaction processes a CSMS-initiated RemoteStartTransaction command.
func (r *Runtime) HandleRemoteStartTransaction(cpID string, req *ocpp.RemoteStartTransactionRequest) (string, error) {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.Lock()
	client := cp.Client
	connected := cp.Connected
	cp.Mu.Unlock()

	if !connected || client == nil {
		return "", fmt.Errorf("charge point %s not connected", cpID)
	}

	// Determine target connector
	connectorID := 0
	if req.ConnectorID != nil {
		connectorID = *req.ConnectorID
	}

	cp.Mu.Lock()
	defer cp.Mu.Unlock()

	if connectorID == 0 {
		// Pick first available connector
		for id, cr := range cp.Connectors {
			if cr.StateMachine.Current() == StateAvailable {
				connectorID = id
				break
			}
		}
		if connectorID == 0 {
			return "Rejected", nil
		}
	}

	cr, exists := cp.Connectors[connectorID]
	if !exists {
		return "Rejected", nil
	}

	// Check connector is in a state that allows starting
	if !cr.StateMachine.CanTransitionTo(StatePreparing) {
		return "Rejected", nil
	}

	// Check no active transaction
	if cr.TransactionID != "" {
		return "Rejected", nil
	}

	// Transition to Preparing
	if err := cr.StateMachine.Transition(StatePreparing); err != nil {
		return "Rejected", nil
	}

	r.publishEvent("remote.start_transaction.accepted", cpID, &connectorID,
		fmt.Sprintf("RemoteStartTransaction accepted for connector %d, idTag=%s", connectorID, req.IDTag))

	// Trigger async StartTransaction flow
	go func() {
		if err := r.StartTransaction(cpID, connectorID, req.IDTag); err != nil {
			log.Printf("[%s] async StartTransaction after RemoteStartTransaction failed: %v", cpID, err)
			r.publishEvent("transaction.failed", cpID, &connectorID,
				fmt.Sprintf("StartTransaction after RemoteStartTransaction failed: %v", err))
		}
	}()

	return "Accepted", nil
}

// HandleRemoteStopTransaction processes a CSMS-initiated RemoteStopTransaction command.
func (r *Runtime) HandleRemoteStopTransaction(cpID string, req *ocpp.RemoteStopTransactionRequest) (string, error) {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.Lock()
	client := cp.Client
	connected := cp.Connected
	cp.Mu.Unlock()

	if !connected || client == nil {
		return "", fmt.Errorf("charge point %s not connected", cpID)
	}

	// Find connector with matching active transaction
	cp.Mu.RLock()
	var targetConnector int
	var targetCR *ConnectorRuntime
	for id, cr := range cp.Connectors {
		if cr.TransactionID != "" {
			// Check if this transaction matches the requested numeric ID
			tx, err := r.transactionRepo.GetByID(context.Background(), cr.TransactionID)
			if err == nil && tx.NumericID != nil && *tx.NumericID == req.TransactionID {
				targetConnector = id
				targetCR = cr
				break
			}
		}
	}
	cp.Mu.RUnlock()

	if targetCR == nil {
		return "Rejected", nil
	}

	r.publishEvent("remote.stop_transaction.accepted", cpID, &targetConnector,
		fmt.Sprintf("RemoteStopTransaction accepted for connector %d, transactionId=%d", targetConnector, req.TransactionID))

	// Trigger async StopTransaction flow
	go func() {
		if err := r.StopTransaction(cpID, targetConnector, "Remote"); err != nil {
			log.Printf("[%s] async StopTransaction after RemoteStopTransaction failed: %v", cpID, err)
			r.publishEvent("transaction.failed", cpID, &targetConnector,
				fmt.Sprintf("StopTransaction after RemoteStopTransaction failed: %v", err))
		}
	}()

	return "Accepted", nil
}

// HandleReset processes a CSMS-initiated Reset command.
func (r *Runtime) HandleReset(cpID string, req *ocpp.ResetRequest) (string, error) {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.RLock()
	connected := cp.Connected
	client := cp.Client
	cp.Mu.RUnlock()

	if !connected || client == nil {
		return "", fmt.Errorf("charge point %s not connected", cpID)
	}

	r.publishEvent("remote.reset", cpID, nil,
		fmt.Sprintf("Reset (%s) accepted", req.Type))

	if req.Type == "Hard" {
		// Hard reset: disconnect and reconnect
		go func() {
			r.Disconnect(cpID)
			time.Sleep(2 * time.Second)
			if err := r.Connect(cpID); err != nil {
				log.Printf("[%s] reconnect after Hard reset failed: %v", cpID, err)
			}
		}()
	}
	// Soft reset: no immediate action needed in simulator context

	return "Accepted", nil
}

// HandleUnlockConnector processes a CSMS-initiated UnlockConnector command.
func (r *Runtime) HandleUnlockConnector(cpID string, connectorID int) (string, error) {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.Lock()
	cr, exists := cp.Connectors[connectorID]
	if !exists {
		cp.Mu.Unlock()
		return "UnlockFailed", nil
	}

	// If there's an active transaction, stop it first
	hasActive := cr.TransactionID != ""
	cp.Mu.Unlock()

	if hasActive {
		if err := r.StopTransaction(cpID, connectorID, "UnlockCommand"); err != nil {
			log.Printf("[%s] StopTransaction during UnlockConnector failed: %v", cpID, err)
		}
	}

	// Transition to Available
	cp.Mu.Lock()
	if err := cr.StateMachine.Transition(StateAvailable); err != nil {
		cp.Mu.Unlock()
		return "UnlockFailed", nil
	}
	cp.Mu.Unlock()

	r.publishEvent("remote.unlock_connector", cpID, &connectorID,
		fmt.Sprintf("Connector %d unlocked", connectorID))

	return "Unlocked", nil
}

// HandleChangeConfiguration processes a CSMS-initiated ChangeConfiguration command.
func (r *Runtime) HandleChangeConfiguration(cpID string, key, value string) (string, error) {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.RLock()
	connected := cp.Connected
	cp.Mu.RUnlock()

	if !connected {
		return "", fmt.Errorf("charge point %s not connected", cpID)
	}

	// Simulate known configuration keys
	knownKeys := map[string]bool{
		"HeartbeatInterval":            true,
		"MeterValueSampleInterval":     true,
		"ClockAlignedDataInterval":     true,
		"NumberOfConnectors":           true,
		"ConnectionTimeOut":            true,
		"WebSocketPingInterval":        true,
		"LocalPreAuthorize":            true,
		"StopTransactionOnEVSideDisconnect": true,
	}

	if _, known := knownKeys[key]; !known {
		return "NotSupported", nil
	}

	r.publishEvent("remote.change_configuration", cpID, nil,
		fmt.Sprintf("Configuration changed: %s=%s", key, value))

	return "Accepted", nil
}

// HandleGetConfiguration processes a CSMS-initiated GetConfiguration command.
func (r *Runtime) HandleGetConfiguration(cpID string, keys []string) ([]ocpp.ConfigurationKey, []string, error) {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return nil, nil, fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.RLock()
	connected := cp.Connected
	cp.Mu.RUnlock()

	if !connected {
		return nil, nil, fmt.Errorf("charge point %s not connected", cpID)
	}

	// Simulated configuration store
	defaultConfig := map[string]string{
		"HeartbeatInterval":            "300",
		"MeterValueSampleInterval":     "60",
		"ClockAlignedDataInterval":     "0",
		"NumberOfConnectors":           fmt.Sprintf("%d", len(cp.Connectors)),
		"ConnectionTimeOut":            "60",
		"WebSocketPingInterval":        "0",
		"LocalPreAuthorize":            "false",
		"StopTransactionOnEVSideDisconnect": "true",
	}

	if len(keys) == 0 {
		// Return all keys
		var configKeys []ocpp.ConfigurationKey
		for k, v := range defaultConfig {
			val := v
			configKeys = append(configKeys, ocpp.ConfigurationKey{
				Key:      k,
				Readonly: false,
				Value:    &val,
			})
		}
		return configKeys, nil, nil
	}

	var configKeys []ocpp.ConfigurationKey
	var unknownKeys []string

	for _, k := range keys {
		if v, ok := defaultConfig[k]; ok {
			val := v
			configKeys = append(configKeys, ocpp.ConfigurationKey{
				Key:      k,
				Readonly: false,
				Value:    &val,
			})
		} else {
			unknownKeys = append(unknownKeys, k)
		}
	}

	return configKeys, unknownKeys, nil
}

// HandleTriggerMessage processes a CSMS-initiated TriggerMessage command.
func (r *Runtime) HandleTriggerMessage(cpID string, requestedMessage string, connectorID *int) (string, error) {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.RLock()
	client := cp.Client
	connected := cp.Connected
	cp.Mu.RUnlock()

	if !connected || client == nil {
		return "", fmt.Errorf("charge point %s not connected", cpID)
	}

	r.publishEvent("remote.trigger_message", cpID, nil,
		fmt.Sprintf("TriggerMessage: %s", requestedMessage))

	// Trigger the requested message asynchronously
	go func() {
		ctx := context.Background()
		var err error
		switch requestedMessage {
		case "BootNotification":
			err = r.SendBootNotification(cpID)
		case "Heartbeat":
			err = r.SendHeartbeat(cpID)
		case "StatusNotification":
			if connectorID != nil {
				cp.Mu.RLock()
				cr, exists := cp.Connectors[*connectorID]
				if exists {
					status := string(cr.StateMachine.Current())
					cp.Mu.RUnlock()
					err = client.SendStatusNotification(ctx, *connectorID, status, "NoError")
				} else {
					cp.Mu.RUnlock()
				}
			}
		case "MeterValues":
			if connectorID != nil {
				err = r.SendMeterValues(cpID, *connectorID)
			}
		default:
			// DiagnosticsStatusNotification, FirmwareStatusNotification not supported
			log.Printf("[%s] unsupported TriggerMessage: %s", cpID, requestedMessage)
		}
		if err != nil {
			log.Printf("[%s] TriggerMessage %s failed: %v", cpID, requestedMessage, err)
		}
	}()

	return "Accepted", nil
}

// HandleChangeAvailability processes a CSMS-initiated ChangeAvailability command.
func (r *Runtime) HandleChangeAvailability(cpID string, connectorID int, availType string) (string, error) {
	r.mu.RLock()
	cp, ok := r.chargePoints[cpID]
	r.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("charge point %s not found", cpID)
	}

	cp.Mu.Lock()
	defer cp.Mu.Unlock()

	// connectorId=0 means whole station
	if connectorID == 0 {
		targetState := StateAvailable
		if availType == "Inoperative" {
			targetState = StateUnavailable
		}
		for _, cr := range cp.Connectors {
			if err := cr.StateMachine.Transition(targetState); err != nil {
				// If there's an active transaction, schedule the change
				if cr.TransactionID != "" && availType == "Inoperative" {
					return "Scheduled", nil
				}
			}
		}
		r.publishEvent("remote.change_availability", cpID, nil,
			fmt.Sprintf("Station availability changed to %s", availType))
		return "Accepted", nil
	}

	cr, exists := cp.Connectors[connectorID]
	if !exists {
		return "Rejected", nil
	}

	targetState := StateAvailable
	if availType == "Inoperative" {
		targetState = StateUnavailable
	}

	if err := cr.StateMachine.Transition(targetState); err != nil {
		// If there's an active transaction, schedule the change
		if cr.TransactionID != "" && availType == "Inoperative" {
			return "Scheduled", nil
		}
		return "Rejected", nil
	}

	r.publishEvent("remote.change_availability", cpID, &connectorID,
		fmt.Sprintf("Connector %d availability changed to %s", connectorID, availType))
	return "Accepted", nil
}

// WireMessageLog persists an OCPP message to the database.
func (r *Runtime) WireMessageLog(cpID, direction, action string, payload json.RawMessage) {
	if r.messageLogRepo == nil {
		return
	}
	r.messageLogRepo.Create(context.Background(), db.OCPPMessageLog{
		ChargePointID: cpID,
		MessageType:   "OCPP",
		Direction:     direction,
		OCPPVersion:   "1.6J",
		Action:        &action,
		PayloadJSON:   string(payload),
		Status:        "ok",
	})
}
