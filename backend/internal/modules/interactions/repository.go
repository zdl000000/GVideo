package interactions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gvideo/backend/internal/domain"
)

// Repository owns the interaction (like, favorite, follow) persistence.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
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

// VideoAuthorID mirrors the visibility predicate of the core VideoByID query
// (internal/repository/repository_videos.go) as the consumer-owned lookup the
// like and favorite flows need; keep the SQL in sync or move both behind a
// shared helper.
func (r *Repository) VideoAuthorID(ctx context.Context, videoID, viewerID int64) (int64, string, error) {
	var authorID int64
	var title string
	err := r.db.QueryRowContext(ctx, `
SELECT user_id, title FROM videos
WHERE id = ? AND (visibility <> 'private' OR user_id = ?)`, videoID, viewerID).Scan(&authorID, &title)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", domain.ErrNotFound
	}
	if err != nil {
		return 0, "", fmt.Errorf("find interactable video: %w", err)
	}
	return authorID, title, nil
}

// UserExists is the consumer-owned existence check the follow flow needs; the
// core UserByID (internal/repository/repository_users.go) returns the full row
// while only existence matters here.
func (r *Repository) UserExists(ctx context.Context, userID int64) error {
	var exists int
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)`, userID).Scan(&exists); err != nil {
		return fmt.Errorf("check followable user: %w", err)
	}
	if exists == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// 镜像核心 CreateNotification；事件总线落地后由 notifications 模块统一接管。
func (r *Repository) createNotification(ctx context.Context, recipientID, actorID int64, notificationType string, videoID, commentID int64, videoTitle, commentPreview string) error {
	if recipientID <= 0 || notificationType == "" || recipientID == actorID {
		return nil
	}
	var actorIDValue any
	if actorID > 0 {
		actorIDValue = actorID
	}
	var videoIDValue any
	if videoID > 0 {
		videoIDValue = videoID
	}
	var commentIDValue any
	if commentID > 0 {
		commentIDValue = commentID
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO notifications(
  recipient_id, actor_id, actor_username, actor_avatar_path, type,
  video_id, video_title, comment_id, comment_preview)
SELECT ?, ?, COALESCE(u.username, ''), COALESCE(u.avatar_path, ''), ?, ?, ?, ?, ?
FROM (SELECT 1) seed LEFT JOIN users u ON u.id = ?`,
		recipientID, actorIDValue, notificationType, videoIDValue, videoTitle, commentIDValue, commentPreview, actorIDValue)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}
