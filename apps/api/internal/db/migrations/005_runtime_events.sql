-- +goose Up
CREATE TABLE IF NOT EXISTS runtime_events (
    id                  TEXT PRIMARY KEY,
    charge_point_id     TEXT NULL,
    connector_number    INTEGER NULL,
    event_type          TEXT NOT NULL,
    severity            TEXT NOT NULL DEFAULT 'info',
    message             TEXT NOT NULL,
    payload_json        TEXT NULL,
    created_at          TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_runtime_events_cp ON runtime_events(charge_point_id);

-- +goose Down
DROP TABLE IF EXISTS runtime_events;
