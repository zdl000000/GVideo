package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"gvideo/backend/internal/domain"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreateUser(ctx context.Context, username, passwordHash string) (domain.User, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO users(username, password_hash) VALUES (?, ?)`, username, passwordHash)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return domain.User{}, domain.ErrConflict
		}
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	id, _ := result.LastInsertId()
	return r.UserByID(ctx, id)
}

func (r *Repository) UserAuthByUsername(ctx context.Context, username string) (domain.User, string, error) {
	var user domain.User
	var avatarPath string
	var passwordHash string
	err := r.db.QueryRowContext(ctx, `SELECT id, username, bio, avatar_path, created_at, password_hash FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.Bio, &avatarPath, &user.CreatedAt, &passwordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, "", domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, "", fmt.Errorf("find user auth: %w", err)
	}
	user.AvatarURL = optionalMediaURL(avatarPath)
	return user, passwordHash, nil
}

func (r *Repository) UserByID(ctx context.Context, id int64) (domain.User, error) {
	var user domain.User
	var avatarPath string
	err := r.db.QueryRowContext(ctx, `SELECT id, username, bio, avatar_path, created_at FROM users WHERE id = ?`, id).
		Scan(&user.ID, &user.Username, &user.Bio, &avatarPath, &user.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("find user: %w", err)
	}
	user.AvatarURL = optionalMediaURL(avatarPath)
	return user, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, userID int64, username, bio string, avatarPath *string) (domain.User, error) {
	var result sql.Result
	var err error
	if avatarPath == nil {
		result, err = r.db.ExecContext(ctx, `UPDATE users SET username = ?, bio = ? WHERE id = ?`, username, bio, userID)
	} else {
		result, err = r.db.ExecContext(ctx, `UPDATE users SET username = ?, bio = ?, avatar_path = ? WHERE id = ?`, username, bio, *avatarPath, userID)
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return domain.User{}, domain.ErrConflict
		}
		return domain.User{}, fmt.Errorf("update user profile: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.User{}, domain.ErrNotFound
	}
	return r.UserByID(ctx, userID)
}

func (r *Repository) CreatorProfile(ctx context.Context, userID, viewerID int64) (domain.CreatorProfile, error) {
	var profile domain.CreatorProfile
	var followed int
	row := r.db.QueryRowContext(ctx, `
SELECT u.id, u.username, u.bio, u.avatar_path, u.created_at,
       (SELECT COUNT(*) FROM user_follows f WHERE f.followed_id = u.id),
       (SELECT COUNT(*) FROM user_follows f WHERE f.follower_id = u.id),
       (SELECT COUNT(*) FROM videos v WHERE v.user_id = u.id AND v.visibility = 'public'),
       CASE WHEN ? > 0 AND EXISTS(
         SELECT 1 FROM user_follows f WHERE f.follower_id = ? AND f.followed_id = u.id
       ) THEN 1 ELSE 0 END
FROM users u WHERE u.id = ?`, viewerID, viewerID, userID)
	var avatarPath string
	err := row.Scan(&profile.ID, &profile.Username, &profile.Bio, &avatarPath, &profile.CreatedAt, &profile.FollowersCount,
		&profile.FollowingCount, &profile.VideosCount, &followed)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CreatorProfile{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.CreatorProfile{}, fmt.Errorf("find creator profile: %w", err)
	}
	profile.Followed = followed == 1
	profile.AvatarURL = optionalMediaURL(avatarPath)
	return profile, nil
}

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

func (r *Repository) CreateSession(ctx context.Context, tokenHash string, userID int64, csrfToken string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO sessions(token_hash, user_id, csrf_token, expires_at) VALUES (?, ?, ?, ?)`, tokenHash, userID, csrfToken, expiresAt)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (r *Repository) SessionByHash(ctx context.Context, tokenHash string) (domain.Session, error) {
	var session domain.Session
	var avatarPath string
	err := r.db.QueryRowContext(ctx, `
SELECT u.id, u.username, u.bio, u.avatar_path, u.created_at, s.csrf_token, s.expires_at
FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.token_hash = ? AND s.expires_at > CURRENT_TIMESTAMP`, tokenHash).
		Scan(&session.User.ID, &session.User.Username, &session.User.Bio, &avatarPath, &session.User.CreatedAt, &session.CSRFToken, &session.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Session{}, domain.ErrInvalidSession
	}
	if err != nil {
		return domain.Session{}, fmt.Errorf("find session: %w", err)
	}
	session.User.AvatarURL = optionalMediaURL(avatarPath)
	return session, nil
}

func (r *Repository) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

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

func (r *Repository) ListVideos(ctx context.Context, filter domain.VideoFilter, viewerID int64) ([]domain.Video, error) {
	where, filterArgs := videoFilterSQL(filter)
	args := []any{viewerID, viewerID, viewerID, viewerID}
	args = append(args, filterArgs...)
	order := "v.created_at DESC"
	if filter.Sort == "popular" {
		order = "(v.views_count + (SELECT COUNT(*) * 4 FROM video_likes l3 WHERE l3.video_id = v.id)) DESC, v.created_at DESC"
	}
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
	return where, args
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

func (r *Repository) MediaAccessByPath(ctx context.Context, storedPath string) (domain.MediaAccess, error) {
	storedPath = strings.TrimPrefix(strings.ReplaceAll(storedPath, `\`, "/"), "/")
	var access domain.MediaAccess
	err := r.db.QueryRowContext(ctx, `
SELECT v.id, v.user_id, v.visibility
FROM videos v
WHERE replace(v.video_path, '\', '/') = ?
   OR replace(v.cover_path, '\', '/') = ?
   OR EXISTS (
     SELECT 1 FROM video_subtitles s
     WHERE s.video_id = v.id AND replace(s.subtitle_path, '\', '/') = ?
   )
LIMIT 1`, storedPath, storedPath, storedPath).Scan(&access.VideoID, &access.UserID, &access.Visibility)
	if err == nil {
		return access, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.MediaAccess{}, fmt.Errorf("find media owner: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, visibility, replace(hls_master_path, '\', '/')
FROM videos WHERE hls_master_path <> ''`)
	if err != nil {
		return domain.MediaAccess{}, fmt.Errorf("list HLS owners: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var masterPath string
		if err := rows.Scan(&access.VideoID, &access.UserID, &access.Visibility, &masterPath); err != nil {
			return domain.MediaAccess{}, fmt.Errorf("scan HLS owner: %w", err)
		}
		masterPath = strings.TrimPrefix(strings.ReplaceAll(masterPath, `\`, "/"), "/")
		dir := path.Dir(masterPath)
		if storedPath == masterPath || (dir != "." && strings.HasPrefix(storedPath, dir+"/")) {
			return access, nil
		}
	}
	if err := rows.Err(); err != nil {
		return domain.MediaAccess{}, fmt.Errorf("iterate HLS owners: %w", err)
	}
	return domain.MediaAccess{}, domain.ErrNotFound
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
  video_codec = ?, audio_codec = ?, hls_master_path = ?, processing_status = 'ready',
  processing_progress = 100, processing_stage = 'ready', processing_error = '', processed_at = CURRENT_TIMESTAMP
WHERE id = ?`, output.Metadata.DurationSeconds, output.Metadata.Width, output.Metadata.Height, output.Metadata.Bitrate,
		output.Metadata.VideoCodec, output.Metadata.AudioCodec, output.HLSMasterPath, videoID); err != nil {
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
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed job: %w", err)
	}
	return nil
}

func (r *Repository) IncrementViews(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE videos SET views_count = views_count + 1 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("increment views: %w", err)
	}
	return nil
}

func (r *Repository) ToggleLike(ctx context.Context, userID, videoID int64) (bool, error) {
	return r.toggle(ctx, "video_likes", userID, videoID)
}

func (r *Repository) ToggleFavorite(ctx context.Context, userID, videoID int64) (bool, error) {
	return r.toggle(ctx, "video_favorites", userID, videoID)
}

func (r *Repository) toggle(ctx context.Context, table string, userID, videoID int64) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin toggle: %w", err)
	}
	defer tx.Rollback()
	var exists int
	err = tx.QueryRowContext(ctx, `SELECT 1 FROM `+table+` WHERE user_id = ? AND video_id = ?`, userID, videoID).Scan(&exists)
	active := false
	if errors.Is(err, sql.ErrNoRows) {
		if _, err = tx.ExecContext(ctx, `INSERT INTO `+table+`(user_id, video_id) VALUES (?, ?)`, userID, videoID); err != nil {
			return false, fmt.Errorf("insert toggle: %w", err)
		}
		active = true
	} else if err != nil {
		return false, fmt.Errorf("read toggle: %w", err)
	} else {
		if _, err = tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE user_id = ? AND video_id = ?`, userID, videoID); err != nil {
			return false, fmt.Errorf("delete toggle: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit toggle: %w", err)
	}
	return active, nil
}

func (r *Repository) ListComments(ctx context.Context, videoID int64) ([]domain.Comment, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT c.id, c.video_id, c.user_id, u.username, u.avatar_path, c.content, c.created_at
FROM comments c JOIN users u ON u.id = c.user_id
WHERE c.video_id = ? ORDER BY c.created_at DESC LIMIT 200`, videoID)
	if err != nil {
		return nil, fmt.Errorf("list comments: %w", err)
	}
	defer rows.Close()
	var comments []domain.Comment
	for rows.Next() {
		var comment domain.Comment
		var avatarPath string
		if err := rows.Scan(&comment.ID, &comment.VideoID, &comment.UserID, &comment.Username, &avatarPath, &comment.Content, &comment.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan comment: %w", err)
		}
		comment.AvatarURL = optionalMediaURL(avatarPath)
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func (r *Repository) CreateComment(ctx context.Context, userID, videoID int64, content string) (domain.Comment, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO comments(user_id, video_id, content) VALUES (?, ?, ?)`, userID, videoID, content)
	if err != nil {
		return domain.Comment{}, fmt.Errorf("create comment: %w", err)
	}
	id, _ := result.LastInsertId()
	var comment domain.Comment
	err = r.db.QueryRowContext(ctx, `
SELECT c.id, c.video_id, c.user_id, u.username, u.avatar_path, c.content, c.created_at
FROM comments c JOIN users u ON u.id = c.user_id WHERE c.id = ?`, id).
		Scan(&comment.ID, &comment.VideoID, &comment.UserID, &comment.Username, &comment.AvatarURL, &comment.Content, &comment.CreatedAt)
	if err != nil {
		return domain.Comment{}, fmt.Errorf("read created comment: %w", err)
	}
	comment.AvatarURL = optionalMediaURL(comment.AvatarURL)
	return comment, nil
}
