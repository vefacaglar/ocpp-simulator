package db

import (
	"path/filepath"
	"testing"
)

// openTestDB returns a fresh sqlite database in a per-test temp
// directory with migrations applied.
func openTestDB(t *testing.T) (*TransactionRepo, *MessageLogRepo, *RuntimeEventRepo, func()) {
	t.Helper()
	dir := t.TempDir()
	d, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := Migrate(d); err != nil {
		d.Close()
		t.Fatalf("migrate: %v", err)
	}
	return NewTransactionRepo(d), NewMessageLogRepo(d), NewRuntimeEventRepo(d), func() { d.Close() }
}
