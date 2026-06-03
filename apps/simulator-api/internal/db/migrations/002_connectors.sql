-- +goose Up
CREATE TABLE IF NOT EXISTS connectors (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    charge_point_id     TEXT NOT NULL,
    evse_id             INTEGER NOT NULL DEFAULT 1,
    connector_number    INTEGER NOT NULL,
    status              TEXT NOT NULL DEFAULT 'Available',
    is_enabled          INTEGER NOT NULL DEFAULT 1,
    is_deleted          INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL,
    updated_at          TEXT NOT NULL,
    deleted_at          TEXT NULL,

    UNIQUE(charge_point_id, connector_number),
    FOREIGN KEY (charge_point_id) REFERENCES charge_points(id)
);

-- +goose Down
DROP TABLE IF EXISTS connectors;
