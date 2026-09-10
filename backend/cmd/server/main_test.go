package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"gvideo/backend/internal/platform/metrics"
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
