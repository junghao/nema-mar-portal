package email

import (
	"os"
	"testing"
)

func TestConfigFromEnv(t *testing.T) {
	// Save and restore env
	origHost := os.Getenv("SMTP_HOST")
	origFrom := os.Getenv("SMTP_FROM")
	origRecipients := os.Getenv("SMTP_RECIPIENTS")
	defer func() {
		os.Setenv("SMTP_HOST", origHost)
		os.Setenv("SMTP_FROM", origFrom)
		os.Setenv("SMTP_RECIPIENTS", origRecipients)
	}()

	t.Run("missing host", func(t *testing.T) {
		os.Setenv("SMTP_HOST", "")
		os.Setenv("SMTP_FROM", "test@example.com")
		os.Setenv("SMTP_RECIPIENTS", "a@b.com")
		_, err := ConfigFromEnv()
		if err == nil {
			t.Error("expected error for missing SMTP_HOST")
		}
	})

	t.Run("missing from", func(t *testing.T) {
		os.Setenv("SMTP_HOST", "smtp.example.com")
		os.Setenv("SMTP_FROM", "")
		os.Setenv("SMTP_RECIPIENTS", "a@b.com")
		_, err := ConfigFromEnv()
		if err == nil {
			t.Error("expected error for missing SMTP_FROM")
		}
	})

	t.Run("missing recipients", func(t *testing.T) {
		os.Setenv("SMTP_HOST", "smtp.example.com")
		os.Setenv("SMTP_FROM", "test@example.com")
		os.Setenv("SMTP_RECIPIENTS", "")
		_, err := ConfigFromEnv()
		if err == nil {
			t.Error("expected error for missing SMTP_RECIPIENTS")
		}
	})

	t.Run("valid config", func(t *testing.T) {
		os.Setenv("SMTP_HOST", "smtp.example.com")
		os.Setenv("SMTP_FROM", "test@example.com")
		os.Setenv("SMTP_RECIPIENTS", "a@b.com, c@d.com")
		cfg, err := ConfigFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Host != "smtp.example.com" {
			t.Errorf("expected host smtp.example.com, got %s", cfg.Host)
		}
		if cfg.Port != "587" {
			t.Errorf("expected default port 587, got %s", cfg.Port)
		}
		if len(cfg.Recipients) != 2 {
			t.Errorf("expected 2 recipients, got %d", len(cfg.Recipients))
		}
		if cfg.Recipients[0] != "a@b.com" {
			t.Errorf("expected first recipient a@b.com, got %s", cfg.Recipients[0])
		}
		if cfg.Recipients[1] != "c@d.com" {
			t.Errorf("expected second recipient c@d.com, got %s", cfg.Recipients[1])
		}
	})
}

func TestBoolStr(t *testing.T) {
	if boolStr(true) != "Yes" {
		t.Error("expected Yes for true")
	}
	if boolStr(false) != "No" {
		t.Error("expected No for false")
	}
}
