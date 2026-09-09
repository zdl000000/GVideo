package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"gvideo/backend/internal/domain"
)

func (r *Repository) CreateVideo(ctx context.Context, input domain.NewVideo) (domain.Video, error) {
	return r.createVideo(ctx, input, nil)
}

func (r *Repository) CreateVideoWithSubtitle(ctx context.Context, input domain.NewVideo, subtitle domain.NewSubtitle) (domain.Video, error) {
	return r.createVideo(ctx, input, &subtitle)
}

func (r *Repository) createVideo(ctx context.Context, input domain.NewVideo, subtitle *domain.NewSubtitle) (domain.Video, error) {
	if input.Visibility == "" {
		input.Visibility = "public"
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Video{}, fmt.Errorf("begin create video: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
INSERT INTO videos(
  user_id, title, description, category, visibility, video_path, cover_path, mime_type,
  duration_seconds, size_bytes, processing_status, processing_progress, processing_stage)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', 0, 'queued')`, input.UserID, input.Title, input.Description, input.Category, input.Visibility,
		input.VideoPath, input.CoverPath, input.MimeType, input.DurationSeconds, input.SizeBytes)
	if err != nil {
		return domain.Video{}, fmt.Errorf("create video: %w", err)
	}
	id, _ := result.LastInsertId()
	if _, err := tx.ExecContext(ctx, `INSERT INTO transcoding_jobs(video_id) VALUES (?)`, id); err != nil {
		return domain.Video{}, fmt.Errorf("create transcoding job: %w", err)
	}
	if subtitle != nil {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO video_subtitles(video_id, language, label, subtitle_path, is_default)
VALUES (?, ?, ?, ?, ?)`, id, subtitle.Language, subtitle.Label, subtitle.Path, subtitle.IsDefault); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "unique") {
				return domain.Video{}, domain.ErrSubtitleExists
			}
			return domain.Video{}, fmt.Errorf("create initial subtitle: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return domain.Video{}, fmt.Errorf("commit video: %w", err)
	}
	return r.VideoByID(ctx, id, input.UserID)
}

func (r *Repository) ListVideos(ctx context.Context, filter domain.VideoFilter, viewerID int64) ([]domain.Video, error) {
	where, filterArgs := videoFilterSQL(filter)
	args := []any{viewerID, viewerID, viewerID, viewerID}
	order := "v.created_at DESC"
	if filter.Sort == "popular" {
		order = "(v.views_count + (SELECT COUNT(*) * 4 FROM video_likes l3 WHERE l3.video_id = v.id)) DESC, v.created_at DESC"
	} else if filter.FavoriteUserID > 0 {
		order = "(SELECT f3.created_at FROM video_favorites f3 WHERE f3.user_id = ? AND f3.video_id = v.id) DESC, v.id DESC"
		args = append(args, filter.FavoriteUserID)
	}
	args = append(args, filterArgs...)
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 24
	}
	args = append(args, limit, filter.Offset)
	query := videoSelect + " WHERE " + strings.Join(where, " AND ") + " ORDER BY " + order + " LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	defer rows.Close()
	var videos []domain.Video
	for rows.Next() {
		video, err := scanVideo(rows)
		if err != nil {
			return nil, err
		}
		videos = append(videos, video)
	}
	return videos, rows.Err()
}

func (r *Repository) CountVideos(ctx context.Context, filter domain.VideoFilter) (int64, error) {
	where, args := videoFilterSQL(filter)
	var total int64
	query := `SELECT COUNT(*) FROM videos v JOIN users u ON u.id = v.user_id WHERE ` + strings.Join(where, " AND ")
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("count videos: %w", err)
	}
	return total, nil
}

func (r *Repository) UpdateVideo(ctx context.Context, videoID, userID int64, input domain.UpdateVideo) (domain.Video, error) {
	var result sql.Result
	var err error
	if input.CoverPath == nil {
		result, err = r.db.ExecContext(ctx, `
UPDATE videos SET title = ?, description = ?, category = ?, visibility = ?
WHERE id = ? AND user_id = ?`, input.Title, input.Description, input.Category, input.Visibility, videoID, userID)
	} else {
		result, err = r.db.ExecContext(ctx, `
UPDATE videos SET title = ?, description = ?, category = ?, visibility = ?, cover_path = ?
WHERE id = ? AND user_id = ?`, input.Title, input.Description, input.Category, input.Visibility, *input.CoverPath, videoID, userID)
	}
	if err != nil {
		return domain.Video{}, fmt.Errorf("update video: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.Video{}, domain.ErrNotFound
	}
	return r.VideoByID(ctx, videoID, userID)
}

func (r *Repository) DeleteVideo(ctx context.Context, videoID, userID int64) (domain.VideoAssets, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.VideoAssets{}, fmt.Errorf("begin delete video: %w", err)
	}
	defer tx.Rollback()
	var assets domain.VideoAssets
	var status string
	err = tx.QueryRowContext(ctx, `
SELECT video_path, cover_path, hls_master_path, processing_status
FROM videos WHERE id = ? AND user_id = ?`, videoID, userID).
		Scan(&assets.VideoPath, &assets.CoverPath, &assets.HLSMasterPath, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.VideoAssets{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.VideoAssets{}, fmt.Errorf("read video assets: %w", err)
	}
	if status == "processing" {
		return domain.VideoAssets{}, domain.ErrVideoProcessing
	}
	rows, err := tx.QueryContext(ctx, `SELECT subtitle_path FROM video_subtitles WHERE video_id = ?`, videoID)
	if err != nil {
		return domain.VideoAssets{}, fmt.Errorf("list video subtitle assets: %w", err)
	}
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			rows.Close()
			return domain.VideoAssets{}, fmt.Errorf("scan video subtitle asset: %w", err)
		}
		assets.SubtitlePaths = append(assets.SubtitlePaths, path)
	}
	if err := rows.Close(); err != nil {
		return domain.VideoAssets{}, fmt.Errorf("close subtitle assets: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM videos WHERE id = ? AND user_id = ? AND processing_status <> 'processing'`, videoID, userID)
	if err != nil {
		return domain.VideoAssets{}, fmt.Errorf("delete video: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.VideoAssets{}, domain.ErrVideoProcessing
	}
	if err := tx.Commit(); err != nil {
		return domain.VideoAssets{}, fmt.Errorf("commit delete video: %w", err)
	}
	return assets, nil
}

func (r *Repository) RetryTranscoding(ctx context.Context, videoID, userID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin retry transcoding: %w", err)
	}
	defer tx.Rollback()
	var status string
	err = tx.QueryRowContext(ctx, `SELECT processing_status FROM videos WHERE id = ? AND user_id = ?`, videoID, userID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read retry video: %w", err)
	}
	if status != "failed" {
		return domain.ErrRetryUnavailable
	}
	result, err := tx.ExecContext(ctx, `
UPDATE transcoding_jobs
SET status = 'pending', attempts = 0, last_error = '', available_at = CURRENT_TIMESTAMP,
    started_at = NULL, finished_at = NULL, updated_at = CURRENT_TIMESTAMP
WHERE video_id = ?`, videoID)
	if err != nil {
		return fmt.Errorf("retry transcoding job: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read retry transcoding result: %w", err)
	}
	if changed != 1 {
		return domain.ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE videos SET processing_status = 'pending', processing_progress = 0, processing_stage = 'queued',
  processing_error = '', processed_at = NULL
WHERE id = ?`, videoID); err != nil {
		return fmt.Errorf("mark video retry pending: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit retry transcoding: %w", err)
	}
	return nil
}

func (r *Repository) VideoByID(ctx context.Context, id, viewerID int64) (domain.Video, error) {
	row := r.db.QueryRowContext(ctx, videoSelect+` WHERE v.id = ? AND (v.visibility <> 'private' OR v.user_id = ?)`, viewerID, viewerID, viewerID, viewerID, id, viewerID)
	video, err := scanVideo(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Video{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Video{}, fmt.Errorf("find video: %w", err)
	}
	video.SubtitleTracks, err = r.ListSubtitles(ctx, video.ID)
	if err != nil {
		return domain.Video{}, err
	}
	return video, nil
}

func (r *Repository) CreatorStats(ctx context.Context, userID int64) (domain.CreatorStats, error) {
	var stats domain.CreatorStats
	err := r.db.QueryRowContext(ctx, `
SELECT
  (SELECT COUNT(*) FROM videos v WHERE v.user_id = u.id),
  (SELECT COUNT(*) FROM user_follows f WHERE f.followed_id = ?),
  COALESCE((SELECT SUM(v.views_count) FROM videos v WHERE v.user_id = u.id), 0),
  (SELECT COUNT(*) FROM video_likes l JOIN videos v ON v.id = l.video_id WHERE v.user_id = u.id),
  (SELECT COUNT(*) FROM video_favorites f JOIN videos v ON v.id = f.video_id WHERE v.user_id = u.id),
  (SELECT COUNT(*) FROM comments c JOIN videos v ON v.id = c.video_id WHERE v.user_id = u.id),
  (SELECT COUNT(*) FROM videos v WHERE v.user_id = u.id AND v.visibility = 'public'),
  (SELECT COUNT(*) FROM videos v WHERE v.user_id = u.id AND v.visibility = 'unlisted'),
  (SELECT COUNT(*) FROM videos v WHERE v.user_id = u.id AND v.visibility = 'private'),
  (SELECT COUNT(*) FROM videos v WHERE v.user_id = u.id AND v.processing_status IN ('pending', 'processing'))
FROM users u
WHERE u.id = ?`, userID, userID).Scan(
		&stats.VideosCount, &stats.FollowersCount, &stats.ViewsCount, &stats.LikesCount,
		&stats.FavoritesCount, &stats.CommentsCount, &stats.PublicCount, &stats.UnlistedCount,
		&stats.PrivateCount, &stats.ProcessingCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CreatorStats{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CreatorStats{}, fmt.Errorf("read creator stats: %w", err)
	}
	stats.RecentVideos, err = r.ListVideos(ctx, domain.VideoFilter{
		UserID: userID, IncludeNonPublic: true, Limit: 5,
	}, userID)
	if err != nil {
		return domain.CreatorStats{}, err
	}
	if stats.RecentVideos == nil {
		stats.RecentVideos = []domain.Video{}
	}
	return stats, nil
}

func (r *Repository) IncrementViews(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE videos SET views_count = views_count + 1 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("increment views: %w", err)
	}
	return nil
}
