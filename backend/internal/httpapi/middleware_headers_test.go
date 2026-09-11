package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gvideo/backend/internal/config"
)

func TestSecurityResponseHeaders(t *testing.T) {
	handler := newRateLimitTestHandler(t, config.Config{})
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil))

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
	}
	for name, value := range want {
		if got := result.Header().Get(name); got != value {
			t.Fatalf("%s = %q, want %q", name, got, value)
		}
	}
	if csp := result.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Fatalf("API Content-Security-Policy = %q", csp)
	}
}

func TestAllResponsesDenyFraming(t *testing.T) {
	handler := newRateLimitTestHandler(t, config.Config{})

	for _, path := range []string{"/", "/livez", "/readyz", "/api/v1/categories", "/media/avatars/missing.png"} {
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, httptest.NewRequest(http.MethodGet, path, nil))
		if got := result.Header().Get("X-Frame-Options"); got != "DENY" {
			t.Fatalf("%s X-Frame-Options = %q", path, got)
		}
		if csp := result.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Fatalf("%s Content-Security-Policy = %q", path, csp)
		}
	}
}
