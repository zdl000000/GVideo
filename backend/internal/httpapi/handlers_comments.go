package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gvideo/backend/internal/domain"
)

func (h *Handler) listComments(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	comments, err := h.service.Comments(r.Context(), id, viewerID(r.Context()))
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

func (h *Handler) deleteComment(w http.ResponseWriter, r *http.Request) {
	videoID, ok := pathID(w, r)
	if !ok {
		return
	}
	commentID, err := strconv.ParseInt(chi.URLParam(r, "commentID"), 10, 64)
	if err != nil || commentID <= 0 {
		writeProblem(w, r, http.StatusBadRequest, "无效的评论编号")
		return
	}
	if err := h.service.DeleteComment(r.Context(), sessionFrom(r.Context()).User.ID, videoID, commentID); err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"deleted": true})
}
