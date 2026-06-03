package transaction

import (
	"context"
	"testing"

	"github.com/user/ocpp-simulator/apps/ocpp-core/internal/db"
)

func newSvc(t *testing.T) (*Service, *db.TransactionRepo, func()) {
	t.Helper()
	d, err := db.Open(db.TestDBURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(d); err != nil {
		t.Fatal(err)
	}
	// Clean state
	for _, tbl := range []string{"transactions", "ocpp_message_logs", "runtime_events"} {
		_, _ = d.ExecContext(context.Background(), "TRUNCATE TABLE "+tbl+" RESTART IDENTITY CASCADE")
	}
	repo := db.NewTransactionRepo(d)
	return NewService(repo), repo, func() { d.Close() }
}

func TestService_InitCounter_SeedsFromMax(t *testing.T) {
	ctx := context.Background()
	svc, repo, cleanup := newSvc(t)
	defer cleanup()

	// Pre-populate two transactions with numeric ids 5 and 12.
	if _, err := repo.Create(ctx, db.Transaction{
		NumericID: intPtr(5), ChargePointID: "CP", ConnectorNumber: 1, IDTag: "T", OCPPVersion: "1.6J", Status: "active",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, db.Transaction{
		NumericID: intPtr(12), ChargePointID: "CP", ConnectorNumber: 1, IDTag: "T", OCPPVersion: "1.6J", Status: "active",
	}); err != nil {
		t.Fatal(err)
	}

	if err := svc.InitCounter(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}

	// The next Start must yield numeric id 13.
	res, err := svc.Start(ctx, StartInput{ChargePointID: "CP", ConnectorNumber: 1, IDTag: "T", MeterStart: 0, OCPPVersion: "1.6J"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if res.TransactionID != 13 {
		t.Errorf("transactionId = %d, want 13 (restart-safe)", res.TransactionID)
	}
	if res.IDTagInfo.Status != "Accepted" {
		t.Errorf("idTagInfo.status = %q, want Accepted", res.IDTagInfo.Status)
	}
}

func TestService_Start_MonotonicPerProcess(t *testing.T) {
	ctx := context.Background()
	svc, _, cleanup := newSvc(t)
	defer cleanup()
	if err := svc.InitCounter(ctx); err != nil {
		t.Fatal(err)
	}
	ids := make(map[int]bool)
	for i := 0; i < 5; i++ {
		res, err := svc.Start(ctx, StartInput{ChargePointID: "CP", ConnectorNumber: 1, IDTag: "T", MeterStart: 0, OCPPVersion: "1.6J"})
		if err != nil {
			t.Fatal(err)
		}
		if ids[res.TransactionID] {
			t.Errorf("duplicate id %d", res.TransactionID)
		}
		ids[res.TransactionID] = true
	}
}

func intPtr(i int) *int { return &i }
