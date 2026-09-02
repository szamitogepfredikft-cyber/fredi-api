package database

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context) (*pgxpool.Pool, error) {
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		host := os.Getenv("PGHOST")
		port := os.Getenv("PGPORT")
		databaseName := os.Getenv("PGDATABASE")
		user := os.Getenv("PGUSER")
		password := os.Getenv("PGPASSWORD")

		if host == "" || port == "" || databaseName == "" || user == "" || password == "" {
			return nil, fmt.Errorf(
				"DATABASE_URL or PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD is required",
			)
		}

		databaseURL = "host=" + host +
			" port=" + port +
			" dbname=" + databaseName +
			" user=" + user +
			" password=" + password +
			" sslmode=disable"
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}

	config.MaxConns = 10
	config.MinConns = 1
	config.MaxConnLifetime = 30 * time.Minute
	config.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingContext); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
