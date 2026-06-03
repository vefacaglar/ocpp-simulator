package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID              string
	NumericID       *int
	ChargePointID   string
	EVSEID          int
	ConnectorNumber int
	IDTag           string
	OCPPVersion     string
	Status          string
	StartedAt       *string
	StoppedAt       *string
	StartMeterWh    int
	StopMeterWh     *int
	StopReason      *string
	CreatedAt       string
	UpdatedAt       string
}

type TransactionRepo struct {
	db *sql.DB
}

func NewTransactionRepo(d *sql.DB) *TransactionRepo { return &TransactionRepo{db: d} }

func (r *TransactionRepo) Create(ctx context.Context, t Transaction) (string, error) {
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if t.CreatedAt == "" {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO transactions (id, numeric_id, charge_point_id, evse_id, connector_number, id_tag, ocpp_version, status, start_meter_wh, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.NumericID, t.ChargePointID, t.EVSEID, t.ConnectorNumber, t.IDTag, t.OCPPVersion, t.Status, t.StartMeterWh, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return "", fmt.Errorf("insert transaction: %w", err)
	}
	return t.ID, nil
}

// AssignNumericID atomically writes a numeric_id for a pending
// transaction. Used by StartTransaction: once a fresh row is
// inserted with numeric_id=NULL, the counter service computes the
// next value and this method stores it in a single SQL statement
// (UPDATE ... WHERE numeric_id IS NULL), so a parallel writer
// cannot assign the same id twice.
func (r *TransactionRepo) AssignNumericID(ctx context.Context, id string, numericID int, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.ExecContext(ctx, `
		UPDATE transactions
		SET numeric_id = ?, status = ?, started_at = COALESCE(started_at, ?), updated_at = ?
		WHERE id = ? AND numeric_id IS NULL
	`, numericID, status, now, now, id)
	if err != nil {
		return fmt.Errorf("assign numeric_id: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("transaction %s already has numeric_id or does not exist", id)
	}
	return nil
}

func (r *TransactionRepo) UpdateStatus(ctx context.Context, id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `UPDATE transactions SET status = ?, updated_at = ? WHERE id = ?`, status, now, id)
	return err
}

func (r *TransactionRepo) Stop(ctx context.Context, id string, meterStop int, reason string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `
		UPDATE transactions SET status = 'stopped', stop_meter_wh = ?, stop_reason = ?, stopped_at = ?, updated_at = ?
		WHERE id = ?
	`, meterStop, reason, now, now, id)
	return err
}

func (r *TransactionRepo) GetByID(ctx context.Context, id string) (*Transaction, error) {
	var t Transaction
	err := r.db.QueryRowContext(ctx, `
		SELECT id, numeric_id, charge_point_id, evse_id, connector_number, id_tag, ocpp_version, status,
		       started_at, stopped_at, start_meter_wh, stop_meter_wh, stop_reason, created_at, updated_at
		FROM transactions WHERE id = ?
	`, id).Scan(&t.ID, &t.NumericID, &t.ChargePointID, &t.EVSEID, &t.ConnectorNumber, &t.IDTag, &t.OCPPVersion, &t.Status,
		&t.StartedAt, &t.StoppedAt, &t.StartMeterWh, &t.StopMeterWh, &t.StopReason, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListByChargePoint returns all transactions for a CP, newest first.
func (r *TransactionRepo) ListByChargePoint(ctx context.Context, chargePointID string) ([]Transaction, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, numeric_id, charge_point_id, evse_id, connector_number, id_tag, ocpp_version, status,
		       started_at, stopped_at, start_meter_wh, stop_meter_wh, stop_reason, created_at, updated_at
		FROM transactions WHERE charge_point_id = ? ORDER BY created_at DESC
	`, chargePointID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.NumericID, &t.ChargePointID, &t.EVSEID, &t.ConnectorNumber, &t.IDTag, &t.OCPPVersion, &t.Status,
			&t.StartedAt, &t.StoppedAt, &t.StartMeterWh, &t.StopMeterWh, &t.StopReason, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// MaxNumericID returns the highest numeric_id currently stored
// across all transactions, or 0 if none. Used at startup to seed
// the in-memory counter so a restart cannot re-use an existing
// id.
func (r *TransactionRepo) MaxNumericID(ctx context.Context) (int, error) {
	var n sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(numeric_id), 0) FROM transactions WHERE numeric_id IS NOT NULL`).Scan(&n)
	if err != nil {
		return 0, err
	}
	return int(n.Int64), nil
}

var ErrNotFound = fmt.Errorf("not found")
