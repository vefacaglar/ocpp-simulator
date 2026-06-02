package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID               string
	NumericID        *int
	ChargePointID    string
	EVSEID           int
	ConnectorNumber  int
	IDTag            string
	OCPPVersion      string
	Status           string
	StartedAt        *string
	StoppedAt        *string
	StartMeterWh     int
	StopMeterWh      *int
	StopReason       *string
	CreatedAt        string
	UpdatedAt        string
}

type TransactionRepo struct {
	db *sql.DB
}

func NewTransactionRepo(db *sql.DB) *TransactionRepo {
	return &TransactionRepo{db: db}
}

func (r *TransactionRepo) Create(ctx context.Context, tx Transaction) (string, error) {
	id := tx.ID
	if id == "" {
		id = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO transactions (id, numeric_id, charge_point_id, evse_id, connector_number, id_tag, ocpp_version, status, start_meter_wh, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, tx.NumericID, tx.ChargePointID, tx.EVSEID, tx.ConnectorNumber, tx.IDTag, tx.OCPPVersion, tx.Status, tx.StartMeterWh, now, now)
	if err != nil {
		return "", fmt.Errorf("insert transaction: %w", err)
	}
	return id, nil
}

func (r *TransactionRepo) UpdateNumericID(ctx context.Context, id string, numericID int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `
		UPDATE transactions SET numeric_id = ?, status = 'active', started_at = ?, updated_at = ? WHERE id = ?
	`, numericID, now, now, id)
	return err
}

func (r *TransactionRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `
		UPDATE transactions SET status = ?, updated_at = ? WHERE id = ?
	`, status, now, now)
	return err
}

func (r *TransactionRepo) Stop(ctx context.Context, id string, meterStop int, reason string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `
		UPDATE transactions SET status = 'stopped', stop_meter_wh = ?, stop_reason = ?, stopped_at = ?, updated_at = ? WHERE id = ?
	`, meterStop, reason, now, now, id)
	return err
}

func (r *TransactionRepo) GetByID(ctx context.Context, id string) (*Transaction, error) {
	var tx Transaction
	err := r.db.QueryRowContext(ctx, `
		SELECT id, numeric_id, charge_point_id, evse_id, connector_number, id_tag, ocpp_version, status, started_at, stopped_at, start_meter_wh, stop_meter_wh, stop_reason, created_at, updated_at
		FROM transactions WHERE id = ?
	`, id).Scan(&tx.ID, &tx.NumericID, &tx.ChargePointID, &tx.EVSEID, &tx.ConnectorNumber, &tx.IDTag, &tx.OCPPVersion, &tx.Status, &tx.StartedAt, &tx.StoppedAt, &tx.StartMeterWh, &tx.StopMeterWh, &tx.StopReason, &tx.CreatedAt, &tx.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *TransactionRepo) GetActiveByConnector(ctx context.Context, chargePointID string, connectorNumber int) (*Transaction, error) {
	var tx Transaction
	err := r.db.QueryRowContext(ctx, `
		SELECT id, numeric_id, charge_point_id, evse_id, connector_number, id_tag, ocpp_version, status, started_at, stopped_at, start_meter_wh, stop_meter_wh, stop_reason, created_at, updated_at
		FROM transactions WHERE charge_point_id = ? AND connector_number = ? AND status IN ('pending_start', 'active', 'stopping')
		ORDER BY created_at DESC LIMIT 1
	`, chargePointID, connectorNumber).Scan(&tx.ID, &tx.NumericID, &tx.ChargePointID, &tx.EVSEID, &tx.ConnectorNumber, &tx.IDTag, &tx.OCPPVersion, &tx.Status, &tx.StartedAt, &tx.StoppedAt, &tx.StartMeterWh, &tx.StopMeterWh, &tx.StopReason, &tx.CreatedAt, &tx.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &tx, nil
}

func (r *TransactionRepo) ListByChargePoint(ctx context.Context, chargePointID string) ([]Transaction, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, numeric_id, charge_point_id, evse_id, connector_number, id_tag, ocpp_version, status, started_at, stopped_at, start_meter_wh, stop_meter_wh, stop_reason, created_at, updated_at
		FROM transactions WHERE charge_point_id = ? ORDER BY created_at DESC
	`, chargePointID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Transaction
	for rows.Next() {
		var tx Transaction
		if err := rows.Scan(&tx.ID, &tx.NumericID, &tx.ChargePointID, &tx.EVSEID, &tx.ConnectorNumber, &tx.IDTag, &tx.OCPPVersion, &tx.Status, &tx.StartedAt, &tx.StoppedAt, &tx.StartMeterWh, &tx.StopMeterWh, &tx.StopReason, &tx.CreatedAt, &tx.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, tx)
	}
	return list, rows.Err()
}
