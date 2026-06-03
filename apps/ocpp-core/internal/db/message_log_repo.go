package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// MessageLog is the canonical OCPP message log row. One row per
// wire frame observed on ocpp/{cpId}/in or ocpp/{cpId}/out. The
// payload is the exact bytes the broker delivered, kept verbatim
// so the log is a faithful source of truth.
type MessageLog struct {
	ID                   string
	ChargePointID        string
	Direction            string
	OCPPVersion          *string
	MessageType          string
	MessageTypeID        *int
	Action               *string
	UniqueID             *string
	TransactionUUID      *string
	TransactionNumericID *int
	PayloadJSON          string
	Status               string
	ErrorCode            *string
	ErrorDescription     *string
	Topic                string
	CreatedAt            string
}

type MessageLogRepo struct {
	db *sql.DB
}

func NewMessageLogRepo(d *sql.DB) *MessageLogRepo { return &MessageLogRepo{db: d} }

func (r *MessageLogRepo) Create(ctx context.Context, m MessageLog) (string, error) {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if m.CreatedAt == "" {
		m.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if m.Status == "" {
		m.Status = "ok"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ocpp_message_logs
		  (id, charge_point_id, direction, ocpp_version, message_type, message_type_id, action, unique_id,
		   transaction_uuid, transaction_numeric_id, payload_json, status, error_code, error_description, topic, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`,
		m.ID, m.ChargePointID, m.Direction, m.OCPPVersion, m.MessageType, m.MessageTypeID, m.Action, m.UniqueID,
		m.TransactionUUID, m.TransactionNumericID, m.PayloadJSON, m.Status, m.ErrorCode, m.ErrorDescription, m.Topic, m.CreatedAt)
	if err != nil {
		return "", fmt.Errorf("insert ocpp_message_log: %w", err)
	}
	return m.ID, nil
}

// ListByChargePoint returns the most recent N message logs for a
// charge point, newest first. Used by the UI / messages API.
func (r *MessageLogRepo) ListByChargePoint(ctx context.Context, chargePointID string, limit int) ([]MessageLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, charge_point_id, direction, ocpp_version, message_type, message_type_id, action, unique_id,
		       transaction_uuid, transaction_numeric_id, payload_json, status, error_code, error_description, topic, created_at
		FROM ocpp_message_logs WHERE charge_point_id = $1 ORDER BY created_at DESC LIMIT $2
	`, chargePointID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MessageLog
	for rows.Next() {
		var m MessageLog
		if err := rows.Scan(&m.ID, &m.ChargePointID, &m.Direction, &m.OCPPVersion, &m.MessageType, &m.MessageTypeID, &m.Action, &m.UniqueID,
			&m.TransactionUUID, &m.TransactionNumericID, &m.PayloadJSON, &m.Status, &m.ErrorCode, &m.ErrorDescription, &m.Topic, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
