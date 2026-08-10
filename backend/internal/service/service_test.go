package service

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
)

func subtitleHeader(t *testing.T, name, content string) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="subtitle"; filename="`+name+`"`)
	header.Set("Content-Type", "text/plain")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(2 << 20); err != nil {
		t.Fatal(err)
	}
	_, file, err := request.FormFile("subtitle")
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestRegisterLoginAndSession(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(repository.New(db), config.Config{SessionTTL: time.Hour, MediaDir: filepath.Join(dir, "media")}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	created, err := svc.Register(ctx, "新用户_01", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if created.Token == "" || created.CSRFToken == "" {
		t.Fatal("expected session credentials")
	}
	session, err := svc.Authenticate(ctx, created.Token)
	if err != nil || session.User.Username != "新用户_01" {
		t.Fatalf("authenticate: %#v err=%v", session, err)
	}
	if _, err := svc.Login(ctx, "新用户_01", "wrong-password"); err != domain.ErrUnauthorized {
		t.Fatalf("expected unauthorized, got %v", err)
	}
	if _, err := svc.Login(ctx, "新用户_01", "password123"); err != nil {
		t.Fatal(err)
	}
}

func TestCreatorProfileAndFollowRules(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	svc := New(repo, config.Config{SessionTTL: time.Hour, MediaDir: filepath.Join(dir, "media")}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	follower, err := repo.CreateUser(ctx, "service_follower", "hash")
	if err != nil {
		t.Fatal(err)
	}
	followed, err := repo.CreateUser(ctx, "service_followed", "hash")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ToggleFollow(ctx, follower.ID, follower.ID); err != domain.ErrForbidden {
		t.Fatalf("self follow error = %v, want forbidden", err)
	}
	if _, err := svc.ToggleFollow(ctx, follower.ID, 99999); err != domain.ErrNotFound {
		t.Fatalf("missing user follow error = %v, want not found", err)
	}
	active, err := svc.ToggleFollow(ctx, follower.ID, followed.ID)
	if err != nil || !active {
		t.Fatalf("follow user: active=%v err=%v", active, err)
	}
	profile, err := svc.CreatorProfile(ctx, followed.ID, follower.ID)
	if err != nil || !profile.Followed || profile.FollowersCount != 1 {
		t.Fatalf("creator profile: %#v err=%v", profile, err)
	}
	active, err = svc.ToggleFollow(ctx, follower.ID, followed.ID)
	if err != nil || active {
		t.Fatalf("unfollow user: active=%v err=%v", active, err)
	}
}

func TestInspectAndConvertSubtitleSRT(t *testing.T) {
	track, err := inspectAndConvertSubtitle(subtitleHeader(t, "captions.srt", "1\n00:00:01,000 --> 00:00:03,500\nHello\n"), "en", "English")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(track.data), "WEBVTT\n\n") || !strings.Contains(string(track.data), "00:00:01.000 --> 00:00:03.500") {
		t.Fatalf("unexpected converted VTT: %q", track.data)
	}
}

func TestInspectAndConvertSubtitleRejectsInvalidInput(t *testing.T) {
	if _, err := inspectAndConvertSubtitle(subtitleHeader(t, "captions.txt", "WEBVTT\n"), "zh-CN", "中文"); err != domain.ErrInvalidInput {
		t.Fatalf("expected invalid extension, got %v", err)
	}
	if _, err := inspectAndConvertSubtitle(subtitleHeader(t, "captions.vtt", "not a webvtt file\n"), "zh-CN", "中文"); err != domain.ErrInvalidInput {
		t.Fatalf("expected invalid VTT, got %v", err)
	}
}

func TestAddSubtitleRequiresVideoAuthor(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	author, err := repo.CreateUser(ctx, "subtitle_author", "hash")
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := repo.CreateUser(ctx, "subtitle_viewer", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: author.ID, Title: "Subtitle permissions", Category: "知识",
		VideoPath: "videos/permission.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := New(repo, config.Config{MediaDir: filepath.Join(dir, "media")}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err = svc.AddSubtitle(ctx, viewer.ID, video.ID, subtitleHeader(t, "captions.vtt", "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nNo access\n"), "en", "English")
	if err != domain.ErrForbidden {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestVideoManagementLifecycle(t *testing.T) {
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	author, err := repo.CreateUser(ctx, "video_author", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := repo.CreateUser(ctx, "video_other", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideoWithSubtitle(ctx, domain.NewVideo{
		UserID: author.ID, Title: "Original title", Category: "知识",
		VideoPath: "videos/lifecycle.mp4", CoverPath: "covers/lifecycle.jpg", MimeType: "video/mp4", SizeBytes: 100,
	}, domain.NewSubtitle{Language: "zh-CN", Label: "简体中文", Path: "subtitles/lifecycle/zh-CN.vtt", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"videos/lifecycle.mp4": "video", "covers/lifecycle.jpg": "cover", "subtitles/lifecycle/zh-CN.vtt": "WEBVTT",
		filepath.ToSlash(filepath.Join("hls", strconv.FormatInt(video.ID, 10), "master.m3u8")): "#EXTM3U",
	} {
		absolute := filepath.Join(mediaDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	job, found, err := repo.ClaimTranscodingJob(ctx)
	if err != nil || !found {
		t.Fatalf("claim job: found=%v err=%v", found, err)
	}
	if err := repo.FailTranscodingJob(ctx, job.ID, video.ID, "ffmpeg internal path", nil); err != nil {
		t.Fatal(err)
	}
	svc := New(repo, config.Config{MediaDir: mediaDir}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if _, err := svc.UpdateVideo(ctx, UpdateVideoInput{UserID: other.ID, VideoID: video.ID, Title: "No access", Category: "知识"}); err != domain.ErrForbidden {
		t.Fatalf("expected forbidden update, got %v", err)
	}
	updated, err := svc.UpdateVideo(ctx, UpdateVideoInput{UserID: author.ID, VideoID: video.ID, Title: "Updated title", Description: "Updated description", Category: "科技"})
	if err != nil || updated.Title != "Updated title" || updated.Description != "Updated description" {
		t.Fatalf("update video: %#v err=%v", updated, err)
	}
	if err := svc.RetryTranscoding(ctx, author.ID, video.ID); err != nil {
		t.Fatal(err)
	}
	retried, err := repo.VideoByID(ctx, video.ID, author.ID)
	if err != nil || retried.ProcessingStatus != "pending" {
		t.Fatalf("retry status: %#v err=%v", retried, err)
	}
	if _, err := os.Stat(filepath.Join(mediaDir, "hls", strconv.FormatInt(video.ID, 10))); !os.IsNotExist(err) {
		t.Fatalf("expected stale HLS directory removed, stat err=%v", err)
	}
	job, found, err = repo.ClaimTranscodingJob(ctx)
	if err != nil || !found {
		t.Fatalf("claim retried job: found=%v err=%v", found, err)
	}
	if err := repo.FailTranscodingJob(ctx, job.ID, video.ID, "failed again", nil); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteVideo(ctx, author.ID, video.ID); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"videos/lifecycle.mp4", "covers/lifecycle.jpg", "subtitles/lifecycle/zh-CN.vtt"} {
		if _, err := os.Stat(filepath.Join(mediaDir, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("expected asset %s removed, stat err=%v", path, err)
		}
	}
	if _, err := repo.VideoByID(ctx, video.ID, author.ID); err != domain.ErrNotFound {
		t.Fatalf("expected deleted video missing, got %v", err)
	}
}

func TestDeleteVideoWhileProcessingIsBlocked(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	author, err := repo.CreateUser(ctx, "processing_author", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{UserID: author.ID, Title: "Processing", Category: "知识", VideoPath: "videos/processing.mp4", MimeType: "video/mp4", SizeBytes: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, found, err := repo.ClaimTranscodingJob(ctx); err != nil || !found {
		t.Fatalf("claim job: found=%v err=%v", found, err)
	}
	svc := New(repo, config.Config{MediaDir: filepath.Join(dir, "media")}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := svc.DeleteVideo(ctx, author.ID, video.ID); err != domain.ErrVideoProcessing {
		t.Fatalf("expected processing conflict, got %v", err)
	}
}
