package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"

	"github.com/pressly/goose/v3"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var TestDBURL = func() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return "postgres://postgres:postgres@localhost:5432/ocpp_core_test?sslmode=disable"
}()

//go:embed migrations/*.sql
var embedMigrations embed.FS

func Open(connStr string) (*sql.DB, error) {
	d, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	return d, nil
}

func Migrate(d *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.Up(d, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
