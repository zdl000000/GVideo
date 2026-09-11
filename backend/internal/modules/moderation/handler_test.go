package moderation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"gvideo/backend/internal/domain"
)

func TestHandlerReportVideo(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}, videoID: 9}
	svc := &handlerServiceStub{report: domain.VideoReport{ID: 3, VideoID: 9, UserID: 7, Status: "pending"}}
	handler := NewHandler(svc, port)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/videos/9/reports", bytes.NewBufferString(`{"reason":"spam","detail":"detail"}`))
	res := httptest.NewRecorder()
	handler.ReportVideo(res, req)
	if port.status != http.StatusCreated || svc.userID != 7 || svc.videoID != 9 || svc.reason != "spam" {
		t.Fatalf("status=%d service=%#v", port.status, svc)
	}
}

func TestHandlerReviewRejectsInvalidID(t *testing.T) {
	port := &handlerPortStub{principal: Principal{Username: "admin"}}
	svc := &handlerServiceStub{}
	handler := NewHandler(svc, port)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/reports/nope", bytes.NewBufferString(`{"status":"resolved"}`))
	route := chi.NewRouteContext()
	route.URLParams.Add("reportID", "nope")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	handler.ReviewReport(httptest.NewRecorder(), req)
	if !errors.Is(port.err, domain.ErrInvalidInput) {
		t.Fatalf("error = %v", port.err)
	}
}

type handlerServiceStub struct {
	report          domain.VideoReport
	userID, videoID int64
	reason, detail  string
}

func (s *handlerServiceStub) ReportVideo(_ context.Context, userID, videoID int64, reason, detail string) (domain.VideoReport, error) {
	s.userID, s.videoID, s.reason, s.detail = userID, videoID, reason, detail
	return s.report, nil
}
func (*handlerServiceStub) AdminVideoReports(context.Context, bool, string, int, int) (domain.VideoReportPage, error) {
	return domain.VideoReportPage{}, nil
}
func (*handlerServiceStub) ReviewVideoReport(context.Context, bool, int64, string) (domain.VideoReport, error) {
	return domain.VideoReport{}, nil
}

type handlerPortStub struct {
	principal Principal
	videoID   int64
	status    int
	data      any
	err       error
}

func (p *handlerPortStub) Principal(context.Context) Principal { return p.principal }
func (*handlerPortStub) DecodeJSON(_ http.ResponseWriter, r *http.Request, target any) bool {
	return json.NewDecoder(r.Body).Decode(target) == nil
}
func (p *handlerPortStub) WriteJSON(_ http.ResponseWriter, _ *http.Request, status int, data any) {
	p.status, p.data = status, data
}
func (p *handlerPortStub) WriteError(_ http.ResponseWriter, _ *http.Request, err error) { p.err = err }
func (p *handlerPortStub) VideoID(http.ResponseWriter, *http.Request) (int64, bool) {
	return p.videoID, p.videoID > 0
}
func (*handlerPortStub) Pagination(*http.Request, int) (int, int) { return 1, 20 }
