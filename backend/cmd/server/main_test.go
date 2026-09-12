package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/platform/metrics"
	"gvideo/backend/internal/repository"
)

func TestStartupStorageLogsDoNotExposePaths(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, forbidden := range []string{`"database_path"`, `"media_dir"`, `logger.Error("open database", "error", err)`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("startup storage logging contains unsafe pattern %q", forbidden)
		}
	}
	for _, required := range []string{`"event", "storage_initialized"`, `"event", "storage_initialization_failed"`, `"error_class", "storage_failed"`} {
		if !strings.Contains(text, required) {
			t.Fatalf("startup storage logging missing safe pattern %q", required)
		}
	}
}

func TestStartAdminServersSupportsIndependentEnablement(t *testing.T) {
	tests := []struct {
		name        string
		metricsAddr string
		pprofAddr   string
		path        string
		body        string
		wantServers int
	}{
		{name: "disabled", wantServers: 0},
		{name: "metrics only", metricsAddr: "127.0.0.1:0", path: "/metrics", wantServers: 1},
		{name: "pprof only", pprofAddr: "127.0.0.1:0", path: "/debug/pprof/", body: "profile", wantServers: 1},
		{name: "both", metricsAddr: "127.0.0.1:0", pprofAddr: "127.0.0.1:0", wantServers: 2},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			servers, err := startAdminServers(testLogger(), testCase.metricsAddr, testCase.pprofAddr, metrics.New())
			if err != nil {
				t.Fatalf("start admin servers: %v", err)
			}
			t.Cleanup(func() { shutdownServers(t, servers) })
			if len(servers) != testCase.wantServers {
				t.Fatalf("server count = %d, want %d", len(servers), testCase.wantServers)
			}
			if testCase.path == "" {
				return
			}

			response, err := http.Get("http://" + servers[0].Addr + testCase.path)
			if err != nil {
				t.Fatalf("request admin endpoint: %v", err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read admin response: %v", err)
			}
			if response.StatusCode != http.StatusOK || !strings.Contains(string(body), testCase.body) {
				t.Fatalf("unexpected response: status=%d body=%q", response.StatusCode, body)
			}
		})
	}
}

func TestStartAdminServersFailsFastWhenPortIsOccupied(t *testing.T) {
	tests := []struct {
		name        string
		metricsAddr func(string) string
		pprofAddr   func(string) string
		wantName    string
	}{
		{
			name:        "metrics collision",
			metricsAddr: func(address string) string { return address },
			pprofAddr:   func(string) string { return "" },
			wantName:    "metrics",
		},
		{
			name:        "pprof collision",
			metricsAddr: func(string) string { return "" },
			pprofAddr:   func(address string) string { return address },
			wantName:    "pprof",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			occupied, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer occupied.Close()

			startedAt := time.Now()
			servers, err := startAdminServers(
				testLogger(),
				testCase.metricsAddr(occupied.Addr().String()),
				testCase.pprofAddr(occupied.Addr().String()),
				metrics.New(),
			)
			if err == nil {
				shutdownServers(t, servers)
				t.Fatal("expected occupied admin port to fail")
			}
			if !strings.Contains(err.Error(), testCase.wantName) {
				t.Fatalf("error %q does not identify %s", err, testCase.wantName)
			}
			if elapsed := time.Since(startedAt); elapsed > time.Second {
				t.Fatalf("port collision did not fail fast: %s", elapsed)
			}
		})
	}
}

func TestDataGrantAdminCommand(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "grant-admin.db")
	db, err := platform.OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	user, err := repo.CreateUser(ctx, "ops_admin", "hash")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if user.IsAdmin {
		db.Close()
		t.Fatal("new user unexpectedly has admin flag")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{DatabasePath: path}
	if err := runDataCommand(ctx, cfg, []string{"data-grant-admin"}); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("missing username error = %v", err)
	}
	if err := runDataCommand(ctx, cfg, []string{"data-grant-admin", "missing_user"}); err == nil {
		t.Fatal("granting admin to a missing user succeeded")
	}
	if err := runDataCommand(ctx, cfg, []string{"data-grant-admin", " ops_admin "}); err != nil {
		t.Fatalf("grant admin: %v", err)
	}
	// Granting again is idempotent and must not report a missing user.
	if err := runDataCommand(ctx, cfg, []string{"data-grant-admin", "ops_admin"}); err != nil {
		t.Fatalf("re-grant admin: %v", err)
	}

	verify, err := platform.OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	defer verify.Close()
	verifyRepo := repository.New(verify)
	hasAdmin, err := verifyRepo.HasAdmin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !hasAdmin {
		t.Fatal("admin flag was not persisted")
	}
	granted, err := verifyRepo.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !granted.IsAdmin {
		t.Fatalf("granted user is_admin = false: %#v", granted)
	}
}

func TestDiagnosticsBindingWarnings(t *testing.T) {
	if warnings := diagnosticsBindingWarnings("production", "0.0.0.0:9090", ":6060"); len(warnings) != 0 {
		t.Fatalf("production should not produce warnings: %v", warnings)
	}
	if warnings := diagnosticsBindingWarnings("development", "127.0.0.1:9090", "localhost:6060"); len(warnings) != 0 {
		t.Fatalf("loopback bindings should not warn: %v", warnings)
	}
	if warnings := diagnosticsBindingWarnings("development", "[::1]:9090", ""); len(warnings) != 0 {
		t.Fatalf("IPv6 loopback should not warn: %v", warnings)
	}
	if warnings := diagnosticsBindingWarnings("development", "0.0.0.0:9090", "[::]:6060"); len(warnings) != 2 {
		t.Fatalf("wildcard bindings warnings = %v, want 2", warnings)
	}
	if warnings := diagnosticsBindingWarnings("development", "10.1.2.3:9090", ""); len(warnings) != 1 {
		t.Fatalf("private address warnings = %v, want 1", warnings)
	}
	if warnings := diagnosticsBindingWarnings("development", ":9090", ""); len(warnings) != 1 {
		t.Fatalf("empty-host wildcard warnings = %v, want 1", warnings)
	}
	for _, warning := range diagnosticsBindingWarnings("development", "0.0.0.0:9090", "") {
		if !strings.Contains(warning, "METRICS_ADDR") {
			t.Fatalf("warning %q does not name the endpoint", warning)
		}
	}
	for _, warning := range diagnosticsBindingWarnings("development", "", "0.0.0.0:6060") {
		if !strings.Contains(warning, "PPROF_ADDR") {
			t.Fatalf("warning %q does not name the endpoint", warning)
		}
	}
	if warnings := diagnosticsBindingWarnings("development", "", ""); len(warnings) != 0 {
		t.Fatalf("disabled diagnostics should not warn: %v", warnings)
	}
	if warnings := diagnosticsBindingWarnings("development", "metrics.example.com:9090", ""); len(warnings) != 1 {
		t.Fatalf("hostname bindings warnings = %v, want 1", warnings)
	}
	// SplitHostPort fails on addresses without a port, so they are skipped.
	if warnings := diagnosticsBindingWarnings("development", "127.0.0.1", ""); len(warnings) != 0 {
		t.Fatalf("missing-port bindings should be skipped: %v", warnings)
	}
	// IPv4-mapped IPv6 loopback unmaps to 127.0.0.1 and must not warn.
	if warnings := diagnosticsBindingWarnings("development", "[::ffff:127.0.0.1]:9090", ""); len(warnings) != 0 {
		t.Fatalf("IPv4-mapped loopback bindings should not warn: %v", warnings)
	}
	// The IPv6 wildcard binds every interface and must warn.
	if warnings := diagnosticsBindingWarnings("development", "", "[::]:6060"); len(warnings) != 1 {
		t.Fatalf("IPv6 wildcard bindings warnings = %v, want 1", warnings)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func shutdownServers(t *testing.T, servers []*http.Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for _, server := range servers {
		if err := server.Shutdown(ctx); err != nil {
			t.Errorf("shutdown admin server: %v", err)
		}
	}
}
