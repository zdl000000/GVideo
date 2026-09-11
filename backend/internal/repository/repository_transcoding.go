package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gvideo/backend/internal/domain"
)

// PendingJobCount reports how many transcoding jobs are waiting or running.
// It backs the media_queue_depth gauge.
func (r *Repository) PendingJobCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM transcoding_jobs WHERE status IN ('pending', 'processing')`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count pending transcoding jobs: %w", err)
	}
	return count, nil
}

func (r *Repository) RecoverTranscodingJobs(ctx context.Context, requireHLS ...bool) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin job recovery: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
UPDATE transcoding_jobs
SET status = 'pending', available_at = CURRENT_TIMESTAMP, started_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE status = 'processing'`)
	if err != nil {
		return 0, fmt.Errorf("recover transcoding jobs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE videos SET processing_status = 'pending', processing_progress = 0, processing_stage = 'queued'
WHERE processing_status = 'processing'
  AND id IN (SELECT video_id FROM transcoding_jobs WHERE status = 'pending')`); err != nil {
		return 0, fmt.Errorf("recover video processing state: %w", err)
	}
	hlsRequired := len(requireHLS) > 0 && requireHLS[0]
	var requeued sql.Result
	if hlsRequired {
		requeued, err = tx.ExecContext(ctx, `
UPDATE transcoding_jobs
SET status = 'pending', attempts = 0, last_error = '', available_at = CURRENT_TIMESTAMP,
    started_at = NULL, finished_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE video_id IN (SELECT id FROM videos WHERE hls_master_path = '')
  AND status <> 'processing'`)
		if err != nil {
			return 0, fmt.Errorf("requeue videos without HLS: %w", err)
		}
	}
	legacyCondition := "source_width = 0 OR source_height = 0 OR video_codec = ''"
	if hlsRequired {
		legacyCondition += " OR hls_master_path = ''"
	}
	queued, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO transcoding_jobs(video_id)
SELECT id FROM videos
WHERE `+legacyCondition)
	if err != nil {
		return 0, fmt.Errorf("queue legacy media probes: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE videos SET processing_status = 'pending', processing_progress = 0, processing_stage = 'queued', processing_error = ''
WHERE id IN (SELECT video_id FROM transcoding_jobs WHERE status = 'pending')`); err != nil {
		return 0, fmt.Errorf("mark queued legacy videos pending: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit job recovery: %w", err)
	}
	recoveredCount, _ := result.RowsAffected()
	requeuedCount := int64(0)
	if requeued != nil {
		requeuedCount, _ = requeued.RowsAffected()
	}
	queuedCount, _ := queued.RowsAffected()
	return recoveredCount + requeuedCount + queuedCount, nil
}

func (r *Repository) ClaimTranscodingJob(ctx context.Context) (domain.TranscodingJob, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.TranscodingJob{}, false, fmt.Errorf("begin claim job: %w", err)
	}
	defer tx.Rollback()
	var job domain.TranscodingJob
	err = tx.QueryRowContext(ctx, `
SELECT j.id, j.video_id, v.video_path, j.attempts, j.available_at
FROM transcoding_jobs j JOIN videos v ON v.id = j.video_id
WHERE j.status = 'pending' AND j.available_at <= CURRENT_TIMESTAMP
ORDER BY j.available_at, j.id LIMIT 1`).Scan(&job.ID, &job.VideoID, &job.VideoPath, &job.Attempts, &job.AvailableAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TranscodingJob{}, false, nil
	}
	if err != nil {
		return domain.TranscodingJob{}, false, fmt.Errorf("find pending transcoding job: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
UPDATE transcoding_jobs
SET status = 'processing', attempts = attempts + 1, started_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND status = 'pending'`, job.ID)
	if err != nil {
		return domain.TranscodingJob{}, false, fmt.Errorf("claim transcoding job: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.TranscodingJob{}, false, nil
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE videos
SET processing_status = 'processing', processing_progress = 10, processing_stage = 'probing', processing_error = ''
WHERE id = ?`, job.VideoID); err != nil {
		return domain.TranscodingJob{}, false, fmt.Errorf("mark video processing: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.TranscodingJob{}, false, fmt.Errorf("commit job claim: %w", err)
	}
	job.Attempts++
	return job, true, nil
}

func (r *Repository) UpdateTranscodingProgress(ctx context.Context, videoID int64, progress int, stage string) error {
	if progress < 0 || progress > 100 || stage == "" {
		return domain.ErrInvalidInput
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE videos SET processing_progress = ?, processing_stage = ?
WHERE id = ? AND processing_status = 'processing'`, progress, stage, videoID)
	if err != nil {
		return fmt.Errorf("update transcoding progress: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) CompleteTranscodingJob(ctx context.Context, jobID, videoID int64, output domain.MediaOutput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin complete job: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
UPDATE videos SET duration_seconds = ?, source_width = ?, source_height = ?, source_bitrate = ?,
  video_codec = ?, audio_codec = ?, hls_master_path = ?, hls_size_bytes = ?, processing_status = 'ready',
  processing_progress = 100, processing_stage = 'ready', processing_error = '', processed_at = CURRENT_TIMESTAMP
WHERE id = ?`, output.Metadata.DurationSeconds, output.Metadata.Width, output.Metadata.Height, output.Metadata.Bitrate,
		output.Metadata.VideoCodec, output.Metadata.AudioCodec, output.HLSMasterPath, output.HLSBytes, videoID); err != nil {
		return fmt.Errorf("save media metadata: %w", err)
	}
	jobResult, err := tx.ExecContext(ctx, `
UPDATE transcoding_jobs SET status = 'completed', last_error = '', finished_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND video_id = ?`, jobID, videoID)
	if err != nil {
		return fmt.Errorf("complete transcoding job: %w", err)
	}
	jobChanged, err := jobResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("check completed job: %w", err)
	}
	if jobChanged != 1 {
		return domain.ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO notifications(recipient_id, type, video_id, video_title)
SELECT v.user_id, 'processing_ready', v.id, v.title FROM videos v WHERE v.id = ?`, videoID); err != nil {
		return fmt.Errorf("create completed processing notification: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit completed job: %w", err)
	}
	return nil
}

func (r *Repository) FailTranscodingJob(ctx context.Context, jobID, videoID int64, message string, retryAt *time.Time) error {
	if len(message) > 1000 {
		message = message[:1000]
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin fail job: %w", err)
	}
	defer tx.Rollback()
	if retryAt != nil {
		result, err := tx.ExecContext(ctx, `
UPDATE transcoding_jobs SET status = 'pending', last_error = ?, available_at = ?, started_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND video_id = ?`, message, *retryAt, jobID, videoID)
		if err != nil {
			return fmt.Errorf("reschedule transcoding job: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check rescheduled job: %w", err)
		}
		if changed != 1 {
			return domain.ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE videos
SET processing_status = 'pending', processing_progress = 0, processing_stage = 'queued', processing_error = ?
WHERE id = ?`, message, videoID); err != nil {
			return fmt.Errorf("mark video pending: %w", err)
		}
	} else {
		result, err := tx.ExecContext(ctx, `
UPDATE transcoding_jobs SET status = 'failed', last_error = ?, finished_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND video_id = ?`, message, jobID, videoID)
		if err != nil {
			return fmt.Errorf("fail transcoding job: %w", err)
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("check failed job: %w", err)
		}
		if changed != 1 {
			return domain.ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE videos SET processing_status = 'failed', processing_stage = 'failed', processing_error = ?
WHERE id = ?`, message, videoID); err != nil {
			return fmt.Errorf("mark video failed: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO notifications(recipient_id, type, video_id, video_title, comment_preview)
SELECT v.user_id, 'processing_failed', v.id, v.title, ? FROM videos v WHERE v.id = ?`, message, videoID); err != nil {
			return fmt.Errorf("create failed processing notification: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed job: %w", err)
	}
	return nil
}
