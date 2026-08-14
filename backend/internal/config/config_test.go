package config

import "testing"

func TestLoadValidatesProductionTransport(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("FRONTEND_URL", "http://video.example.com")
	t.Setenv("COOKIE_SECURE", "false")
	if _, err := Load(); err == nil {
		t.Fatal("expected insecure production configuration to fail")
	}

	t.Setenv("FRONTEND_URL", "https://video.example.com/")
	t.Setenv("COOKIE_SECURE", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load secure production configuration: %v", err)
	}
	if cfg.FrontendURL != "https://video.example.com" {
		t.Fatalf("unexpected normalized frontend URL %q", cfg.FrontendURL)
	}
}

func TestLoadRejectsFrontendURLWithPath(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("FRONTEND_URL", "https://video.example.com/app")
	if _, err := Load(); err == nil {
		t.Fatal("expected frontend URL with a path to fail")
	}
}
