package media

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/repository"
)

// StatsRecorder is the consumer-defined narrow interface the worker uses to
// report job lifecycle events. Implementations must be concurrency-safe.
type StatsRecorder interface {
	MediaJobClaimed()
	MediaJobCompleted(duration time.Duration)
	MediaJobFailed(stage string)
}

type Worker struct {
	repo         *repository.Repository
	prober       Prober
	transcoder   Transcoder
	mediaDir     string
	pollInterval time.Duration
	maxAttempts  int
	logger       *slog.Logger
	stats        StatsRecorder
}

func NewWorker(repo *repository.Repository, prober Prober, transcoder Transcoder, mediaDir string, pollInterval time.Duration, logger *slog.Logger) *Worker {
	return &Worker{
		repo: repo, prober: prober, transcoder: transcoder, mediaDir: mediaDir, pollInterval: pollInterval,
		maxAttempts: 3, logger: logger,
	}
}

// WithStats attaches an optional StatsRecorder and returns the worker for chaining.
func (w *Worker) WithStats(stats StatsRecorder) *Worker {
	w.stats = stats
	return w
}

func (w *Worker) Run(ctx context.Context) {
	recovered, err := w.repo.RecoverTranscodingJobs(ctx, w.transcoder.Enabled())
	if err != nil {
		w.logger.Error("media worker operation failed",
			"event", "media_worker_error",
			"stage", "recover",
			"error_class", "storage_failed",
		)
		return
	}
	w.logger.Info("media worker started", "event", "media_worker_started", "recovered_jobs", recovered, "poll_interval", w.pollInterval.String())

	for {
		processed, err := w.processOne(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			w.logger.Error("media worker operation failed",
				"event", "media_worker_error",
				"stage", "claim_or_persist",
				"error_class", "storage_failed",
			)
		}
		if ctx.Err() != nil {
			w.logger.Info("media worker stopped", "event", "media_worker_stopped")
			return
		}
		if processed {
			continue
		}
		timer := time.NewTimer(w.pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
	}
}

func (w *Worker) processOne(ctx context.Context) (bool, error) {
	job, found, err := w.repo.ClaimTranscodingJob(ctx)
	if err != nil || !found {
		return false, err
	}
	startedAt := time.Now()
	if w.stats != nil {
		w.stats.MediaJobClaimed()
	}
	w.logJob(slog.LevelInfo, "media job started", "media_job_started", job.ID, job.VideoID, job.Attempts, "claimed", false, startedAt, "")
	inputPath, err := resolveMediaPath(w.mediaDir, job.VideoPath)
	if err != nil {
		if w.stats != nil {
			w.stats.MediaJobFailed("resolve")
		}
		if saveErr := w.repo.FailTranscodingJob(ctx, job.ID, job.VideoID, err.Error(), nil); saveErr != nil {
			w.logJob(slog.LevelWarn, "media job failed", "media_job_failed", job.ID, job.VideoID, job.Attempts, "resolve", false, startedAt, "storage_failed")
			return true, saveErr
		}
		w.logJob(slog.LevelWarn, "media job failed", "media_job_failed", job.ID, job.VideoID, job.Attempts, "resolve", false, startedAt, "invalid_media_path")
		return true, nil
	}
	metadata, err := w.prober.Probe(ctx, inputPath)
	if err != nil {
		if ctx.Err() != nil {
			w.logJob(slog.LevelInfo, "media job interrupted", "media_job_interrupted", job.ID, job.VideoID, job.Attempts, "probe", false, startedAt, "context_canceled")
			return true, ctx.Err()
		}
		var retryAt *time.Time
		if job.Attempts < w.maxAttempts {
			next := time.Now().Add(time.Duration(1<<(job.Attempts-1)) * 5 * time.Second)
			retryAt = &next
		}
		if w.stats != nil {
			w.stats.MediaJobFailed("probe")
		}
		if saveErr := w.repo.FailTranscodingJob(ctx, job.ID, job.VideoID, err.Error(), retryAt); saveErr != nil {
			w.logJob(slog.LevelWarn, "media job failed", "media_job_failed", job.ID, job.VideoID, job.Attempts, "probe", false, startedAt, "storage_failed")
			return true, saveErr
		}
		w.logJob(slog.LevelWarn, "media job failed", "media_job_failed", job.ID, job.VideoID, job.Attempts, "probe", retryAt != nil, startedAt, mediaErrorClass("probe", err))
		return true, nil
	}
	if err := w.repo.UpdateTranscodingProgress(ctx, job.VideoID, 35, "transcoding"); err != nil {
		if w.stats != nil {
			w.stats.MediaJobFailed("transcode")
		}
		w.logJob(slog.LevelWarn, "media job failed", "media_job_failed", job.ID, job.VideoID, job.Attempts, "transcode", false, startedAt, "storage_failed")
		return true, err
	}
	hlsMasterPath, err := w.transcoder.Transcode(ctx, inputPath, job.VideoID, metadata)
	if err != nil {
		if ctx.Err() != nil {
			w.logJob(slog.LevelInfo, "media job interrupted", "media_job_interrupted", job.ID, job.VideoID, job.Attempts, "transcode", false, startedAt, "context_canceled")
			return true, ctx.Err()
		}
		var retryAt *time.Time
		if job.Attempts < w.maxAttempts {
			next := time.Now().Add(time.Duration(1<<(job.Attempts-1)) * 5 * time.Second)
			retryAt = &next
		}
		if w.stats != nil {
			w.stats.MediaJobFailed("transcode")
		}
		if saveErr := w.repo.FailTranscodingJob(ctx, job.ID, job.VideoID, err.Error(), retryAt); saveErr != nil {
			w.logJob(slog.LevelWarn, "media job failed", "media_job_failed", job.ID, job.VideoID, job.Attempts, "transcode", false, startedAt, "storage_failed")
			return true, saveErr
		}
		w.logJob(slog.LevelWarn, "media job failed", "media_job_failed", job.ID, job.VideoID, job.Attempts, "transcode", retryAt != nil, startedAt, mediaErrorClass("transcode", err))
		return true, nil
	}
	if err := w.repo.UpdateTranscodingProgress(ctx, job.VideoID, 90, "finalizing"); err != nil {
		if w.stats != nil {
			w.stats.MediaJobFailed("finalize")
		}
		w.logJob(slog.LevelWarn, "media job failed", "media_job_failed", job.ID, job.VideoID, job.Attempts, "finalize", false, startedAt, "storage_failed")
		return true, err
	}
	output := domain.MediaOutput{Metadata: metadata, HLSMasterPath: hlsMasterPath}
	if hlsMasterPath != "" {
		output.HLSBytes = hlsDirectorySize(w.mediaDir, job.VideoID, w.logger)
	}
	if err := w.repo.CompleteTranscodingJob(ctx, job.ID, job.VideoID, output); err != nil {
		if w.stats != nil {
			w.stats.MediaJobFailed("complete")
		}
		w.logJob(slog.LevelWarn, "media job failed", "media_job_failed", job.ID, job.VideoID, job.Attempts, "complete", false, startedAt, "storage_failed")
		return true, err
	}
	if w.stats != nil {
		w.stats.MediaJobCompleted(time.Since(startedAt))
	}
	w.logJob(slog.LevelInfo, "media job completed", "media_job_completed", job.ID, job.VideoID, job.Attempts, "complete", false, startedAt, "")
	return true, nil
}

func (w *Worker) logJob(level slog.Level, message, event string, jobID, videoID int64, attempt int, stage string, retry bool, startedAt time.Time, errorClass string) {
	attributes := []any{
		"event", event,
		"job_id", jobID,
		"video_id", videoID,
		"attempt", attempt,
		"stage", stage,
		"retry", retry,
		"duration_ms", time.Since(startedAt).Milliseconds(),
	}
	if errorClass != "" {
		attributes = append(attributes, "error_class", errorClass)
	}
	w.logger.Log(context.Background(), level, message, attributes...)
}

func mediaErrorClass(stage string, err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return stage + "_timeout"
	}
	return stage + "_failed"
}

// hlsDirectorySize sums the bytes written for a completed HLS rendition set so
// storage quotas can account for the transcoded output. Measurement failures
// are logged and reported as zero: accounting must never block completion.
func hlsDirectorySize(mediaDir string, videoID int64, logger *slog.Logger) int64 {
	dir := filepath.Join(mediaDir, "hls", strconv.FormatInt(videoID, 10))
	var total int64
	err := filepath.WalkDir(dir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		// A missing directory is normal (HLS disabled or nothing written);
		// only unexpected failures deserve a warning.
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warn("measure hls output", "video_id", videoID, "error", err)
		}
		return 0
	}
	return total
}

func resolveMediaPath(mediaDir, storedPath string) (string, error) {
	normalized := filepath.FromSlash(strings.ReplaceAll(storedPath, `\`, "/"))
	if filepath.IsAbs(normalized) {
		return "", fmt.Errorf("media path must be relative")
	}
	root, err := filepath.Abs(mediaDir)
	if err != nil {
		return "", fmt.Errorf("resolve media directory: %w", err)
	}
	resolved, err := filepath.Abs(filepath.Join(root, normalized))
	if err != nil {
		return "", fmt.Errorf("resolve media path: %w", err)
	}
	relative, err := filepath.Rel(root, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("media path escapes media directory")
	}
	return resolved, nil
}
