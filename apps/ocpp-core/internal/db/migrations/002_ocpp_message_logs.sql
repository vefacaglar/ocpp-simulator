-- +goose Up
CREATE TABLE ocpp_message_logs (
  id TEXT PRIMARY KEY,
  charge_point_id TEXT NOT NULL,
  direction TEXT NOT NULL,
  ocpp_version TEXT NULL,
  message_type TEXT NOT NULL,
  message_type_id INTEGER NULL,
  action TEXT NULL,
  unique_id TEXT NULL,
  transaction_uuid TEXT NULL,
  transaction_numeric_id INTEGER NULL,
  payload_json TEXT NOT NULL,
  status TEXT NOT NULL,
  error_code TEXT NULL,
  error_description TEXT NULL,
  topic TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX idx_ocpp_logs_cp ON ocpp_message_logs(charge_point_id);
CREATE INDEX idx_ocpp_logs_created ON ocpp_message_logs(created_at);
CREATE INDEX idx_ocpp_logs_tx ON ocpp_message_logs(transaction_uuid);

-- +goose Down
DROP INDEX IF EXISTS idx_ocpp_logs_tx;
DROP INDEX IF EXISTS idx_ocpp_logs_created;
DROP INDEX IF EXISTS idx_ocpp_logs_cp;
DROP TABLE IF EXISTS ocpp_message_logs;
