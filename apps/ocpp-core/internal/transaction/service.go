// Package transaction owns the transaction lifecycle in ocpp-core.
// The numeric counter is initialized from MAX(numeric_id) on startup
// so a process restart cannot reuse an existing id, then advanced
// atomically per StartTransaction call.
//
// 2.0.1 transactions are identified by a string GUID (the
// transactionInfo.transactionId the CP chose when starting the
// transaction). The 1.6J path still gets a numeric id assigned
// by the CSMS in StartTransaction.conf. See plan.md §7.6/§8.4/§13.
package transaction

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/vefacaglar/ocpp-simulator/apps/ocpp-core/internal/db"
	"github.com/google/uuid"
)

// Service is the transaction lifecycle owner.
type Service struct {
	repo *db.TransactionRepo

	mu     sync.Mutex
	nextID int
}

func NewService(repo *db.TransactionRepo) *Service {
	return &Service{repo: repo}
}

// InitCounter seeds the in-memory numeric counter from the highest
// numeric_id currently in storage. Must be called once at startup
// before any StartTransaction requests are served. Only relevant
// to the 1.6J path; 2.0.1 transactions do not get a numeric id.
func (s *Service) InitCounter(ctx context.Context) error {
	maxID, err := s.repo.MaxNumericID(ctx)
	if err != nil {
		return fmt.Errorf("read max numeric_id: %w", err)
	}
	s.mu.Lock()
	s.nextID = maxID + 1
	s.mu.Unlock()
	log.Printf("[ocpp-core/transaction] numeric counter initialized at %d (max stored: %d)", maxID+1, maxID)
	return nil
}

// StartInput is the data the message-processor forwards from a
// CP→server StartTransaction.req (1.6J) or a TransactionEvent
// eventType=Started (2.0.1). The OCPPVersion field selects the
// path; for 2.0.1 the TransactionID field carries the
// CP-chosen string GUID.
type StartInput struct {
	ChargePointID   string
	EVSEID          int
	ConnectorNumber int
	IDTag           string
	MeterStart      int
	OCPPVersion     string
	// TransactionID is the CP-chosen string GUID for 2.0.1.
	// Ignored for 1.6J (the CSMS-assigned numeric id is what the
	// CP will see in the response).
	TransactionID string
}

// StartResult is the data message-processor forwards to the
// CALLRESULT payload for the CP. For 1.6J, TransactionID is the
// CSMS-assigned integer; for 2.0.1, it is unused (the
// TransactionEvent.conf body carries idTokenInfo and the
// transaction context stays in transactionInfo).
type StartResult struct {
	TransactionID int
	IDTagInfo     struct {
		Status string `json:"status"`
	}
}

// Start creates a transaction row, assigns the appropriate
// identifier (numeric for 1.6J, GUID for 2.0.1), marks it
// active, and returns the spec-exact response fields. 1.6J
// returns the numeric id; 2.0.1 returns an empty StartResult
// (the TransactionEvent.conf body is built by the API layer from
// the stored idTokenInfo).
func (s *Service) Start(ctx context.Context, in StartInput) (StartResult, error) {
	if in.OCPPVersion == "2.0.1" {
		return s.startV201(ctx, in)
	}
	return s.startV16(ctx, in)
}

func (s *Service) startV16(ctx context.Context, in StartInput) (StartResult, error) {
	s.mu.Lock()
	numericID := s.nextID
	s.nextID++
	s.mu.Unlock()

	id, err := s.repo.Create(ctx, db.Transaction{
		NumericID:       &numericID,
		ChargePointID:   in.ChargePointID,
		EVSEID:          in.EVSEID,
		ConnectorNumber: in.ConnectorNumber,
		IDTag:           in.IDTag,
		OCPPVersion:     in.OCPPVersion,
		Status:          "active",
		StartMeterWh:    in.MeterStart,
	})
	if err != nil {
		// Roll back the in-memory counter so a transient failure
		// does not silently skip an id. The next call will
		// reuse the same numeric value.
		s.mu.Lock()
		s.nextID = numericID
		s.mu.Unlock()
		return StartResult{}, fmt.Errorf("create transaction: %w", err)
	}
	log.Printf("[ocpp-core/transaction] started 1.6J tx %s (cp=%s numeric=%d)", id, in.ChargePointID, numericID)

	res := StartResult{TransactionID: numericID}
	res.IDTagInfo.Status = "Accepted"
	return res, nil
}

// startV201 persists a 2.0.1 transaction row with the CP-chosen
// string GUID and no numeric id. The result's TransactionID is
// unused; the API layer builds the TransactionEvent.conf body
// (idTokenInfo {status: Accepted}).
func (s *Service) startV201(ctx context.Context, in StartInput) (StartResult, error) {
	txID := in.TransactionID
	if txID == "" {
		// CP-driven identity: the CP's TransactionEvent
		// transactionInfo.transactionId IS the canonical
		// transaction identity. A missing id is a malformed
		// event; we synthesize a UUID so the row is not lost,
		// but log loudly so the simulator can be fixed.
		txID = uuid.NewString()
		log.Printf("[ocpp-core/transaction] WARNING: 2.0.1 Started event arrived with no transactionId; synthesized %s", txID)
	}
	_, err := s.repo.Create(ctx, db.Transaction{
		// Persist the CP-chosen GUID (or the synthesized fallback)
		// as the row's primary key so a later Ended event with
		// the same transactionId matches the row and can update
		// it to "stopped". Without this the repo would mint a
		// fresh UUID and the Stop call would 0-row-update.
		ID:              txID,
		NumericID:       nil, // 2.0.1: no async numeric assignment
		ChargePointID:   in.ChargePointID,
		EVSEID:          in.EVSEID,
		ConnectorNumber: in.ConnectorNumber,
		IDTag:           in.IDTag,
		OCPPVersion:     in.OCPPVersion,
		Status:          "active",
		StartMeterWh:    in.MeterStart,
	})
	if err != nil {
		return StartResult{}, fmt.Errorf("create 2.0.1 transaction: %w", err)
	}
	log.Printf("[ocpp-core/transaction] started 2.0.1 tx id=%s (cp=%s)", txID, in.ChargePointID)

	res := StartResult{}
	res.IDTagInfo.Status = "Accepted"
	return res, nil
}

// Stop finalizes a transaction by id (UUID for 2.0.1, internal
// UUID for 1.6J since the repo writes with NumericID as a
// separate field).
func (s *Service) Stop(ctx context.Context, id string, meterStop int, reason string) error {
	return s.repo.Stop(ctx, id, meterStop, reason)
}

// GetByID is a thin pass-through used by handlers that need to
// resolve a UUID from a numeric id (CSMS-initiated RemoteStop).
func (s *Service) GetByID(ctx context.Context, id string) (*db.Transaction, error) {
	return s.repo.GetByID(ctx, id)
}

// EventInput is the data the message-processor forwards from a
// 2.0.1 TransactionEvent.req.
type EventInput struct {
	ChargePointID   string
	EventType       string // "Started" | "Updated" | "Ended"
	TransactionID   string // CP-chosen GUID
	EVSEID          int
	ConnectorNumber int
	IDTag           string
	SeqNo           int
	MeterValue      json.RawMessage // optional, attached for audit
}

// EventResult is the data message-processor forwards to the
// TransactionEvent.conf body.
type EventResult struct {
	// IDTokenInfo is the only meaningful field on
	// TransactionEvent.conf in the MVP. The full schema allows
	// updatedPersonalMessage and chargingPriority; both are
	// omitted here (the CSMS does not impose them).
	IDTokenInfo struct {
		Status string `json:"status"`
	}
}

// HandleEvent routes a TransactionEvent to the right
// transaction-lifecycle action: Started creates a row, Ended
// finalizes it, Updated is a no-op (the meter data was already
// accepted on the wire). The result feeds the TransactionEvent.conf
// body the processor publishes.
func (s *Service) HandleEvent(ctx context.Context, in EventInput) (EventResult, error) {
	res := EventResult{}
	res.IDTokenInfo.Status = "Accepted"

	switch in.EventType {
	case "Started":
		_, err := s.Start(ctx, StartInput{
			ChargePointID:   in.ChargePointID,
			EVSEID:          in.EVSEID,
			ConnectorNumber: in.ConnectorNumber,
			IDTag:           in.IDTag,
			OCPPVersion:     "2.0.1",
			TransactionID:   in.TransactionID,
		})
		if err != nil {
			return EventResult{}, err
		}
		return res, nil
	case "Updated":
		// No DB write for Updated events in MVP; the meter
		// values on the wire are already on the canonical log.
		return res, nil
	case "Ended":
		// Use the CP's transactionId directly as the row id.
		// The transaction was inserted at Started with a
		// CP-chosen GUID; Ended just sets the stop fields.
		if err := s.repo.Stop(ctx, in.TransactionID, 0, ""); err != nil {
			return EventResult{}, err
		}
		return res, nil
	default:
		return EventResult{}, fmt.Errorf("unknown transaction event type %q", in.EventType)
	}
}
