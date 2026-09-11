package notifications

import (
	"context"
	"net/http"

	"gvideo/backend/internal/domain"
)

// Principal is the authentication data notifications need from the HTTP layer.
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
	NotificationID(http.ResponseWriter, *http.Request) (int64, bool)
	Pagination(*http.Request, int) (int, int)
}

type notificationService interface {
	Notifications(context.Context, int64, int, int) (domain.NotificationPage, error)
	MarkNotificationRead(context.Context, int64, int64) error
	MarkAllNotificationsRead(context.Context, int64) error
}

// Handler implements the notification HTTP endpoints through an injected protocol port.
type Handler struct {
	service notificationService
	http    HTTPPort
}

func NewHandler(service notificationService, httpPort HTTPPort) *Handler {
	return &Handler{service: service, http: httpPort}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, pageSize := h.http.Pagination(r, 20)
	principal := h.http.Principal(r.Context())
	result, err := h.service.Notifications(r.Context(), principal.UserID, page, pageSize)
	if err != nil {
		h.http.WriteError(w, r, err)
		return
	}
	h.http.WriteJSON(w, r, http.StatusOK, result)
}

func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	notificationID, ok := h.http.NotificationID(w, r)
	if !ok {
		return
	}
	principal := h.http.Principal(r.Context())
	if err := h.service.MarkNotificationRead(r.Context(), principal.UserID, notificationID); err != nil {
		h.http.WriteError(w, r, err)
		return
	}
	h.http.WriteJSON(w, r, http.StatusOK, map[string]bool{"read": true})
}

func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	principal := h.http.Principal(r.Context())
	if err := h.service.MarkAllNotificationsRead(r.Context(), principal.UserID); err != nil {
		h.http.WriteError(w, r, err)
		return
	}
	h.http.WriteJSON(w, r, http.StatusOK, map[string]bool{"read": true})
}
