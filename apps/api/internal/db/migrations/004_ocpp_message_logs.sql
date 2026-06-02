-- +goose Up
CREATE TABLE IF NOT EXISTS ocpp_message_logs (
    id                      TEXT PRIMARY KEY,
    charge_point_id         TEXT NOT NULL,
    connector_number        INTEGER NULL,
    transaction_uuid        TEXT NULL,
    transaction_numeric_id  INTEGER NULL,
    unique_id               TEXT NULL,
    message_type            TEXT NOT NULL,
    message_type_id         INTEGER NULL,
    direction               TEXT NOT NULL,
    ocpp_version            TEXT NOT NULL,
    action                  TEXT NULL,
    payload_json            TEXT NOT NULL,
    status                  TEXT NOT NULL,
    error_code              TEXT NULL,
    error_description       TEXT NULL,
    created_at              TEXT NOT NULL,

    FOREIGN KEY (charge_point_id) REFERENCES charge_points(id)
);

CREATE INDEX IF NOT EXISTS idx_ocpp_message_logs_cp ON ocpp_message_logs(charge_point_id);
CREATE INDEX IF NOT EXISTS idx_ocpp_message_logs_tx ON ocpp_message_logs(transaction_uuid);

-- +goose Down
DROP TABLE IF EXISTS ocpp_message_logs;
