package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// connectDB establishes a connection to the database using the provided DATABASE_URL.
// It now accepts a context for the connection process.
func connectDB(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if databaseURL == "" {
		// This case should ideally be caught before calling connectDB,
		// but as a safeguard:
		return nil, fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	pool, err := pgxpool.New(ctx, databaseURL) // Use the passed context
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Ping the database to verify the connection, using the passed context.
	if err := pool.Ping(ctx); err != nil {
		pool.Close() // Close the pool if ping fails
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}
	// No "Successfully connected" log here, main will handle it.
	return pool, nil
}
