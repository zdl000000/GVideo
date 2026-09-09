package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gvideo/backend/internal/domain"
)

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

func (r *Repository) DeleteOtherSessions(ctx context.Context, userID int64, currentTokenHash string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ? AND token_hash <> ?`, userID, currentTokenHash); err != nil {
		return fmt.Errorf("delete other sessions: %w", err)
	}
	return nil
}
