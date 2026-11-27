// Package config loads typed runtime configuration from environment variables
// matching .env.example.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all service configuration keys defined in .env.example.
type Config struct {
	AppEnv             string
	APIPort            int
	APIBaseURL         string
	CORSAllowedOrigins []string

	PostgresHost     string
	PostgresPort     int
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string

	JWTSigningKey   string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	QRTokenRotation     time.Duration
	QRTokenAcceptWindow time.Duration

	NCANodeURL  string
	NCATestMode bool

	SMSProvider string
	SMSAPIKey   string
}

// Load reads configuration from environment variables, applying sensible local
// development defaults and validating required settings.
func Load() (*Config, error) {
	appEnv := getEnv("APP_ENV", "development")

	apiPort, err := getEnvInt("API_PORT", 8080)
	if err != nil {
		return nil, err
	}

	pgPort, err := getEnvInt("POSTGRES_PORT", 5432)
	if err != nil {
		return nil, err
	}

	accessTTL, err := getEnvDuration("ACCESS_TOKEN_TTL", 15*time.Minute)
	if err != nil {
		return nil, err
	}

	refreshTTL, err := getEnvDuration("REFRESH_TOKEN_TTL", 720*time.Hour)
	if err != nil {
		return nil, err
	}

	qrRotation, err := getEnvDuration("QR_TOKEN_ROTATION", 60*time.Second)
	if err != nil {
		return nil, err
	}

	qrAcceptWindow, err := getEnvDuration("QR_TOKEN_ACCEPT_WINDOW", 90*time.Second)
	if err != nil {
		return nil, err
	}

	ncaTestMode, err := getEnvBool("NCA_TEST_MODE", appEnv != "production")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		AppEnv:              appEnv,
		APIPort:             apiPort,
		APIBaseURL:          getEnv("API_BASE_URL", "http://localhost:8080"),
		CORSAllowedOrigins:  splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		PostgresHost:        getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:        pgPort,
		PostgresDB:          getEnv("POSTGRES_DB", "erx"),
		PostgresUser:        getEnv("POSTGRES_USER", "erx_app"),
		PostgresPassword:    getEnv("POSTGRES_PASSWORD", "change-me"),
		JWTSigningKey:       getEnv("JWT_SIGNING_KEY", "change-me-to-a-long-random-string"),
		AccessTokenTTL:      accessTTL,
		RefreshTokenTTL:     refreshTTL,
		QRTokenRotation:     qrRotation,
		QRTokenAcceptWindow: qrAcceptWindow,
		NCANodeURL:          getEnv("NCANODE_URL", "http://localhost:14579"),
		NCATestMode:         ncaTestMode,
		SMSProvider:         getEnv("SMS_PROVIDER", "console"),
		SMSAPIKey:           getEnv("SMS_API_KEY", "placeholder"),
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate enforces mandatory configuration invariants.
func (c *Config) Validate() error {
	if c.APIPort <= 0 || c.APIPort > 65535 {
		return fmt.Errorf("invalid API_PORT: %d", c.APIPort)
	}
	if strings.TrimSpace(c.PostgresHost) == "" {
		return fmt.Errorf("POSTGRES_HOST must not be empty")
	}
	if c.PostgresPort <= 0 || c.PostgresPort > 65535 {
		return fmt.Errorf("invalid POSTGRES_PORT: %d", c.PostgresPort)
	}
	if strings.TrimSpace(c.PostgresDB) == "" {
		return fmt.Errorf("POSTGRES_DB must not be empty")
	}
	if strings.TrimSpace(c.PostgresUser) == "" {
		return fmt.Errorf("POSTGRES_USER must not be empty")
	}
	if strings.TrimSpace(c.PostgresPassword) == "" {
		return fmt.Errorf("POSTGRES_PASSWORD must not be empty")
	}
	if strings.TrimSpace(c.JWTSigningKey) == "" {
		return fmt.Errorf("JWT_SIGNING_KEY must not be empty")
	}
	if c.AppEnv == "production" {
		if c.PostgresPassword == "change-me" {
			return fmt.Errorf("POSTGRES_PASSWORD must be changed in production")
		}
		if c.JWTSigningKey == "change-me-to-a-long-random-string" || len(c.JWTSigningKey) < 32 {
			return fmt.Errorf("JWT_SIGNING_KEY must be at least 32 characters and non-default in production")
		}
		if c.NCATestMode {
			return fmt.Errorf("NCA_TEST_MODE must be false in production")
		}
	}
	if c.QRTokenRotation <= 0 || c.QRTokenAcceptWindow <= 0 {
		return fmt.Errorf("QR token durations must be positive")
	}
	if c.QRTokenAcceptWindow < c.QRTokenRotation {
		return fmt.Errorf("QR_TOKEN_ACCEPT_WINDOW (%s) must be >= QR_TOKEN_ROTATION (%s)", c.QRTokenAcceptWindow, c.QRTokenRotation)
	}
	return nil
}

// DatabaseURL constructs a PostgreSQL connection URI for pgxpool.
func (c *Config) DatabaseURL() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.PostgresUser, c.PostgresPassword),
		Host:   fmt.Sprintf("%s:%d", c.PostgresHost, c.PostgresPort),
		Path:   c.PostgresDB,
	}
	q := u.Query()
	if c.AppEnv == "production" {
		q.Set("sslmode", "require")
	} else {
		q.Set("sslmode", "disable")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return parsed, nil
}

func getEnvBool(key string, fallback bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return false, fmt.Errorf("invalid %s: %w", key, err)
	}
	return parsed, nil
}

func getEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return parsed, nil
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
