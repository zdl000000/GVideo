package comments

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"gvideo/backend/internal/domain"
)

// Repository owns the comment persistence and read model.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// VideoAuthorID mirrors the visibility predicate of the core VideoByID query
// (internal/repository/repository_videos.go) as the consumer-owned lookup the
// comment flows need; keep the SQL in sync or move both behind a shared helper.
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
		return 0, "", fmt.Errorf("find commentable video: %w", err)
	}
	return authorID, title, nil
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

func (r *Repository) DeleteComment(ctx context.Context, userID, videoID, commentID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete comment: %w", err)
	}
	defer tx.Rollback()
	var commentUserID, videoUserID int64
	err = tx.QueryRowContext(ctx, `
SELECT c.user_id, v.user_id
FROM comments c JOIN videos v ON v.id = c.video_id
WHERE c.id = ? AND c.video_id = ?`, commentID, videoID).Scan(&commentUserID, &videoUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read comment ownership: %w", err)
	}
	if userID != commentUserID && userID != videoUserID {
		return domain.ErrForbidden
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM comments WHERE id = ? AND video_id = ?`, commentID, videoID)
	if err != nil {
		return fmt.Errorf("delete comment: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("read deleted comment count: %w", err)
	} else if affected == 0 {
		return domain.ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete comment: %w", err)
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

// mediaURL/optionalMediaURL mirror the core repository helpers of the same
// name (internal/repository/repository.go); keep the two in sync or move both
// behind a shared helper when the media prefix logic changes.
func mediaURL(path string) string {
	return "/media/" + strings.ReplaceAll(path, `\`, "/")
}

func optionalMediaURL(path string) string {
	if path == "" {
		return ""
	}
	return mediaURL(path)
}
