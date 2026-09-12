package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"gvideo/backend/internal/config"
)

func TestLivezReportsProcessLivenessWithoutDependencies(t *testing.T) {
	cfg := config.Config{MediaDir: filepath.Join(t.TempDir(), "media")}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Nil application services prove that liveness does not query the database,
	// media worker, queue, or external commands.
	routes := New(nil, nil, nil, nil, nil, cfg, logger).Routes()

	for _, path := range []string{"/livez", "/healthz"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.AddCookie(&http.Cookie{Name: sessionCookie, Value: "must-not-be-authenticated"})
			routes.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status=%d, want %d; body=%s", response.Code, http.StatusOK, response.Body.String())
			}
			contentType, _, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
			if err != nil || contentType != "application/json" {
				t.Fatalf("content-type=%q, want application/json: %v", contentType, err)
			}

			var envelope testEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
			}
			var payload map[string]string
			if err := json.Unmarshal(envelope.Data, &payload); err != nil {
				t.Fatalf("decode payload: %v; data=%s", err, envelope.Data)
			}
			if payload["status"] != "ok" {
				t.Fatalf("status payload=%q, want ok", payload["status"])
			}
			if envelope.RequestID == "" {
				t.Fatal("request_id is empty")
			}
			if headerID := response.Header().Get("X-Request-ID"); headerID != envelope.RequestID {
				t.Fatalf("X-Request-ID=%q, want %q", headerID, envelope.RequestID)
			}
		})
	}

	methodNotAllowed := httptest.NewRecorder()
	routes.ServeHTTP(methodNotAllowed, httptest.NewRequest(http.MethodPost, "/livez", nil))
	if methodNotAllowed.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /livez status=%d, want %d", methodNotAllowed.Code, http.StatusMethodNotAllowed)
	}
}

type readinessFunc func(context.Context) error

func (f readinessFunc) Ready(ctx context.Context) error { return f(ctx) }

func TestReadyzReportsOnlyDatabaseReadiness(t *testing.T) {
	cfg := config.Config{MediaDir: filepath.Join(t.TempDir(), "media")}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name       string
		checker    ReadinessChecker
		wantStatus int
		wantReady  bool
	}{
		{name: "ready", checker: readinessFunc(func(context.Context) error { return nil }), wantStatus: http.StatusOK, wantReady: true},
		{name: "database unavailable", checker: readinessFunc(func(context.Context) error { return errors.New(`open C:\secret\gvideo.db: SELECT 1 failed`) }), wantStatus: http.StatusServiceUnavailable},
		{name: "not configured", checker: nil, wantStatus: http.StatusServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			routes := New(nil, nil, nil, nil, nil, cfg, logger).WithReadiness(test.checker).Routes()
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			request.AddCookie(&http.Cookie{Name: sessionCookie, Value: "must-not-be-authenticated"})
			routes.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status=%d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			var envelope testEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode response: %v; body=%s", err, response.Body.String())
			}
			if envelope.RequestID == "" || response.Header().Get("X-Request-ID") != envelope.RequestID {
				t.Fatalf("request id header=%q envelope=%q", response.Header().Get("X-Request-ID"), envelope.RequestID)
			}
			if test.wantReady {
				var payload map[string]string
				if err := json.Unmarshal(envelope.Data, &payload); err != nil || payload["status"] != "ready" {
					t.Fatalf("ready payload=%s error=%v", envelope.Data, err)
				}
			} else {
				if envelope.Error != "服务尚未就绪" {
					t.Fatalf("error=%q, want fixed readiness message", envelope.Error)
				}
				if body := response.Body.String(); strings.Contains(body, "secret") || strings.Contains(body, "SELECT") {
					t.Fatalf("response leaked database details: %s", body)
				}
			}
		})
	}

	notAllowed := httptest.NewRecorder()
	New(nil, nil, nil, nil, nil, cfg, logger).Routes().ServeHTTP(notAllowed, httptest.NewRequest(http.MethodPost, "/readyz", nil))
	if notAllowed.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST /readyz status=%d, want %d", notAllowed.Code, http.StatusMethodNotAllowed)
	}
}
