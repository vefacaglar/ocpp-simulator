// Package transaction owns the transaction lifecycle in ocpp-core.
// The numeric counter is initialized from MAX(numeric_id) on startup
// so a process restart cannot reuse an existing id, then advanced
// atomically per StartTransaction call.
package transaction

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/db"
)

// Service is the transaction lifecycle owner.
type Service struct {
	repo *db.TransactionRepo

	mu      sync.Mutex
	nextID  int
}

func NewService(repo *db.TransactionRepo) *Service {
	return &Service{repo: repo}
}

// InitCounter seeds the in-memory numeric counter from the highest
// numeric_id currently in storage. Must be called once at startup
// before any StartTransaction requests are served.
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
// CP→server StartTransaction.req.
type StartInput struct {
	ChargePointID   string
	EVSEID          int
	ConnectorNumber int
	IDTag           string
	MeterStart      int
	OCPPVersion     string
}

// StartResult is the data message-processor forwards to the
// CALLRESULT payload for the CP.
type StartResult struct {
	TransactionID int
	IDTagInfo     struct {
		Status string `json:"status"`
	}
}

// Start creates a transaction row with a fresh UUID, assigns the
// next available numeric id, marks it active, and returns the
// numeric id + Accepted idTagInfo. idTagInfo.status is hard-coded
// Accepted for MVP (nextplan §2b minimum business policy).
func (s *Service) Start(ctx context.Context, in StartInput) (StartResult, error) {
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
	log.Printf("[ocpp-core/transaction] started tx %s (cp=%s numeric=%d)", id, in.ChargePointID, numericID)

	res := StartResult{TransactionID: numericID}
	res.IDTagInfo.Status = "Accepted"
	return res, nil
}

// Stop finalizes a transaction by id.
func (s *Service) Stop(ctx context.Context, id string, meterStop int, reason string) error {
	return s.repo.Stop(ctx, id, meterStop, reason)
}

// GetByID is a thin pass-through used by handlers that need to
// resolve a UUID from a numeric id (CSMS-initiated RemoteStop).
func (s *Service) GetByID(ctx context.Context, id string) (*db.Transaction, error) {
	return s.repo.GetByID(ctx, id)
}
