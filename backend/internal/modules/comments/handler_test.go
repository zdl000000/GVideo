package comments

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"gvideo/backend/internal/domain"
)

var (
	errInvalidVideoID   = errors.New("invalid video id")
	errInvalidCommentID = errors.New("invalid comment id")
)

func TestHandlerListComments(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{comments: []domain.Comment{{ID: 3, Content: "接口流程正常"}}}
	handler := NewHandler(svc, port)
	req := withRoute(httptest.NewRequest(http.MethodGet, "/api/v1/videos/5/comments", nil), "videoID", "5")
	res := httptest.NewRecorder()
	handler.List(res, req)
	if port.status != http.StatusOK || !svc.listCalled || svc.listVideoID != 5 || svc.listViewerID != 7 {
		t.Fatalf("status=%d service=%#v", port.status, svc)
	}
	comments, ok := port.data.([]domain.Comment)
	if !ok || len(comments) != 1 || comments[0].Content != "接口流程正常" {
		t.Fatalf("written data = %#v", port.data)
	}
}

func TestHandlerListNormalizesNilComments(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{}
	handler := NewHandler(svc, port)
	handler.List(httptest.NewRecorder(), withRoute(httptest.NewRequest(http.MethodGet, "/api/v1/videos/5/comments", nil), "videoID", "5"))
	if comments, ok := port.data.([]domain.Comment); !ok || comments == nil {
		t.Fatalf("nil comments must be written as an empty JSON array, got %#v", port.data)
	}
}

func TestHandlerCreateAndDeleteComments(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{createdComment: domain.Comment{ID: 9, UserID: 7, Content: "hello"}}
	handler := NewHandler(svc, port)

	createReq := withRoute(httptest.NewRequest(http.MethodPost, "/api/v1/videos/5/comments", strings.NewReader(`{"content":"hello"}`)), "videoID", "5")
	createRes := httptest.NewRecorder()
	handler.Create(createRes, createReq)
	if port.status != http.StatusCreated || !svc.createCalled || svc.createUserID != 7 || svc.createVideoID != 5 || svc.createContent != "hello" {
		t.Fatalf("create status=%d service=%#v", port.status, svc)
	}
	if comment, ok := port.data.(domain.Comment); !ok || comment.ID != 9 {
		t.Fatalf("written data = %#v", port.data)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/videos/5/comments/9", nil)
	deleteReq = withRoute(deleteReq, "videoID", "5", "commentID", "9")
	handler.Delete(httptest.NewRecorder(), deleteReq)
	if port.status != http.StatusOK || !svc.deleteCalled || svc.deleteUserID != 7 || svc.deleteVideoID != 5 || svc.deleteCommentID != 9 {
		t.Fatalf("delete status=%d service=%#v", port.status, svc)
	}
	if deleted, ok := port.data.(map[string]bool); !ok || !deleted["deleted"] {
		t.Fatalf("written data = %#v", port.data)
	}
}

func TestHandlerRejectsInvalidVideoID(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{}
	handler := NewHandler(svc, port)
	req := withRoute(httptest.NewRequest(http.MethodGet, "/api/v1/videos/nope/comments", nil), "videoID", "nope")
	handler.List(httptest.NewRecorder(), req)
	if !errors.Is(port.err, errInvalidVideoID) || svc.listCalled || svc.createCalled || svc.deleteCalled {
		t.Fatalf("error = %v service=%#v", port.err, svc)
	}
}

func TestHandlerRejectsInvalidCommentID(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{}
	handler := NewHandler(svc, port)
	req := withRoute(httptest.NewRequest(http.MethodDelete, "/api/v1/videos/5/comments/nope", nil), "videoID", "5", "commentID", "nope")
	handler.Delete(httptest.NewRecorder(), req)
	if !errors.Is(port.err, errInvalidCommentID) || svc.deleteCalled {
		t.Fatalf("error = %v service=%#v", port.err, svc)
	}
}

func TestHandlerPropagatesServiceErrors(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{listErr: domain.ErrForbidden}
	handler := NewHandler(svc, port)
	handler.List(httptest.NewRecorder(), withRoute(httptest.NewRequest(http.MethodGet, "/api/v1/videos/5/comments", nil), "videoID", "5"))
	if !errors.Is(port.err, domain.ErrForbidden) {
		t.Fatalf("error = %v", port.err)
	}
}

func withRoute(req *http.Request, pairs ...string) *http.Request {
	route := chi.NewRouteContext()
	for index := 0; index+1 < len(pairs); index += 2 {
		route.URLParams.Add(pairs[index], pairs[index+1])
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
}

type handlerServiceStub struct {
	comments                    []domain.Comment
	listErr                     error
	listCalled                  bool
	listVideoID, listViewerID   int64
	createdComment              domain.Comment
	createErr                   error
	createCalled                bool
	createUserID, createVideoID int64
	createContent               string
	deleteCalled                bool
	deleteErr                   error
	deleteUserID, deleteVideoID int64
	deleteCommentID             int64
}

func (s *handlerServiceStub) Comments(_ context.Context, videoID, viewerID int64) ([]domain.Comment, error) {
	s.listCalled, s.listVideoID, s.listViewerID = true, videoID, viewerID
	return s.comments, s.listErr
}

func (s *handlerServiceStub) CreateComment(_ context.Context, userID, videoID int64, content string) (domain.Comment, error) {
	s.createCalled, s.createUserID, s.createVideoID, s.createContent = true, userID, videoID, content
	return s.createdComment, s.createErr
}

func (s *handlerServiceStub) DeleteComment(_ context.Context, userID, videoID, commentID int64) error {
	s.deleteCalled, s.deleteUserID, s.deleteVideoID, s.deleteCommentID = true, userID, videoID, commentID
	return s.deleteErr
}

type handlerPortStub struct {
	principal Principal
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
func (p *handlerPortStub) VideoID(_ http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "videoID"), 10, 64)
	if err != nil || id <= 0 {
		p.err = errInvalidVideoID
		return 0, false
	}
	return id, true
}
func (p *handlerPortStub) CommentID(_ http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "commentID"), 10, 64)
	if err != nil || id <= 0 {
		p.err = errInvalidCommentID
		return 0, false
	}
	return id, true
}
