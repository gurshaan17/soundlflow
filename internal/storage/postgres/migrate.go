package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	iofs "github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/gurshaan17/soundlflow/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// RunMigrations applies all pending migrations embedded in the migrations
// package against the given Postgres DSN.
func RunMigrations(databaseURL string) error {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("migrate: source: %w", err)
	}

	// golang-migrate uses session-level pg_advisory_lock, which hangs on
	// connection poolers (e.g. Neon's -pooler endpoint) that rotate backends
	// between transactions. Migrations must go to the direct endpoint.
	db, err := sql.Open("pgx", directDatabaseURL(databaseURL))
	if err != nil {
		return fmt.Errorf("migrate: open database: %w", err)
	}
	defer db.Close()

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("migrate: driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "", driver)
	if err != nil {
		return fmt.Errorf("migrate: init: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: up: %w", err)
	}
	return nil
}

// directDatabaseURL rewrites a Neon pooled connection URL (host label ending
// in "-pooler") to the direct endpoint. Non-Neon URLs are returned unchanged.
func directDatabaseURL(databaseURL string) string {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return databaseURL
	}
	host := u.Hostname()
	if !strings.Contains(host, "-pooler.") {
		return databaseURL
	}
	u.Host = strings.Replace(u.Host, "-pooler.", ".", 1)
	return u.String()
}
