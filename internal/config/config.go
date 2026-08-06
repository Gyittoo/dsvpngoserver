package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            int
	Env             string
	DatabaseURL     string
	JWTSecret       string
	JWTAccessExpiry time.Duration
	JWTRefreshExpiry time.Duration
	GoogleClientID  string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	port, err := getEnvInt("PORT", 8080)
	if err != nil {
		return Config{}, err
	}

	accessExpiry, err := getEnvDuration("JWT_ACCESS_EXPIRY", "24h")
	if err != nil {
		return Config{}, err
	}

	refreshExpiry, err := getEnvDuration("JWT_REFRESH_EXPIRY", "720h")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Port:             port,
		Env:              getEnv("ENV", "development"),
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		JWTSecret:        getEnv("JWT_SECRET", ""),
		JWTAccessExpiry:  accessExpiry,
		JWTRefreshExpiry: refreshExpiry,
		GoogleClientID:   getEnv("GOOGLE_CLIENT_ID", ""),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	val := getEnv(key, strconv.Itoa(fallback))
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}

func getEnvDuration(key, fallback string) (time.Duration, error) {
	val := getEnv(key, fallback)
	d, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration (example: 24h): %w", key, err)
	}
	return d, nil
}
