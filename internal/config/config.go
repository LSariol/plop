package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServerPort        string
	PayloadDir        string
	MaxUploadBytes    int64
	PayloadTTL        time.Duration
	DatabaseURL       string
	LogFormat         string
	LogLevel          string
	AllowRegistration bool
	TrustedProxy      bool
}

func Load() (Config, error) {
	ttlHours, err := strconv.Atoi(getEnv("PAYLOAD_TTL_HOURS", "1"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid PAYLOAD_TTL_HOURS: %w", err)
	}
	maxMB, err := strconv.ParseInt(getEnv("MAX_UPLOAD_MB", "50"), 10, 64)
	if err != nil {
		return Config{}, fmt.Errorf("invalid MAX_UPLOAD_MB: %w", err)
	}

	return Config{
		ServerPort:        getEnv("SERVER_PORT", "8080"),
		PayloadDir:        getEnv("PAYLOAD_DIR", "/var/plop/payloads"),
		MaxUploadBytes:    maxMB * 1024 * 1024,
		PayloadTTL:        time.Duration(ttlHours) * time.Hour,
		DatabaseURL:       requireEnv("DATABASE_URL"),
		LogFormat:         getEnv("LOG_FORMAT", "text"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		AllowRegistration: getEnv("ALLOW_REGISTRATION", "false") == "true",
		TrustedProxy:      getEnv("TRUSTED_PROXY", "false") == "true",
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}
