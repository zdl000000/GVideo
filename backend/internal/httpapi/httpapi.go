package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/modules/moderation"
	"gvideo/backend/internal/platform/metrics"
	"gvideo/backend/internal/service"
)

const sessionCookie = "gvideo_session"

type contextKey string

const sessionKey contextKey = "session"

type Handler struct {
	service    *service.Service
	moderation *moderation.Handler
	cfg        config.Config
	logger     *slog.Logger
	metrics    *metrics.Registry
}

type response struct {
	Data      any    `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
	RequestID string `json:"request_id"`
}

func New(service *service.Service, moderationService *moderation.Service, cfg config.Config, logger *slog.Logger, registries ...*metrics.Registry) *Handler {
	var registry *metrics.Registry
	if len(registries) > 0 {
		registry = registries[0]
	}
	if registry == nil {
		registry = metrics.New()
	}
	h := &Handler{service: service, cfg: cfg, logger: logger, metrics: registry}
	h.moderation = moderation.NewHandler(moderationService, moderationHTTPPort{handler: h})
	return h
}

func (h *Handler) Routes() http.Handler {
	if err := os.MkdirAll(h.cfg.MediaDir, 0o755); err != nil {
		panic(fmt.Errorf("create media directory: %w", err))
	}
	router := chi.NewRouter()
	router.Use(h.instrument)
	router.Use(h.requestID)
	router.Use(h.recoverer)
	router.Use(h.accessLog)
	router.Use(middleware.RealIP)
	router.Use(middleware.Compress(5))
	router.Use(h.responseHeaders)
	router.Use(h.optionalSession)

	router.Get("/", h.root)
	router.Get("/healthz", h.health)
	router.Get("/media/*", h.media)
	router.Route("/api/v1", func(api chi.Router) {
		api.Get("/categories", h.categories)
		api.Post("/auth/register", h.register)
		api.Post("/auth/login", h.login)
		api.With(h.requireAuth, h.requireCSRF).Post("/auth/logout", h.logout)
		api.With(h.requireAuth).Get("/auth/me", h.me)
		api.With(h.requireAuth, h.requireCSRF).Patch("/me/profile", h.updateProfile)
		api.With(h.requireAuth, h.requireCSRF).Post("/me/password", h.changePassword)
		api.With(h.requireAuth).Get("/me/creator/stats", h.creatorStats)
		api.With(h.requireAuth).Get("/me/notifications", h.notifications)
		api.With(h.requireAuth, h.requireCSRF).Patch("/me/notifications/{notificationID}/read", h.markNotificationRead)
		api.With(h.requireAuth, h.requireCSRF).Post("/me/notifications/read-all", h.markAllNotificationsRead)
		api.With(h.requireAuth).Get("/admin/reports", h.moderation.AdminReports)
		api.With(h.requireAuth, h.requireCSRF).Patch("/admin/reports/{reportID}", h.moderation.ReviewReport)

		api.Get("/videos", h.listVideos)
		api.Get("/videos/{videoID}", h.getVideo)
		api.Get("/videos/{videoID}/comments", h.listComments)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos", h.uploadVideo)
		api.With(h.requireAuth, h.requireCSRF).Patch("/videos/{videoID}", h.updateVideo)
		api.With(h.requireAuth, h.requireCSRF).Delete("/videos/{videoID}", h.deleteVideo)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos/{videoID}/retry", h.retryVideo)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos/{videoID}/subtitles", h.uploadSubtitle)
		api.With(h.requireAuth, h.requireCSRF).Patch("/videos/{videoID}/subtitles/{subtitleID}/default", h.setDefaultSubtitle)
		api.With(h.requireAuth, h.requireCSRF).Delete("/videos/{videoID}/subtitles/{subtitleID}", h.deleteSubtitle)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos/{videoID}/like", h.toggleLike)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos/{videoID}/favorite", h.toggleFavorite)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos/{videoID}/reports", h.moderation.ReportVideo)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos/{videoID}/comments", h.createComment)
		api.With(h.requireAuth, h.requireCSRF).Delete("/videos/{videoID}/comments/{commentID}", h.deleteComment)
		api.With(h.requireAuth).Get("/me/videos", h.myVideos)
		api.With(h.requireAuth).Get("/me/following/videos", h.followingVideos)
		api.With(h.requireAuth).Get("/me/favorites", h.favoriteVideos)
		api.Get("/users/{userID}", h.getCreator)
		api.Get("/users/{userID}/videos", h.creatorVideos)
		api.With(h.requireAuth, h.requireCSRF).Post("/users/{userID}/follow", h.toggleFollow)
	})
	return router
}

func (h *Handler) media(w http.ResponseWriter, r *http.Request) {
	storedPath := strings.TrimPrefix(strings.ReplaceAll(chi.URLParam(r, "*"), `\`, "/"), "/")
	if storedPath == "" || strings.Contains(storedPath, "\x00") {
		http.NotFound(w, r)
		return
	}
	cleanPath := filepath.ToSlash(filepath.Clean(filepath.FromSlash(storedPath)))
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		http.NotFound(w, r)
		return
	}
	if err := h.service.AuthorizeMedia(r.Context(), cleanPath, viewerID(r.Context())); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			h.logger.Error("authorize media", "request_id", requestID(r), "path", cleanPath, "error", err)
		}
		http.NotFound(w, r)
		return
	}
	resolved, err := resolveServedMediaPath(h.cfg.MediaDir, cleanPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch strings.ToLower(filepath.Ext(cleanPath)) {
	case ".m3u8":
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	case ".ts":
		w.Header().Set("Content-Type", "video/mp2t")
	case ".vtt":
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	}
	http.ServeFile(w, r, resolved)
}

func resolveServedMediaPath(root, storedPath string) (string, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Abs(filepath.Join(absoluteRoot, filepath.FromSlash(storedPath)))
	if err != nil {
		return "", err
	}
	realRoot, err := filepath.EvalSymlinks(absoluteRoot)
	if err != nil {
		return "", err
	}
	realResolved, err := filepath.EvalSymlinks(resolved)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(realRoot, realResolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("media path escapes media directory")
	}
	return realResolved, nil
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) root(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]string{
		"service":  "gvideo-backend",
		"status":   "ok",
		"frontend": h.cfg.FrontendURL,
		"health":   "/healthz",
	})
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		writeProblem(w, r, http.StatusBadRequest, "输入内容不符合要求")
	case errors.Is(err, domain.ErrConflict):
		writeProblem(w, r, http.StatusConflict, "用户名已被使用")
	case errors.Is(err, domain.ErrSubtitleExists):
		writeProblem(w, r, http.StatusConflict, "该语言的字幕已存在")
	case errors.Is(err, domain.ErrVideoProcessing):
		writeProblem(w, r, http.StatusConflict, "视频正在转码，请处理完成后再删除")
	case errors.Is(err, domain.ErrRetryUnavailable):
		writeProblem(w, r, http.StatusConflict, "只有处理失败的视频可以重新转码")
	case errors.Is(err, domain.ErrUnauthorized), errors.Is(err, domain.ErrInvalidSession):
		writeProblem(w, r, http.StatusUnauthorized, "用户名或密码不正确")
	case errors.Is(err, domain.ErrForbidden):
		writeProblem(w, r, http.StatusForbidden, "没有权限执行此操作")
	case errors.Is(err, domain.ErrNotFound):
		writeProblem(w, r, http.StatusNotFound, "内容不存在")
	default:
		h.logger.Error("request failed", "request_id", requestID(r), "error", err)
		writeProblem(w, r, http.StatusInternalServerError, "服务暂时不可用")
	}
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response{Data: data, RequestID: requestID(r)})
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response{Error: message, RequestID: requestID(r)})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "请求内容不是有效的 JSON")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProblem(w, r, http.StatusBadRequest, "请求内容只能包含一个 JSON 对象")
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "videoID"), 10, 64)
	if err != nil || id <= 0 {
		writeProblem(w, r, http.StatusBadRequest, "视频编号无效")
		return 0, false
	}
	return id, true
}

func userPathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil || id <= 0 {
		writeProblem(w, r, http.StatusBadRequest, "用户编号无效")
		return 0, false
	}
	return id, true
}

func subtitlePathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "subtitleID"), 10, 64)
	if err != nil || id <= 0 {
		writeProblem(w, r, http.StatusBadRequest, "字幕编号无效")
		return 0, false
	}
	return id, true
}

func fileHeader(r *http.Request, name string, required bool) (*multipart.FileHeader, error) {
	_, header, err := r.FormFile(name)
	if err != nil {
		if !required && errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, err
	}
	return header, nil
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func pagination(r *http.Request, defaultSize int) (int, int) {
	page := parseInt(r.URL.Query().Get("page"), 1)
	if page < 1 {
		page = 1
	}
	pageSize := parseInt(r.URL.Query().Get("page_size"), 0)
	if pageSize == 0 {
		pageSize = parseInt(r.URL.Query().Get("limit"), defaultSize)
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = defaultSize
	}
	if offset := parseInt(r.URL.Query().Get("offset"), -1); offset >= 0 && r.URL.Query().Get("page") == "" {
		page = offset/pageSize + 1
	}
	return page, pageSize
}

func sessionFrom(ctx context.Context) domain.Session {
	session, _ := ctx.Value(sessionKey).(domain.Session)
	return session
}

func viewerID(ctx context.Context) int64 { return sessionFrom(ctx).User.ID }

// moderationHTTPPort adapts application HTTP concerns without exposing the
// httpapi package to the moderation module.
type moderationHTTPPort struct{ handler *Handler }

func (p moderationHTTPPort) Principal(ctx context.Context) moderation.Principal {
	session := sessionFrom(ctx)
	return moderation.Principal{UserID: session.User.ID, Username: session.User.Username}
}
func (p moderationHTTPPort) DecodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	return decodeJSON(w, r, target)
}
func (p moderationHTTPPort) WriteJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	writeJSON(w, r, status, data)
}
func (p moderationHTTPPort) WriteError(w http.ResponseWriter, r *http.Request, err error) {
	p.handler.writeError(w, r, err)
}
func (p moderationHTTPPort) VideoID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return pathID(w, r)
}
func (p moderationHTTPPort) Pagination(r *http.Request, defaultSize int) (int, int) {
	return pagination(r, defaultSize)
}
