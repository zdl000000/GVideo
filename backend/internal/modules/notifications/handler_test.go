package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"gvideo/backend/internal/domain"
)

var errInvalidNotificationID = errors.New("invalid notification id")

func TestHandlerListNotifications(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}, page: 2, pageSize: 24}
	svc := &handlerServiceStub{page: domain.NotificationPage{Total: 3, UnreadCount: 1}}
	handler := NewHandler(svc, port)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/notifications?page=2&page_size=24", nil)
	res := httptest.NewRecorder()
	handler.List(res, req)
	if port.status != http.StatusOK || !svc.listCalled || svc.userID != 7 || svc.listPage != 2 || svc.listPageSize != 24 {
		t.Fatalf("status=%d service=%#v", port.status, svc)
	}
	if page, ok := port.data.(domain.NotificationPage); !ok || page.Total != 3 {
		t.Fatalf("written data = %#v", port.data)
	}
}

func TestHandlerMarkReadRejectsInvalidID(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{}
	handler := NewHandler(svc, port)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me/notifications/nope/read", nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("notificationID", "nope")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	handler.MarkRead(httptest.NewRecorder(), req)
	if !errors.Is(port.err, errInvalidNotificationID) || svc.markCalled {
		t.Fatalf("error = %v service=%#v", port.err, svc)
	}
}

func TestHandlerMarkReadAndMarkAllRead(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{}
	handler := NewHandler(svc, port)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me/notifications/5/read", nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("notificationID", "5")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	handler.MarkRead(httptest.NewRecorder(), req)
	if port.status != http.StatusOK || !svc.markCalled || svc.markUserID != 7 || svc.markNotificationID != 5 {
		t.Fatalf("status=%d service=%#v", port.status, svc)
	}
	if read, ok := port.data.(map[string]bool); !ok || !read["read"] {
		t.Fatalf("written data = %#v", port.data)
	}
	handler.MarkAllRead(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/me/notifications/read-all", nil))
	if port.status != http.StatusOK || !svc.markAllCalled || svc.markAllUserID != 7 {
		t.Fatalf("mark all status=%d service=%#v", port.status, svc)
	}
}

func TestHandlerPropagatesServiceErrors(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{listErr: domain.ErrForbidden}
	handler := NewHandler(svc, port)
	handler.List(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/me/notifications", nil))
	if !errors.Is(port.err, domain.ErrForbidden) {
		t.Fatalf("error = %v", port.err)
	}
}

type handlerServiceStub struct {
	page                                          domain.NotificationPage
	listErr                                       error
	userID                                        int64
	listPage, listPageSize                        int
	listCalled, markCalled, markAllCalled         bool
	markUserID, markNotificationID, markAllUserID int64
}

func (s *handlerServiceStub) Notifications(_ context.Context, userID int64, page, pageSize int) (domain.NotificationPage, error) {
	s.listCalled, s.userID, s.listPage, s.listPageSize = true, userID, page, pageSize
	return s.page, s.listErr
}

func (s *handlerServiceStub) MarkNotificationRead(_ context.Context, userID, notificationID int64) error {
	s.markCalled, s.markUserID, s.markNotificationID = true, userID, notificationID
	return nil
}

func (s *handlerServiceStub) MarkAllNotificationsRead(_ context.Context, userID int64) error {
	s.markAllCalled, s.markAllUserID = true, userID
	return nil
}

type handlerPortStub struct {
	principal      Principal
	page, pageSize int
	status         int
	data           any
	err            error
}

func (p *handlerPortStub) Principal(context.Context) Principal { return p.principal }
func (*handlerPortStub) DecodeJSON(_ http.ResponseWriter, r *http.Request, target any) bool {
	return json.NewDecoder(r.Body).Decode(target) == nil
}
func (p *handlerPortStub) WriteJSON(_ http.ResponseWriter, _ *http.Request, status int, data any) {
	p.status, p.data = status, data
}
func (p *handlerPortStub) WriteError(_ http.ResponseWriter, _ *http.Request, err error) { p.err = err }
func (p *handlerPortStub) NotificationID(_ http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "notificationID"), 10, 64)
	if err != nil || id <= 0 {
		p.err = errInvalidNotificationID
		return 0, false
	}
	return id, true
}
func (p *handlerPortStub) Pagination(*http.Request, int) (int, int) { return p.page, p.pageSize }
