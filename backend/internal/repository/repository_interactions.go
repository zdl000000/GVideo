package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

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
