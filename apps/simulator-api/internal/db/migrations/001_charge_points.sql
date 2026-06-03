-- +goose Up
CREATE TABLE IF NOT EXISTS charge_points (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    ocpp_version        TEXT NOT NULL DEFAULT '1.6J',
    central_system_url  TEXT NOT NULL,
    auto_connect        INTEGER NOT NULL DEFAULT 0,
    is_enabled          INTEGER NOT NULL DEFAULT 1,
    is_deleted          INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    deleted_at          TEXT NULL
);

-- +goose Down
DROP TABLE IF EXISTS charge_points;
