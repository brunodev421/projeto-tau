package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment               string
	ServiceName               string
	Port                      string
	ProfileAPIURL             string
	TransactionsAPIURL        string
	AgentServiceURL           string
	DownstreamTimeout         time.Duration
	ProfileCacheTTL           time.Duration
	BulkheadLimit             int64
	RetryMaxAttempts          int
	CircuitBreakerFailRatio   float64
	CircuitBreakerMinRequests uint32
	CircuitBreakerOpenTimeout time.Duration
	OTLPEndpoint              string
	LogLevel                  string
}

func Load() (Config, error) {
	timeout, err := durationEnv("BFA_DOWNSTREAM_TIMEOUT", 2*time.Second)
	if err != nil {
		return Config{}, err
	}
	cacheTTL, err := durationEnv("BFA_PROFILE_CACHE_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	openTimeout, err := durationEnv("BFA_CIRCUIT_BREAKER_OPEN_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment:               stringEnv("ENVIRONMENT", "local"),
		ServiceName:               stringEnv("BFA_SERVICE_NAME", "bfa-go"),
		Port:                      stringEnv("BFA_PORT", "8080"),
		ProfileAPIURL:             stringEnv("BFA_PROFILE_API_URL", "http://localhost:8081"),
		TransactionsAPIURL:        stringEnv("BFA_TRANSACTIONS_API_URL", "http://localhost:8081"),
		AgentServiceURL:           stringEnv("BFA_AGENT_SERVICE_URL", "http://localhost:8090"),
		DownstreamTimeout:         timeout,
		ProfileCacheTTL:           cacheTTL,
		BulkheadLimit:             int64(intEnv("BFA_BULKHEAD_LIMIT", 32)),
		RetryMaxAttempts:          intEnv("BFA_RETRY_MAX_ATTEMPTS", 3),
		CircuitBreakerFailRatio:   floatEnv("BFA_CIRCUIT_BREAKER_FAILURE_RATIO", 0.6),
		CircuitBreakerMinRequests: uint32(intEnv("BFA_CIRCUIT_BREAKER_MIN_REQUESTS", 5)),
		CircuitBreakerOpenTimeout: openTimeout,
		OTLPEndpoint:              os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		LogLevel:                  stringEnv("LOG_LEVEL", "info"),
	}

	if cfg.ProfileAPIURL == "" || cfg.TransactionsAPIURL == "" || cfg.AgentServiceURL == "" {
		return Config{}, fmt.Errorf("downstream URLs must be configured")
	}

	return cfg, nil
}

func stringEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func floatEnv(key string, fallback float64) float64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %s: %w", key, err)
	}
	return parsed, nil
}
