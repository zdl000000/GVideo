package httpapi

import (
	"net/http"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/service"
)

func (h *Handler) categories(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, service.Categories)
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
	result, err := h.service.ListVideos(r.Context(), domain.VideoFilter{
		UserID: userID, IncludeNonPublic: true, Limit: pageSize, Offset: (page - 1) * pageSize,
	}, userID)
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

func (h *Handler) favoriteVideos(w http.ResponseWriter, r *http.Request) {
	userID := sessionFrom(r.Context()).User.ID
	page, pageSize := pagination(r, 24)
	result, err := h.service.ListVideos(r.Context(), domain.VideoFilter{
		FavoriteUserID: userID, Limit: pageSize, Offset: (page - 1) * pageSize,
	}, userID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, result)
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
		Category: r.FormValue("category"), Visibility: r.FormValue("visibility"), Video: video, Cover: cover, Subtitle: subtitle,
		SubtitleLanguage: r.FormValue("subtitle_language"), SubtitleLabel: r.FormValue("subtitle_label"),
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, created)
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
		Title: r.FormValue("title"), Description: r.FormValue("description"), Category: r.FormValue("category"),
		Visibility: r.FormValue("visibility"), Cover: cover,
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
