package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"gvideo/backend/internal/config"
)

func TestLivezReportsProcessLivenessWithoutDependencies(t *testing.T) {
	cfg := config.Config{MediaDir: filepath.Join(t.TempDir(), "media")}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Nil application services prove that liveness does not query the database,
	// media worker, queue, or external commands.
	routes := New(nil, nil, cfg, logger).Routes()

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
