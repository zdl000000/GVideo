package interactions

import (
	"context"
	"net/http"
)

// Principal is the authentication data interactions need from the HTTP layer.
type Principal struct {
	UserID int64
}

// HTTPPort keeps protocol envelopes, request IDs, decoding, and error mapping
// owned by the application HTTP layer.
type HTTPPort interface {
	Principal(context.Context) Principal
	WriteJSON(http.ResponseWriter, *http.Request, int, any)
	WriteError(http.ResponseWriter, *http.Request, error)
	VideoID(http.ResponseWriter, *http.Request) (int64, bool)
	UserID(http.ResponseWriter, *http.Request) (int64, bool)
}

type interactionService interface {
	ToggleLike(context.Context, int64, int64) (bool, error)
	ToggleFavorite(context.Context, int64, int64) (bool, error)
	ToggleFollow(context.Context, int64, int64) (bool, error)
}

// Handler implements the interaction HTTP endpoints through an injected protocol port.
type Handler struct {
	service interactionService
	http    HTTPPort
}

func NewHandler(service interactionService, httpPort HTTPPort) *Handler {
	return &Handler{service: service, http: httpPort}
}

func (h *Handler) Like(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.service.ToggleLike)
}

func (h *Handler) Favorite(w http.ResponseWriter, r *http.Request) {
	h.toggle(w, r, h.service.ToggleFavorite)
}

func (h *Handler) Follow(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.http.UserID(w, r)
	if !ok {
		return
	}
	principal := h.http.Principal(r.Context())
	active, err := h.service.ToggleFollow(r.Context(), principal.UserID, userID)
	if err != nil {
		h.http.WriteError(w, r, err)
		return
	}
	h.http.WriteJSON(w, r, http.StatusOK, map[string]bool{"active": active})
}

func (h *Handler) toggle(w http.ResponseWriter, r *http.Request, action func(context.Context, int64, int64) (bool, error)) {
	videoID, ok := h.http.VideoID(w, r)
	if !ok {
		return
	}
	principal := h.http.Principal(r.Context())
	active, err := action(r.Context(), principal.UserID, videoID)
	if err != nil {
		h.http.WriteError(w, r, err)
		return
	}
	h.http.WriteJSON(w, r, http.StatusOK, map[string]bool{"active": active})
}
