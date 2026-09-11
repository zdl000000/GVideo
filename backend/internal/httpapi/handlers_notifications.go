package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) notifications(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pagination(r, 20)
	result, err := h.service.Notifications(r.Context(), sessionFrom(r.Context()).User.ID, page, pageSize)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, result)
}

func (h *Handler) markNotificationRead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "notificationID"), 10, 64)
	if err != nil || id <= 0 {
		writeProblem(w, r, http.StatusBadRequest, "通知编号无效")
		return
	}
	if err := h.service.MarkNotificationRead(r.Context(), sessionFrom(r.Context()).User.ID, id); err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"read": true})
}

func (h *Handler) markAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if err := h.service.MarkAllNotificationsRead(r.Context(), sessionFrom(r.Context()).User.ID); err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]bool{"read": true})
}
