package db

import (
	"context"
	"testing"
)

// openTestDB connects to the shared postgres test database.
// It truncates all tables before returning so each test starts clean.
func openTestDB(t *testing.T) (*TransactionRepo, *MessageLogRepo, *RuntimeEventRepo, func()) {
	t.Helper()
	d, err := Open(TestDBURL)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := Migrate(d); err != nil {
		d.Close()
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	for _, tbl := range []string{"transactions", "ocpp_message_logs", "runtime_events"} {
		_, _ = d.ExecContext(ctx, "TRUNCATE TABLE "+tbl+" RESTART IDENTITY CASCADE")
	}
	return NewTransactionRepo(d), NewMessageLogRepo(d), NewRuntimeEventRepo(d), func() { d.Close() }
}
