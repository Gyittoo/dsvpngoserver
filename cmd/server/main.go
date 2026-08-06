package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"dsvpn-backend/internal/config"
	"dsvpn-backend/internal/database"
	"dsvpn-backend/internal/router"
	"dsvpn-backend/internal/services"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database error: %v", err)
	}
	defer db.Close()

	migrationDir := filepath.Join("internal", "database", "migrations")
	if err := database.RunMigrations(context.Background(), db, migrationDir); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	authService := services.NewAuthService(cfg.JWTSecret, cfg.JWTAccessExpiry, cfg.JWTRefreshExpiry)
	engine := router.New(db, authService, cfg.GoogleClientID)

	srvErr := make(chan error, 1)
	go func() {
		addr := fmt.Sprintf(":%d", cfg.Port)
		log.Printf("server listening on %s", addr)
		srvErr <- engine.Run(addr)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("shutdown signal: %s", sig.String())
	case err := <-srvErr:
		if err != nil {
			log.Fatalf("server failed: %v", err)
		}
	}
}
