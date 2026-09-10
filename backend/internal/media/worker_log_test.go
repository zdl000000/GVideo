package media

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
)

func TestWorkerStructuredJobLogs(t *testing.T) {
	tests := []struct {
		name        string
		prober      fakeProber
		transcoder  fakeTranscoder
		maxAttempts int
		wantEvent   string
		wantStage   string
		wantRetry   bool
		wantClass   string
	}{
		{
			name:        "completed",
			prober:      fakeProber{metadata: domain.MediaMetadata{DurationSeconds: 2, Width: 640, Height: 360}},
			transcoder:  fakeTranscoder{enabled: true, path: "hls/output/master.m3u8"},
			maxAttempts: 3, wantEvent: "media_job_completed", wantStage: "complete",
		},
		{
			name:       "probe timeout retries",
			prober:     fakeProber{err: context.DeadlineExceeded},
			transcoder: fakeTranscoder{}, maxAttempts: 3,
			wantEvent: "media_job_failed", wantStage: "probe", wantRetry: true, wantClass: "probe_timeout",
		},
		{
			name:        "probe failure final",
			prober:      fakeProber{err: errors.New("probe failed with token=log-canary")},
			transcoder:  fakeTranscoder{},
			maxAttempts: 1,
			wantEvent:   "media_job_failed",
			wantStage:   "probe",
			wantClass:   "probe_failed",
		},
		{
			name:        "transcode timeout final",
			prober:      fakeProber{metadata: domain.MediaMetadata{DurationSeconds: 2}},
			transcoder:  fakeTranscoder{enabled: true, err: context.DeadlineExceeded},
			maxAttempts: 1,
			wantEvent:   "media_job_failed",
			wantStage:   "transcode",
			wantClass:   "transcode_timeout",
		},
		{
			name:        "transcode failure final",
			prober:      fakeProber{metadata: domain.MediaMetadata{DurationSeconds: 2}},
			transcoder:  fakeTranscoder{enabled: true, err: errors.New(`ffmpeg -i C:\secret\upload.mp4 token=log-canary password=log-canary stderr=private-upload-content`)},
			maxAttempts: 1, wantEvent: "media_job_failed", wantStage: "transcode", wantClass: "transcode_failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "worker-log.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			repo := repository.New(db)
			ctx := context.Background()
			user, err := repo.CreateUser(ctx, "worker_log_user", "hash")
			if err != nil {
				t.Fatal(err)
			}
			video, err := repo.CreateVideo(ctx, domain.NewVideo{
				UserID: user.ID, Title: "Log video", Category: "knowledge",
				VideoPath: "videos/input.mp4", MimeType: "video/mp4", SizeBytes: 100,
			})
			if err != nil {
				t.Fatal(err)
			}

			var output bytes.Buffer
			worker := NewWorker(repo, test.prober, test.transcoder, t.TempDir(), time.Millisecond, slog.New(slog.NewJSONHandler(&output, nil)))
			worker.maxAttempts = test.maxAttempts
			processed, err := worker.processOne(ctx)
			if err != nil || !processed {
				t.Fatalf("processOne() processed=%v error=%v", processed, err)
			}

			records := decodeWorkerLogs(t, output.Bytes())
			if len(records) != 2 {
				t.Fatalf("records=%d, want started and terminal: %s", len(records), output.String())
			}
			started := records[0]
			terminal := records[1]
			assertWorkerLog(t, started, "media_job_started", "claimed", false, video.ID)
			assertWorkerLog(t, terminal, test.wantEvent, test.wantStage, test.wantRetry, video.ID)
			if got := terminal["error_class"]; got != nil && got != test.wantClass {
				t.Fatalf("error_class=%v, want %q; record=%#v", got, test.wantClass, terminal)
			}
			if test.wantClass != "" && terminal["error_class"] != test.wantClass {
				t.Fatalf("error_class=%v, want %q", terminal["error_class"], test.wantClass)
			}
			lower := strings.ToLower(output.String())
			for _, forbidden := range []string{"log-canary", "private-upload-content", "ffmpeg -i", `c:\\secret`, "hls_master_path", `"error":`, `"path":`, `"command":`, `"args":`, `"stderr":`} {
				if strings.Contains(lower, strings.ToLower(forbidden)) {
					t.Fatalf("worker log contains forbidden data %q: %s", forbidden, output.String())
				}
			}
		})
	}
}

func TestWorkerResolveFailureHasSafeClass(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "resolve-log.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "resolve_log_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{UserID: user.ID, Title: "Traversal", Category: "knowledge", VideoPath: "../private.mp4", MimeType: "video/mp4", SizeBytes: 1})
	if err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	worker := NewWorker(repo, fakeProber{}, fakeTranscoder{}, t.TempDir(), time.Millisecond, slog.New(slog.NewJSONHandler(&output, nil)))
	processed, err := worker.processOne(ctx)
	if err != nil || !processed {
		t.Fatalf("processOne() processed=%v error=%v", processed, err)
	}
	records := decodeWorkerLogs(t, output.Bytes())
	terminal := records[len(records)-1]
	assertWorkerLog(t, terminal, "media_job_failed", "resolve", false, video.ID)
	if terminal["error_class"] != "invalid_media_path" {
		t.Fatalf("error_class=%v", terminal["error_class"])
	}
	if strings.Contains(output.String(), "private.mp4") {
		t.Fatalf("resolve log leaked path: %s", output.String())
	}
}

func decodeWorkerLogs(t *testing.T, data []byte) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			t.Fatalf("decode worker log: %v; line=%s", err, line)
		}
		records = append(records, record)
	}
	return records
}

func assertWorkerLog(t *testing.T, record map[string]any, event, stage string, retry bool, videoID int64) {
	t.Helper()
	if record["event"] != event || record["stage"] != stage || record["retry"] != retry {
		t.Fatalf("event/stage/retry mismatch: %#v", record)
	}
	for _, key := range []string{"job_id", "video_id", "attempt", "duration_ms"} {
		value, ok := record[key].(float64)
		if !ok || value < 0 {
			t.Fatalf("%s=%v, want non-negative number", key, record[key])
		}
	}
	if record["job_id"].(float64) < 1 || record["attempt"].(float64) != 1 {
		t.Fatalf("job_id/attempt mismatch: %#v", record)
	}
	if int64(record["video_id"].(float64)) != videoID {
		t.Fatalf("video_id=%v, want %d", record["video_id"], videoID)
	}
}

func TestWorkerInterruptedJobLogs(t *testing.T) {
	tests := []struct {
		name       string
		stage      string
		buildFakes func(context.CancelFunc) (fakeProber, fakeTranscoder)
	}{
		{
			name:  "probe",
			stage: "probe",
			buildFakes: func(cancel context.CancelFunc) (fakeProber, fakeTranscoder) {
				return fakeProber{err: context.Canceled, onProbe: cancel}, fakeTranscoder{}
			},
		},
		{
			name:  "transcode",
			stage: "transcode",
			buildFakes: func(cancel context.CancelFunc) (fakeProber, fakeTranscoder) {
				return fakeProber{metadata: domain.MediaMetadata{DurationSeconds: 1}}, fakeTranscoder{enabled: true, err: context.Canceled, onTranscode: cancel}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, repo, video := createLoggedWorkerJob(t, "interrupt_"+test.name, "videos/input.mp4")
			defer db.Close()
			ctx, cancel := context.WithCancel(context.Background())
			prober, transcoder := test.buildFakes(cancel)
			var output bytes.Buffer
			worker := NewWorker(repo, prober, transcoder, t.TempDir(), time.Millisecond, slog.New(slog.NewJSONHandler(&output, nil)))
			processed, err := worker.processOne(ctx)
			if !processed || !errors.Is(err, context.Canceled) {
				t.Fatalf("processOne() processed=%v error=%v", processed, err)
			}
			records := decodeWorkerLogs(t, output.Bytes())
			terminal := records[len(records)-1]
			assertWorkerLog(t, terminal, "media_job_interrupted", test.stage, false, video.ID)
			if terminal["error_class"] != "context_canceled" {
				t.Fatalf("error_class=%v", terminal["error_class"])
			}
		})
	}
}

func TestWorkerStorageFailureLogsSafeContract(t *testing.T) {
	tests := []struct {
		name        string
		triggerSQL  string
		videoPath   string
		prober      fakeProber
		transcoder  fakeTranscoder
		maxAttempts int
		wantStage   string
		wantRetry   bool
	}{
		{
			name: "resolve failure persistence",
			triggerSQL: `CREATE TRIGGER reject_failed_job BEFORE UPDATE ON transcoding_jobs
				WHEN NEW.status = 'failed' BEGIN SELECT RAISE(FAIL, 'C:\secret\db token=storage-canary'); END`,
			videoPath: "../outside.mp4", wantStage: "resolve",
		},
		{
			name: "probe failure persistence",
			triggerSQL: `CREATE TRIGGER reject_pending_job BEFORE UPDATE ON transcoding_jobs
				WHEN NEW.status = 'pending' BEGIN SELECT RAISE(FAIL, 'C:\secret\db token=storage-canary'); END`,
			videoPath: "videos/input.mp4", prober: fakeProber{err: errors.New("probe failure")}, maxAttempts: 2, wantStage: "probe", wantRetry: true,
		},
		{
			name: "transcode progress persistence",
			triggerSQL: `CREATE TRIGGER reject_progress_35 BEFORE UPDATE ON videos
				WHEN NEW.processing_progress = 35 BEGIN SELECT RAISE(FAIL, 'C:\secret\db token=storage-canary'); END`,
			videoPath: "videos/input.mp4", prober: fakeProber{metadata: domain.MediaMetadata{DurationSeconds: 1}}, wantStage: "transcode",
		},
		{
			name: "transcode failure persistence",
			triggerSQL: `CREATE TRIGGER reject_failed_transcode BEFORE UPDATE ON transcoding_jobs
				WHEN NEW.status = 'failed' BEGIN SELECT RAISE(FAIL, 'C:\secret\db token=storage-canary'); END`,
			videoPath: "videos/input.mp4", prober: fakeProber{metadata: domain.MediaMetadata{DurationSeconds: 1}}, transcoder: fakeTranscoder{enabled: true, err: errors.New("transcode failure")}, wantStage: "transcode",
		},
		{
			name: "finalize persistence",
			triggerSQL: `CREATE TRIGGER reject_progress_90 BEFORE UPDATE ON videos
				WHEN NEW.processing_progress = 90 BEGIN SELECT RAISE(FAIL, 'C:\secret\db token=storage-canary'); END`,
			videoPath: "videos/input.mp4", prober: fakeProber{metadata: domain.MediaMetadata{DurationSeconds: 1}}, transcoder: fakeTranscoder{enabled: true, path: "hls/master.m3u8"}, wantStage: "finalize",
		},
		{
			name: "complete persistence",
			triggerSQL: `CREATE TRIGGER reject_progress_100 BEFORE UPDATE ON videos
				WHEN NEW.processing_progress = 100 BEGIN SELECT RAISE(FAIL, 'C:\secret\db token=storage-canary'); END`,
			videoPath: "videos/input.mp4", prober: fakeProber{metadata: domain.MediaMetadata{DurationSeconds: 1}}, transcoder: fakeTranscoder{enabled: true, path: "hls/master.m3u8"}, wantStage: "complete",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, repo, video := createLoggedWorkerJob(t, "storage_"+strings.ReplaceAll(test.name, " ", "_"), test.videoPath)
			defer db.Close()
			if _, err := db.Exec(test.triggerSQL); err != nil {
				t.Fatal(err)
			}
			var output bytes.Buffer
			worker := NewWorker(repo, test.prober, test.transcoder, t.TempDir(), time.Millisecond, slog.New(slog.NewJSONHandler(&output, nil)))
			worker.maxAttempts = test.maxAttempts
			if worker.maxAttempts == 0 {
				worker.maxAttempts = 1
			}
			processed, err := worker.processOne(context.Background())
			if !processed || err == nil {
				t.Fatalf("processOne() processed=%v error=%v", processed, err)
			}
			records := decodeWorkerLogs(t, output.Bytes())
			terminal := records[len(records)-1]
			assertWorkerLog(t, terminal, "media_job_failed", test.wantStage, test.wantRetry, video.ID)
			if terminal["error_class"] != "storage_failed" {
				t.Fatalf("error_class=%v", terminal["error_class"])
			}
			if strings.Contains(output.String(), "storage-canary") || strings.Contains(output.String(), `C:\secret`) {
				t.Fatalf("storage error leaked: %s", output.String())
			}
		})
	}
}

func TestWorkerRecoverFailureUsesSafeSystemEvent(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "recover-log.db"))
	if err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	worker := NewWorker(repo, fakeProber{}, fakeTranscoder{}, t.TempDir(), time.Millisecond, slog.New(slog.NewJSONHandler(&output, nil)))
	worker.Run(context.Background())
	records := decodeWorkerLogs(t, output.Bytes())
	if len(records) != 1 || records[0]["event"] != "media_worker_error" || records[0]["stage"] != "recover" || records[0]["error_class"] != "storage_failed" {
		t.Fatalf("unexpected recover log: %#v", records)
	}
	if _, exists := records[0]["error"]; exists {
		t.Fatalf("raw error field present: %#v", records[0])
	}
}

func createLoggedWorkerJob(t *testing.T, username, videoPath string) (*sql.DB, *repository.Repository, domain.Video) {
	t.Helper()
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "worker-log-fixture.db"))
	if err != nil {
		t.Fatal(err)
	}
	repo := repository.New(db)
	user, err := repo.CreateUser(context.Background(), username, "hash")
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(context.Background(), domain.NewVideo{UserID: user.ID, Title: "Log fixture", Category: "knowledge", VideoPath: videoPath, MimeType: "video/mp4", SizeBytes: 1})
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	return db, repo, video
}
