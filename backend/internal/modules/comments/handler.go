package comments

import (
	"context"
	"net/http"

	"gvideo/backend/internal/domain"
)

// Principal is the authentication data comments need from the HTTP layer.
type Principal struct {
	UserID int64
}

// HTTPPort keeps protocol envelopes, request IDs, decoding, and error mapping
// owned by the application HTTP layer.
type HTTPPort interface {
	Principal(context.Context) Principal
	DecodeJSON(http.ResponseWriter, *http.Request, any) bool
	WriteJSON(http.ResponseWriter, *http.Request, int, any)
	WriteError(http.ResponseWriter, *http.Request, error)
	VideoID(http.ResponseWriter, *http.Request) (int64, bool)
	CommentID(http.ResponseWriter, *http.Request) (int64, bool)
}

type commentService interface {
	Comments(context.Context, int64, int64) ([]domain.Comment, error)
	CreateComment(context.Context, int64, int64, string) (domain.Comment, error)
	DeleteComment(context.Context, int64, int64, int64) error
}

// Handler implements the comment HTTP endpoints through an injected protocol port.
type Handler struct {
	service commentService
	http    HTTPPort
}

func NewHandler(service commentService, httpPort HTTPPort) *Handler {
	return &Handler{service: service, http: httpPort}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	videoID, ok := h.http.VideoID(w, r)
	if !ok {
		return
	}
	principal := h.http.Principal(r.Context())
	comments, err := h.service.Comments(r.Context(), videoID, principal.UserID)
	if err != nil {
		h.http.WriteError(w, r, err)
		return
	}
	if comments == nil {
		comments = []domain.Comment{}
	}
	h.http.WriteJSON(w, r, http.StatusOK, comments)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	videoID, ok := h.http.VideoID(w, r)
	if !ok {
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if !h.http.DecodeJSON(w, r, &input) {
		return
	}
	principal := h.http.Principal(r.Context())
	comment, err := h.service.CreateComment(r.Context(), principal.UserID, videoID, input.Content)
	if err != nil {
		h.http.WriteError(w, r, err)
		return
	}
	h.http.WriteJSON(w, r, http.StatusCreated, comment)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	videoID, ok := h.http.VideoID(w, r)
	if !ok {
		return
	}
	commentID, ok := h.http.CommentID(w, r)
	if !ok {
		return
	}
	principal := h.http.Principal(r.Context())
	if err := h.service.DeleteComment(r.Context(), principal.UserID, videoID, commentID); err != nil {
		h.http.WriteError(w, r, err)
		return
	}
	h.http.WriteJSON(w, r, http.StatusOK, map[string]bool{"deleted": true})
}
