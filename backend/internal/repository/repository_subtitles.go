package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"gvideo/backend/internal/domain"
)

func (r *Repository) CreateSubtitle(ctx context.Context, videoID int64, language, label, path string, isDefault bool) (domain.SubtitleTrack, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.SubtitleTrack{}, fmt.Errorf("begin create subtitle: %w", err)
	}
	defer tx.Rollback()
	if isDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE video_subtitles SET is_default = 0 WHERE video_id = ?`, videoID); err != nil {
			return domain.SubtitleTrack{}, fmt.Errorf("clear default subtitle: %w", err)
		}
	}
	result, err := tx.ExecContext(ctx, `
INSERT INTO video_subtitles(video_id, language, label, subtitle_path, is_default)
VALUES (?, ?, ?, ?, ?)`, videoID, language, label, path, isDefault)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return domain.SubtitleTrack{}, domain.ErrSubtitleExists
		}
		return domain.SubtitleTrack{}, fmt.Errorf("create subtitle: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domain.SubtitleTrack{}, fmt.Errorf("read subtitle id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.SubtitleTrack{}, fmt.Errorf("commit create subtitle: %w", err)
	}
	return r.subtitleByID(ctx, id)
}

func (r *Repository) SetDefaultSubtitle(ctx context.Context, videoID, subtitleID int64) ([]domain.SubtitleTrack, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin set default subtitle: %w", err)
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM video_subtitles WHERE id = ? AND video_id = ?`, subtitleID, videoID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find subtitle for default: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE video_subtitles SET is_default = CASE WHEN id = ? THEN 1 ELSE 0 END
WHERE video_id = ?`, subtitleID, videoID); err != nil {
		return nil, fmt.Errorf("set default subtitle: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit default subtitle: %w", err)
	}
	return r.ListSubtitles(ctx, videoID)
}

func (r *Repository) DeleteSubtitle(ctx context.Context, videoID, subtitleID int64) (string, []domain.SubtitleTrack, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", nil, fmt.Errorf("begin delete subtitle: %w", err)
	}
	defer tx.Rollback()
	var subtitlePath string
	var wasDefault int
	err = tx.QueryRowContext(ctx, `
SELECT subtitle_path, is_default FROM video_subtitles
WHERE id = ? AND video_id = ?`, subtitleID, videoID).Scan(&subtitlePath, &wasDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, domain.ErrNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("find subtitle for deletion: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM video_subtitles WHERE id = ? AND video_id = ?`, subtitleID, videoID); err != nil {
		return "", nil, fmt.Errorf("delete subtitle: %w", err)
	}
	if wasDefault == 1 {
		if _, err := tx.ExecContext(ctx, `
UPDATE video_subtitles SET is_default = CASE
  WHEN id = (SELECT id FROM video_subtitles WHERE video_id = ? ORDER BY id LIMIT 1) THEN 1
  ELSE 0
END
WHERE video_id = ?`, videoID, videoID); err != nil {
			return "", nil, fmt.Errorf("select fallback default subtitle: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return "", nil, fmt.Errorf("commit delete subtitle: %w", err)
	}
	tracks, err := r.ListSubtitles(ctx, videoID)
	if err != nil {
		return "", nil, err
	}
	return subtitlePath, tracks, nil
}

func (r *Repository) subtitleByID(ctx context.Context, id int64) (domain.SubtitleTrack, error) {
	var track domain.SubtitleTrack
	var path string
	var isDefault int
	err := r.db.QueryRowContext(ctx, `
SELECT id, language, label, subtitle_path, is_default
FROM video_subtitles WHERE id = ?`, id).
		Scan(&track.ID, &track.Language, &track.Label, &path, &isDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SubtitleTrack{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.SubtitleTrack{}, fmt.Errorf("find subtitle: %w", err)
	}
	track.URL = mediaURL(path)
	track.IsDefault = isDefault == 1
	return track, nil
}

func (r *Repository) ListSubtitles(ctx context.Context, videoID int64) ([]domain.SubtitleTrack, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, language, label, subtitle_path, is_default
FROM video_subtitles WHERE video_id = ? ORDER BY is_default DESC, id`, videoID)
	if err != nil {
		return nil, fmt.Errorf("list subtitles: %w", err)
	}
	defer rows.Close()
	tracks := make([]domain.SubtitleTrack, 0)
	for rows.Next() {
		var track domain.SubtitleTrack
		var path string
		var isDefault int
		if err := rows.Scan(&track.ID, &track.Language, &track.Label, &path, &isDefault); err != nil {
			return nil, fmt.Errorf("scan subtitle: %w", err)
		}
		track.URL = mediaURL(path)
		track.IsDefault = isDefault == 1
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}
