package httpapi

import (
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"gvideo/backend/internal/domain"
)

const (
	rateLimitIdleTTL     = 10 * time.Minute
	rateLimitSweepPeriod = time.Minute
	rateLimitMaxBuckets  = 8192
	clientAddressV6Bits  = 64
)

// tokenBucketLimiter is a small in-process token bucket keyed by an arbitrary
// string. It intentionally avoids external dependencies; per-instance state is
// acceptable while the service runs as a single writer.
type tokenBucketLimiter struct {
	mu            sync.Mutex
	buckets       map[string]*tokenBucket
	ratePerSecond float64
	burst         float64
	idleTTL       time.Duration
	maxBuckets    int
	now           func() time.Time
	lastSweep     time.Time
}

type tokenBucket struct {
	tokens  float64
	updated time.Time
}

func newTokenBucketLimiter(perMinute int) *tokenBucketLimiter {
	if perMinute <= 0 {
		return nil
	}
	return &tokenBucketLimiter{
		buckets:       map[string]*tokenBucket{},
		ratePerSecond: float64(perMinute) / 60.0,
		burst:         float64(perMinute),
		idleTTL:       rateLimitIdleTTL,
		maxBuckets:    rateLimitMaxBuckets,
		now:           time.Now,
	}
}

// Allow consumes one token for key. An empty key is never limited. When the
// bucket is exhausted it returns the wait time until the next token.
func (l *tokenBucketLimiter) Allow(key string) (bool, time.Duration) {
	if l == nil || key == "" {
		return true, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweepLocked(now)

	bucket, exists := l.buckets[key]
	if !exists {
		if len(l.buckets) >= l.maxBuckets {
			l.evictOneLocked()
		}
		bucket = &tokenBucket{tokens: l.burst, updated: now}
		l.buckets[key] = bucket
	} else if elapsed := now.Sub(bucket.updated).Seconds(); elapsed > 0 {
		bucket.tokens = math.Min(l.burst, bucket.tokens+elapsed*l.ratePerSecond)
		bucket.updated = now
	}
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true, 0
	}
	wait := time.Duration((1 - bucket.tokens) / l.ratePerSecond * float64(time.Second))
	return false, wait
}

// sweepLocked drops buckets idle beyond the TTL, at most once per period. It is
// time-based on purpose: sweeping on every call above a size threshold would
// turn a key flood into repeated O(n) scans.
func (l *tokenBucketLimiter) sweepLocked(now time.Time) {
	if now.Sub(l.lastSweep) < rateLimitSweepPeriod {
		return
	}
	l.lastSweep = now
	for key, bucket := range l.buckets {
		if now.Sub(bucket.updated) > l.idleTTL {
			delete(l.buckets, key)
		}
	}
}

// evictOneLocked frees one bucket when the hard cap is reached. Go randomizes
// map iteration order, so this behaves like random eviction; the trade-off is
// that an evicted key regains a full burst instead of growing memory without
// bound.
func (l *tokenBucketLimiter) evictOneLocked() {
	for key := range l.buckets {
		delete(l.buckets, key)
		return
	}
}

// rateLimit bounds request rates for a named tier. A nil limiter (tier
// disabled) passes every request through.
func (h *Handler) rateLimit(tier string, key func(*http.Request) string) func(http.Handler) http.Handler {
	limiter := h.limiters[tier]
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, retryAfter := limiter.Allow(key(r))
			if !allowed {
				seconds := int(math.Ceil(retryAfter.Seconds()))
				if seconds < 1 {
					seconds = 1
				}
				w.Header().Set("Retry-After", strconv.Itoa(seconds))
				// Rejected requests may still carry a large unread body (uploads);
				// closing explicitly avoids the server discarding it mid-stream.
				w.Header().Set("Connection", "close")
				h.writeError(w, r, domain.ErrRateLimited)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientAddress replaces the peer address with the client address forwarded by
// a trusted reverse proxy. Only loopback and private peers may supply
// X-Forwarded-For or X-Real-IP; True-Client-IP is never trusted because the
// bundled gateway forwards client-controlled values for it.
func (h *Handler) clientAddress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if client := forwardedClientAddress(r); client != "" {
			r.RemoteAddr = client
		}
		next.ServeHTTP(w, r)
	})
}

func forwardedClientAddress(r *http.Request) string {
	peer, err := netip.ParseAddr(remoteHost(r.RemoteAddr))
	if err != nil {
		return ""
	}
	peer = peer.Unmap()
	if !peer.IsLoopback() && !peer.IsPrivate() {
		return ""
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		if ip := selectForwardedClient(forwarded); ip != "" {
			return ip
		}
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		if ip, err := netip.ParseAddr(realIP); err == nil {
			if ip = ip.Unmap(); !ip.IsUnspecified() {
				return ip.String()
			}
		}
	}
	return ""
}

// selectForwardedClient picks the client address from an X-Forwarded-For
// chain. Each hop appends the peer it received from, so the rightmost public
// entry is the address of the last hop before our trusted chain: a value the
// client cannot forge even when an upstream proxy (for example Cloudflare)
// preserves a client-supplied prefix. Chains without any public entry (LAN or
// local development) fall back to the rightmost valid address.
func selectForwardedClient(header string) string {
	parts := strings.Split(header, ",")
	fallback := ""
	for i := len(parts) - 1; i >= 0; i-- {
		ip, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			continue
		}
		ip = ip.Unmap()
		if ip.IsUnspecified() {
			continue
		}
		if fallback == "" {
			fallback = ip.String()
		}
		if !ip.IsLoopback() && !ip.IsPrivate() {
			return ip.String()
		}
	}
	return fallback
}

// rateLimitClientIP keys unauthenticated tiers by the peer address.
func rateLimitClientIP(r *http.Request) string {
	return normalizeClientKey(remoteHost(r.RemoteAddr))
}

// rateLimitUser keys authenticated tiers by the session user. Routes using it
// must run requireAuth first.
func rateLimitUser(r *http.Request) string {
	id := viewerID(r.Context())
	if id <= 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

// remoteHost strips an optional port from a remote address.
func remoteHost(address string) string {
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return address
}

// normalizeClientKey collapses IPv6 addresses to their /64 prefix so one
// allocation cannot rotate through addresses to bypass limits.
func normalizeClientKey(host string) string {
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return host
	}
	ip = ip.Unmap()
	if ip.Is4() {
		return ip.String()
	}
	return netip.PrefixFrom(ip, clientAddressV6Bits).Masked().String()
}
