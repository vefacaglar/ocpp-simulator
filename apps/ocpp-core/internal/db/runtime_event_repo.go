package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RuntimeEvent struct {
	ID             string
	ChargePointID  *string
	ConnectorNum   *int
	EventType      string
	Severity       string
	Message        string
	PayloadJSON    *string
	CreatedAt      string
}

type RuntimeEventRepo struct {
	db *sql.DB
}

func NewRuntimeEventRepo(d *sql.DB) *RuntimeEventRepo { return &RuntimeEventRepo{db: d} }

func (r *RuntimeEventRepo) Create(ctx context.Context, ev RuntimeEvent) (string, error) {
	if ev.ID == "" {
		ev.ID = uuid.NewString()
	}
	if ev.CreatedAt == "" {
		ev.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if ev.Severity == "" {
		ev.Severity = "info"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO runtime_events (id, charge_point_id, connector_number, event_type, severity, message, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		ev.ID, ev.ChargePointID, ev.ConnectorNum, ev.EventType, ev.Severity, ev.Message, ev.PayloadJSON, ev.CreatedAt)
	if err != nil {
		return "", fmt.Errorf("insert runtime_event: %w", err)
	}
	return ev.ID, nil
}

// ListByChargePoint returns the most recent N events for a CP.
func (r *RuntimeEventRepo) ListByChargePoint(ctx context.Context, chargePointID string, limit int) ([]RuntimeEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, charge_point_id, connector_number, event_type, severity, message, payload_json, created_at
		FROM runtime_events WHERE charge_point_id = ? OR charge_point_id IS NULL
		ORDER BY created_at DESC LIMIT ?
	`, chargePointID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RuntimeEvent
	for rows.Next() {
		var ev RuntimeEvent
		if err := rows.Scan(&ev.ID, &ev.ChargePointID, &ev.ConnectorNum, &ev.EventType, &ev.Severity, &ev.Message, &ev.PayloadJSON, &ev.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}
