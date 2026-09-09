package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gvideo/backend/internal/domain"
)

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
