package notifications

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gvideo/backend/internal/domain"
)

// Repository owns the notification read and write model: creation is the
// single notification write path, fed by the process-internal event bus.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create 落一条通知（通知写路径的唯一所有者）。收件人无效、类型为空或
// 自评时静默跳过（不写行、不报错），与历史 CreateNotification 语义一致。
func (r *Repository) Create(ctx context.Context, recipientID, actorID int64, kind string, videoID, commentID int64, videoTitle, commentPreview string) error {
	if recipientID <= 0 || kind == "" || recipientID == actorID {
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
		recipientID, actorIDValue, kind, videoIDValue, videoTitle, commentIDValue, commentPreview, actorIDValue)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

func (r *Repository) ListNotifications(ctx context.Context, userID int64, page, pageSize int) (domain.NotificationPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	var result domain.NotificationPage
	result.Page, result.PageSize = page, pageSize
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE recipient_id = ?`, userID).Scan(&result.Total); err != nil {
		return domain.NotificationPage{}, fmt.Errorf("count notifications: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE recipient_id = ? AND read_at IS NULL`, userID).Scan(&result.UnreadCount); err != nil {
		return domain.NotificationPage{}, fmt.Errorf("count unread notifications: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id, type, actor_id, actor_username, actor_avatar_path, video_id, video_title,
       comment_id, comment_preview, read_at, created_at
FROM notifications WHERE recipient_id = ? ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, userID, pageSize, offset)
	if err != nil {
		return domain.NotificationPage{}, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()
	result.Items = make([]domain.Notification, 0)
	for rows.Next() {
		var item domain.Notification
		var actorID, videoID, commentID sql.NullInt64
		var actorUsername, actorAvatarPath, videoTitle, commentPreview string
		var readAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Type, &actorID, &actorUsername, &actorAvatarPath, &videoID, &videoTitle, &commentID, &commentPreview, &readAt, &item.CreatedAt); err != nil {
			return domain.NotificationPage{}, fmt.Errorf("scan notification: %w", err)
		}
		item.ActorID, item.VideoID, item.CommentID = actorID.Int64, videoID.Int64, commentID.Int64
		item.ActorUsername, item.VideoTitle, item.CommentPreview = actorUsername, videoTitle, commentPreview
		item.ActorAvatarURL = optionalMediaURL(actorAvatarPath)
		if readAt.Valid {
			item.ReadAt = &readAt.Time
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.NotificationPage{}, fmt.Errorf("iterate notifications: %w", err)
	}
	result.HasNext = int64(offset+len(result.Items)) < result.Total
	return result, nil
}

func (r *Repository) MarkNotificationRead(ctx context.Context, userID, notificationID int64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE notifications SET read_at = CURRENT_TIMESTAMP WHERE id = ? AND recipient_id = ?`, notificationID, userID)
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *Repository) MarkAllNotificationsRead(ctx context.Context, userID int64) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE notifications SET read_at = CURRENT_TIMESTAMP WHERE recipient_id = ? AND read_at IS NULL`, userID); err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
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
