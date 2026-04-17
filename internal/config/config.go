package config

import (
	"log"
	"os"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   []byte
	JWTTTL      time.Duration
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: mustEnv("DATABASE_URL"),
		JWTSecret:   []byte(mustEnv("JWT_SECRET")),
		JWTTTL:      24 * time.Hour,
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func mustEnv(key string) string {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return v
}
