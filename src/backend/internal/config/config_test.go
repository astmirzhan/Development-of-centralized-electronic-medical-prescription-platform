package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("NCA_MOCK_SECRET", "dev-test-mock-secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.APIPort != 8080 {
		t.Fatalf("expected APIPort 8080, got %d", cfg.APIPort)
	}
	if !cfg.NCATestMode {
		t.Fatal("expected NCATestMode=true by default in development")
	}
	if cfg.QRTokenRotation != 60*time.Second || cfg.QRTokenAcceptWindow != 90*time.Second {
		t.Fatalf("unexpected QR durations: rotation=%s accept=%s", cfg.QRTokenRotation, cfg.QRTokenAcceptWindow)
	}
}

func TestValidateFailClosedNCATestMode(t *testing.T) {
	for _, env := range []string{"staging", "prod", "Production", "PRODUCTION"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv("APP_ENV", env)
			t.Setenv("POSTGRES_PASSWORD", "super-strong-prod-pw")
			t.Setenv("JWT_SIGNING_KEY", "0123456789abcdef0123456789abcdef")

			// Unset NCA_TEST_MODE must fail closed (false) in non-dev/test environments.
			t.Setenv("NCA_TEST_MODE", "")
			cfg, err := Load()
			if err != nil {
				t.Fatalf("expected valid config for %s with default NCA_TEST_MODE, got %v", env, err)
			}
			if cfg.NCATestMode {
				t.Fatalf("expected NCATestMode=false for APP_ENV=%s", env)
			}

			// Explicitly enabling NCA_TEST_MODE=true must be rejected outside development/test.
			t.Setenv("NCA_TEST_MODE", "true")
			if _, err := Load(); err == nil {
				t.Fatalf("expected error when NCA_TEST_MODE=true for APP_ENV=%s", env)
			}
		})
	}
}

func TestValidateProductionGuards(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("POSTGRES_PASSWORD", "super-strong-prod-pw")
	t.Setenv("JWT_SIGNING_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("NCA_TEST_MODE", "false")
	t.Setenv("CORS_ALLOWED_ORIGINS", "*")

	if _, err := Load(); err == nil {
		t.Fatal("expected error when CORS_ALLOWED_ORIGINS contains '*' in production")
	}

	t.Setenv("CORS_ALLOWED_ORIGINS", "https://erx.kz")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected valid production config, got %v", err)
	}
	if cfg.NCATestMode {
		t.Fatal("expected NCATestMode=false in production")
	}

	t.Setenv("APP_ENV", "development")
	t.Setenv("NCA_TEST_MODE", "true")
	t.Setenv("NCA_MOCK_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when NCA_TEST_MODE=true without NCA_MOCK_SECRET")
	}
}
