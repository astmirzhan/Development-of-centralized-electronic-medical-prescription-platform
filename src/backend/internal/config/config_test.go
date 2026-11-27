package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.APIPort != 8080 {
		t.Fatalf("expected APIPort 8080, got %d", cfg.APIPort)
	}
	if cfg.QRTokenRotation != 60*time.Second || cfg.QRTokenAcceptWindow != 90*time.Second {
		t.Fatalf("unexpected QR durations: rotation=%s accept=%s", cfg.QRTokenRotation, cfg.QRTokenAcceptWindow)
	}
}

func TestValidateProductionGuards(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("POSTGRES_PASSWORD", "super-strong-prod-pw")
	t.Setenv("JWT_SIGNING_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("NCA_TEST_MODE", "true")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when NCA_TEST_MODE=true in production")
	}

	t.Setenv("NCA_TEST_MODE", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid production config, got %v", err)
	}
	if cfg.NCATestMode {
		t.Fatal("expected NCATestMode=false in production")
	}
}
