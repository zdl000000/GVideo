package media

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
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
		w.logger.Error("recover media jobs", "error", err)
		return
	}
	w.logger.Info("media worker started", "recovered_jobs", recovered, "poll_interval", w.pollInterval.String())

	for {
		processed, err := w.processOne(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			w.logger.Error("process media job", "error", err)
		}
		if ctx.Err() != nil {
			w.logger.Info("media worker stopped")
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
	w.logger.Info("media job started", "job_id", job.ID, "video_id", job.VideoID, "attempt", job.Attempts)
	inputPath, err := resolveMediaPath(w.mediaDir, job.VideoPath)
	if err != nil {
		if w.stats != nil {
			w.stats.MediaJobFailed("resolve")
		}
		if saveErr := w.repo.FailTranscodingJob(ctx, job.ID, job.VideoID, err.Error(), nil); saveErr != nil {
			return true, saveErr
		}
		return true, nil
	}
	metadata, err := w.prober.Probe(ctx, inputPath)
	if err != nil {
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		var retryAt *time.Time
		if job.Attempts < w.maxAttempts {
			next := time.Now().Add(time.Duration(1<<(job.Attempts-1)) * 5 * time.Second)
			retryAt = &next
		}
		if saveErr := w.repo.FailTranscodingJob(ctx, job.ID, job.VideoID, err.Error(), retryAt); saveErr != nil {
			return true, saveErr
		}
		if w.stats != nil {
			w.stats.MediaJobFailed("probe")
		}
		w.logger.Warn("media job failed", "job_id", job.ID, "video_id", job.VideoID, "attempt", job.Attempts, "retry", retryAt != nil, "error", err)
		return true, nil
	}
	if err := w.repo.UpdateTranscodingProgress(ctx, job.VideoID, 35, "transcoding"); err != nil {
		return true, err
	}
	hlsMasterPath, err := w.transcoder.Transcode(ctx, inputPath, job.VideoID, metadata)
	if err != nil {
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		var retryAt *time.Time
		if job.Attempts < w.maxAttempts {
			next := time.Now().Add(time.Duration(1<<(job.Attempts-1)) * 5 * time.Second)
			retryAt = &next
		}
		if saveErr := w.repo.FailTranscodingJob(ctx, job.ID, job.VideoID, err.Error(), retryAt); saveErr != nil {
			return true, saveErr
		}
		if w.stats != nil {
			w.stats.MediaJobFailed("transcode")
		}
		w.logger.Warn("HLS transcode failed", "job_id", job.ID, "video_id", job.VideoID, "attempt", job.Attempts, "retry", retryAt != nil, "error", err)
		return true, nil
	}
	if err := w.repo.UpdateTranscodingProgress(ctx, job.VideoID, 90, "finalizing"); err != nil {
		return true, err
	}
	output := domain.MediaOutput{Metadata: metadata, HLSMasterPath: hlsMasterPath}
	if err := w.repo.CompleteTranscodingJob(ctx, job.ID, job.VideoID, output); err != nil {
		if w.stats != nil {
			w.stats.MediaJobFailed("complete")
		}
		return true, err
	}
	if w.stats != nil {
		w.stats.MediaJobCompleted(time.Since(startedAt))
	}
	w.logger.Info("media job completed", "job_id", job.ID, "video_id", job.VideoID, "duration", time.Since(startedAt), "width", metadata.Width, "height", metadata.Height, "hls_master_path", hlsMasterPath)
	return true, nil
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
