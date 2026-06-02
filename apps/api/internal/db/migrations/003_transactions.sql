-- +goose Up
CREATE TABLE IF NOT EXISTS transactions (
    id                  TEXT PRIMARY KEY,
    numeric_id          INTEGER NULL,
    charge_point_id     TEXT NOT NULL,
    evse_id             INTEGER NOT NULL DEFAULT 1,
    connector_number    INTEGER NOT NULL,
    id_tag              TEXT NOT NULL,
    ocpp_version        TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'pending_start',
    started_at          TEXT NULL,
    stopped_at          TEXT NULL,
    start_meter_wh      INTEGER NOT NULL DEFAULT 0,
    stop_meter_wh       INTEGER NULL,
    stop_reason         TEXT NULL,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,

    UNIQUE(charge_point_id, numeric_id),
    FOREIGN KEY (charge_point_id) REFERENCES charge_points(id)
);

-- +goose Down
DROP TABLE IF EXISTS transactions;
