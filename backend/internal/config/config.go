package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr              string
	DatabaseURL       string
	CookieSecure      bool
	AllowedOrigins    []string
	SessionTTL        time.Duration
	AllowRegistration bool
	SeedOnStart       bool
	Environment       string
}

func Load() Config {
	return Config{
		Addr:              env("ADDR", ":8080"),
		DatabaseURL:       env("DATABASE_URL", "postgres://xiwei:xiwei_dev@localhost:5432/xiwei?sslmode=disable"),
		CookieSecure:      envBool("COOKIE_SECURE", false),
		AllowedOrigins:    splitCSV(env("ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173,http://localhost:8088")),
		SessionTTL:        time.Duration(envInt("SESSION_TTL_HOURS", 168)) * time.Hour,
		AllowRegistration: envBool("ALLOW_REGISTRATION", false),
		SeedOnStart:       envBool("SEED_ON_START", true),
		Environment:       env("APP_ENV", "development"),
	}
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func splitCSV(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, strings.TrimRight(p, "/"))
		}
	}
	return out
}
