package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"gvideo/backend/internal/domain"
)

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
	err := r.db.QueryRowContext(ctx, `SELECT id, username, bio, avatar_path, created_at, password_hash, is_admin FROM users WHERE username = ?`, username).
		Scan(&user.ID, &user.Username, &user.Bio, &avatarPath, &user.CreatedAt, &passwordHash, &user.IsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, "", domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, "", fmt.Errorf("find user auth: %w", err)
	}
	user.AvatarURL = optionalMediaURL(avatarPath)
	return user, passwordHash, nil
}

func (r *Repository) UserAuthByID(ctx context.Context, id int64) (domain.User, string, error) {
	var user domain.User
	var avatarPath string
	var passwordHash string
	err := r.db.QueryRowContext(ctx, `SELECT id, username, bio, avatar_path, created_at, password_hash, is_admin FROM users WHERE id = ?`, id).
		Scan(&user.ID, &user.Username, &user.Bio, &avatarPath, &user.CreatedAt, &passwordHash, &user.IsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, "", domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, "", fmt.Errorf("find user auth by id: %w", err)
	}
	user.AvatarURL = optionalMediaURL(avatarPath)
	return user, passwordHash, nil
}

func (r *Repository) UserByID(ctx context.Context, id int64) (domain.User, error) {
	var user domain.User
	var avatarPath string
	err := r.db.QueryRowContext(ctx, `SELECT id, username, bio, avatar_path, created_at, is_admin FROM users WHERE id = ?`, id).
		Scan(&user.ID, &user.Username, &user.Bio, &avatarPath, &user.CreatedAt, &user.IsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("find user: %w", err)
	}
	user.AvatarURL = optionalMediaURL(avatarPath)
	return user, nil
}

// CreateAdminUser creates the first administrator atomically: the insert only
// succeeds while no administrator exists, so concurrent registrations cannot
// both claim the reserved name.
func (r *Repository) CreateAdminUser(ctx context.Context, username, passwordHash string) (domain.User, error) {
	result, err := r.db.ExecContext(ctx, `
INSERT INTO users(username, password_hash, is_admin)
SELECT ?, ?, 1
WHERE NOT EXISTS (SELECT 1 FROM users WHERE is_admin = 1)`, username, passwordHash)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return domain.User{}, domain.ErrConflict
		}
		return domain.User{}, fmt.Errorf("create admin user: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.User{}, domain.ErrConflict
	}
	id, _ := result.LastInsertId()
	return r.UserByID(ctx, id)
}

// HasAdmin reports whether any administrator exists.
func (r *Repository) HasAdmin(ctx context.Context) (bool, error) {
	var exists int
	if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE is_admin = 1)`).Scan(&exists); err != nil {
		return false, fmt.Errorf("check admin existence: %w", err)
	}
	return exists == 1, nil
}

// SetAdminFlag updates the administrator flag for one user.
func (r *Repository) SetAdminFlag(ctx context.Context, userID int64, isAdmin bool) error {
	value := 0
	if isAdmin {
		value = 1
	}
	result, err := r.db.ExecContext(ctx, `UPDATE users SET is_admin = ? WHERE id = ?`, value, userID)
	if err != nil {
		return fmt.Errorf("set admin flag: %w", err)
	}
	if changed, _ := result.RowsAffected(); changed != 1 {
		return domain.ErrNotFound
	}
	return nil
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

func (r *Repository) UpdatePasswordHash(ctx context.Context, userID int64, passwordHash string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.ErrNotFound
	}
	return nil
}
