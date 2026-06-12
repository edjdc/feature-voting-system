package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL               string
	RedisURL                  string
	JWTAccessSecret           string
	JWTRefreshSecret          string
	JWTAccessTTL              time.Duration
	JWTRefreshTTL             time.Duration
	TrendingRecomputeInterval time.Duration
	Port                      string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:      requireEnv("DATABASE_URL"),
		RedisURL:         requireEnv("REDIS_URL"),
		JWTAccessSecret:  requireEnv("JWT_ACCESS_SECRET"),
		JWTRefreshSecret: requireEnv("JWT_REFRESH_SECRET"),
		Port:             envOr("PORT", "8080"),
	}

	var err error
	cfg.JWTAccessTTL, err = parseDuration("JWT_ACCESS_TTL", "15m")
	if err != nil {
		return nil, err
	}
	cfg.JWTRefreshTTL, err = parseDuration("JWT_REFRESH_TTL", "168h") // 7 days
	if err != nil {
		return nil, err
	}
	cfg.TrendingRecomputeInterval, err = parseDuration("TRENDING_RECOMPUTE_INTERVAL", "2m")
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		// fail fast — missing required config is programmer error
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(key, fallback string) (time.Duration, error) {
	raw := envOr(key, fallback)
	d, err := time.ParseDuration(raw)
	if err != nil {
		// try days suffix (e.g. "7d") since Go doesn't support it
		if len(raw) > 1 && raw[len(raw)-1] == 'd' {
			days, e := strconv.Atoi(raw[:len(raw)-1])
			if e == nil {
				return time.Duration(days) * 24 * time.Hour, nil
			}
		}
		return 0, fmt.Errorf("invalid duration for %s=%q: %w", key, raw, err)
	}
	return d, nil
}
