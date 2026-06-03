package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/user/ocpp-simulator/apps/simulator-api/internal/common"
)

type ConnectorRepo struct {
	db *sql.DB
}

func NewConnectorRepo(db *sql.DB) *ConnectorRepo {
	return &ConnectorRepo{db: db}
}

func (r *ConnectorRepo) ListByChargePoint(ctx context.Context, chargePointID string) ([]common.Connector, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, charge_point_id, evse_id, connector_number, status, created_at, updated_at
		FROM connectors
		WHERE charge_point_id = ? AND is_deleted = 0
		ORDER BY connector_number
	`, chargePointID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []common.Connector
	for rows.Next() {
		var c common.Connector
		var createdAt, updatedAt string
		if err := rows.Scan(&c.ID, &c.ChargePointID, &c.EVSEID, &c.ConnectorNumber, &c.Status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		c.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		list = append(list, c)
	}
	return list, rows.Err()
}

func (r *ConnectorRepo) Create(ctx context.Context, c common.Connector) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO connectors (charge_point_id, evse_id, connector_number, status, is_enabled, is_deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, 1, 0, ?, ?)
	`, c.ChargePointID, c.EVSEID, c.ConnectorNumber, c.Status, now, now)
	return err
}

func (r *ConnectorRepo) Delete(ctx context.Context, id int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `
		UPDATE connectors SET is_deleted = 1, deleted_at = ?, updated_at = ? WHERE id = ?
	`, now, now, id)
	return err
}

func (r *ConnectorRepo) NextConnectorNumber(ctx context.Context, chargePointID string) (int, error) {
	var num int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(connector_number), 0) + 1
		FROM connectors
		WHERE charge_point_id = ? AND is_deleted = 0
	`, chargePointID).Scan(&num)
	return num, err
}
