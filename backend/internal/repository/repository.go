package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"gvideo/backend/internal/domain"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) ToggleFollow(ctx context.Context, followerID, followedID int64) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin follow toggle: %w", err)
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM user_follows WHERE follower_id = ? AND followed_id = ?`, followerID, followedID).Scan(&exists)
	active := false
	if errors.Is(err, sql.ErrNoRows) {
		if _, err = tx.ExecContext(ctx, `INSERT INTO user_follows(follower_id, followed_id) VALUES (?, ?)`, followerID, followedID); err != nil {
			return false, fmt.Errorf("follow user: %w", err)
		}
		active = true
	} else if err != nil {
		return false, fmt.Errorf("read follow: %w", err)
	} else if _, err = tx.ExecContext(ctx, `DELETE FROM user_follows WHERE follower_id = ? AND followed_id = ?`, followerID, followedID); err != nil {
		return false, fmt.Errorf("unfollow user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit follow toggle: %w", err)
	}
	return active, nil
}

const videoSelect = `
SELECT v.id, v.user_id, u.username, u.avatar_path, v.title, v.description, v.category, v.visibility, v.video_path, v.hls_master_path, v.cover_path,
       v.mime_type, v.duration_seconds, v.size_bytes, v.processing_status, v.processing_progress, v.processing_stage, v.source_width, v.source_height,
       v.source_bitrate, v.video_codec, v.audio_codec, v.processing_error, v.processed_at, v.views_count, v.created_at,
       (SELECT COUNT(*) FROM video_likes l WHERE l.video_id = v.id),
       (SELECT COUNT(*) FROM video_favorites f WHERE f.video_id = v.id),
       (SELECT COUNT(*) FROM comments c WHERE c.video_id = v.id),
       CASE WHEN ? > 0 AND EXISTS(SELECT 1 FROM video_likes l2 WHERE l2.video_id = v.id AND l2.user_id = ?) THEN 1 ELSE 0 END,
       CASE WHEN ? > 0 AND EXISTS(SELECT 1 FROM video_favorites f2 WHERE f2.video_id = v.id AND f2.user_id = ?) THEN 1 ELSE 0 END
FROM videos v JOIN users u ON u.id = v.user_id`

func videoFilterSQL(filter domain.VideoFilter) ([]string, []any) {
	where := []string{"1 = 1"}
	args := make([]any, 0, 5)
	if !filter.IncludeNonPublic {
		where = append(where, `v.visibility = 'public'`)
	}
	if filter.Query != "" {
		where = append(where, `(v.title LIKE ? OR v.description LIKE ? OR u.username LIKE ?)`)
		q := "%" + filter.Query + "%"
		args = append(args, q, q, q)
	}
	if filter.Category != "" {
		where = append(where, `v.category = ?`)
		args = append(args, filter.Category)
	}
	if filter.UserID > 0 {
		where = append(where, `v.user_id = ?`)
		args = append(args, filter.UserID)
	}
	if filter.FollowingUserID > 0 {
		where = append(where, `EXISTS (
  SELECT 1 FROM user_follows uf
  WHERE uf.follower_id = ? AND uf.followed_id = v.user_id
)`)
		args = append(args, filter.FollowingUserID)
	}
	if filter.FavoriteUserID > 0 {
		where = append(where, `EXISTS (
  SELECT 1 FROM video_favorites vf
  WHERE vf.user_id = ? AND vf.video_id = v.id
)`)
		args = append(args, filter.FavoriteUserID)
	}
	return where, args
}

type scanner interface{ Scan(...any) error }

func scanVideo(row scanner) (domain.Video, error) {
	video := domain.Video{SubtitleTracks: make([]domain.SubtitleTrack, 0)}
	var avatarPath, videoPath, hlsMasterPath, coverPath string
	var liked, favorited int
	err := row.Scan(&video.ID, &video.UserID, &video.Username, &avatarPath, &video.Title, &video.Description, &video.Category, &video.Visibility,
		&videoPath, &hlsMasterPath, &coverPath, &video.MimeType, &video.DurationSeconds, &video.SizeBytes, &video.ProcessingStatus,
		&video.ProcessingProgress, &video.ProcessingStage, &video.SourceWidth, &video.SourceHeight, &video.SourceBitrate, &video.VideoCodec, &video.AudioCodec,
		&video.ProcessingError, &video.ProcessedAt, &video.ViewsCount, &video.CreatedAt,
		&video.LikesCount, &video.FavoritesCount, &video.CommentsCount, &liked, &favorited)
	if err != nil {
		return domain.Video{}, err
	}
	video.VideoURL = "/media/" + strings.ReplaceAll(videoPath, `\`, "/")
	video.AvatarURL = optionalMediaURL(avatarPath)
	if hlsMasterPath != "" {
		video.HLSURL = "/media/" + strings.ReplaceAll(hlsMasterPath, `\`, "/")
	}
	if coverPath != "" {
		video.CoverURL = "/media/" + strings.ReplaceAll(coverPath, `\`, "/")
	}
	video.Liked = liked == 1
	video.Favorited = favorited == 1
	return video, nil
}

func mediaURL(path string) string {
	return "/media/" + strings.ReplaceAll(path, `\`, "/")
}

func optionalMediaURL(path string) string {
	if path == "" {
		return ""
	}
	return mediaURL(path)
}
