package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/service"
)

const sessionCookie = "gvideo_session"

type contextKey string

const sessionKey contextKey = "session"

type Handler struct {
	service *service.Service
	cfg     config.Config
	logger  *slog.Logger
}

type response struct {
	Data      any    `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
	RequestID string `json:"request_id"`
}

func New(service *service.Service, cfg config.Config, logger *slog.Logger) *Handler {
	return &Handler{service: service, cfg: cfg, logger: logger}
}

func (h *Handler) Routes() http.Handler {
	if err := os.MkdirAll(h.cfg.MediaDir, 0o755); err != nil {
		panic(fmt.Errorf("create media directory: %w", err))
	}
	router := chi.NewRouter()
	router.Use(h.requestID)
	router.Use(h.recoverer)
	router.Use(h.accessLog)
	router.Use(middleware.RealIP)
	router.Use(middleware.Compress(5))
	router.Use(h.optionalSession)

	router.Get("/healthz", h.health)
	router.Handle("/media/*", http.StripPrefix("/media/", mediaFileServer(h.cfg.MediaDir)))
	router.Route("/api/v1", func(api chi.Router) {
		api.Get("/categories", h.categories)
		api.Post("/auth/register", h.register)
		api.Post("/auth/login", h.login)
		api.With(h.requireAuth, h.requireCSRF).Post("/auth/logout", h.logout)
		api.With(h.requireAuth).Get("/auth/me", h.me)

		api.Get("/videos", h.listVideos)
		api.Get("/videos/{videoID}", h.getVideo)
		api.Get("/videos/{videoID}/comments", h.listComments)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos", h.uploadVideo)
		api.With(h.requireAuth, h.requireCSRF).Patch("/videos/{videoID}", h.updateVideo)
		api.With(h.requireAuth, h.requireCSRF).Delete("/videos/{videoID}", h.deleteVideo)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos/{videoID}/retry", h.retryVideo)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos/{videoID}/subtitles", h.uploadSubtitle)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos/{videoID}/like", h.toggleLike)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos/{videoID}/favorite", h.toggleFavorite)
		api.With(h.requireAuth, h.requireCSRF).Post("/videos/{videoID}/comments", h.createComment)
		api.With(h.requireAuth).Get("/me/videos", h.myVideos)
		api.With(h.requireAuth).Get("/me/following/videos", h.followingVideos)
		api.Get("/users/{userID}", h.getCreator)
		api.Get("/users/{userID}/videos", h.creatorVideos)
		api.With(h.requireAuth, h.requireCSRF).Post("/users/{userID}/follow", h.toggleFollow)
	})
	return router
}

func mediaFileServer(mediaDir string) http.Handler {
	files := http.FileServer(http.Dir(mediaDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.ToLower(filepath.Ext(r.URL.Path)) {
		case ".m3u8":
			w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		case ".ts":
			w.Header().Set("Content-Type", "video/mp2t")
		case ".vtt":
			w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		}
		files.ServeHTTP(w, r)
	})
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) categories(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, service.Categories)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	created, err := h.service.Register(r.Context(), input.Username, input.Password)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.setSessionCookie(w, created.Token)
	w.Header().Set("X-CSRF-Token", created.CSRFToken)
	writeJSON(w, r, http.StatusCreated, map[string]any{"user": created.User, "csrf_token": created.CSRFToken})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	created, err := h.service.Login(r.Context(), input.Username, input.Password)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	h.setSessionCookie(w, created.Token)
	w.Header().Set("X-CSRF-Token", created.CSRFToken)
	writeJSON(w, r, http.StatusOK, map[string]any{"user": created.User, "csrf_token": created.CSRFToken})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	cookie, _ := r.Cookie(sessionCookie)
	if cookie != nil {
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			h.writeError(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: h.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
	writeJSON(w, r, http.StatusOK, map[string]bool{"logged_out": true})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	session := sessionFrom(r.Context())
	writeJSON(w, r, http.StatusOK, map[string]any{"user": session.User, "csrf_token": session.CSRFToken})
}

func (h *Handler) listVideos(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pagination(r, 24)
	filter := domain.VideoFilter{
		Query: r.URL.Query().Get("q"), Category: r.URL.Query().Get("category"), Sort: r.URL.Query().Get("sort"),
		Limit: pageSize, Offset: (page - 1) * pageSize,
	}
	result, err := h.service.ListVideos(r.Context(), filter, viewerID(r.Context()))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, result)
}

func (h *Handler) myVideos(w http.ResponseWriter, r *http.Request) {
	userID := sessionFrom(r.Context()).User.ID
	page, pageSize := pagination(r, 12)
	result, err := h.service.ListVideos(r.Context(), domain.VideoFilter{UserID: userID, Limit: pageSize, Offset: (page - 1) * pageSize}, userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, result)
}

func (h *Handler) followingVideos(w http.ResponseWriter, r *http.Request) {
	userID := sessionFrom(r.Context()).User.ID
	page, pageSize := pagination(r, 24)
	result, err := h.service.ListVideos(r.Context(), domain.VideoFilter{
		FollowingUserID: userID, Limit: pageSize, Offset: (page - 1) * pageSize,
	}, userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, result)
}

func (h *Handler) getCreator(w http.ResponseWriter, r *http.Request) {
	id, ok := userPathID(w, r)
	if !ok {
		return
	}
	profile, err := h.service.CreatorProfile(r.Context(), id, viewerID(r.Context()))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, profile)
}

func (h *Handler) creatorVideos(w http.ResponseWriter, r *http.Request) {
	id, ok := userPathID(w, r)
	if !ok {
		return
	}
	if _, err := h.service.CreatorProfile(r.Context(), id, viewerID(r.Context())); err != nil {
		h.writeError(w, r, err)
		return
	}
	page, pageSize := pagination(r, 12)
	result, err := h.service.ListVideos(r.Context(), domain.VideoFilter{UserID: id, Limit: pageSize, Offset: (page - 1) * pageSize}, viewerID(r.Context()))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, result)
}

func (h *Handler) toggleFollow(w http.ResponseWriter, r *http.Request) {
	id, ok := userPathID(w, r)
	if !ok {
		return
	}
	active, err := h.service.ToggleFollow(r.Context(), sessionFrom(r.Context()).User.ID, id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"active": active})
}

func (h *Handler) getVideo(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	video, err := h.service.Video(r.Context(), id, viewerID(r.Context()), r.URL.Query().Get("count_view") != "false")
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, video)
}

func (h *Handler) uploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.cfg.MaxUploadBytes+(12<<20))
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "上传内容无效或文件过大")
		return
	}
	video, err := fileHeader(r, "video", true)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "请选择支持的视频文件")
		return
	}
	cover, _ := fileHeader(r, "cover", false)
	subtitle, subtitleErr := fileHeader(r, "subtitle", false)
	if subtitleErr != nil {
		writeProblem(w, r, http.StatusBadRequest, "字幕文件无效")
		return
	}
	created, err := h.service.UploadVideo(r.Context(), service.UploadInput{
		UserID: sessionFrom(r.Context()).User.ID, Title: r.FormValue("title"), Description: r.FormValue("description"),
		Category: r.FormValue("category"), Video: video, Cover: cover, Subtitle: subtitle,
		SubtitleLanguage: r.FormValue("subtitle_language"), SubtitleLabel: r.FormValue("subtitle_label"),
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, created)
}

func (h *Handler) uploadSubtitle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
	if err := r.ParseMultipartForm(3 << 20); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "字幕上传内容无效")
		return
	}
	header, err := fileHeader(r, "subtitle", true)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "请选择字幕文件")
		return
	}
	track, err := h.service.AddSubtitle(r.Context(), sessionFrom(r.Context()).User.ID, id, header, r.FormValue("subtitle_language"), r.FormValue("subtitle_label"))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, track)
}

func (h *Handler) updateVideo(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "编辑内容过大或格式无效")
		return
	}
	cover, err := fileHeader(r, "cover", false)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "封面文件无效")
		return
	}
	updated, err := h.service.UpdateVideo(r.Context(), service.UpdateVideoInput{
		UserID: sessionFrom(r.Context()).User.ID, VideoID: id,
		Title: r.FormValue("title"), Description: r.FormValue("description"), Category: r.FormValue("category"), Cover: cover,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, updated)
}

func (h *Handler) deleteVideo(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.service.DeleteVideo(r.Context(), sessionFrom(r.Context()).User.ID, id); err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"deleted": true})
}

func (h *Handler) retryVideo(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.service.RetryTranscoding(r.Context(), sessionFrom(r.Context()).User.ID, id); err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusAccepted, map[string]string{"processing_status": "pending"})
}

func (h *Handler) toggleLike(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.service.ToggleLike)
}

func (h *Handler) toggleFavorite(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.service.ToggleFavorite)
}

func (h *Handler) toggle(w http.ResponseWriter, r *http.Request, action func(context.Context, int64, int64) (bool, error)) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	active, err := action(r.Context(), sessionFrom(r.Context()).User.ID, id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"active": active})
}

func (h *Handler) listComments(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	comments, err := h.service.Comments(r.Context(), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if comments == nil {
		comments = []domain.Comment{}
	}
	writeJSON(w, r, http.StatusOK, comments)
}

func (h *Handler) createComment(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	comment, err := h.service.CreateComment(r.Context(), sessionFrom(r.Context()).User.ID, id, input.Content)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, comment)
}

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

func requestID(r *http.Request) string {
	value, _ := r.Context().Value(middleware.RequestIDKey).(string)
	return value
}
