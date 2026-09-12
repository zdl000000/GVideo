package repository

import (
	"context"
	"fmt"
)

// CreateNotification is the cross-cutting notification write path; the
// notification read side lives in internal/modules/notifications. The
// comments and interactions modules mirror this INSERT until the event bus
// lands and the notifications module takes over the write side.
func (r *Repository) CreateNotification(ctx context.Context, recipientID, actorID int64, notificationType string, videoID, commentID int64, videoTitle, commentPreview string) error {
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
