package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/modules/comments"
	"gvideo/backend/internal/modules/moderation"
	"gvideo/backend/internal/modules/notifications"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
	"gvideo/backend/internal/service"
)

func newRateLimitTestHandler(t *testing.T, cfg config.Config) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "ratelimit.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	cfg.MediaDir = filepath.Join(t.TempDir(), "media")
	if cfg.FrontendURL == "" {
		cfg.FrontendURL = "https://video.example.test"
	}
	if cfg.SessionTTL == 0 {
		cfg.SessionTTL = time.Hour
	}
	return New(service.New(repository.New(db), cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notifications.NewRepository(db)), comments.NewService(comments.NewRepository(db), logger), cfg, logger).Routes()
}

func TestTokenBucketLimiterAllowsBurstThenRefills(t *testing.T) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limiter := &tokenBucketLimiter{
		buckets:       map[string]*tokenBucket{},
		ratePerSecond: 1,
		burst:         2,
		idleTTL:       time.Minute,
		maxBuckets:    rateLimitMaxBuckets,
		now:           func() time.Time { return clock },
	}

	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("first token was rejected")
	}
	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("second token was rejected")
	}
	ok, wait := limiter.Allow("client")
	if ok || wait <= 0 {
		t.Fatalf("exhausted bucket allowed=%v wait=%s", ok, wait)
	}

	clock = clock.Add(time.Second)
	if ok, _ := limiter.Allow("client"); !ok {
		t.Fatal("token did not refill after one second")
	}
	if ok, _ := limiter.Allow("client"); ok {
		t.Fatal("refill exceeded the burst allowance")
	}
}

func TestTokenBucketLimiterDisabledAndEmptyKeyPassThrough(t *testing.T) {
	var disabled *tokenBucketLimiter
	if ok, _ := disabled.Allow("client"); !ok {
		t.Fatal("nil limiter should allow every request")
	}
	limiter := newTokenBucketLimiter(1)
	if limiter == nil {
		t.Fatal("positive per-minute limit should build a limiter")
	}
	if ok, _ := limiter.Allow(""); !ok {
		t.Fatal("empty key should never be limited")
	}
}

func TestTokenBucketLimiterSweepsIdleBuckets(t *testing.T) {
	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	limiter := &tokenBucketLimiter{
		buckets:       map[string]*tokenBucket{},
		ratePerSecond: 1,
		burst:         1,
		idleTTL:       10 * time.Minute,
		maxBuckets:    rateLimitMaxBuckets,
		now:           func() time.Time { return clock },
	}
	limiter.Allow("idle-client")
	clock = clock.Add(11 * time.Minute)
	limiter.Allow("active-client")

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if _, exists := limiter.buckets["idle-client"]; exists {
		t.Fatal("idle bucket was not swept")
	}
	if len(limiter.buckets) != 1 {
		t.Fatalf("bucket count after sweep = %d, want 1", len(limiter.buckets))
	}
}

func TestAuthRateLimitIsPerClientAddress(t *testing.T) {
	handler := newRateLimitTestHandler(t, config.Config{RateLimitAuthPerMinute: 2})

	attempt := func(remoteAddr string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"nobody","password":"wrong-password"}`))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = remoteAddr
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, request)
		return result
	}

	for i := 0; i < 2; i++ {
		if code := attempt("203.0.113.7:4000").Code; code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status=%d, want 401", i+1, code)
		}
	}
	limited := attempt("203.0.113.7:4001")
	if limited.Code != http.StatusTooManyRequests {
		t.Fatalf("limited status=%d, want 429", limited.Code)
	}
	if retryAfter := limited.Header().Get("Retry-After"); retryAfter == "" {
		t.Fatal("429 response is missing Retry-After")
	}
	var envelope testEnvelope
	decodeResponse(t, limited, &envelope)
	if envelope.Error == "" || envelope.RequestID == "" {
		t.Fatalf("unexpected 429 envelope: %#v", envelope)
	}

	// A different client address has its own bucket.
	if code := attempt("203.0.113.8:4000").Code; code != http.StatusUnauthorized {
		t.Fatalf("unrelated client status=%d, want 401", code)
	}
}

func TestCommentRateLimitIsPerUser(t *testing.T) {
	handler := newRateLimitTestHandler(t, config.Config{RateLimitCommentPerMinute: 2})
	alice := registerTestUser(t, handler, "ratelimit_alice")
	bob := registerTestUser(t, handler, "ratelimit_bob")

	post := func(client testAuthClient) int {
		request := authorizedRequest(http.MethodPost, "/api/v1/videos/999/comments", strings.NewReader(`{"content":"hello"}`), client.Cookie, client.CSRFToken)
		result := httptest.NewRecorder()
		handler.ServeHTTP(result, request)
		return result.Code
	}

	for i := 0; i < 2; i++ {
		if code := post(alice); code == http.StatusTooManyRequests {
			t.Fatalf("alice request %d was limited too early", i+1)
		}
	}
	if code := post(alice); code != http.StatusTooManyRequests {
		t.Fatalf("alice third request status=%d, want 429", code)
	}
	// Bob keeps his own budget.
	if code := post(bob); code == http.StatusTooManyRequests {
		t.Fatal("bob was limited by alice's usage")
	}
}

func TestUploadRateLimitIsPerUser(t *testing.T) {
	handler := newRateLimitTestHandler(t, config.Config{RateLimitUploadPerMinute: 1})
	user := registerTestUser(t, handler, "ratelimit_uploader")

	request := authorizedRequest(http.MethodPost, "/api/v1/videos", strings.NewReader(""), user.Cookie, user.CSRFToken)
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code == http.StatusTooManyRequests {
		t.Fatal("first upload request was limited")
	}

	second := authorizedRequest(http.MethodPost, "/api/v1/videos", strings.NewReader(""), user.Cookie, user.CSRFToken)
	secondResult := httptest.NewRecorder()
	handler.ServeHTTP(secondResult, second)
	if secondResult.Code != http.StatusTooManyRequests {
		t.Fatalf("second upload request status=%d, want 429", secondResult.Code)
	}
}

func TestForwardedClientAddressTrustsOnlyProxyPeers(t *testing.T) {
	request := func(remoteAddr, forwardedFor, realIP, trueClientIP string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = remoteAddr
		if forwardedFor != "" {
			r.Header.Set("X-Forwarded-For", forwardedFor)
		}
		if realIP != "" {
			r.Header.Set("X-Real-IP", realIP)
		}
		if trueClientIP != "" {
			r.Header.Set("True-Client-IP", trueClientIP)
		}
		return r
	}

	// A public peer cannot speak for the client.
	if got := forwardedClientAddress(request("203.0.113.9:1234", "198.51.100.7", "", "")); got != "" {
		t.Fatalf("public peer forwarded address = %q, want empty", got)
	}
	// A trusted proxy peer's chain wins, preferring the rightmost public entry.
	if got := forwardedClientAddress(request("172.18.0.5:5555", "198.51.100.7, 10.0.0.1", "", "")); got != "198.51.100.7" {
		t.Fatalf("trusted peer forwarded address = %q, want 198.51.100.7", got)
	}
	// A spoofed client prefix survives proxies that append (Cloudflare); the
	// rightmost public entry is the real client.
	if got := forwardedClientAddress(request("172.18.0.5:5555", "203.0.113.66, 198.51.100.7, 127.0.0.1", "", "")); got != "198.51.100.7" {
		t.Fatalf("spoofed prefix was trusted: %q", got)
	}
	// Chains without any public entry fall back to the rightmost valid address.
	if got := forwardedClientAddress(request("172.18.0.5:5555", "192.168.1.5, 172.18.0.9", "", "")); got != "172.18.0.9" {
		t.Fatalf("all-private chain fallback = %q, want 172.18.0.9", got)
	}
	// True-Client-IP is never trusted, even from a proxy peer.
	if got := forwardedClientAddress(request("172.18.0.5:5555", "", "", "198.51.100.9")); got != "" {
		t.Fatalf("True-Client-IP was trusted: %q", got)
	}
	// X-Real-IP is the fallback for proxies that omit X-Forwarded-For.
	if got := forwardedClientAddress(request("127.0.0.1:5555", "", "198.51.100.11", "")); got != "198.51.100.11" {
		t.Fatalf("X-Real-IP fallback = %q, want 198.51.100.11", got)
	}
	// Malformed and unspecified values are ignored.
	if got := forwardedClientAddress(request("172.18.0.5:5555", "not-an-ip", "", "")); got != "" {
		t.Fatalf("malformed forwarded address = %q, want empty", got)
	}
	if got := forwardedClientAddress(request("172.18.0.5:5555", "0.0.0.0", "", "")); got != "" {
		t.Fatalf("unspecified forwarded address = %q, want empty", got)
	}
}

func TestNormalizeClientKeyCollapsesIPv6ToSlash64(t *testing.T) {
	if got := normalizeClientKey("2001:db8:1:2:3:4:5:6"); got != "2001:db8:1:2::/64" {
		t.Fatalf("IPv6 key = %q", got)
	}
	if got := normalizeClientKey("::ffff:192.0.2.5"); got != "192.0.2.5" {
		t.Fatalf("IPv4-mapped key = %q", got)
	}
	if got := normalizeClientKey("192.0.2.5"); got != "192.0.2.5" {
		t.Fatalf("IPv4 key = %q", got)
	}
	if got := normalizeClientKey("not-an-address"); got != "not-an-address" {
		t.Fatalf("non-address key = %q", got)
	}
}

func TestTokenBucketLimiterEvictsAtHardCap(t *testing.T) {
	limiter := &tokenBucketLimiter{
		buckets:       map[string]*tokenBucket{},
		ratePerSecond: 1,
		burst:         1,
		idleTTL:       time.Hour,
		maxBuckets:    2,
		now:           time.Now,
	}
	for _, key := range []string{"a", "b", "c"} {
		if ok, _ := limiter.Allow(key); !ok {
			t.Fatalf("key %s was rejected", key)
		}
	}
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.buckets) != 2 {
		t.Fatalf("bucket count = %d, want hard cap 2", len(limiter.buckets))
	}
}

func TestRejectedCSRFDoesNotConsumeRateLimit(t *testing.T) {
	handler := newRateLimitTestHandler(t, config.Config{RateLimitCommentPerMinute: 1})
	user := registerTestUser(t, handler, "ratelimit_csrf")

	// A missing CSRF token is rejected before the limiter runs.
	rejected := authorizedRequest(http.MethodPost, "/api/v1/videos/999/comments", strings.NewReader(`{"content":"hello"}`), user.Cookie, "")
	rejectedResult := httptest.NewRecorder()
	handler.ServeHTTP(rejectedResult, rejected)
	if rejectedResult.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d, want 403", rejectedResult.Code)
	}

	// The single token is still available for a valid request.
	valid := authorizedRequest(http.MethodPost, "/api/v1/videos/999/comments", strings.NewReader(`{"content":"hello"}`), user.Cookie, user.CSRFToken)
	validResult := httptest.NewRecorder()
	handler.ServeHTTP(validResult, valid)
	if validResult.Code == http.StatusTooManyRequests {
		t.Fatal("a rejected CSRF request consumed the rate limit token")
	}

	// The following valid request is limited.
	second := authorizedRequest(http.MethodPost, "/api/v1/videos/999/comments", strings.NewReader(`{"content":"hello"}`), user.Cookie, user.CSRFToken)
	secondResult := httptest.NewRecorder()
	handler.ServeHTTP(secondResult, second)
	if secondResult.Code != http.StatusTooManyRequests {
		t.Fatalf("second valid request status=%d, want 429", secondResult.Code)
	}
}
