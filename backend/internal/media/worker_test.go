package media

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
)

type fakeProber struct {
	metadata domain.MediaMetadata
	err      error
	onProbe  func()
}

type fakeTranscoder struct {
	enabled     bool
	path        string
	err         error
	onTranscode func()
}

func (t fakeTranscoder) Enabled() bool { return t.enabled }

func (t fakeTranscoder) Transcode(context.Context, string, int64, domain.MediaMetadata) (string, error) {
	if t.onTranscode != nil {
		t.onTranscode()
	}
	return t.path, t.err
}

func (p fakeProber) Probe(context.Context, string) (domain.MediaMetadata, error) {
	if p.onProbe != nil {
		p.onProbe()
	}
	return p.metadata, p.err
}

func TestWorkerCompletesMediaProbeJob(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "worker_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Worker video", Category: "knowledge", VideoPath: "videos/test.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertStage := func(progress int, stage string) {
		t.Helper()
		current, err := repo.VideoByID(ctx, video.ID, user.ID)
		if err != nil || current.ProcessingProgress != progress || current.ProcessingStage != stage {
			t.Fatalf("processing stage=%#v err=%v, want %d/%s", current, err, progress, stage)
		}
	}
	worker := NewWorker(repo, fakeProber{
		metadata: domain.MediaMetadata{
			DurationSeconds: 8.5, Width: 1280, Height: 720, Bitrate: 1200000, VideoCodec: "h264", AudioCodec: "aac",
		},
		onProbe: func() { assertStage(10, "probing") },
	}, fakeTranscoder{
		enabled: true, path: "hls/1/master.m3u8",
		onTranscode: func() { assertStage(35, "transcoding") },
	}, t.TempDir(), time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	processed, err := worker.processOne(ctx)
	if err != nil || !processed {
		t.Fatalf("process job: processed=%v err=%v", processed, err)
	}
	updated, err := repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProcessingStatus != "ready" || updated.ProcessingProgress != 100 || updated.ProcessingStage != "ready" ||
		updated.SourceWidth != 1280 || updated.SourceHeight != 720 || updated.DurationSeconds != 8.5 || updated.HLSURL != "/media/hls/1/master.m3u8" {
		t.Fatalf("unexpected processed video: %#v", updated)
	}
}

func TestWorkerRetriesThenFailsWithProgressState(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "worker_retry", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Retry video", Category: "knowledge",
		VideoPath: "videos/retry.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(repo, fakeProber{err: context.DeadlineExceeded}, fakeTranscoder{}, t.TempDir(), time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	worker.maxAttempts = 2
	if processed, err := worker.processOne(ctx); err != nil || !processed {
		t.Fatalf("first attempt processed=%v err=%v", processed, err)
	}
	current, err := repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || current.ProcessingStatus != "pending" || current.ProcessingProgress != 0 || current.ProcessingStage != "queued" {
		t.Fatalf("retry state: %#v err=%v", current, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE transcoding_jobs SET available_at = CURRENT_TIMESTAMP WHERE video_id = ?`, video.ID); err != nil {
		t.Fatal(err)
	}
	if processed, err := worker.processOne(ctx); err != nil || !processed {
		t.Fatalf("second attempt processed=%v err=%v", processed, err)
	}
	current, err = repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || current.ProcessingStatus != "failed" || current.ProcessingProgress != 10 || current.ProcessingStage != "failed" {
		t.Fatalf("final failure state: %#v err=%v", current, err)
	}
}

func TestWorkerTranscodeFailurePreservesTranscodingProgress(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "worker_transcode_failure", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Transcode failure", Category: "knowledge",
		VideoPath: "videos/transcode-failure.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(repo, fakeProber{metadata: domain.MediaMetadata{Width: 640, Height: 360}},
		fakeTranscoder{err: context.DeadlineExceeded}, t.TempDir(), time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	worker.maxAttempts = 1
	if processed, err := worker.processOne(ctx); err != nil || !processed {
		t.Fatalf("process transcode failure: processed=%v err=%v", processed, err)
	}
	current, err := repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || current.ProcessingStatus != "failed" || current.ProcessingProgress != 35 || current.ProcessingStage != "failed" {
		t.Fatalf("transcode failure state: %#v err=%v", current, err)
	}
}

func TestWorkerPersistsFinalizingBeforeCompletion(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "worker_finalizing", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Finalizing", Category: "knowledge",
		VideoPath: "videos/finalizing.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
CREATE TRIGGER block_ready_update
BEFORE UPDATE OF processing_status ON videos
WHEN NEW.processing_status = 'ready'
BEGIN
  SELECT RAISE(ABORT, 'blocked completion');
END;`); err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(repo, fakeProber{metadata: domain.MediaMetadata{Width: 640, Height: 360}},
		fakeTranscoder{path: "hls/final/master.m3u8"}, t.TempDir(), time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if processed, err := worker.processOne(ctx); err == nil || !processed {
		t.Fatalf("completion should fail after finalizing: processed=%v err=%v", processed, err)
	}
	current, err := repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil || current.ProcessingStatus != "processing" || current.ProcessingProgress != 90 || current.ProcessingStage != "finalizing" {
		t.Fatalf("finalizing state: %#v err=%v", current, err)
	}
}

func TestWorkerWithRealFFprobe(t *testing.T) {
	mediaFile := os.Getenv("FFPROBE_INTEGRATION_FILE")
	executable := os.Getenv("FFPROBE_INTEGRATION_PATH")
	if mediaFile == "" || executable == "" {
		t.Skip("set FFPROBE_INTEGRATION_FILE and FFPROBE_INTEGRATION_PATH to run the real worker test")
	}
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "integration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "real_worker", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Real probe video", Category: "knowledge", VideoPath: filepath.Base(mediaFile), MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(repo, NewFFprobe(executable, 15*time.Second), fakeTranscoder{}, filepath.Dir(mediaFile), time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	processed, err := worker.processOne(ctx)
	if err != nil || !processed {
		t.Fatalf("process real job: processed=%v err=%v", processed, err)
	}
	updated, err := repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProcessingStatus != "ready" || updated.SourceWidth <= 0 || updated.SourceHeight <= 0 || updated.VideoCodec == "" {
		t.Fatalf("real worker did not persist metadata: %#v", updated)
	}
	t.Logf("persisted status=%s resolution=%dx%d video=%s audio=%s", updated.ProcessingStatus, updated.SourceWidth, updated.SourceHeight, updated.VideoCodec, updated.AudioCodec)
}

func TestResolveMediaPathSupportsWindowsStoredPaths(t *testing.T) {
	root := t.TempDir()
	resolved, err := resolveMediaPath(root, `videos\legacy.mp4`)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "videos", "legacy.mp4")
	if resolved != want {
		t.Fatalf("resolved = %q, want %q", resolved, want)
	}
	if _, err := resolveMediaPath(root, `..\outside.mp4`); err == nil {
		t.Fatal("expected traversal path rejection")
	}
}
