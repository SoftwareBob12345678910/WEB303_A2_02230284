// Package database opens PostgreSQL connections for a service's own database.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver

	"stargazer/internal/platform/config"
)

// DSNFromEnv builds the connection string from DB_* environment variables.
// url.URL takes care of escaping special characters in the password.
func DSNFromEnv() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(config.MustGet("DB_USER"), config.MustGet("DB_PASSWORD")),
		Host:     config.MustGet("DB_HOST") + ":" + config.Get("DB_PORT", "5432"),
		Path:     "/" + config.MustGet("DB_NAME"),
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

// Connect opens the pool and waits until the database answers a ping.
// Retrying here makes start-up robust even if the DB container is slow.
func Connect(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	const attempts = 15
	var lastErr error
	for i := 1; i <= attempts; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = db.PingContext(pingCtx)
		cancel()
		if lastErr == nil {
			return db, nil
		}
		log.Printf("database not ready (attempt %d/%d): %v", i, attempts, lastErr)
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("database unreachable after %d attempts: %w", attempts, lastErr)
}
