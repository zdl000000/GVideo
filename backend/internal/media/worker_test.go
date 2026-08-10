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
}

type fakeTranscoder struct {
	enabled bool
	path    string
	err     error
}

func (t fakeTranscoder) Enabled() bool { return t.enabled }

func (t fakeTranscoder) Transcode(context.Context, string, int64, domain.MediaMetadata) (string, error) {
	return t.path, t.err
}

func (p fakeProber) Probe(context.Context, string) (domain.MediaMetadata, error) {
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
	worker := NewWorker(repo, fakeProber{metadata: domain.MediaMetadata{
		DurationSeconds: 8.5, Width: 1280, Height: 720, Bitrate: 1200000, VideoCodec: "h264", AudioCodec: "aac",
	}}, fakeTranscoder{enabled: true, path: "hls/1/master.m3u8"}, t.TempDir(), time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	processed, err := worker.processOne(ctx)
	if err != nil || !processed {
		t.Fatalf("process job: processed=%v err=%v", processed, err)
	}
	updated, err := repo.VideoByID(ctx, video.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ProcessingStatus != "ready" || updated.SourceWidth != 1280 || updated.SourceHeight != 720 || updated.DurationSeconds != 8.5 || updated.HLSURL != "/media/hls/1/master.m3u8" {
		t.Fatalf("unexpected processed video: %#v", updated)
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
