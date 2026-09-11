package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (h *Handler) optionalSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/livez" || r.URL.Path == "/readyz" || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		cookie, err := r.Cookie(sessionCookie)
		if err == nil && cookie.Value != "" {
			session, authErr := h.service.Authenticate(r.Context(), cookie.Value)
			if authErr == nil {
				r = r.WithContext(context.WithValue(r.Context(), sessionKey, session))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) responseHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// Backend responses are JSON, media, or plain text — never documents;
		// denying every resource type keeps accidentally rendered HTML inert
		// and blocks framing of error pages.
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") ||
			strings.HasPrefix(r.URL.Path, "/api/v1/me/") ||
			strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sessionFrom(r.Context()).User.ID == 0 {
			writeProblem(w, r, http.StatusUnauthorized, "请先登录")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) requireCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := r.Header.Get("X-CSRF-Token")
		if provided == "" || provided != sessionFrom(r.Context()).CSRFToken {
			writeProblem(w, r, http.StatusForbidden, "页面凭证已过期，请刷新后重试")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if !validRequestID(requestID) {
			buffer := make([]byte, 12)
			_, _ = rand.Read(buffer)
			requestID = hex.EncodeToString(buffer)
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), middleware.RequestIDKey, requestID)))
	})
}

func validRequestID(value string) bool {
	if len(value) < 1 || len(value) > 128 {
		return false
	}
	for _, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == ':' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func (h *Handler) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				h.logger.Error("http panic recovered",
					"event", "http_panic",
					"request_id", requestID(r),
					"error_class", "panic",
				)
				writeProblem(w, r, http.StatusInternalServerError, "服务暂时不可用")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()
		next.ServeHTTP(wrapped, r)
		h.logger.Info("http request completed",
			"event", "http_request",
			"request_id", requestID(r),
			"method", r.Method,
			"route", requestRoute(r),
			"status", normalizedHTTPStatus(wrapped.Status()),
			"bytes", wrapped.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

func requestRoute(r *http.Request) string {
	route := chi.RouteContext(r.Context()).RoutePattern()
	if route == "" {
		return "unmatched"
	}
	return route
}

func normalizedHTTPStatus(status int) int {
	if status == 0 {
		return http.StatusOK
	}
	return status
}

func (h *Handler) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", MaxAge: int(h.cfg.SessionTTL.Seconds()),
		HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func requestID(r *http.Request) string {
	value, _ := r.Context().Value(middleware.RequestIDKey).(string)
	return value
}
