package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func RunMigrations(ctx context.Context, db *pgxpool.Pool, migrationsDir string) error {
	// 1. Create schema_migrations table if not exists
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	// 2. Read migration files
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	// 3. Apply each migration only once
	for _, file := range files {
		var exists bool
		err := db.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", file,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", file, err)
		}
		if exists {
			continue
		}

		log.Printf("[migration] applying: %s", file)

		sqlBytes, err := os.ReadFile(filepath.Join(migrationsDir, file))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		if _, err := db.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("execute migration %s: %w", file, err)
		}

		if _, err := db.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", file); err != nil {
			return fmt.Errorf("record migration %s: %w", file, err)
		}

		log.Printf("[migration] applied: %s", file)
	}

	// 4. Seed default admin if empty
	if err := seedDefaultAdmin(ctx, db); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	log.Printf("[migration] all migrations completed successfully")
	return nil
}

func seedDefaultAdmin(ctx context.Context, db *pgxpool.Pool) error {
	var count int
	if err := db.QueryRow(ctx, "SELECT COUNT(*) FROM admins").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123456"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx,
		"INSERT INTO admins (email, password_hash) VALUES ($1, $2)",
		"admin@dsvpn.com", string(hash))
	if err != nil {
		return err
	}

	log.Printf("[migration] seeded default admin: admin@dsvpn.com / admin123456")
	return nil
}
