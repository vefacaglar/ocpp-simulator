-- +goose Up
CREATE TABLE IF NOT EXISTS charge_points (
    id                  TEXT PRIMARY KEY,
    name                TEXT NOT NULL,
    ocpp_version        TEXT NOT NULL DEFAULT '1.6J',
    central_system_url  TEXT NOT NULL,
	auto_connect        BOOLEAN NOT NULL DEFAULT false,
	is_enabled          BOOLEAN NOT NULL DEFAULT true,
	is_deleted          BOOLEAN NOT NULL DEFAULT false,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    deleted_at          TEXT NULL
);

-- +goose Down
DROP TABLE IF EXISTS charge_points;
