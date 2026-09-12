package interactions

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"gvideo/backend/internal/domain"
)

var (
	errInvalidVideoID = errors.New("invalid video id")
	errInvalidUserID  = errors.New("invalid user id")
)

func TestHandlerToggleLikeAndFavorite(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{active: true}
	handler := NewHandler(svc, port)

	likeRes := httptest.NewRecorder()
	handler.Like(likeRes, withRoute(httptest.NewRequest(http.MethodPost, "/api/v1/videos/5/like", nil), "videoID", "5"))
	if port.status != http.StatusOK || !svc.likeCalled || svc.likeUserID != 7 || svc.likeVideoID != 5 {
		t.Fatalf("like status=%d service=%#v", port.status, svc)
	}
	if active, ok := port.data.(map[string]bool); !ok || !active["active"] {
		t.Fatalf("written data = %#v", port.data)
	}

	favoriteRes := httptest.NewRecorder()
	handler.Favorite(favoriteRes, withRoute(httptest.NewRequest(http.MethodPost, "/api/v1/videos/5/favorite", nil), "videoID", "5"))
	if port.status != http.StatusOK || !svc.favoriteCalled || svc.favoriteUserID != 7 || svc.favoriteVideoID != 5 {
		t.Fatalf("favorite status=%d service=%#v", port.status, svc)
	}
}

func TestHandlerFollow(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{}
	handler := NewHandler(svc, port)
	handler.Follow(httptest.NewRecorder(), withRoute(httptest.NewRequest(http.MethodPost, "/api/v1/users/4/follow", nil), "userID", "4"))
	if port.status != http.StatusOK || !svc.followCalled || svc.followFollowerID != 7 || svc.followedID != 4 {
		t.Fatalf("follow status=%d service=%#v", port.status, svc)
	}
	if active, ok := port.data.(map[string]bool); !ok || active["active"] {
		t.Fatalf("written data = %#v", port.data)
	}
}

func TestHandlerRejectsInvalidVideoID(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{}
	handler := NewHandler(svc, port)
	handler.Like(httptest.NewRecorder(), withRoute(httptest.NewRequest(http.MethodPost, "/api/v1/videos/nope/like", nil), "videoID", "nope"))
	handler.Favorite(httptest.NewRecorder(), withRoute(httptest.NewRequest(http.MethodPost, "/api/v1/videos/nope/favorite", nil), "videoID", "nope"))
	if !errors.Is(port.err, errInvalidVideoID) || svc.likeCalled || svc.favoriteCalled {
		t.Fatalf("error = %v service=%#v", port.err, svc)
	}
}

func TestHandlerRejectsInvalidUserID(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{}
	handler := NewHandler(svc, port)
	handler.Follow(httptest.NewRecorder(), withRoute(httptest.NewRequest(http.MethodPost, "/api/v1/users/nope/follow", nil), "userID", "nope"))
	if !errors.Is(port.err, errInvalidUserID) || svc.followCalled {
		t.Fatalf("error = %v service=%#v", port.err, svc)
	}
}

func TestHandlerPropagatesServiceErrors(t *testing.T) {
	port := &handlerPortStub{principal: Principal{UserID: 7}}
	svc := &handlerServiceStub{likeErr: domain.ErrNotFound, followErr: domain.ErrForbidden}
	handler := NewHandler(svc, port)
	handler.Like(httptest.NewRecorder(), withRoute(httptest.NewRequest(http.MethodPost, "/api/v1/videos/5/like", nil), "videoID", "5"))
	if !errors.Is(port.err, domain.ErrNotFound) {
		t.Fatalf("like error = %v", port.err)
	}
	handler.Follow(httptest.NewRecorder(), withRoute(httptest.NewRequest(http.MethodPost, "/api/v1/users/4/follow", nil), "userID", "4"))
	if !errors.Is(port.err, domain.ErrForbidden) {
		t.Fatalf("follow error = %v", port.err)
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
	active                          bool
	likeCalled                      bool
	likeErr                         error
	likeUserID, likeVideoID         int64
	favoriteCalled                  bool
	favoriteErr                     error
	favoriteUserID, favoriteVideoID int64
	followCalled                    bool
	followErr                       error
	followFollowerID, followedID    int64
}

func (s *handlerServiceStub) ToggleLike(_ context.Context, userID, videoID int64) (bool, error) {
	s.likeCalled, s.likeUserID, s.likeVideoID = true, userID, videoID
	return s.active, s.likeErr
}

func (s *handlerServiceStub) ToggleFavorite(_ context.Context, userID, videoID int64) (bool, error) {
	s.favoriteCalled, s.favoriteUserID, s.favoriteVideoID = true, userID, videoID
	return s.active, s.favoriteErr
}

func (s *handlerServiceStub) ToggleFollow(_ context.Context, followerID, followedID int64) (bool, error) {
	s.followCalled, s.followFollowerID, s.followedID = true, followerID, followedID
	return s.active, s.followErr
}

type handlerPortStub struct {
	principal Principal
	status    int
	data      any
	err       error
}

func (p *handlerPortStub) Principal(context.Context) Principal { return p.principal }
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
func (p *handlerPortStub) UserID(_ http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
	if err != nil || id <= 0 {
		p.err = errInvalidUserID
		return 0, false
	}
	return id, true
}
