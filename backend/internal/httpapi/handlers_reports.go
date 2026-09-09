package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gvideo/backend/internal/domain"
)

func (h *Handler) reportVideo(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	report, err := h.service.ReportVideo(r.Context(), sessionFrom(r.Context()).User.ID, id, input.Reason, input.Detail)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusCreated, report)
}

func (h *Handler) adminReports(w http.ResponseWriter, r *http.Request) {
	page, pageSize := pagination(r, 20)
	result, err := h.service.AdminVideoReports(r.Context(), sessionFrom(r.Context()).User.Username, r.URL.Query().Get("status"), page, pageSize)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, result)
}

func (h *Handler) reviewReport(w http.ResponseWriter, r *http.Request) {
	reportID, err := strconv.ParseInt(chi.URLParam(r, "reportID"), 10, 64)
	if err != nil || reportID <= 0 {
		h.writeError(w, r, domain.ErrInvalidInput)
		return
	}
	var input struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	report, err := h.service.ReviewVideoReport(r.Context(), sessionFrom(r.Context()).User.Username, reportID, input.Status)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, report)
}
