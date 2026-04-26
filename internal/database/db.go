// Package database provides a thin wrapper around *sql.DB that automatically
// runs schema migrations on first connect using the embedded migration files.
package database

import (
	"database/sql"
	"fmt"

	"github.com/arsykor/go-url-shortener/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

// DB wraps *sql.DB with automatic schema migration support.
type DB struct {
	*sql.DB
}

// NewDB opens a PostgreSQL connection using the given DSN, verifies the
// connection with a ping, and runs all pending migrations before returning
// the ready-to-use DB. Returns an error if the DSN is empty, the connection
// cannot be established, or a migration fails.
func NewDB(dsn string) (*DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("database DSN is empty")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{DB: db}, nil
}

// runMigrations applies all pending "up" migrations embedded in the migrations
// package. It is a no-op when the schema is already up to date.
func runMigrations(db *sql.DB) error {
	migrationsDir := migrations.FS

	sourceDriver, err := iofs.New(migrationsDir, ".")
	if err != nil {
		return fmt.Errorf("failed to create source driver: %w", err)
	}

	dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		sourceDriver,
		"postgres",
		dbDriver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
