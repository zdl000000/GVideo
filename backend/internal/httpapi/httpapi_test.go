package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/modules/moderation"
	"gvideo/backend/internal/modules/notifications"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
	"gvideo/backend/internal/service"
)

type testEnvelope struct {
	Data      json.RawMessage `json:"data"`
	Error     string          `json:"error"`
	RequestID string          `json:"request_id"`
}

func TestRootEndpoint(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{FrontendURL: "https://video.example.test", MediaDir: filepath.Join(dir, "media"), SessionTTL: time.Hour}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := platform.OpenDatabase(filepath.Join(dir, "root.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	handler := New(service.New(repository.New(db), cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notifications.NewRepository(db)), cfg, logger).Routes()

	result := httptest.NewRecorder()
	handler.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/", nil))
	if result.Code != http.StatusOK {
		t.Fatalf("root status=%d body=%s", result.Code, result.Body.String())
	}
	var envelope testEnvelope
	decodeResponse(t, result, &envelope)
	var payload map[string]string
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatalf("decode root payload: %v data=%s", err, envelope.Data)
	}
	want := map[string]string{
		"service":  "gvideo-backend",
		"status":   "ok",
		"frontend": "https://video.example.test",
		"health":   "/healthz",
	}
	for key, value := range want {
		if payload[key] != value {
			t.Fatalf("root payload[%q]=%q, want %q; payload=%#v", key, payload[key], value, payload)
		}
	}
}

func TestResponseHeadersAndStrictJSON(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{MediaDir: filepath.Join(dir, "media"), SessionTTL: time.Hour}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db, err := platform.OpenDatabase(filepath.Join(dir, "headers.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	handler := New(service.New(repository.New(db), cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notifications.NewRepository(db)), cfg, logger).Routes()

	categories := httptest.NewRecorder()
	handler.ServeHTTP(categories, httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil))
	if categories.Code != http.StatusOK || categories.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("categories status=%d nosniff=%q", categories.Code, categories.Header().Get("X-Content-Type-Options"))
	}
	if categories.Header().Get("Cache-Control") != "" {
		t.Fatalf("public response unexpectedly disabled caching: %q", categories.Header().Get("Cache-Control"))
	}

	me := httptest.NewRecorder()
	handler.ServeHTTP(me, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if me.Code != http.StatusUnauthorized || me.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("me status=%d cache-control=%q", me.Code, me.Header().Get("Cache-Control"))
	}

	register := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(
		`{"username":"strict_json_user","password":"password123"}{"unexpected":true}`,
	))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(register, request)
	if register.Code != http.StatusBadRequest || !strings.Contains(register.Body.String(), "只能包含一个 JSON 对象") {
		t.Fatalf("trailing JSON status=%d body=%s", register.Code, register.Body.String())
	}
}

func TestCoreVideoFlow(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	cfg := config.Config{
		MediaDir:       filepath.Join(dir, "media"),
		SessionTTL:     time.Hour,
		MaxUploadBytes: 10 << 20,
		FFmpegPath:     "missing-ffmpeg-for-test",
		FFprobePath:    "missing-ffprobe-for-test",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(service.New(repository.New(db), cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notifications.NewRepository(db)), cfg, logger).Routes()

	registerBody := strings.NewReader(`{"username":"api_user","password":"password123"}`)
	register := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", registerBody)
	register.Header.Set("Content-Type", "application/json")
	registerResult := httptest.NewRecorder()
	handler.ServeHTTP(registerResult, register)
	if registerResult.Code != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", registerResult.Code, registerResult.Body.String())
	}
	cookie := registerResult.Result().Cookies()[0]
	var registered testEnvelope
	decodeResponse(t, registerResult, &registered)
	var auth struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(registered.Data, &auth); err != nil || auth.CSRFToken == "" {
		t.Fatalf("read auth payload: %v data=%s", err, registered.Data)
	}

	var uploadBody bytes.Buffer
	writer := multipart.NewWriter(&uploadBody)
	_ = writer.WriteField("title", "HTTP 流程测试视频")
	_ = writer.WriteField("description", "穿过协议层、服务层和数据层")
	_ = writer.WriteField("category", "知识")
	videoPart, err := writer.CreateFormFile("video", "sample.mp4")
	if err != nil {
		t.Fatal(err)
	}
	// The standard library's MIME sniffer recognizes this minimal MP4 signature.
	_, _ = videoPart.Write([]byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom<\x06t\xbfmdat"))
	_ = writer.WriteField("subtitle_language", "zh-CN")
	_ = writer.WriteField("subtitle_label", "中文")
	subtitlePart, err := writer.CreateFormFile("subtitle", "captions.srt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = subtitlePart.Write([]byte("1\n00:00:00,000 --> 00:00:02,000\n接口字幕测试\n"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	upload := httptest.NewRequest(http.MethodPost, "/api/v1/videos", &uploadBody)
	upload.Header.Set("Content-Type", writer.FormDataContentType())
	upload.Header.Set("X-CSRF-Token", auth.CSRFToken)
	upload.AddCookie(cookie)
	uploadResult := httptest.NewRecorder()
	handler.ServeHTTP(uploadResult, upload)
	if uploadResult.Code != http.StatusCreated {
		t.Fatalf("upload status=%d body=%s", uploadResult.Code, uploadResult.Body.String())
	}
	var uploaded testEnvelope
	decodeResponse(t, uploadResult, &uploaded)
	var video struct {
		ID                 int64  `json:"id"`
		VideoURL           string `json:"video_url"`
		ProcessingProgress int    `json:"processing_progress"`
		ProcessingStage    string `json:"processing_stage"`
		SubtitleTracks     []struct {
			Language  string `json:"language"`
			Label     string `json:"label"`
			URL       string `json:"url"`
			IsDefault bool   `json:"is_default"`
		} `json:"subtitle_tracks"`
	}
	if err := json.Unmarshal(uploaded.Data, &video); err != nil || video.ID == 0 || video.VideoURL == "" {
		t.Fatalf("read uploaded video: %v data=%s", err, uploaded.Data)
	}
	if video.ProcessingProgress != 0 || video.ProcessingStage != "queued" {
		t.Fatalf("unexpected upload processing state: %d/%q", video.ProcessingProgress, video.ProcessingStage)
	}
	if len(video.SubtitleTracks) != 1 || video.SubtitleTracks[0].Language != "zh-CN" || !video.SubtitleTracks[0].IsDefault {
		t.Fatalf("unexpected subtitle tracks: %#v", video.SubtitleTracks)
	}
	subtitle := httptest.NewRequest(http.MethodGet, video.SubtitleTracks[0].URL, nil)
	subtitleResult := httptest.NewRecorder()
	handler.ServeHTTP(subtitleResult, subtitle)
	if subtitleResult.Code != http.StatusOK || !strings.Contains(subtitleResult.Header().Get("Content-Type"), "text/vtt") ||
		!strings.Contains(subtitleResult.Body.String(), "WEBVTT") || !strings.Contains(subtitleResult.Body.String(), "00:00:00.000 --> 00:00:02.000") {
		t.Fatalf("subtitle status=%d content-type=%q body=%q", subtitleResult.Code, subtitleResult.Header().Get("Content-Type"), subtitleResult.Body.String())
	}

	videoID := strconv.FormatInt(video.ID, 10)
	like := authorizedRequest(http.MethodPost, "/api/v1/videos/"+videoID+"/like", nil, cookie, auth.CSRFToken)
	likeResult := httptest.NewRecorder()
	handler.ServeHTTP(likeResult, like)
	if likeResult.Code != http.StatusOK || !strings.Contains(likeResult.Body.String(), `"active":true`) {
		t.Fatalf("like status=%d body=%s", likeResult.Code, likeResult.Body.String())
	}

	favorite := authorizedRequest(http.MethodPost, "/api/v1/videos/"+videoID+"/favorite", nil, cookie, auth.CSRFToken)
	favoriteResult := httptest.NewRecorder()
	handler.ServeHTTP(favoriteResult, favorite)
	if favoriteResult.Code != http.StatusOK || !strings.Contains(favoriteResult.Body.String(), `"active":true`) {
		t.Fatalf("favorite status=%d body=%s", favoriteResult.Code, favoriteResult.Body.String())
	}

	comment := authorizedRequest(http.MethodPost, "/api/v1/videos/"+videoID+"/comments", strings.NewReader(`{"content":"接口流程正常"}`), cookie, auth.CSRFToken)
	comment.Header.Set("Content-Type", "application/json")
	commentResult := httptest.NewRecorder()
	handler.ServeHTTP(commentResult, comment)
	if commentResult.Code != http.StatusCreated {
		t.Fatalf("comment status=%d body=%s", commentResult.Code, commentResult.Body.String())
	}

	comments := httptest.NewRequest(http.MethodGet, "/api/v1/videos/"+videoID+"/comments", nil)
	commentsResult := httptest.NewRecorder()
	handler.ServeHTTP(commentsResult, comments)
	if commentsResult.Code != http.StatusOK || !strings.Contains(commentsResult.Body.String(), `"content":"接口流程正常"`) {
		t.Fatalf("comments status=%d body=%s", commentsResult.Code, commentsResult.Body.String())
	}

	detail := authorizedRequest(http.MethodGet, "/api/v1/videos/"+videoID+"?count_view=false", nil, cookie, "")
	detailResult := httptest.NewRecorder()
	handler.ServeHTTP(detailResult, detail)
	if detailResult.Code != http.StatusOK {
		t.Fatalf("detail status=%d body=%s", detailResult.Code, detailResult.Body.String())
	}
	var detailed testEnvelope
	decodeResponse(t, detailResult, &detailed)
	var current struct {
		LikesCount     int64 `json:"likes_count"`
		FavoritesCount int64 `json:"favorites_count"`
		CommentsCount  int64 `json:"comments_count"`
		Liked          bool  `json:"liked"`
		Favorited      bool  `json:"favorited"`
	}
	if err := json.Unmarshal(detailed.Data, &current); err != nil {
		t.Fatalf("decode video detail: %v data=%s", err, detailed.Data)
	}
	if current.LikesCount != 1 || current.FavoritesCount != 1 || current.CommentsCount != 1 || !current.Liked || !current.Favorited {
		t.Fatalf("unexpected interaction state: %#v", current)
	}

	media := httptest.NewRequest(http.MethodGet, video.VideoURL, nil)
	mediaResult := httptest.NewRecorder()
	handler.ServeHTTP(mediaResult, media)
	if mediaResult.Code != http.StatusOK || mediaResult.Body.Len() == 0 {
		t.Fatalf("media status=%d bytes=%d", mediaResult.Code, mediaResult.Body.Len())
	}

	rangeRequest := httptest.NewRequest(http.MethodGet, video.VideoURL, nil)
	rangeRequest.Header.Set("Range", "bytes=0-7")
	rangeResult := httptest.NewRecorder()
	handler.ServeHTTP(rangeResult, rangeRequest)
	if rangeResult.Code != http.StatusPartialContent || rangeResult.Body.Len() != 8 || rangeResult.Header().Get("Content-Range") == "" {
		t.Fatalf("media range status=%d bytes=%d content-range=%q", rangeResult.Code, rangeResult.Body.Len(), rangeResult.Header().Get("Content-Range"))
	}

	list := httptest.NewRequest(http.MethodGet, "/api/v1/videos?page=1&page_size=1", nil)
	listResult := httptest.NewRecorder()
	handler.ServeHTTP(listResult, list)
	if listResult.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", listResult.Code, listResult.Body.String())
	}
	var listed testEnvelope
	decodeResponse(t, listResult, &listed)
	var page struct {
		Items    []json.RawMessage `json:"items"`
		Page     int               `json:"page"`
		PageSize int               `json:"page_size"`
		Total    int64             `json:"total"`
		HasNext  bool              `json:"has_next"`
	}
	if err := json.Unmarshal(listed.Data, &page); err != nil || len(page.Items) != 1 || page.Page != 1 || page.PageSize != 1 || page.Total != 1 || page.HasNext {
		t.Fatalf("unexpected pagination: %#v err=%v data=%s", page, err, listed.Data)
	}

	otherRegister := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"other_user","password":"password123"}`))
	otherRegister.Header.Set("Content-Type", "application/json")
	otherRegisterResult := httptest.NewRecorder()
	handler.ServeHTTP(otherRegisterResult, otherRegister)
	if otherRegisterResult.Code != http.StatusCreated {
		t.Fatalf("register other status=%d body=%s", otherRegisterResult.Code, otherRegisterResult.Body.String())
	}
	otherCookie := otherRegisterResult.Result().Cookies()[0]
	var otherRegistered testEnvelope
	decodeResponse(t, otherRegisterResult, &otherRegistered)
	var otherAuth struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.Unmarshal(otherRegistered.Data, &otherAuth); err != nil || otherAuth.CSRFToken == "" {
		t.Fatalf("read other auth payload: %v data=%s", err, otherRegistered.Data)
	}

	forbiddenBody, forbiddenContentType := videoEditBody(t, "越权编辑", "无权修改", "科技")
	forbiddenEdit := authorizedRequest(http.MethodPatch, "/api/v1/videos/"+videoID, forbiddenBody, otherCookie, otherAuth.CSRFToken)
	forbiddenEdit.Header.Set("Content-Type", forbiddenContentType)
	forbiddenResult := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenResult, forbiddenEdit)
	if forbiddenResult.Code != http.StatusForbidden {
		t.Fatalf("forbidden edit status=%d body=%s", forbiddenResult.Code, forbiddenResult.Body.String())
	}

	editBody, editContentType := videoEditBody(t, "HTTP 已编辑视频", "编辑接口已通过", "科技")
	edit := authorizedRequest(http.MethodPatch, "/api/v1/videos/"+videoID, editBody, cookie, auth.CSRFToken)
	edit.Header.Set("Content-Type", editContentType)
	editResult := httptest.NewRecorder()
	handler.ServeHTTP(editResult, edit)
	if editResult.Code != http.StatusOK || !strings.Contains(editResult.Body.String(), `"title":"HTTP 已编辑视频"`) || !strings.Contains(editResult.Body.String(), `"category":"科技"`) {
		t.Fatalf("edit status=%d body=%s", editResult.Code, editResult.Body.String())
	}

	remove := authorizedRequest(http.MethodDelete, "/api/v1/videos/"+videoID, nil, cookie, auth.CSRFToken)
	removeResult := httptest.NewRecorder()
	handler.ServeHTTP(removeResult, remove)
	if removeResult.Code != http.StatusOK || !strings.Contains(removeResult.Body.String(), `"deleted":true`) {
		t.Fatalf("delete status=%d body=%s", removeResult.Code, removeResult.Body.String())
	}

	deletedDetail := httptest.NewRequest(http.MethodGet, "/api/v1/videos/"+videoID+"?count_view=false", nil)
	deletedDetailResult := httptest.NewRecorder()
	handler.ServeHTTP(deletedDetailResult, deletedDetail)
	if deletedDetailResult.Code != http.StatusNotFound {
		t.Fatalf("deleted detail status=%d body=%s", deletedDetailResult.Code, deletedDetailResult.Body.String())
	}
	deletedMedia := httptest.NewRequest(http.MethodGet, video.VideoURL, nil)
	deletedMediaResult := httptest.NewRecorder()
	handler.ServeHTTP(deletedMediaResult, deletedMedia)
	if deletedMediaResult.Code != http.StatusNotFound {
		t.Fatalf("deleted media status=%d body=%s", deletedMediaResult.Code, deletedMediaResult.Body.String())
	}
}

type testAuthClient struct {
	User      domain.User
	Cookie    *http.Cookie
	CSRFToken string
}

func TestCreatorSpaceAndFollowingHTTPFlow(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Config{MediaDir: filepath.Join(dir, "media"), SessionTTL: time.Hour}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := repository.New(db)
	handler := New(service.New(repo, cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notifications.NewRepository(db)), cfg, logger).Routes()

	viewer := registerTestUser(t, handler, "following_viewer")
	author := registerTestUser(t, handler, "following_author")
	other := registerTestUser(t, handler, "following_other")
	ctx := context.Background()
	for index := 1; index <= 13; index++ {
		if _, err := repo.CreateVideo(ctx, domain.NewVideo{
			UserID: author.User.ID, Title: fmt.Sprintf("Author video %02d", index), Category: "知识",
			VideoPath: fmt.Sprintf("videos/author-%02d.mp4", index), MimeType: "video/mp4", SizeBytes: 100,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: other.User.ID, Title: "Other video", Category: "知识",
		VideoPath: "videos/other.mp4", MimeType: "video/mp4", SizeBytes: 100,
	}); err != nil {
		t.Fatal(err)
	}
	authorPath := strconv.FormatInt(author.User.ID, 10)

	profileResult := httptest.NewRecorder()
	handler.ServeHTTP(profileResult, httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorPath, nil))
	if profileResult.Code != http.StatusOK {
		t.Fatalf("creator profile status=%d body=%s", profileResult.Code, profileResult.Body.String())
	}
	var profileEnvelope testEnvelope
	decodeResponse(t, profileResult, &profileEnvelope)
	var profile domain.CreatorProfile
	if err := json.Unmarshal(profileEnvelope.Data, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.VideosCount != 13 || profile.FollowersCount != 0 || profile.Followed {
		t.Fatalf("unexpected anonymous profile: %#v", profile)
	}

	videosResult := httptest.NewRecorder()
	handler.ServeHTTP(videosResult, httptest.NewRequest(http.MethodGet, "/api/v1/users/"+authorPath+"/videos?page=2&page_size=12", nil))
	if videosResult.Code != http.StatusOK {
		t.Fatalf("creator videos status=%d body=%s", videosResult.Code, videosResult.Body.String())
	}
	var videosEnvelope testEnvelope
	decodeResponse(t, videosResult, &videosEnvelope)
	var authorVideos domain.VideoPage
	if err := json.Unmarshal(videosEnvelope.Data, &authorVideos); err != nil {
		t.Fatal(err)
	}
	if authorVideos.Page != 2 || authorVideos.PageSize != 12 || authorVideos.Total != 13 || len(authorVideos.Items) != 1 || authorVideos.HasNext {
		t.Fatalf("unexpected creator pagination: %#v", authorVideos)
	}
	if authorVideos.Items[0].UserID != author.User.ID {
		t.Fatalf("creator page leaked another author: %#v", authorVideos.Items[0])
	}

	unauthorizedFollow := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedFollow, httptest.NewRequest(http.MethodPost, "/api/v1/users/"+authorPath+"/follow", nil))
	if unauthorizedFollow.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous follow status=%d body=%s", unauthorizedFollow.Code, unauthorizedFollow.Body.String())
	}
	unauthorizedFeed := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedFeed, httptest.NewRequest(http.MethodGet, "/api/v1/me/following/videos", nil))
	if unauthorizedFeed.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous following feed status=%d body=%s", unauthorizedFeed.Code, unauthorizedFeed.Body.String())
	}

	missingCSRF := httptest.NewRecorder()
	handler.ServeHTTP(missingCSRF, authorizedRequest(http.MethodPost, "/api/v1/users/"+authorPath+"/follow", nil, viewer.Cookie, ""))
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status=%d body=%s", missingCSRF.Code, missingCSRF.Body.String())
	}
	wrongCSRF := httptest.NewRecorder()
	handler.ServeHTTP(wrongCSRF, authorizedRequest(http.MethodPost, "/api/v1/users/"+authorPath+"/follow", nil, viewer.Cookie, "wrong-token"))
	if wrongCSRF.Code != http.StatusForbidden {
		t.Fatalf("wrong csrf status=%d body=%s", wrongCSRF.Code, wrongCSRF.Body.String())
	}
	selfFollow := httptest.NewRecorder()
	handler.ServeHTTP(selfFollow, authorizedRequest(http.MethodPost, "/api/v1/users/"+strconv.FormatInt(viewer.User.ID, 10)+"/follow", nil, viewer.Cookie, viewer.CSRFToken))
	if selfFollow.Code != http.StatusForbidden {
		t.Fatalf("self follow status=%d body=%s", selfFollow.Code, selfFollow.Body.String())
	}
	missingUser := httptest.NewRecorder()
	handler.ServeHTTP(missingUser, authorizedRequest(http.MethodPost, "/api/v1/users/99999/follow", nil, viewer.Cookie, viewer.CSRFToken))
	if missingUser.Code != http.StatusNotFound {
		t.Fatalf("missing user follow status=%d body=%s", missingUser.Code, missingUser.Body.String())
	}

	followResult := httptest.NewRecorder()
	handler.ServeHTTP(followResult, authorizedRequest(http.MethodPost, "/api/v1/users/"+authorPath+"/follow", nil, viewer.Cookie, viewer.CSRFToken))
	if followResult.Code != http.StatusOK || !strings.Contains(followResult.Body.String(), `"active":true`) {
		t.Fatalf("follow status=%d body=%s", followResult.Code, followResult.Body.String())
	}
	viewerProfileResult := httptest.NewRecorder()
	handler.ServeHTTP(viewerProfileResult, authorizedRequest(http.MethodGet, "/api/v1/users/"+authorPath, nil, viewer.Cookie, ""))
	var viewerProfileEnvelope testEnvelope
	decodeResponse(t, viewerProfileResult, &viewerProfileEnvelope)
	if err := json.Unmarshal(viewerProfileEnvelope.Data, &profile); err != nil {
		t.Fatal(err)
	}
	if !profile.Followed || profile.FollowersCount != 1 {
		t.Fatalf("unexpected viewer profile after follow: %#v", profile)
	}

	feedResult := httptest.NewRecorder()
	handler.ServeHTTP(feedResult, authorizedRequest(http.MethodGet, "/api/v1/me/following/videos?page=2&page_size=5", nil, viewer.Cookie, ""))
	if feedResult.Code != http.StatusOK {
		t.Fatalf("following feed status=%d body=%s", feedResult.Code, feedResult.Body.String())
	}
	var feedEnvelope testEnvelope
	decodeResponse(t, feedResult, &feedEnvelope)
	var feed domain.VideoPage
	if err := json.Unmarshal(feedEnvelope.Data, &feed); err != nil {
		t.Fatal(err)
	}
	if feed.Page != 2 || feed.PageSize != 5 || feed.Total != 13 || len(feed.Items) != 5 || !feed.HasNext {
		t.Fatalf("unexpected following pagination: %#v", feed)
	}
	for _, video := range feed.Items {
		if video.UserID != author.User.ID {
			t.Fatalf("following feed included unfollowed author: %#v", video)
		}
	}

	unfollowResult := httptest.NewRecorder()
	handler.ServeHTTP(unfollowResult, authorizedRequest(http.MethodPost, "/api/v1/users/"+authorPath+"/follow", nil, viewer.Cookie, viewer.CSRFToken))
	if unfollowResult.Code != http.StatusOK || !strings.Contains(unfollowResult.Body.String(), `"active":false`) {
		t.Fatalf("unfollow status=%d body=%s", unfollowResult.Code, unfollowResult.Body.String())
	}
}

func TestSubtitleManagementHTTPAuthorizationAndResponses(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Config{MediaDir: filepath.Join(dir, "media"), SessionTTL: time.Hour}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := repository.New(db)
	handler := New(service.New(repo, cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notifications.NewRepository(db)), cfg, logger).Routes()
	owner := registerTestUser(t, handler, "subtitle_http_owner")
	other := registerTestUser(t, handler, "subtitle_http_other")
	ctx := context.Background()
	video, err := repo.CreateVideoWithSubtitle(ctx, domain.NewVideo{
		UserID: owner.User.ID, Title: "Subtitle HTTP", Category: "knowledge",
		VideoPath: "videos/subtitle-http.mp4", MimeType: "video/mp4", SizeBytes: 100,
	}, domain.NewSubtitle{
		Language: "zh-CN", Label: "Chinese", Path: "subtitles/http-first/zh-CN.vtt", IsDefault: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateSubtitle(ctx, video.ID, "en", "English", "subtitles/http-second/en.vtt", false)
	if err != nil {
		t.Fatal(err)
	}
	firstID := video.SubtitleTracks[0].ID
	defaultPath := fmt.Sprintf("/api/v1/videos/%d/subtitles/%d/default", video.ID, second.ID)
	deletePath := fmt.Sprintf("/api/v1/videos/%d/subtitles/%d", video.ID, second.ID)

	anonymous := httptest.NewRecorder()
	handler.ServeHTTP(anonymous, httptest.NewRequest(http.MethodPatch, defaultPath, nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous set default status=%d body=%s", anonymous.Code, anonymous.Body.String())
	}
	missingCSRF := httptest.NewRecorder()
	handler.ServeHTTP(missingCSRF, authorizedRequest(http.MethodPatch, defaultPath, nil, owner.Cookie, ""))
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d body=%s", missingCSRF.Code, missingCSRF.Body.String())
	}
	forbidden := httptest.NewRecorder()
	handler.ServeHTTP(forbidden, authorizedRequest(http.MethodPatch, defaultPath, nil, other.Cookie, other.CSRFToken))
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-author set default status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, authorizedRequest(http.MethodPatch,
		fmt.Sprintf("/api/v1/videos/%d/subtitles/99999/default", video.ID), nil, owner.Cookie, owner.CSRFToken))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing subtitle status=%d body=%s", missing.Code, missing.Body.String())
	}

	setDefault := httptest.NewRecorder()
	handler.ServeHTTP(setDefault, authorizedRequest(http.MethodPatch, defaultPath, nil, owner.Cookie, owner.CSRFToken))
	if setDefault.Code != http.StatusOK {
		t.Fatalf("set default status=%d body=%s", setDefault.Code, setDefault.Body.String())
	}
	var setEnvelope testEnvelope
	decodeResponse(t, setDefault, &setEnvelope)
	var tracks []domain.SubtitleTrack
	if err := json.Unmarshal(setEnvelope.Data, &tracks); err != nil || len(tracks) != 2 || tracks[0].ID != second.ID || !tracks[0].IsDefault {
		t.Fatalf("set default tracks=%#v err=%v", tracks, err)
	}

	anonymousDelete := httptest.NewRecorder()
	handler.ServeHTTP(anonymousDelete, httptest.NewRequest(http.MethodDelete, deletePath, nil))
	if anonymousDelete.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous delete status=%d body=%s", anonymousDelete.Code, anonymousDelete.Body.String())
	}
	missingDeleteCSRF := httptest.NewRecorder()
	handler.ServeHTTP(missingDeleteCSRF, authorizedRequest(http.MethodDelete, deletePath, nil, owner.Cookie, ""))
	if missingDeleteCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing delete CSRF status=%d body=%s", missingDeleteCSRF.Code, missingDeleteCSRF.Body.String())
	}
	deleteForbidden := httptest.NewRecorder()
	handler.ServeHTTP(deleteForbidden, authorizedRequest(http.MethodDelete, deletePath, nil, other.Cookie, other.CSRFToken))
	if deleteForbidden.Code != http.StatusForbidden {
		t.Fatalf("non-author delete status=%d body=%s", deleteForbidden.Code, deleteForbidden.Body.String())
	}
	deleted := httptest.NewRecorder()
	handler.ServeHTTP(deleted, authorizedRequest(http.MethodDelete, deletePath, nil, owner.Cookie, owner.CSRFToken))
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete default status=%d body=%s", deleted.Code, deleted.Body.String())
	}
	var deletedEnvelope testEnvelope
	decodeResponse(t, deleted, &deletedEnvelope)
	if err := json.Unmarshal(deletedEnvelope.Data, &tracks); err != nil || len(tracks) != 1 || tracks[0].ID != firstID || !tracks[0].IsDefault {
		t.Fatalf("delete fallback tracks=%#v err=%v", tracks, err)
	}

	lastPath := fmt.Sprintf("/api/v1/videos/%d/subtitles/%d", video.ID, firstID)
	deletedLast := httptest.NewRecorder()
	handler.ServeHTTP(deletedLast, authorizedRequest(http.MethodDelete, lastPath, nil, owner.Cookie, owner.CSRFToken))
	if deletedLast.Code != http.StatusOK {
		t.Fatalf("delete last status=%d body=%s", deletedLast.Code, deletedLast.Body.String())
	}
	var lastEnvelope testEnvelope
	decodeResponse(t, deletedLast, &lastEnvelope)
	if err := json.Unmarshal(lastEnvelope.Data, &tracks); err != nil || len(tracks) != 0 {
		t.Fatalf("delete last tracks=%#v err=%v", tracks, err)
	}
}

func TestProfileVisibilityCommentsAndPrivateMediaHTTP(t *testing.T) {
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := config.Config{MediaDir: mediaDir, SessionTTL: time.Hour, MaxUploadBytes: 10 << 20}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := repository.New(db)
	handler := New(service.New(repo, cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notifications.NewRepository(db)), cfg, logger).Routes()
	ctx := context.Background()

	owner := registerTestUser(t, handler, "http_profile_owner")
	viewer := registerTestUser(t, handler, "http_profile_viewer")

	unauthorizedProfile := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedProfile, httptest.NewRequest(http.MethodPatch, "/api/v1/me/profile", nil))
	if unauthorizedProfile.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous profile update status=%d body=%s", unauthorizedProfile.Code, unauthorizedProfile.Body.String())
	}
	missingCSRFBody, missingCSRFType := profileBody(t, "http_profile_owner", "bio", false)
	missingCSRFRequest := authorizedRequest(http.MethodPatch, "/api/v1/me/profile", missingCSRFBody, owner.Cookie, "")
	missingCSRFRequest.Header.Set("Content-Type", missingCSRFType)
	missingCSRFResult := httptest.NewRecorder()
	handler.ServeHTTP(missingCSRFResult, missingCSRFRequest)
	if missingCSRFResult.Code != http.StatusForbidden {
		t.Fatalf("profile missing CSRF status=%d body=%s", missingCSRFResult.Code, missingCSRFResult.Body.String())
	}
	invalidMultipart := authorizedRequest(http.MethodPatch, "/api/v1/me/profile", strings.NewReader("not multipart"), owner.Cookie, owner.CSRFToken)
	invalidMultipart.Header.Set("Content-Type", "text/plain")
	invalidMultipartResult := httptest.NewRecorder()
	handler.ServeHTTP(invalidMultipartResult, invalidMultipart)
	if invalidMultipartResult.Code != http.StatusBadRequest {
		t.Fatalf("invalid profile multipart status=%d body=%s", invalidMultipartResult.Code, invalidMultipartResult.Body.String())
	}

	profilePayload, profileType := profileBody(t, "http_owner_renamed", "updated profile", true)
	profileRequest := authorizedRequest(http.MethodPatch, "/api/v1/me/profile", profilePayload, owner.Cookie, owner.CSRFToken)
	profileRequest.Header.Set("Content-Type", profileType)
	profileResult := httptest.NewRecorder()
	handler.ServeHTTP(profileResult, profileRequest)
	if profileResult.Code != http.StatusOK {
		t.Fatalf("profile update status=%d body=%s", profileResult.Code, profileResult.Body.String())
	}
	var profileEnvelope testEnvelope
	decodeResponse(t, profileResult, &profileEnvelope)
	var updatedUser domain.User
	if err := json.Unmarshal(profileEnvelope.Data, &updatedUser); err != nil {
		t.Fatal(err)
	}
	if updatedUser.Username != "http_owner_renamed" || updatedUser.Bio != "updated profile" ||
		!strings.HasPrefix(updatedUser.AvatarURL, "/media/avatars/") {
		t.Fatalf("unexpected updated user: %#v", updatedUser)
	}

	conflictBody, conflictType := profileBody(t, viewer.User.Username, "conflict", true)
	conflictRequest := authorizedRequest(http.MethodPatch, "/api/v1/me/profile", conflictBody, owner.Cookie, owner.CSRFToken)
	conflictRequest.Header.Set("Content-Type", conflictType)
	conflictResult := httptest.NewRecorder()
	handler.ServeHTTP(conflictResult, conflictRequest)
	if conflictResult.Code != http.StatusConflict {
		t.Fatalf("profile conflict status=%d body=%s", conflictResult.Code, conflictResult.Body.String())
	}
	invalidBody, invalidType := profileBody(t, "x", strings.Repeat("a", 301), false)
	invalidRequest := authorizedRequest(http.MethodPatch, "/api/v1/me/profile", invalidBody, owner.Cookie, owner.CSRFToken)
	invalidRequest.Header.Set("Content-Type", invalidType)
	invalidResult := httptest.NewRecorder()
	handler.ServeHTTP(invalidResult, invalidRequest)
	if invalidResult.Code != http.StatusBadRequest {
		t.Fatalf("invalid profile status=%d body=%s", invalidResult.Code, invalidResult.Body.String())
	}

	statsResult := httptest.NewRecorder()
	handler.ServeHTTP(statsResult, authorizedRequest(http.MethodGet, "/api/v1/me/creator/stats", nil, owner.Cookie, ""))
	if statsResult.Code != http.StatusOK || !strings.Contains(statsResult.Body.String(), `"videos_count":0`) {
		t.Fatalf("empty creator stats status=%d body=%s", statsResult.Code, statsResult.Body.String())
	}
	anonymousStats := httptest.NewRecorder()
	handler.ServeHTTP(anonymousStats, httptest.NewRequest(http.MethodGet, "/api/v1/me/creator/stats", nil))
	if anonymousStats.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous creator stats status=%d body=%s", anonymousStats.Code, anonymousStats.Body.String())
	}

	publicVideo, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: owner.User.ID, Title: "HTTP public", Category: "knowledge", Visibility: "public",
		VideoPath: "videos/http-public.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	unlistedVideo, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: owner.User.ID, Title: "HTTP unlisted", Category: "knowledge", Visibility: "unlisted",
		VideoPath: "videos/http-unlisted.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	privateVideo, err := repo.CreateVideoWithSubtitle(ctx, domain.NewVideo{
		UserID: owner.User.ID, Title: "HTTP private", Category: "knowledge", Visibility: "private",
		VideoPath: "videos/http-private.mp4", CoverPath: "covers/http-private.jpg", MimeType: "video/mp4", SizeBytes: 100,
	}, domain.NewSubtitle{Language: "en", Label: "English", Path: "subtitles/http-private/en.vtt", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE videos SET hls_master_path = 'hls/http-private/master.m3u8' WHERE id = ?`, privateVideo.ID); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"videos/http-public.mp4":          "0123456789-public",
		"videos/http-unlisted.mp4":        "0123456789-unlisted",
		"videos/http-private.mp4":         "0123456789-private",
		"covers/http-private.jpg":         "private-cover",
		"subtitles/http-private/en.vtt":   "WEBVTT\n",
		"hls/http-private/master.m3u8":    "#EXTM3U\nsegment-001.ts\n",
		"hls/http-private/segment-001.ts": "transport-stream",
	} {
		absolute := filepath.Join(mediaDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	editBody, editType := videoVisibilityBody(t, "HTTP unlisted edited", "private")
	editRequest := authorizedRequest(http.MethodPatch, "/api/v1/videos/"+strconv.FormatInt(unlistedVideo.ID, 10), editBody, owner.Cookie, owner.CSRFToken)
	editRequest.Header.Set("Content-Type", editType)
	editResult := httptest.NewRecorder()
	handler.ServeHTTP(editResult, editRequest)
	if editResult.Code != http.StatusOK || !strings.Contains(editResult.Body.String(), `"visibility":"private"`) {
		t.Fatalf("visibility edit status=%d body=%s", editResult.Code, editResult.Body.String())
	}

	followResult := httptest.NewRecorder()
	handler.ServeHTTP(followResult, authorizedRequest(http.MethodPost, "/api/v1/users/"+strconv.FormatInt(owner.User.ID, 10)+"/follow", nil, viewer.Cookie, viewer.CSRFToken))
	if followResult.Code != http.StatusOK {
		t.Fatalf("follow owner status=%d body=%s", followResult.Code, followResult.Body.String())
	}
	for name, test := range map[string]struct {
		request   *http.Request
		wantTotal int64
	}{
		"public list": {
			request: httptest.NewRequest(http.MethodGet, "/api/v1/videos", nil), wantTotal: 1,
		},
		"creator list": {
			request: httptest.NewRequest(http.MethodGet, "/api/v1/users/"+strconv.FormatInt(owner.User.ID, 10)+"/videos", nil), wantTotal: 1,
		},
		"owner list": {
			request: authorizedRequest(http.MethodGet, "/api/v1/me/videos", nil, owner.Cookie, ""), wantTotal: 3,
		},
		"following list": {
			request: authorizedRequest(http.MethodGet, "/api/v1/me/following/videos", nil, viewer.Cookie, ""), wantTotal: 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			result := httptest.NewRecorder()
			handler.ServeHTTP(result, test.request)
			if result.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", result.Code, result.Body.String())
			}
			var envelope testEnvelope
			decodeResponse(t, result, &envelope)
			var page domain.VideoPage
			if err := json.Unmarshal(envelope.Data, &page); err != nil || page.Total != test.wantTotal {
				t.Fatalf("page=%#v err=%v", page, err)
			}
		})
	}

	unlistedDetail := httptest.NewRecorder()
	handler.ServeHTTP(unlistedDetail, httptest.NewRequest(http.MethodGet, "/api/v1/videos/"+strconv.FormatInt(unlistedVideo.ID, 10)+"?count_view=false", nil))
	if unlistedDetail.Code != http.StatusNotFound {
		t.Fatalf("edited private detail status=%d body=%s", unlistedDetail.Code, unlistedDetail.Body.String())
	}
	privatePath := "/api/v1/videos/" + strconv.FormatInt(privateVideo.ID, 10)
	anonymousPrivate := httptest.NewRecorder()
	handler.ServeHTTP(anonymousPrivate, httptest.NewRequest(http.MethodGet, privatePath+"?count_view=false", nil))
	if anonymousPrivate.Code != http.StatusNotFound {
		t.Fatalf("anonymous private detail status=%d body=%s", anonymousPrivate.Code, anonymousPrivate.Body.String())
	}
	ownerPrivate := httptest.NewRecorder()
	handler.ServeHTTP(ownerPrivate, authorizedRequest(http.MethodGet, privatePath+"?count_view=false", nil, owner.Cookie, ""))
	if ownerPrivate.Code != http.StatusOK {
		t.Fatalf("owner private detail status=%d body=%s", ownerPrivate.Code, ownerPrivate.Body.String())
	}
	anonymousComments := httptest.NewRecorder()
	handler.ServeHTTP(anonymousComments, httptest.NewRequest(http.MethodGet, privatePath+"/comments", nil))
	if anonymousComments.Code != http.StatusNotFound {
		t.Fatalf("anonymous private comments status=%d body=%s", anonymousComments.Code, anonymousComments.Body.String())
	}
	viewerComment := authorizedRequest(http.MethodPost, privatePath+"/comments", strings.NewReader(`{"content":"blocked"}`), viewer.Cookie, viewer.CSRFToken)
	viewerComment.Header.Set("Content-Type", "application/json")
	viewerCommentResult := httptest.NewRecorder()
	handler.ServeHTTP(viewerCommentResult, viewerComment)
	if viewerCommentResult.Code != http.StatusNotFound {
		t.Fatalf("viewer private comment status=%d body=%s", viewerCommentResult.Code, viewerCommentResult.Body.String())
	}
	ownerComment := authorizedRequest(http.MethodPost, privatePath+"/comments", strings.NewReader(`{"content":"allowed"}`), owner.Cookie, owner.CSRFToken)
	ownerComment.Header.Set("Content-Type", "application/json")
	ownerCommentResult := httptest.NewRecorder()
	handler.ServeHTTP(ownerCommentResult, ownerComment)
	if ownerCommentResult.Code != http.StatusCreated || !strings.Contains(ownerCommentResult.Body.String(), `"avatar_url":"`+updatedUser.AvatarURL+`"`) {
		t.Fatalf("owner private comment status=%d body=%s", ownerCommentResult.Code, ownerCommentResult.Body.String())
	}

	avatarResult := httptest.NewRecorder()
	handler.ServeHTTP(avatarResult, httptest.NewRequest(http.MethodGet, updatedUser.AvatarURL, nil))
	if avatarResult.Code != http.StatusOK {
		t.Fatalf("public avatar status=%d body=%s", avatarResult.Code, avatarResult.Body.String())
	}
	for name, mediaPath := range map[string]string{
		"source":   "/media/videos/http-private.mp4",
		"cover":    "/media/covers/http-private.jpg",
		"subtitle": "/media/subtitles/http-private/en.vtt",
		"playlist": "/media/hls/http-private/master.m3u8",
		"segment":  "/media/hls/http-private/segment-001.ts",
	} {
		t.Run(name, func(t *testing.T) {
			anonymous := httptest.NewRecorder()
			handler.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, mediaPath, nil))
			if anonymous.Code != http.StatusNotFound {
				t.Fatalf("anonymous status=%d body=%s", anonymous.Code, anonymous.Body.String())
			}
			other := httptest.NewRecorder()
			handler.ServeHTTP(other, authorizedRequest(http.MethodGet, mediaPath, nil, viewer.Cookie, ""))
			if other.Code != http.StatusNotFound {
				t.Fatalf("other user status=%d body=%s", other.Code, other.Body.String())
			}
			ownerResult := httptest.NewRecorder()
			handler.ServeHTTP(ownerResult, authorizedRequest(http.MethodGet, mediaPath, nil, owner.Cookie, ""))
			if ownerResult.Code != http.StatusOK {
				t.Fatalf("owner status=%d body=%s", ownerResult.Code, ownerResult.Body.String())
			}
		})
	}
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, authorizedRequest(http.MethodGet, "/media/hls/http-private/master.m3u8", nil, owner.Cookie, ""))
	if playlist.Header().Get("Content-Type") != "application/vnd.apple.mpegurl" {
		t.Fatalf("playlist content type = %q", playlist.Header().Get("Content-Type"))
	}
	segment := httptest.NewRecorder()
	handler.ServeHTTP(segment, authorizedRequest(http.MethodGet, "/media/hls/http-private/segment-001.ts", nil, owner.Cookie, ""))
	if segment.Header().Get("Content-Type") != "video/mp2t" {
		t.Fatalf("segment content type = %q", segment.Header().Get("Content-Type"))
	}
	rangeRequest := authorizedRequest(http.MethodGet, "/media/videos/http-private.mp4", nil, owner.Cookie, "")
	rangeRequest.Header.Set("Range", "bytes=0-4")
	rangeResult := httptest.NewRecorder()
	handler.ServeHTTP(rangeResult, rangeRequest)
	if rangeResult.Code != http.StatusPartialContent || rangeResult.Body.Len() != 5 || rangeResult.Header().Get("Content-Range") == "" {
		t.Fatalf("private range status=%d bytes=%d content-range=%q", rangeResult.Code, rangeResult.Body.Len(), rangeResult.Header().Get("Content-Range"))
	}
	if publicVideo.ID == 0 {
		t.Fatal("public video was not created")
	}
}

func registerTestUser(t *testing.T, handler http.Handler, username string) testAuthClient {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"username":"`+username+`","password":"password123"}`))
	request.Header.Set("Content-Type", "application/json")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusCreated {
		t.Fatalf("register %s status=%d body=%s", username, result.Code, result.Body.String())
	}
	var envelope testEnvelope
	decodeResponse(t, result, &envelope)
	var payload struct {
		User      domain.User `json:"user"`
		CSRFToken string      `json:"csrf_token"`
	}
	if err := json.Unmarshal(envelope.Data, &payload); err != nil {
		t.Fatal(err)
	}
	cookies := result.Result().Cookies()
	if payload.User.ID == 0 || payload.CSRFToken == "" || len(cookies) == 0 {
		t.Fatalf("incomplete auth payload for %s: %#v", username, payload)
	}
	return testAuthClient{User: payload.User, Cookie: cookies[0], CSRFToken: payload.CSRFToken}
}

func TestDeleteCommentHTTPAuthorization(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "comments.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	cfg := config.Config{MediaDir: filepath.Join(dir, "media"), SessionTTL: time.Hour}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(service.New(repo, cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notifications.NewRepository(db)), cfg, logger).Routes()
	owner := registerTestUser(t, handler, "delete_comment_owner")
	author := registerTestUser(t, handler, "delete_comment_author")
	other := registerTestUser(t, handler, "delete_comment_other")
	video, err := repo.CreateVideo(context.Background(), domain.NewVideo{UserID: owner.User.ID, Title: "Delete comment", Category: "knowledge", VideoPath: "videos/delete-comment.mp4", MimeType: "video/mp4", SizeBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	comment, err := repo.CreateComment(context.Background(), author.User.ID, video.ID, "delete through HTTP")
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/api/v1/videos/%d/comments/%d", video.ID, comment.ID)

	result := httptest.NewRecorder()
	handler.ServeHTTP(result, httptest.NewRequest(http.MethodDelete, path, nil))
	if result.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d body=%s", result.Code, result.Body.String())
	}
	result = httptest.NewRecorder()
	handler.ServeHTTP(result, authorizedRequest(http.MethodDelete, path, nil, author.Cookie, ""))
	if result.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d body=%s", result.Code, result.Body.String())
	}
	result = httptest.NewRecorder()
	handler.ServeHTTP(result, authorizedRequest(http.MethodDelete, path, nil, other.Cookie, other.CSRFToken))
	if result.Code != http.StatusForbidden {
		t.Fatalf("other status=%d body=%s", result.Code, result.Body.String())
	}
	result = httptest.NewRecorder()
	handler.ServeHTTP(result, authorizedRequest(http.MethodDelete, path, nil, owner.Cookie, owner.CSRFToken))
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), `"deleted":true`) {
		t.Fatalf("owner status=%d body=%s", result.Code, result.Body.String())
	}
}

func TestAdminReportEndpoints(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "admin-reports.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	cfg := config.Config{AdminUsername: "http_report_admin", MediaDir: filepath.Join(dir, "media"), SessionTTL: time.Hour}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(service.New(repo, cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notifications.NewRepository(db)), cfg, logger).Routes()
	admin := registerTestUser(t, handler, "http_report_admin")
	author := registerTestUser(t, handler, "http_report_author")
	reporter := registerTestUser(t, handler, "http_report_viewer")
	video, err := repo.CreateVideo(context.Background(), domain.NewVideo{UserID: author.User.ID, Title: "Admin report", Category: service.Categories[0], VideoPath: "videos/admin-report.mp4", MimeType: "video/mp4", SizeBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	report, err := moderation.NewRepository(db).UpsertVideoReport(context.Background(), video.ID, reporter.User.ID, "spam", "review through HTTP")
	if err != nil {
		t.Fatal(err)
	}
	forbidden := httptest.NewRecorder()
	handler.ServeHTTP(forbidden, authorizedRequest(http.MethodGet, "/api/v1/admin/reports", nil, reporter.Cookie, ""))
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("ordinary list status=%d body=%s", forbidden.Code, forbidden.Body.String())
	}
	list := httptest.NewRecorder()
	handler.ServeHTTP(list, authorizedRequest(http.MethodGet, "/api/v1/admin/reports?status=pending", nil, admin.Cookie, ""))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"total":1`) || !strings.Contains(list.Body.String(), `"video_title":"Admin report"`) {
		t.Fatalf("admin list status=%d body=%s", list.Code, list.Body.String())
	}
	updateRequest := authorizedRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/reports/%d", report.ID), strings.NewReader(`{"status":"resolved"}`), admin.Cookie, admin.CSRFToken)
	updateRequest.Header.Set("Content-Type", "application/json")
	update := httptest.NewRecorder()
	handler.ServeHTTP(update, updateRequest)
	if update.Code != http.StatusOK || !strings.Contains(update.Body.String(), `"status":"resolved"`) {
		t.Fatalf("admin update status=%d body=%s", update.Code, update.Body.String())
	}
	noCSRFRequest := authorizedRequest(http.MethodPatch, fmt.Sprintf("/api/v1/admin/reports/%d", report.ID), strings.NewReader(`{"status":"dismissed"}`), admin.Cookie, "")
	noCSRFRequest.Header.Set("Content-Type", "application/json")
	noCSRF := httptest.NewRecorder()
	handler.ServeHTTP(noCSRF, noCSRFRequest)
	if noCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status=%d body=%s", noCSRF.Code, noCSRF.Body.String())
	}
}

func TestFavoriteVideosEndpoint(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "favorites.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	cfg := config.Config{MediaDir: filepath.Join(dir, "media"), SessionTTL: time.Hour}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(service.New(repo, cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notifications.NewRepository(db)), cfg, logger).Routes()
	auth := registerTestUser(t, handler, "favorite_endpoint_user")
	video, err := repo.CreateVideo(context.Background(), domain.NewVideo{UserID: auth.User.ID, Title: "Favorite endpoint", Category: "knowledge", VideoPath: "videos/favorite-endpoint.mp4", MimeType: "video/mp4", SizeBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ToggleFavorite(context.Background(), auth.User.ID, video.ID); err != nil {
		t.Fatal(err)
	}
	request := authorizedRequest(http.MethodGet, "/api/v1/me/favorites?page=1&page_size=24", nil, auth.Cookie, "")
	result := httptest.NewRecorder()
	handler.ServeHTTP(result, request)
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), "Favorite endpoint") {
		t.Fatalf("favorites status=%d body=%s", result.Code, result.Body.String())
	}
}

func TestNotificationEndpointsAuthenticationCSRFAndOwnership(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "notifications.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	notificationRepo := notifications.NewRepository(db)
	cfg := config.Config{MediaDir: filepath.Join(dir, "media"), SessionTTL: time.Hour}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(service.New(repo, cfg, logger), moderation.NewService(moderation.NewRepository(db)), notifications.NewService(notificationRepo), cfg, logger).Routes()
	recipient := registerTestUser(t, handler, "notify_recipient")
	actor := registerTestUser(t, handler, "notify_actor")
	other := registerTestUser(t, handler, "notify_other")
	ctx := context.Background()
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: recipient.User.ID, Title: "HTTP notification video", Category: "knowledge",
		VideoPath: "videos/http-notifications.mp4", MimeType: "video/mp4", SizeBytes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateNotification(ctx, recipient.User.ID, actor.User.ID, "like", video.ID, 0, video.Title, ""); err != nil {
		t.Fatal(err)
	}

	result := httptest.NewRecorder()
	handler.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/api/v1/me/notifications", nil))
	if result.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous list status=%d body=%s", result.Code, result.Body.String())
	}
	result = httptest.NewRecorder()
	handler.ServeHTTP(result, authorizedRequest(http.MethodGet, "/api/v1/me/notifications?page=1&page_size=20", nil, recipient.Cookie, ""))
	if result.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", result.Code, result.Body.String())
	}
	var envelope testEnvelope
	decodeResponse(t, result, &envelope)
	var page domain.NotificationPage
	if err := json.Unmarshal(envelope.Data, &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.UnreadCount != 1 || len(page.Items) != 1 || page.Items[0].ActorID != actor.User.ID {
		t.Fatalf("unexpected notification page: %#v", page)
	}
	notificationID := strconv.FormatInt(page.Items[0].ID, 10)
	readPath := "/api/v1/me/notifications/" + notificationID + "/read"

	result = httptest.NewRecorder()
	handler.ServeHTTP(result, authorizedRequest(http.MethodPatch, readPath, nil, recipient.Cookie, ""))
	if result.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d body=%s", result.Code, result.Body.String())
	}
	result = httptest.NewRecorder()
	handler.ServeHTTP(result, authorizedRequest(http.MethodPatch, readPath, nil, other.Cookie, other.CSRFToken))
	if result.Code != http.StatusNotFound {
		t.Fatalf("cross-user mark status=%d body=%s", result.Code, result.Body.String())
	}
	result = httptest.NewRecorder()
	handler.ServeHTTP(result, authorizedRequest(http.MethodPatch, readPath, nil, recipient.Cookie, recipient.CSRFToken))
	if result.Code != http.StatusOK {
		t.Fatalf("mark read status=%d body=%s", result.Code, result.Body.String())
	}

	if err := repo.CreateNotification(ctx, recipient.User.ID, actor.User.ID, "favorite", video.ID, 0, video.Title, ""); err != nil {
		t.Fatal(err)
	}
	result = httptest.NewRecorder()
	handler.ServeHTTP(result, authorizedRequest(http.MethodPost, "/api/v1/me/notifications/read-all", nil, recipient.Cookie, "wrong-token"))
	if result.Code != http.StatusForbidden {
		t.Fatalf("wrong CSRF read-all status=%d body=%s", result.Code, result.Body.String())
	}
	result = httptest.NewRecorder()
	handler.ServeHTTP(result, authorizedRequest(http.MethodPost, "/api/v1/me/notifications/read-all", nil, recipient.Cookie, recipient.CSRFToken))
	if result.Code != http.StatusOK {
		t.Fatalf("read-all status=%d body=%s", result.Code, result.Body.String())
	}
	page, err = notificationRepo.ListNotifications(ctx, recipient.User.ID, 1, 20)
	if err != nil || page.UnreadCount != 0 {
		t.Fatalf("read-all page=%#v err=%v", page, err)
	}
}

func videoEditBody(t *testing.T, title, description, category string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("title", title)
	_ = writer.WriteField("description", description)
	_ = writer.WriteField("category", category)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

func profileBody(t *testing.T, username, bio string, avatar bool) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("username", username)
	_ = writer.WriteField("bio", bio)
	if avatar {
		part, err := writer.CreateFormFile("avatar", "avatar.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte("\x89PNG\r\n\x1a\navatar")); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

func videoVisibilityBody(t *testing.T, title, visibility string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("title", title)
	_ = writer.WriteField("description", "visibility edit")
	_ = writer.WriteField("category", service.Categories[0])
	_ = writer.WriteField("visibility", visibility)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}

func authorizedRequest(method, path string, body io.Reader, cookie *http.Cookie, csrf string) *http.Request {
	request := httptest.NewRequest(method, path, body)
	request.AddCookie(cookie)
	request.Header.Set("X-CSRF-Token", csrf)
	return request
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(recorder.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestResolveServedMediaPathRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	outsideDir := filepath.Join(dir, "outside")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(mediaDir, "linked.txt")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable on this platform: %v", err)
	}
	if _, err := resolveServedMediaPath(mediaDir, "linked.txt"); err == nil {
		t.Fatal("symlink escaping media root was accepted")
	}
}
