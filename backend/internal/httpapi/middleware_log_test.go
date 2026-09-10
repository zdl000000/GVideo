package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestAccessLogUsesStableRouteAndSafeFields(t *testing.T) {
	var output bytes.Buffer
	h := &Handler{logger: slog.New(slog.NewJSONHandler(&output, nil))}
	router := chi.NewRouter()
	router.Use(h.requestID)
	router.Use(h.accessLog)
	router.Use(h.recoverer)
	router.Get("/videos/{videoID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	})

	request := httptest.NewRequest(http.MethodGet, "/videos/42?token=query-secret", nil)
	request.Header.Set("Authorization", "Bearer authorization-secret")
	request.Header.Set("Cookie", "password=cookie-secret")
	request.Header.Set("X-CSRF-Token", "csrf-secret")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	record := decodeLogRecord(t, output.Bytes())
	assertLogValue(t, record, "event", "http_request")
	assertLogValue(t, record, "method", http.MethodGet)
	assertLogValue(t, record, "route", "/videos/{videoID}")
	assertLogNumber(t, record, "status", http.StatusCreated)
	assertLogNumber(t, record, "bytes", 2)
	assertNonNegativeLogNumber(t, record, "duration_ms")
	if record["request_id"] == "" {
		t.Fatal("request_id is empty")
	}
	for _, forbidden := range []string{"path", "url", "query", "authorization", "cookie", "password", "csrf", "42", "query-secret", "authorization-secret", "cookie-secret"} {
		if strings.Contains(strings.ToLower(output.String()), forbidden) {
			t.Fatalf("request log contains forbidden value %q: %s", forbidden, output.String())
		}
	}
}

func TestAccessLogNormalizesImplicitStatusAndUnmatchedRoute(t *testing.T) {
	var output bytes.Buffer
	h := &Handler{logger: slog.New(slog.NewJSONHandler(&output, nil))}
	router := chi.NewRouter()
	router.Use(h.requestID)
	router.Use(h.accessLog)
	router.Get("/empty", func(http.ResponseWriter, *http.Request) {})

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/not-found/private-value", nil))

	record := decodeLogRecord(t, output.Bytes())
	assertLogValue(t, record, "route", "unmatched")
	assertLogNumber(t, record, "status", http.StatusNotFound)
	if strings.Contains(output.String(), "private-value") {
		t.Fatalf("unmatched route leaked raw path: %s", output.String())
	}

	output.Reset()
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/empty", nil))
	record = decodeLogRecord(t, output.Bytes())
	assertLogNumber(t, record, "status", http.StatusOK)
	assertLogNumber(t, record, "bytes", 0)
}

func TestAccessLogRecordsRecoveredPanic(t *testing.T) {
	var output bytes.Buffer
	h := &Handler{logger: slog.New(slog.NewJSONHandler(&output, nil))}
	router := chi.NewRouter()
	router.Use(h.requestID)
	router.Use(h.accessLog)
	router.Use(h.recoverer)
	router.Get("/panic", func(http.ResponseWriter, *http.Request) { panic("password=panic-secret") })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", response.Code)
	}

	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
	if len(lines) != 2 {
		t.Fatalf("log lines=%d, want panic and request records: %s", len(lines), output.String())
	}
	var requestRecord map[string]any
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("decode log: %v", err)
		}
		if record["event"] == "http_request" {
			requestRecord = record
		}
	}
	if requestRecord == nil {
		t.Fatalf("request completion log missing: %s", output.String())
	}
	assertLogNumber(t, requestRecord, "status", http.StatusInternalServerError)
	if strings.Contains(output.String(), "panic-secret") {
		t.Fatalf("panic log leaked recovered value: %s", output.String())
	}
}

func decodeLogRecord(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &record); err != nil {
		t.Fatalf("decode log record: %v; log=%s", err, data)
	}
	return record
}

func assertLogValue(t *testing.T, record map[string]any, key string, want any) {
	t.Helper()
	if got := record[key]; got != want {
		t.Fatalf("%s=%v, want %v; record=%#v", key, got, want, record)
	}
}

func assertLogNumber(t *testing.T, record map[string]any, key string, want int) {
	t.Helper()
	got, ok := record[key].(float64)
	if !ok || int(got) != want {
		t.Fatalf("%s=%v, want %d; record=%#v", key, record[key], want, record)
	}
}

func assertNonNegativeLogNumber(t *testing.T, record map[string]any, key string) {
	t.Helper()
	got, ok := record[key].(float64)
	if !ok || got < 0 {
		t.Fatalf("%s=%v, want non-negative number; record=%#v", key, record[key], record)
	}
}

func TestRequestIDRejectsUnsafeCallerValues(t *testing.T) {
	tests := []string{
		"password=header-secret",
		"contains space",
		strings.Repeat("a", 129),
		"非ASCII",
	}
	for _, supplied := range tests {
		t.Run(supplied, func(t *testing.T) {
			var output bytes.Buffer
			h := &Handler{logger: slog.New(slog.NewJSONHandler(&output, nil))}
			router := chi.NewRouter()
			router.Use(h.requestID)
			router.Use(h.accessLog)
			router.Get("/ok", func(http.ResponseWriter, *http.Request) {})
			request := httptest.NewRequest(http.MethodGet, "/ok", nil)
			request.Header.Set("X-Request-ID", supplied)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			record := decodeLogRecord(t, output.Bytes())
			got, _ := record["request_id"].(string)
			if got == supplied || !validRequestID(got) {
				t.Fatalf("request_id=%q, unsafe supplied value=%q", got, supplied)
			}
			if response.Header().Get("X-Request-ID") != got {
				t.Fatalf("response request ID=%q, log=%q", response.Header().Get("X-Request-ID"), got)
			}
			if strings.Contains(output.String(), "header-secret") {
				t.Fatalf("unsafe caller request ID leaked: %s", output.String())
			}
		})
	}

	for _, valid := range []string{"a", "client.ID_1:part-two", strings.Repeat("x", 128)} {
		if !validRequestID(valid) {
			t.Fatalf("valid request ID rejected: %q", valid)
		}
	}
}
