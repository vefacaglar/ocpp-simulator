package db

import (
	"context"
	"testing"
)

func TestTransactionRepo_CreateAndAssignNumericID(t *testing.T) {
	ctx := context.Background()
	repo, _, _, cleanup := openTestDB(t)
	defer cleanup()

	// First transaction gets numeric_id assigned at Create time.
	id1, err := repo.Create(ctx, Transaction{
		NumericID:       intPtr(1),
		ChargePointID:   "CP-001",
		ConnectorNumber: 1,
		IDTag:           "TAG",
		OCPPVersion:     "1.6J",
		Status:          "active",
		StartMeterWh:    0,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Second is created without numeric_id (pending_start).
	id2, err := repo.Create(ctx, Transaction{
		ChargePointID:   "CP-001",
		ConnectorNumber: 1,
		IDTag:           "TAG",
		OCPPVersion:     "1.6J",
		Status:          "pending_start",
	})
	if err != nil {
		t.Fatalf("create pending: %v", err)
	}

	// Atomic assignment from "pending_start" to "active" with a
	// numeric id.
	if err := repo.AssignNumericID(ctx, id2, 2, "active"); err != nil {
		t.Fatalf("assign numeric_id: %v", err)
	}

	// Reassigning the same id must fail (no row updated).
	if err := repo.AssignNumericID(ctx, id2, 3, "active"); err == nil {
		t.Errorf("expected error re-assigning numeric_id, got nil")
	}

	// Read both back.
	got1, err := repo.GetByID(ctx, id1)
	if err != nil {
		t.Fatalf("get id1: %v", err)
	}
	if got1.NumericID == nil || *got1.NumericID != 1 {
		t.Errorf("id1 numeric_id = %v, want 1", got1.NumericID)
	}
	got2, err := repo.GetByID(ctx, id2)
	if err != nil {
		t.Fatalf("get id2: %v", err)
	}
	if got2.NumericID == nil || *got2.NumericID != 2 {
		t.Errorf("id2 numeric_id = %v, want 2", got2.NumericID)
	}
	if got2.Status != "active" {
		t.Errorf("id2 status = %q, want active", got2.Status)
	}
}

func TestTransactionRepo_MaxNumericID(t *testing.T) {
	ctx := context.Background()
	repo, _, _, cleanup := openTestDB(t)
	defer cleanup()

	// Empty: max is 0.
	if n, err := repo.MaxNumericID(ctx); err != nil || n != 0 {
		t.Fatalf("max empty: n=%d err=%v", n, err)
	}

	// Insert two with numeric ids.
	if _, err := repo.Create(ctx, Transaction{NumericID: intPtr(7), ChargePointID: "CP", ConnectorNumber: 1, IDTag: "T", OCPPVersion: "1.6J", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, Transaction{NumericID: intPtr(3), ChargePointID: "CP", ConnectorNumber: 1, IDTag: "T", OCPPVersion: "1.6J", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	// Insert one without numeric id — must be ignored by MAX.
	if _, err := repo.Create(ctx, Transaction{ChargePointID: "CP", ConnectorNumber: 1, IDTag: "T", OCPPVersion: "1.6J", Status: "pending_start"}); err != nil {
		t.Fatal(err)
	}
	n, err := repo.MaxNumericID(ctx)
	if err != nil {
		t.Fatalf("max: %v", err)
	}
	if n != 7 {
		t.Errorf("max = %d, want 7", n)
	}
}

func TestTransactionRepo_Stop(t *testing.T) {
	ctx := context.Background()
	repo, _, _, cleanup := openTestDB(t)
	defer cleanup()

	id, err := repo.Create(ctx, Transaction{
		NumericID: intPtr(5), ChargePointID: "CP", ConnectorNumber: 1, IDTag: "T", OCPPVersion: "1.6J", Status: "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Stop(ctx, id, 1234, "Local"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	got, _ := repo.GetByID(ctx, id)
	if got.Status != "stopped" {
		t.Errorf("status = %q, want stopped", got.Status)
	}
	if got.StopMeterWh == nil || *got.StopMeterWh != 1234 {
		t.Errorf("stop_meter_wh = %v, want 1234", got.StopMeterWh)
	}
}

func intPtr(i int) *int { return &i }

func TestMessageLogRepo_CreateAndList(t *testing.T) {
	ctx := context.Background()
	_, logs, _, cleanup := openTestDB(t)
	defer cleanup()

	action := "BootNotification"
	direction := "outbound"
	uid := "uid-1"
	if _, err := logs.Create(ctx, MessageLog{
		ChargePointID: "CP-001",
		Direction:     direction,
		MessageType:   "CALLRESULT",
		Action:        &action,
		UniqueID:      &uid,
		PayloadJSON:   `[3,"uid-1",{"status":"Accepted","currentTime":"2025-01-01T00:00:00Z","interval":300}]`,
		Status:        "ok",
		Topic:         "ocpp/CP-001/out",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	list, err := logs.ListByChargePoint(ctx, "CP-001", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 log, got %d", len(list))
	}
	if list[0].PayloadJSON == "" {
		t.Errorf("payload must be persisted verbatim")
	}
	if list[0].Direction != "outbound" {
		t.Errorf("direction = %q", list[0].Direction)
	}
}

func TestRuntimeEventRepo_CreateAndList(t *testing.T) {
	ctx := context.Background()
	_, _, rt, cleanup := openTestDB(t)
	defer cleanup()

	cpID := "CP-001"
	if _, err := rt.Create(ctx, RuntimeEvent{
		ChargePointID: &cpID,
		EventType:     "transaction.authorized",
		Severity:      "info",
		Message:       "idTag accepted",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}
	list, err := rt.ListByChargePoint(ctx, "CP-001", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 event, got %d", len(list))
	}
	if list[0].EventType != "transaction.authorized" {
		t.Errorf("event_type = %q", list[0].EventType)
	}
}
