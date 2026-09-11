package httpapi

import (
	"net/http"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/service"
)

func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 11<<20)
	if err := r.ParseMultipartForm(11 << 20); err != nil {
		writeProblem(w, r, http.StatusBadRequest, "资料内容过大或格式无效")
		return
	}
	avatar, err := fileHeader(r, "avatar", false)
	if err != nil {
		writeProblem(w, r, http.StatusBadRequest, "头像文件无效")
		return
	}
	updated, err := h.service.UpdateProfile(r.Context(), service.UpdateProfileInput{
		UserID: sessionFrom(r.Context()).User.ID, Username: r.FormValue("username"),
		Bio: r.FormValue("bio"), Avatar: avatar,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, updated)
}

func (h *Handler) creatorStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.CreatorStats(r.Context(), sessionFrom(r.Context()).User.ID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, stats)
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
