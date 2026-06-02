package db

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type OCPPMessageLog struct {
	ID                  string
	ChargePointID       string
	ConnectorNumber     *int
	TransactionUUID     *int
	TransactionNumericID *int
	UniqueID            *string
	MessageType         string
	MessageTypeID       *int
	Direction           string
	OCPPVersion         string
	Action              *string
	PayloadJSON         string
	Status              string
	ErrorCode           *string
	ErrorDescription    *string
	CreatedAt           string
}

type MessageLogRepo struct {
	db *sql.DB
}

func NewMessageLogRepo(db *sql.DB) *MessageLogRepo {
	return &MessageLogRepo{db: db}
}

func (r *MessageLogRepo) Create(ctx context.Context, log OCPPMessageLog) error {
	id := log.ID
	if id == "" {
		id = uuid.New().String()
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ocpp_message_logs (id, charge_point_id, connector_number, transaction_uuid, transaction_numeric_id, unique_id, message_type, message_type_id, direction, ocpp_version, action, payload_json, status, error_code, error_description, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, log.ChargePointID, log.ConnectorNumber, log.TransactionUUID, log.TransactionNumericID, log.UniqueID, log.MessageType, log.MessageTypeID, log.Direction, log.OCPPVersion, log.Action, log.PayloadJSON, log.Status, log.ErrorCode, log.ErrorDescription, now)
	return err
}

func (r *MessageLogRepo) ListByChargePoint(ctx context.Context, chargePointID string, limit int) ([]OCPPMessageLog, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, charge_point_id, connector_number, unique_id, message_type, message_type_id, direction, ocpp_version, action, payload_json, status, error_code, error_description, created_at
		FROM ocpp_message_logs WHERE charge_point_id = ? ORDER BY created_at DESC LIMIT ?
	`, chargePointID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []OCPPMessageLog
	for rows.Next() {
		var l OCPPMessageLog
		if err := rows.Scan(&l.ID, &l.ChargePointID, &l.ConnectorNumber, &l.UniqueID, &l.MessageType, &l.MessageTypeID, &l.Direction, &l.OCPPVersion, &l.Action, &l.PayloadJSON, &l.Status, &l.ErrorCode, &l.ErrorDescription, &l.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}
