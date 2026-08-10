package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform"
)

func TestVideoInteractions(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := New(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "tester", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideoWithSubtitle(ctx, domain.NewVideo{
		UserID: user.ID, Title: "A test video", Description: "desc", Category: "知识",
		VideoPath: "videos/test.mp4", MimeType: "video/mp4", SizeBytes: 100,
	}, domain.NewSubtitle{Language: "zh-CN", Label: "中文", Path: "subtitles/1/zh-CN.vtt", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	if video.ProcessingStatus != "pending" {
		t.Fatalf("expected pending media processing, got %q", video.ProcessingStatus)
	}
	job, found, err := repo.ClaimTranscodingJob(ctx)
	if err != nil || !found || job.VideoID != video.ID || job.Attempts != 1 {
		t.Fatalf("claim media job: %#v found=%v err=%v", job, found, err)
	}
	if err := repo.CompleteTranscodingJob(ctx, job.ID, video.ID, domain.MediaOutput{
		Metadata:      domain.MediaMetadata{DurationSeconds: 15.5, Width: 1920, Height: 1080, Bitrate: 4000000, VideoCodec: "h264", AudioCodec: "aac"},
		HLSMasterPath: "hls/1/master.m3u8",
	}); err != nil {
		t.Fatal(err)
	}
	video, err = repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || video.ProcessingStatus != "ready" || video.SourceWidth != 1920 || video.VideoCodec != "h264" || video.HLSURL != "/media/hls/1/master.m3u8" {
		t.Fatalf("read media metadata: %#v err=%v", video, err)
	}
	video, err = repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || len(video.SubtitleTracks) != 1 || video.SubtitleTracks[0].Language != "zh-CN" || video.SubtitleTracks[0].URL != "/media/subtitles/1/zh-CN.vtt" || !video.SubtitleTracks[0].IsDefault {
		t.Fatalf("read subtitle tracks: %#v err=%v", video.SubtitleTracks, err)
	}
	if _, err := repo.CreateSubtitle(ctx, video.ID, "zh-CN", "中文（二）", "subtitles/1/zh-CN-2.vtt", false); err != domain.ErrSubtitleExists {
		t.Fatalf("expected duplicate subtitle error, got %v", err)
	}

	active, err := repo.ToggleLike(ctx, user.ID, video.ID)
	if err != nil || !active {
		t.Fatalf("toggle like on: active=%v err=%v", active, err)
	}
	active, err = repo.ToggleLike(ctx, user.ID, video.ID)
	if err != nil || active {
		t.Fatalf("toggle like off: active=%v err=%v", active, err)
	}
	comment, err := repo.CreateComment(ctx, user.ID, video.ID, "useful")
	if err != nil || comment.Content != "useful" {
		t.Fatalf("create comment: %#v err=%v", comment, err)
	}

	if err := repo.CreateSession(ctx, "hash-token", user.ID, "csrf", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	session, err := repo.SessionByHash(ctx, "hash-token")
	if err != nil || session.User.ID != user.ID || session.CSRFToken != "csrf" {
		t.Fatalf("read session: %#v err=%v", session, err)
	}
}

func TestCreatorProfilesFollowToggleAndFollowingVideos(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := New(db)
	ctx := context.Background()

	viewer, err := repo.CreateUser(ctx, "follow_viewer", "hash")
	if err != nil {
		t.Fatal(err)
	}
	author, err := repo.CreateUser(ctx, "follow_author", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := repo.CreateUser(ctx, "follow_other", "hash")
	if err != nil {
		t.Fatal(err)
	}
	authorVideo, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: author.ID, Title: "Followed author video", Category: "知识",
		VideoPath: "videos/followed.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: other.ID, Title: "Other author video", Category: "知识",
		VideoPath: "videos/other.mp4", MimeType: "video/mp4", SizeBytes: 100,
	}); err != nil {
		t.Fatal(err)
	}

	active, err := repo.ToggleFollow(ctx, viewer.ID, author.ID)
	if err != nil || !active {
		t.Fatalf("toggle follow on: active=%v err=%v", active, err)
	}
	profile, err := repo.CreatorProfile(ctx, author.ID, viewer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !profile.Followed || profile.FollowersCount != 1 || profile.FollowingCount != 0 || profile.VideosCount != 1 {
		t.Fatalf("unexpected author profile: %#v", profile)
	}
	viewerProfile, err := repo.CreatorProfile(ctx, viewer.ID, author.ID)
	if err != nil {
		t.Fatal(err)
	}
	if viewerProfile.Followed || viewerProfile.FollowersCount != 0 || viewerProfile.FollowingCount != 1 {
		t.Fatalf("unexpected viewer profile: %#v", viewerProfile)
	}

	items, err := repo.ListVideos(ctx, domain.VideoFilter{FollowingUserID: viewer.ID, Limit: 10}, viewer.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != authorVideo.ID || items[0].UserID != author.ID {
		t.Fatalf("unexpected following videos: %#v", items)
	}
	count, err := repo.CountVideos(ctx, domain.VideoFilter{FollowingUserID: viewer.ID})
	if err != nil || count != 1 {
		t.Fatalf("following video count=%d err=%v", count, err)
	}

	active, err = repo.ToggleFollow(ctx, viewer.ID, author.ID)
	if err != nil || active {
		t.Fatalf("toggle follow off: active=%v err=%v", active, err)
	}
	profile, err = repo.CreatorProfile(ctx, author.ID, viewer.ID)
	if err != nil || profile.Followed || profile.FollowersCount != 0 {
		t.Fatalf("profile after unfollow: %#v err=%v", profile, err)
	}
	items, err = repo.ListVideos(ctx, domain.VideoFilter{FollowingUserID: viewer.ID, Limit: 10}, viewer.ID)
	if err != nil || len(items) != 0 {
		t.Fatalf("following videos after unfollow: %#v err=%v", items, err)
	}
}

func TestRecoverTranscodingJobsQueuesLegacyMetadata(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "legacy_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Legacy metadata", Category: "knowledge", VideoPath: "videos/legacy.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	job, found, err := repo.ClaimTranscodingJob(ctx)
	if err != nil || !found {
		t.Fatalf("claim initial job: found=%v err=%v", found, err)
	}
	if err := repo.CompleteTranscodingJob(ctx, job.ID, video.ID, domain.MediaOutput{Metadata: domain.MediaMetadata{DurationSeconds: 2}}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM transcoding_jobs WHERE video_id = ?`, video.ID); err != nil {
		t.Fatal(err)
	}
	queued, err := repo.RecoverTranscodingJobs(ctx)
	if err != nil || queued != 1 {
		t.Fatalf("queue legacy job: count=%d err=%v", queued, err)
	}
	job, found, err = repo.ClaimTranscodingJob(ctx)
	if err != nil || !found || job.VideoID != video.ID {
		t.Fatalf("claim legacy job: %#v found=%v err=%v", job, found, err)
	}
}

func TestRecoverTranscodingJobsQueuesMissingHLS(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "hls_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{UserID: user.ID, Title: "Missing HLS", Category: "knowledge", VideoPath: "videos/hls.mp4", MimeType: "video/mp4", SizeBytes: 100})
	if err != nil {
		t.Fatal(err)
	}
	job, found, err := repo.ClaimTranscodingJob(ctx)
	if err != nil || !found {
		t.Fatalf("claim initial job: found=%v err=%v", found, err)
	}
	if err := repo.CompleteTranscodingJob(ctx, job.ID, video.ID, domain.MediaOutput{Metadata: domain.MediaMetadata{DurationSeconds: 2, Width: 640, Height: 360, VideoCodec: "h264"}}); err != nil {
		t.Fatal(err)
	}
	queued, err := repo.RecoverTranscodingJobs(ctx, true)
	if err != nil || queued != 1 {
		t.Fatalf("queue missing HLS: count=%d err=%v", queued, err)
	}
	job, found, err = repo.ClaimTranscodingJob(ctx)
	if err != nil || !found || job.VideoID != video.ID {
		t.Fatalf("claim missing HLS job: %#v found=%v err=%v", job, found, err)
	}
}
