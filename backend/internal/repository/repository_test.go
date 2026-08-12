package repository

import (
	"context"
	"errors"
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
	if video.ProcessingStatus != "pending" || video.ProcessingProgress != 0 || video.ProcessingStage != "queued" {
		t.Fatalf("expected queued media processing, got %#v", video)
	}
	job, found, err := repo.ClaimTranscodingJob(ctx)
	if err != nil || !found || job.VideoID != video.ID || job.Attempts != 1 {
		t.Fatalf("claim media job: %#v found=%v err=%v", job, found, err)
	}
	video, err = repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || video.ProcessingProgress != 10 || video.ProcessingStage != "probing" {
		t.Fatalf("claimed processing stage: %#v err=%v", video, err)
	}
	if err := repo.UpdateTranscodingProgress(ctx, video.ID, 35, "transcoding"); err != nil {
		t.Fatal(err)
	}
	if err := repo.CompleteTranscodingJob(ctx, job.ID, video.ID, domain.MediaOutput{
		Metadata:      domain.MediaMetadata{DurationSeconds: 15.5, Width: 1920, Height: 1080, Bitrate: 4000000, VideoCodec: "h264", AudioCodec: "aac"},
		HLSMasterPath: "hls/1/master.m3u8",
	}); err != nil {
		t.Fatal(err)
	}
	video, err = repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || video.ProcessingStatus != "ready" || video.ProcessingProgress != 100 || video.ProcessingStage != "ready" ||
		video.SourceWidth != 1920 || video.VideoCodec != "h264" || video.HLSURL != "/media/hls/1/master.m3u8" {
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

func TestTranscodingProgressRetryFailureAndCompletion(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "progress_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Progress", Category: "knowledge",
		VideoPath: "videos/progress.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	job, found, err := repo.ClaimTranscodingJob(ctx)
	if err != nil || !found {
		t.Fatalf("claim job: found=%v err=%v", found, err)
	}
	if err := repo.UpdateTranscodingProgress(ctx, video.ID, 35, "transcoding"); err != nil {
		t.Fatal(err)
	}
	retryAt := time.Now().Add(time.Minute)
	if err := repo.FailTranscodingJob(ctx, job.ID, video.ID, "temporary probe failure", &retryAt); err != nil {
		t.Fatal(err)
	}
	current, err := repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || current.ProcessingStatus != "pending" || current.ProcessingProgress != 0 || current.ProcessingStage != "queued" {
		t.Fatalf("retry state: %#v err=%v", current, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE transcoding_jobs SET available_at = CURRENT_TIMESTAMP WHERE video_id = ?`, video.ID); err != nil {
		t.Fatal(err)
	}
	job, found, err = repo.ClaimTranscodingJob(ctx)
	if err != nil || !found {
		t.Fatalf("claim retry: found=%v err=%v", found, err)
	}
	if err := repo.UpdateTranscodingProgress(ctx, video.ID, 90, "finalizing"); err != nil {
		t.Fatal(err)
	}
	if err := repo.FailTranscodingJob(ctx, job.ID, video.ID, `C:\private\video.mp4: ffmpeg failed`, nil); err != nil {
		t.Fatal(err)
	}
	current, err = repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || current.ProcessingStatus != "failed" || current.ProcessingProgress != 90 || current.ProcessingStage != "failed" {
		t.Fatalf("failed state: %#v err=%v", current, err)
	}
	if err := repo.RetryTranscoding(ctx, video.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	current, err = repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || current.ProcessingStatus != "pending" || current.ProcessingProgress != 0 || current.ProcessingStage != "queued" {
		t.Fatalf("manual retry state: %#v err=%v", current, err)
	}
	job, found, err = repo.ClaimTranscodingJob(ctx)
	if err != nil || !found {
		t.Fatalf("claim manual retry: found=%v err=%v", found, err)
	}
	if err := repo.UpdateTranscodingProgress(ctx, video.ID, 90, "finalizing"); err != nil {
		t.Fatal(err)
	}
	if err := repo.CompleteTranscodingJob(ctx, job.ID, video.ID, domain.MediaOutput{}); err != nil {
		t.Fatal(err)
	}
	current, err = repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || current.ProcessingStatus != "ready" || current.ProcessingProgress != 100 || current.ProcessingStage != "ready" {
		t.Fatalf("completed state: %#v err=%v", current, err)
	}
}

func TestRetryTranscodingKeepsFailedVideoWhenJobIsMissing(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "missing-retry-job.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "missing_retry_job", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Missing retry job", Category: "knowledge",
		VideoPath: "videos/missing-retry-job.mp4", MimeType: "video/mp4", SizeBytes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE videos SET processing_status = 'failed', processing_stage = 'failed' WHERE id = ?`, video.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM transcoding_jobs WHERE video_id = ?`, video.ID); err != nil {
		t.Fatal(err)
	}

	if err := repo.RetryTranscoding(ctx, video.ID, user.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("retry missing job error = %v, want not found", err)
	}
	current, err := repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.ProcessingStatus != "failed" || current.ProcessingStage != "failed" {
		t.Fatalf("video changed after missing-job retry: %#v", current)
	}
}

func TestTranscodingJobVideoPairingRollsBackOnMismatch(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "pairing.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "pairing_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	first, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "First", Category: "knowledge", VideoPath: "videos/first.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Second", Category: "knowledge", VideoPath: "videos/second.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	firstJob, found, err := repo.ClaimTranscodingJob(ctx)
	if err != nil || !found {
		t.Fatalf("claim first job: %#v found=%v err=%v", firstJob, found, err)
	}
	secondJob, found, err := repo.ClaimTranscodingJob(ctx)
	if err != nil || !found {
		t.Fatalf("claim second job: %#v found=%v err=%v", secondJob, found, err)
	}

	if err := repo.CompleteTranscodingJob(ctx, secondJob.ID, first.ID, domain.MediaOutput{}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("mismatched completion error=%v, want not found", err)
	}
	current, err := repo.VideoByID(ctx, first.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.ProcessingStatus != "processing" || current.ProcessingStage != "probing" {
		t.Fatalf("mismatched completion changed first video: %#v", current)
	}

	retryAt := time.Now().Add(time.Minute)
	if err := repo.FailTranscodingJob(ctx, firstJob.ID, second.ID, "wrong retry pair", &retryAt); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("mismatched retry error=%v, want not found", err)
	}
	current, err = repo.VideoByID(ctx, second.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.ProcessingStatus != "processing" || current.ProcessingStage != "probing" {
		t.Fatalf("mismatched retry changed second video: %#v", current)
	}
	if err := repo.FailTranscodingJob(ctx, firstJob.ID, second.ID, "wrong final pair", nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("mismatched final failure error=%v, want not found", err)
	}
	current, err = repo.VideoByID(ctx, second.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.ProcessingStatus != "processing" || current.ProcessingStage != "probing" {
		t.Fatalf("mismatched final failure changed second video: %#v", current)
	}
}

func TestSubtitleDefaultAndDeleteTransactions(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "subtitle_repo_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideoWithSubtitle(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Subtitles", Category: "knowledge",
		VideoPath: "videos/subtitles.mp4", MimeType: "video/mp4", SizeBytes: 100,
	}, domain.NewSubtitle{Language: "zh-CN", Label: "Chinese", Path: "subtitles/track-1/zh-CN.vtt", IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateSubtitle(ctx, video.ID, "en", "English", "subtitles/track-2/en.vtt", false)
	if err != nil {
		t.Fatal(err)
	}
	third, err := repo.CreateSubtitle(ctx, video.ID, "ja", "Japanese", "subtitles/track-3/ja.vtt", false)
	if err != nil {
		t.Fatal(err)
	}
	tracks, err := repo.SetDefaultSubtitle(ctx, video.ID, second.ID)
	if err != nil || len(tracks) != 3 || tracks[0].ID != second.ID || !tracks[0].IsDefault {
		t.Fatalf("set default tracks=%#v err=%v", tracks, err)
	}
	defaults := 0
	for _, track := range tracks {
		if track.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Fatalf("default subtitle count=%d, want 1", defaults)
	}
	if _, err := repo.SetDefaultSubtitle(ctx, video.ID, 99999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing default subtitle error=%v", err)
	}
	deletedPath, tracks, err := repo.DeleteSubtitle(ctx, video.ID, second.ID)
	if err != nil || deletedPath != "subtitles/track-2/en.vtt" || len(tracks) != 2 || !tracks[0].IsDefault || tracks[0].Language != "zh-CN" {
		t.Fatalf("delete default path=%q tracks=%#v err=%v", deletedPath, tracks, err)
	}
	if _, tracks, err = repo.DeleteSubtitle(ctx, video.ID, third.ID); err != nil || len(tracks) != 1 || !tracks[0].IsDefault {
		t.Fatalf("delete non-default tracks=%#v err=%v", tracks, err)
	}
	if _, tracks, err = repo.DeleteSubtitle(ctx, video.ID, tracks[0].ID); err != nil || len(tracks) != 0 {
		t.Fatalf("delete final subtitle tracks=%#v err=%v", tracks, err)
	}
	if _, _, err := repo.DeleteSubtitle(ctx, video.ID, 99999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing delete subtitle error=%v", err)
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

func TestProfileVisibilityStatsAndMediaAccess(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := New(db)
	ctx := context.Background()

	owner, err := repo.CreateUser(ctx, "profile_owner", "hash")
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := repo.CreateUser(ctx, "profile_viewer", "hash")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := repo.CreateUser(ctx, "empty_creator", "hash")
	if err != nil {
		t.Fatal(err)
	}
	avatarPath := `avatars\owner.png`
	owner, err = repo.UpdateProfile(ctx, owner.ID, "profile_owner_new", "creator bio", &avatarPath)
	if err != nil {
		t.Fatal(err)
	}
	if owner.Username != "profile_owner_new" || owner.Bio != "creator bio" || owner.AvatarURL != "/media/avatars/owner.png" {
		t.Fatalf("unexpected updated profile: %#v", owner)
	}
	if _, err := repo.UpdateProfile(ctx, viewer.ID, owner.Username, "", nil); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate username error = %v, want conflict", err)
	}
	if _, err := repo.UpdateProfile(ctx, 99999, "missing_user", "", nil); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing profile error = %v, want not found", err)
	}

	publicVideo, err := repo.CreateVideoWithSubtitle(ctx, domain.NewVideo{
		UserID: owner.ID, Title: "Public", Category: "knowledge", Visibility: "public",
		VideoPath: `videos\public.mp4`, CoverPath: `covers\public.jpg`, MimeType: "video/mp4", SizeBytes: 100,
	}, domain.NewSubtitle{Language: "en", Label: "English", Path: `subtitles\public\en.vtt`, IsDefault: true})
	if err != nil {
		t.Fatal(err)
	}
	unlistedVideo, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: owner.ID, Title: "Unlisted", Category: "knowledge", Visibility: "unlisted",
		VideoPath: "videos/unlisted.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	privateVideo, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: owner.ID, Title: "Private", Category: "knowledge", Visibility: "private",
		VideoPath: "videos/private.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
UPDATE videos SET views_count = CASE id WHEN ? THEN 10 WHEN ? THEN 20 ELSE 30 END
`, publicVideo.ID, unlistedVideo.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE videos SET hls_master_path = ? WHERE id = ?`, `hls\42\master.m3u8`, privateVideo.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE videos SET processing_status = 'ready' WHERE id = ?`, publicVideo.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ToggleFollow(ctx, viewer.ID, owner.ID); err != nil {
		t.Fatal(err)
	}
	for _, videoID := range []int64{publicVideo.ID, unlistedVideo.ID, privateVideo.ID} {
		if _, err := repo.ToggleLike(ctx, viewer.ID, videoID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.ToggleFavorite(ctx, viewer.ID, videoID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.CreateComment(ctx, viewer.ID, videoID, "comment"); err != nil {
			t.Fatal(err)
		}
	}

	stats, err := repo.CreatorStats(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.VideosCount != 3 || stats.FollowersCount != 1 || stats.ViewsCount != 60 ||
		stats.LikesCount != 3 || stats.FavoritesCount != 3 || stats.CommentsCount != 3 ||
		stats.PublicCount != 1 || stats.UnlistedCount != 1 || stats.PrivateCount != 1 ||
		stats.ProcessingCount != 2 || len(stats.RecentVideos) != 3 {
		t.Fatalf("unexpected creator stats: %#v", stats)
	}
	emptyStats, err := repo.CreatorStats(ctx, empty.ID)
	if err != nil {
		t.Fatal(err)
	}
	if emptyStats.VideosCount != 0 || emptyStats.ViewsCount != 0 || len(emptyStats.RecentVideos) != 0 {
		t.Fatalf("unexpected empty creator stats: %#v", emptyStats)
	}
	if _, err := repo.CreatorStats(ctx, 99999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing creator stats error = %v, want not found", err)
	}

	publicItems, err := repo.ListVideos(ctx, domain.VideoFilter{UserID: owner.ID, Limit: 10}, viewer.ID)
	if err != nil || len(publicItems) != 1 || publicItems[0].ID != publicVideo.ID {
		t.Fatalf("public list = %#v err=%v", publicItems, err)
	}
	ownerItems, err := repo.ListVideos(ctx, domain.VideoFilter{UserID: owner.ID, IncludeNonPublic: true, Limit: 10}, owner.ID)
	if err != nil || len(ownerItems) != 3 {
		t.Fatalf("owner list = %#v err=%v", ownerItems, err)
	}
	followingItems, err := repo.ListVideos(ctx, domain.VideoFilter{FollowingUserID: viewer.ID, Limit: 10}, viewer.ID)
	if err != nil || len(followingItems) != 1 || followingItems[0].ID != publicVideo.ID {
		t.Fatalf("following list = %#v err=%v", followingItems, err)
	}
	if _, err := repo.VideoByID(ctx, unlistedVideo.ID, viewer.ID); err != nil {
		t.Fatalf("unlisted detail should be accessible by link: %v", err)
	}
	if _, err := repo.VideoByID(ctx, privateVideo.ID, viewer.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("private detail error = %v, want not found", err)
	}
	if _, err := repo.VideoByID(ctx, privateVideo.ID, owner.ID); err != nil {
		t.Fatalf("owner private detail: %v", err)
	}

	for _, test := range []struct {
		path    string
		videoID int64
	}{
		{path: "videos/public.mp4", videoID: publicVideo.ID},
		{path: "covers/public.jpg", videoID: publicVideo.ID},
		{path: "subtitles/public/en.vtt", videoID: publicVideo.ID},
		{path: "hls/42/master.m3u8", videoID: privateVideo.ID},
		{path: "hls/42/segment-001.ts", videoID: privateVideo.ID},
	} {
		access, err := repo.MediaAccessByPath(ctx, test.path)
		if err != nil || access.VideoID != test.videoID || access.UserID != owner.ID {
			t.Fatalf("media access %q = %#v err=%v", test.path, access, err)
		}
	}
	if _, err := repo.MediaAccessByPath(ctx, "hls/420/segment.ts"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("HLS prefix collision error = %v, want not found", err)
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
