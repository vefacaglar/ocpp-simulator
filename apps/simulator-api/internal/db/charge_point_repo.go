package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/user/ocpp-simulator/apps/simulator-api/internal/common"
)

type ChargePointRepo struct {
	db *sql.DB
}

func NewChargePointRepo(db *sql.DB) *ChargePointRepo {
	return &ChargePointRepo{db: db}
}

func (r *ChargePointRepo) List(ctx context.Context) ([]common.ChargePoint, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, ocpp_version, central_system_url, auto_connect, created_at, updated_at
		FROM charge_points
		WHERE is_deleted = false
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []common.ChargePoint
	for rows.Next() {
		var cp common.ChargePoint
		var createdAt, updatedAt string
		if err := rows.Scan(&cp.ID, &cp.Name, &cp.OCPPVersion, &cp.CentralSystemURL, &cp.AutoConnect, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		cp.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		cp.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		cp.Status = common.StatusDisconnected
		list = append(list, cp)
	}
	return list, rows.Err()
}

func (r *ChargePointRepo) GetByID(ctx context.Context, id string) (*common.ChargePoint, error) {
	var cp common.ChargePoint
	var createdAt, updatedAt string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, name, ocpp_version, central_system_url, auto_connect, created_at, updated_at
		FROM charge_points
		WHERE id = $1 AND is_deleted = false
	`, id).Scan(&cp.ID, &cp.Name, &cp.OCPPVersion, &cp.CentralSystemURL, &cp.AutoConnect, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	cp.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	cp.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	cp.Status = common.StatusDisconnected
	return &cp, nil
}

func (r *ChargePointRepo) Create(ctx context.Context, cp common.ChargePoint) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO charge_points (id, name, ocpp_version, central_system_url, auto_connect, is_enabled, is_deleted, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, false, $6, $7)
	`, cp.ID, cp.Name, cp.OCPPVersion, cp.CentralSystemURL, cp.AutoConnect, now, now)
	return err
}

func (r *ChargePointRepo) Delete(ctx context.Context, id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `
		UPDATE charge_points SET is_deleted = true, deleted_at = $1, updated_at = $2 WHERE id = $3
	`, now, now, id)
	return err
}
