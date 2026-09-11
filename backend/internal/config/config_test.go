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

func TestDiagnosticsAddressesAreDisabledByDefaultAndIndependent(t *testing.T) {
	t.Setenv("METRICS_ADDR", "")
	t.Setenv("PPROF_ADDR", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MetricsAddr != "" || cfg.PprofAddr != "" {
		t.Fatalf("diagnostics should default off: metrics=%q pprof=%q", cfg.MetricsAddr, cfg.PprofAddr)
	}

	t.Setenv("METRICS_ADDR", "127.0.0.1:9090")
	t.Setenv("PPROF_ADDR", "127.0.0.1:6060")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MetricsAddr != "127.0.0.1:9090" || cfg.PprofAddr != "127.0.0.1:6060" {
		t.Fatalf("diagnostics addresses not loaded independently: metrics=%q pprof=%q", cfg.MetricsAddr, cfg.PprofAddr)
	}
}

func TestLoadValidatesProductionDiagnosticsAddresses(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("FRONTEND_URL", "https://video.example.com")
	t.Setenv("COOKIE_SECURE", "true")

	tests := []struct {
		name        string
		metricsAddr string
		pprofAddr   string
		wantErr     bool
	}{
		{name: "disabled", wantErr: false},
		{name: "IPv4 loopback", metricsAddr: "127.0.0.1:9090", wantErr: false},
		{name: "IPv6 loopback", pprofAddr: "[::1]:6060", wantErr: false},
		{name: "localhost", metricsAddr: "localhost:9090", wantErr: false},
		{name: "RFC1918 private address", metricsAddr: "10.20.30.40:9090", wantErr: false},
		{name: "IPv6 private address", pprofAddr: "[fd00::1]:6060", wantErr: false},
		{name: "empty host wildcard", metricsAddr: ":9090", wantErr: true},
		{name: "IPv4 wildcard", metricsAddr: "0.0.0.0:9090", wantErr: true},
		{name: "IPv6 wildcard", pprofAddr: "[::]:6060", wantErr: true},
		{name: "public IPv4", metricsAddr: "203.0.113.10:9090", wantErr: true},
		{name: "public IPv6", pprofAddr: "[2001:db8::1]:6060", wantErr: true},
		{name: "untrusted hostname", metricsAddr: "metrics.example.com:9090", wantErr: true},
		{name: "missing port", metricsAddr: "127.0.0.1", wantErr: true},
		{name: "invalid port", pprofAddr: "127.0.0.1:invalid", wantErr: true},
		{name: "out of range port", metricsAddr: "127.0.0.1:65536", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("METRICS_ADDR", tt.metricsAddr)
			t.Setenv("PPROF_ADDR", tt.pprofAddr)

			_, err := Load()
			if tt.wantErr && err == nil {
				t.Fatal("expected production diagnostics address validation to fail")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected production diagnostics address to be accepted: %v", err)
			}
		})
	}
}

func TestLoadLeavesDevelopmentDiagnosticsBindingToTheOperator(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("METRICS_ADDR", "0.0.0.0:9090")
	t.Setenv("PPROF_ADDR", ":6060")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load development diagnostics addresses: %v", err)
	}
	if cfg.MetricsAddr != "0.0.0.0:9090" || cfg.PprofAddr != ":6060" {
		t.Fatalf("unexpected development diagnostics addresses: metrics=%q pprof=%q", cfg.MetricsAddr, cfg.PprofAddr)
	}
}

func TestLoadRateLimitDefaultsAndOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("RATE_LIMIT_AUTH_PER_MINUTE", "")
	t.Setenv("RATE_LIMIT_COMMENT_PER_MINUTE", "")
	t.Setenv("RATE_LIMIT_UPLOAD_PER_MINUTE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RateLimitAuthPerMinute != 20 || cfg.RateLimitCommentPerMinute != 30 || cfg.RateLimitUploadPerMinute != 10 {
		t.Fatalf("unexpected rate limit defaults: auth=%d comment=%d upload=%d",
			cfg.RateLimitAuthPerMinute, cfg.RateLimitCommentPerMinute, cfg.RateLimitUploadPerMinute)
	}

	t.Setenv("RATE_LIMIT_AUTH_PER_MINUTE", "5")
	t.Setenv("RATE_LIMIT_COMMENT_PER_MINUTE", "0")
	t.Setenv("RATE_LIMIT_UPLOAD_PER_MINUTE", "12")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RateLimitAuthPerMinute != 5 || cfg.RateLimitCommentPerMinute != 0 || cfg.RateLimitUploadPerMinute != 12 {
		t.Fatalf("rate limit overrides not applied: auth=%d comment=%d upload=%d",
			cfg.RateLimitAuthPerMinute, cfg.RateLimitCommentPerMinute, cfg.RateLimitUploadPerMinute)
	}
}

func TestLoadRejectsInvalidRateLimits(t *testing.T) {
	t.Setenv("APP_ENV", "development")

	t.Setenv("RATE_LIMIT_AUTH_PER_MINUTE", "-1")
	if _, err := Load(); err == nil {
		t.Fatal("expected a negative auth rate limit to fail")
	}
	t.Setenv("RATE_LIMIT_AUTH_PER_MINUTE", "not-a-number")
	if _, err := Load(); err == nil {
		t.Fatal("expected a non-numeric auth rate limit to fail")
	}
	t.Setenv("RATE_LIMIT_AUTH_PER_MINUTE", "20")
	t.Setenv("RATE_LIMIT_UPLOAD_PER_MINUTE", "-2")
	if _, err := Load(); err == nil {
		t.Fatal("expected a negative upload rate limit to fail")
	}
}

func TestLoadUserStorageQuota(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("USER_STORAGE_QUOTA_BYTES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UserStorageQuotaBytes != 0 {
		t.Fatalf("default quota = %d, want 0 (unlimited)", cfg.UserStorageQuotaBytes)
	}

	t.Setenv("USER_STORAGE_QUOTA_BYTES", "21474836480")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UserStorageQuotaBytes != 21474836480 {
		t.Fatalf("quota = %d, want 21474836480", cfg.UserStorageQuotaBytes)
	}

	t.Setenv("USER_STORAGE_QUOTA_BYTES", "-1")
	if _, err := Load(); err == nil {
		t.Fatal("expected a negative quota to fail")
	}
	t.Setenv("USER_STORAGE_QUOTA_BYTES", "many")
	if _, err := Load(); err == nil {
		t.Fatal("expected a non-numeric quota to fail")
	}
}
