package database

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	// 1. Auto-create database if not exists
	if err := ensureDatabaseExists(ctx, databaseURL); err != nil {
		log.Printf("[db] warning: could not auto-create database: %v", err)
	}

	// 2. Connect to the target database
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	log.Printf("[db] connected successfully")
	return pool, nil
}

// ensureDatabaseExists connects to the default "postgres" database
// and creates the target database if it doesn't exist.
func ensureDatabaseExists(ctx context.Context, databaseURL string) error {
	// Parse the URL to get database name
	dbName, adminURL, err := parseDBNameAndAdminURL(databaseURL)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	// Connect to default "postgres" database
	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := pgx.Connect(connCtx, adminURL)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer conn.Close(ctx)

	// Check if database exists
	var exists bool
	err = conn.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname=$1)", dbName,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check database: %w", err)
	}

	if !exists {
		// Create database (can't use parameterized query for CREATE DATABASE)
		_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", sanitizeDBName(dbName)))
		if err != nil {
			return fmt.Errorf("create database: %w", err)
		}
		log.Printf("[db] created database: %s", dbName)
	} else {
		log.Printf("[db] database exists: %s", dbName)
	}

	return nil
}

// parseDBNameAndAdminURL extracts the database name from the URL
// and returns a URL pointing to the default "postgres" database.
func parseDBNameAndAdminURL(databaseURL string) (string, string, error) {
	u, err := url.Parse(databaseURL)
	if err != nil {
		return "", "", err
	}

	// Get database name from path (e.g., "/dsvpn" -> "dsvpn")
	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return "", "", fmt.Errorf("no database name in URL")
	}

	// Replace database name with "postgres"
	u.Path = "/postgres"
	adminURL := u.String()

	return dbName, adminURL, nil
}

// sanitizeDBName prevents SQL injection in CREATE DATABASE statement
func sanitizeDBName(name string) string {
	// Only allow alphanumeric and underscores
	var safe strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			safe.WriteRune(c)
		}
	}
	return safe.String()
}
