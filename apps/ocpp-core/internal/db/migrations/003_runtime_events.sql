-- +goose Up
CREATE TABLE runtime_events (
  id TEXT PRIMARY KEY,
  charge_point_id TEXT NULL,
  connector_number INTEGER NULL,
  event_type TEXT NOT NULL,
  severity TEXT NOT NULL,
  message TEXT NOT NULL,
  payload_json TEXT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX idx_runtime_events_cp ON runtime_events(charge_point_id);
CREATE INDEX idx_runtime_events_created ON runtime_events(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_runtime_events_created;
DROP INDEX IF EXISTS idx_runtime_events_cp;
DROP TABLE IF EXISTS runtime_events;
