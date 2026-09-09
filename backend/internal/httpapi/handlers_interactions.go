package httpapi

import (
	"context"
	"net/http"
)

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
