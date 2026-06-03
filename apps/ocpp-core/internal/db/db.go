package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func Open(dbPath string) (*sql.DB, error) {
	d, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	d.SetMaxOpenConns(1)
	if _, err := d.ExecContext(context.Background(), "PRAGMA journal_mode=WAL"); err != nil {
		d.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := d.ExecContext(context.Background(), "PRAGMA foreign_keys=ON"); err != nil {
		d.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	return d, nil
}

func Migrate(d *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(d, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
