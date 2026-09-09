package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func (h *Handler) optionalSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		if requestID == "" {
			buffer := make([]byte, 12)
			_, _ = rand.Read(buffer)
			requestID = hex.EncodeToString(buffer)
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), middleware.RequestIDKey, requestID)))
	})
}

func (h *Handler) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				h.logger.Error("panic recovered", "request_id", requestID(r), "panic", recovered)
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
		h.logger.Info("http request", "request_id", requestID(r), "method", r.Method, "path", r.URL.Path,
			"status", wrapped.Status(), "bytes", wrapped.BytesWritten(), "duration_ms", time.Since(start).Milliseconds())
	})
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
