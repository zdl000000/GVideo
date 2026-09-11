package moderation

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gvideo/backend/internal/domain"
)

// Principal is the authentication data moderation needs from the HTTP layer.
type Principal struct {
	UserID   int64
	Username string
	IsAdmin  bool
}

// HTTPPort keeps protocol envelopes, request IDs, decoding, and error mapping
// owned by the application HTTP layer.
type HTTPPort interface {
	Principal(context.Context) Principal
	DecodeJSON(http.ResponseWriter, *http.Request, any) bool
	WriteJSON(http.ResponseWriter, *http.Request, int, any)
	WriteError(http.ResponseWriter, *http.Request, error)
	VideoID(http.ResponseWriter, *http.Request) (int64, bool)
	Pagination(*http.Request, int) (int, int)
}

type reportService interface {
	ReportVideo(context.Context, int64, int64, string, string) (domain.VideoReport, error)
	AdminVideoReports(context.Context, bool, string, int, int) (domain.VideoReportPage, error)
	ReviewVideoReport(context.Context, bool, int64, string) (domain.VideoReport, error)
}

// Handler implements the report HTTP endpoints through an injected protocol port.
type Handler struct {
	service reportService
	http    HTTPPort
}

func NewHandler(service reportService, httpPort HTTPPort) *Handler {
	return &Handler{service: service, http: httpPort}
}

func (h *Handler) ReportVideo(w http.ResponseWriter, r *http.Request) {
	videoID, ok := h.http.VideoID(w, r)
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}
	if !h.http.DecodeJSON(w, r, &input) {
		return
	}
	principal := h.http.Principal(r.Context())
	report, err := h.service.ReportVideo(r.Context(), principal.UserID, videoID, input.Reason, input.Detail)
	if err != nil {
		h.http.WriteError(w, r, err)
		return
	}
	h.http.WriteJSON(w, r, http.StatusCreated, report)
}

func (h *Handler) AdminReports(w http.ResponseWriter, r *http.Request) {
	page, pageSize := h.http.Pagination(r, 20)
	principal := h.http.Principal(r.Context())
	result, err := h.service.AdminVideoReports(r.Context(), principal.IsAdmin, r.URL.Query().Get("status"), page, pageSize)
	if err != nil {
		h.http.WriteError(w, r, err)
		return
	}
	h.http.WriteJSON(w, r, http.StatusOK, result)
}

func (h *Handler) ReviewReport(w http.ResponseWriter, r *http.Request) {
	reportID, err := strconv.ParseInt(chi.URLParam(r, "reportID"), 10, 64)
	if err != nil || reportID <= 0 {
		h.http.WriteError(w, r, domain.ErrInvalidInput)
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if !h.http.DecodeJSON(w, r, &input) {
		return
	}
	principal := h.http.Principal(r.Context())
	report, err := h.service.ReviewVideoReport(r.Context(), principal.IsAdmin, reportID, input.Status)
	if err != nil {
		h.http.WriteError(w, r, err)
		return
	}
	h.http.WriteJSON(w, r, http.StatusOK, report)
}
