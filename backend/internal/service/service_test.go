package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

func multipartFileHeader(t *testing.T, field, name, contentType string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="`+field+`"; filename="`+name+`"`)
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(12 << 20); err != nil {
		t.Fatal(err)
	}
	_, file, err := request.FormFile(field)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func subtitleHeader(t *testing.T, name, content string) *multipart.FileHeader {
	t.Helper()
	return multipartFileHeader(t, "subtitle", name, "text/plain", []byte(content))
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

func TestChangePasswordKeepsCurrentSessionAndRevokesOthers(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "password.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(repository.New(db), config.Config{SessionTTL: time.Hour, MediaDir: filepath.Join(dir, "media")}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	first, err := svc.Register(ctx, "password_owner", "password123")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Login(ctx, "password_owner", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ChangePassword(ctx, first.User.ID, first.Token, "password123", "newpassword123"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, second.Token); err != domain.ErrInvalidSession {
		t.Fatalf("other session error = %v, want invalid session", err)
	}
	if _, err := svc.Authenticate(ctx, first.Token); err != nil {
		t.Fatalf("current session was revoked: %v", err)
	}
	if _, err := svc.Login(ctx, "password_owner", "password123"); err != domain.ErrUnauthorized {
		t.Fatalf("old password error = %v, want unauthorized", err)
	}
	if _, err := svc.Login(ctx, "password_owner", "newpassword123"); err != nil {
		t.Fatalf("new password login: %v", err)
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

func TestInteractionNotificationsAndSelfSuppression(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "notifications.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	svc := New(repo, config.Config{MediaDir: filepath.Join(dir, "media")}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "notification_owner", "hash")
	if err != nil {
		t.Fatal(err)
	}
	actor, err := repo.CreateUser(ctx, "notification_actor", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: owner.ID, Title: "Service notifications", Category: "knowledge",
		VideoPath: "videos/service-notifications.mp4", MimeType: "video/mp4", SizeBytes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if active, err := svc.ToggleFollow(ctx, actor.ID, owner.ID); err != nil || !active {
		t.Fatalf("follow: active=%v err=%v", active, err)
	}
	if active, err := svc.ToggleLike(ctx, actor.ID, video.ID); err != nil || !active {
		t.Fatalf("like: active=%v err=%v", active, err)
	}
	if active, err := svc.ToggleFavorite(ctx, actor.ID, video.ID); err != nil || !active {
		t.Fatalf("favorite: active=%v err=%v", active, err)
	}
	comment, err := svc.CreateComment(ctx, actor.ID, video.ID, "service notification comment")
	if err != nil {
		t.Fatal(err)
	}
	page, err := svc.Notifications(ctx, owner.ID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 || page.UnreadCount != 4 {
		t.Fatalf("notification totals = %#v, want four unread", page)
	}
	types := make(map[string]domain.Notification, len(page.Items))
	for _, item := range page.Items {
		types[item.Type] = item
	}
	for _, notificationType := range []string{"follow", "like", "favorite", "comment"} {
		if _, ok := types[notificationType]; !ok {
			t.Fatalf("missing %q notification in %#v", notificationType, page.Items)
		}
	}
	if types["comment"].CommentID != comment.ID || types["comment"].CommentPreview != comment.Content {
		t.Fatalf("comment notification = %#v", types["comment"])
	}

	if active, err := svc.ToggleFollow(ctx, actor.ID, owner.ID); err != nil || active {
		t.Fatalf("unfollow: active=%v err=%v", active, err)
	}
	if active, err := svc.ToggleLike(ctx, actor.ID, video.ID); err != nil || active {
		t.Fatalf("unlike: active=%v err=%v", active, err)
	}
	if active, err := svc.ToggleFavorite(ctx, actor.ID, video.ID); err != nil || active {
		t.Fatalf("unfavorite: active=%v err=%v", active, err)
	}
	if _, err := svc.ToggleLike(ctx, owner.ID, video.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ToggleFavorite(ctx, owner.ID, video.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateComment(ctx, owner.ID, video.ID, "owner comment"); err != nil {
		t.Fatal(err)
	}
	page, err = svc.Notifications(ctx, owner.ID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 {
		t.Fatalf("toggle-off or self interaction created notifications: %#v", page.Items)
	}

	if err := svc.MarkNotificationRead(ctx, actor.ID, page.Items[0].ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other user mark read error = %v, want not found", err)
	}
	if err := svc.MarkNotificationRead(ctx, owner.ID, page.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkAllNotificationsRead(ctx, owner.ID); err != nil {
		t.Fatal(err)
	}
	page, err = svc.Notifications(ctx, owner.ID, 1, 20)
	if err != nil || page.UnreadCount != 0 {
		t.Fatalf("mark all result = %#v err=%v", page, err)
	}
}

func TestUpdateProfileValidationAndAvatarLifecycle(t *testing.T) {
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "profile_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := repo.CreateUser(ctx, "profile_other", "hash")
	if err != nil {
		t.Fatal(err)
	}
	oldRelative := filepath.Join("avatars", "old.png")
	if _, err := repo.UpdateProfile(ctx, user.ID, user.Username, "old bio", &oldRelative); err != nil {
		t.Fatal(err)
	}
	oldAbsolute := filepath.Join(mediaDir, oldRelative)
	if err := os.MkdirAll(filepath.Dir(oldAbsolute), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldAbsolute, []byte("old avatar"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := New(repo, config.Config{MediaDir: mediaDir}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for name, input := range map[string]UpdateProfileInput{
		"invalid user":     {UserID: 0, Username: "valid_name"},
		"invalid username": {UserID: user.ID, Username: "x"},
		"bio too long":     {UserID: user.ID, Username: "valid_name", Bio: strings.Repeat("界", 301)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.UpdateProfile(ctx, input); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("error = %v, want invalid input", err)
			}
		})
	}
	if _, err := svc.UpdateProfile(ctx, UpdateProfileInput{
		UserID: user.ID, Username: "valid_name",
		Avatar: multipartFileHeader(t, "avatar", "avatar.txt", "text/plain", []byte("not an image")),
	}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid avatar error = %v, want invalid input", err)
	}

	updated, err := svc.UpdateProfile(ctx, UpdateProfileInput{
		UserID: user.ID, Username: "profile_renamed", Bio: "new bio",
		Avatar: multipartFileHeader(t, "avatar", "avatar.png", "image/png", []byte("\x89PNG\r\n\x1a\navatar")),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Username != "profile_renamed" || updated.Bio != "new bio" || !strings.HasPrefix(updated.AvatarURL, "/media/avatars/") {
		t.Fatalf("unexpected updated profile: %#v", updated)
	}
	if _, err := os.Stat(oldAbsolute); !os.IsNotExist(err) {
		t.Fatalf("old avatar should be removed, stat err=%v", err)
	}
	newAvatar := filepath.Join(mediaDir, filepath.FromSlash(strings.TrimPrefix(updated.AvatarURL, "/media/")))
	if _, err := os.Stat(newAvatar); err != nil {
		t.Fatalf("new avatar should exist: %v", err)
	}

	if _, err := svc.UpdateProfile(ctx, UpdateProfileInput{
		UserID: user.ID, Username: other.Username, Bio: "conflict",
		Avatar: multipartFileHeader(t, "avatar", "replacement.png", "image/png", []byte("\x89PNG\r\n\x1a\nreplacement")),
	}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("conflicting update error = %v, want conflict", err)
	}
	entries, err := os.ReadDir(filepath.Join(mediaDir, "avatars"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(newAvatar) {
		t.Fatalf("failed profile update left avatar files: %#v", entries)
	}
}

func TestPrivateVideoCommentsAndMediaAuthorization(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "private_owner", "hash")
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := repo.CreateUser(ctx, "private_viewer", "hash")
	if err != nil {
		t.Fatal(err)
	}
	privateVideo, err := repo.CreateVideoWithSubtitle(ctx, domain.NewVideo{
		UserID: owner.ID, Title: "Private video", Category: "knowledge", Visibility: "private",
		VideoPath: "videos/private.mp4", CoverPath: "covers/private.jpg", MimeType: "video/mp4", SizeBytes: 100,
	}, domain.NewSubtitle{Language: "en", Label: "English", Path: "subtitles/private/en.vtt", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE videos SET hls_master_path = 'hls/private/master.m3u8' WHERE id = ?`, privateVideo.ID); err != nil {
		t.Fatal(err)
	}
	svc := New(repo, config.Config{MediaDir: filepath.Join(dir, "media")}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if _, err := svc.Comments(ctx, privateVideo.ID, 0); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("anonymous private comments error = %v, want not found", err)
	}
	if _, err := svc.CreateComment(ctx, viewer.ID, privateVideo.ID, "no access"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("viewer private comment error = %v, want not found", err)
	}
	comment, err := svc.CreateComment(ctx, owner.ID, privateVideo.ID, "owner comment")
	if err != nil || comment.UserID != owner.ID {
		t.Fatalf("owner private comment = %#v err=%v", comment, err)
	}
	comments, err := svc.Comments(ctx, privateVideo.ID, owner.ID)
	if err != nil || len(comments) != 1 {
		t.Fatalf("owner private comments = %#v err=%v", comments, err)
	}

	for _, mediaPath := range []string{
		"videos/private.mp4", "covers/private.jpg", "subtitles/private/en.vtt", "hls/private/segment-001.ts",
	} {
		if err := svc.AuthorizeMedia(ctx, mediaPath, viewer.ID); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("viewer media %q error = %v, want not found", mediaPath, err)
		}
		if err := svc.AuthorizeMedia(ctx, mediaPath, owner.ID); err != nil {
			t.Fatalf("owner media %q: %v", mediaPath, err)
		}
	}
	if err := svc.AuthorizeMedia(ctx, "avatars/public.png", 0); err != nil {
		t.Fatalf("avatar should be public: %v", err)
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

func TestSubtitleManagementAndFileCleanup(t *testing.T) {
	dir := t.TempDir()
	mediaDir := filepath.Join(dir, "media")
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	author, err := repo.CreateUser(ctx, "subtitle_manager", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := repo.CreateUser(ctx, "subtitle_outsider", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideoWithSubtitle(ctx, domain.NewVideo{
		UserID: author.ID, Title: "Subtitle management", Category: "knowledge",
		VideoPath: "videos/subtitle-management.mp4", MimeType: "video/mp4", SizeBytes: 100,
	}, domain.NewSubtitle{
		Language: "zh-CN", Label: "Chinese", Path: "subtitles/first/zh-CN.vtt", IsDefault: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateSubtitle(ctx, video.ID, "en", "English", "subtitles/second/en.vtt", false)
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"subtitles/first/zh-CN.vtt": "WEBVTT\n",
		"subtitles/second/en.vtt":   "WEBVTT\n",
	} {
		absolute := filepath.Join(mediaDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	svc := New(repo, config.Config{MediaDir: mediaDir}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if _, err := svc.SetDefaultSubtitle(ctx, other.ID, video.ID, second.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("non-author set default error=%v", err)
	}
	if _, err := svc.DeleteSubtitle(ctx, other.ID, video.ID, second.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("non-author delete error=%v", err)
	}
	if _, err := svc.SetDefaultSubtitle(ctx, author.ID, video.ID, 99999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing subtitle error=%v", err)
	}
	tracks, err := svc.SetDefaultSubtitle(ctx, author.ID, video.ID, second.ID)
	if err != nil || len(tracks) != 2 || tracks[0].ID != second.ID || !tracks[0].IsDefault {
		t.Fatalf("set default tracks=%#v err=%v", tracks, err)
	}
	tracks, err = svc.DeleteSubtitle(ctx, author.ID, video.ID, second.ID)
	if err != nil || len(tracks) != 1 || !tracks[0].IsDefault || tracks[0].Language != "zh-CN" {
		t.Fatalf("delete default tracks=%#v err=%v", tracks, err)
	}
	if _, err := os.Stat(filepath.Join(mediaDir, "subtitles", "second", "en.vtt")); !os.IsNotExist(err) {
		t.Fatalf("deleted subtitle file still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(mediaDir, "subtitles", "second")); !os.IsNotExist(err) {
		t.Fatalf("empty subtitle directory still exists: %v", err)
	}
	tracks, err = svc.DeleteSubtitle(ctx, author.ID, video.ID, tracks[0].ID)
	if err != nil || len(tracks) != 0 {
		t.Fatalf("delete last subtitle tracks=%#v err=%v", tracks, err)
	}
	if _, err := os.Stat(filepath.Join(mediaDir, "subtitles", "first")); !os.IsNotExist(err) {
		t.Fatalf("last subtitle directory still exists: %v", err)
	}
}

func TestPublicVideoSanitizesProcessingFailure(t *testing.T) {
	video := publicVideo(domain.Video{
		ProcessingStatus: "failed",
		ProcessingError:  `C:\Users\private\video.mp4: ffmpeg invalid data found when processing input`,
	})
	if video.ProcessingMessage == "" || strings.Contains(video.ProcessingMessage, `C:\`) || strings.Contains(strings.ToLower(video.ProcessingMessage), "ffmpeg") {
		t.Fatalf("unsafe processing message: %q", video.ProcessingMessage)
	}
	payload, err := json.Marshal(video)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `C:\\Users`) || strings.Contains(strings.ToLower(string(payload)), "ffmpeg") {
		t.Fatalf("serialized video exposed internal error: %s", payload)
	}
	timeout := publicVideo(domain.Video{ProcessingStatus: "failed", ProcessingError: "transcode timeout"})
	if !strings.Contains(timeout.ProcessingMessage, "超时") {
		t.Fatalf("timeout message is not actionable: %q", timeout.ProcessingMessage)
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
	if _, err := svc.UpdateVideo(ctx, UpdateVideoInput{
		UserID: author.ID, VideoID: video.ID, Title: "Invalid visibility", Category: "知识", Visibility: "friends",
	}); err != domain.ErrInvalidInput {
		t.Fatalf("expected invalid visibility error, got %v", err)
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
