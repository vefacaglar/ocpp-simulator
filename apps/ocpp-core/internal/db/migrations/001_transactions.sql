-- +goose Up
CREATE TABLE transactions (
  id TEXT PRIMARY KEY,
  numeric_id INTEGER NULL,
  charge_point_id TEXT NOT NULL,
  evse_id INTEGER NOT NULL DEFAULT 1,
  connector_number INTEGER NOT NULL,
  id_tag TEXT NOT NULL,
  ocpp_version TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NULL,
  stopped_at TEXT NULL,
  start_meter_wh INTEGER NOT NULL DEFAULT 0,
  stop_meter_wh INTEGER NULL,
  stop_reason TEXT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_transactions_cp_numeric
  ON transactions(charge_point_id, numeric_id)
  WHERE numeric_id IS NOT NULL;

CREATE INDEX idx_transactions_cp ON transactions(charge_point_id);
CREATE INDEX idx_transactions_status ON transactions(status);

-- +goose Down
DROP INDEX IF EXISTS idx_transactions_status;
DROP INDEX IF EXISTS idx_transactions_cp;
DROP INDEX IF EXISTS idx_transactions_cp_numeric;
DROP TABLE IF EXISTS transactions;

CREATE UNIQUE INDEX idx_transactions_cp_numeric
  ON transactions(charge_point_id, numeric_id)
  WHERE numeric_id IS NOT NULL;

CREATE INDEX idx_transactions_cp ON transactions(charge_point_id);
CREATE INDEX idx_transactions_status ON transactions(status);
