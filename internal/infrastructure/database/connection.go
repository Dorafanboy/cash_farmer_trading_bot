package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"cash-farmer/internal/infrastructure/database/sqlc"
)

// Config holds database configuration
type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// Connection holds database connection and queries
type Connection struct {
	Pool    *pgxpool.Pool
	Queries *sqlc.Queries
}

// NewConnection creates new database connection
func NewConnection(ctx context.Context, cfg Config) (*Connection, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	queries := sqlc.New(pool)

	return &Connection{
		Pool:    pool,
		Queries: queries,
	}, nil
}

// Close closes database connection
func (c *Connection) Close() {
	if c.Pool != nil {
		c.Pool.Close()
	}
}
