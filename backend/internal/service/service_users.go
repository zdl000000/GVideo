package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gvideo/backend/internal/domain"
)

func (s *Service) CreatorProfile(ctx context.Context, userID, viewerID int64) (domain.CreatorProfile, error) {
	if userID <= 0 {
		return domain.CreatorProfile{}, domain.ErrInvalidInput
	}
	return s.repo.CreatorProfile(ctx, userID, viewerID)
}

func (s *Service) UpdateProfile(ctx context.Context, input UpdateProfileInput) (domain.User, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.Bio = strings.TrimSpace(input.Bio)
	if input.UserID <= 0 || !usernamePattern.MatchString(input.Username) || len([]rune(input.Bio)) > 300 {
		return domain.User{}, domain.ErrInvalidInput
	}
	current, err := s.repo.UserByID(ctx, input.UserID)
	if err != nil {
		return domain.User{}, err
	}
	// Renaming to the reserved administrator name would silently claim the
	// administrator identity, so it is rejected; keeping the current name is
	// still fine for profile-only edits.
	if s.isReservedAdminName(input.Username) && !strings.EqualFold(strings.TrimSpace(current.Username), input.Username) {
		return domain.User{}, domain.ErrForbidden
	}

	var avatarPath *string
	var savedPath string
	if input.Avatar != nil && input.Avatar.Size > 0 {
		ext, err := inspectImage(input.Avatar)
		if err != nil {
			return domain.User{}, err
		}
		token, err := randomToken(16)
		if err != nil {
			return domain.User{}, err
		}
		relative := filepath.Join("avatars", token+ext)
		savedPath = filepath.Join(s.cfg.MediaDir, relative)
		if err := os.MkdirAll(filepath.Dir(savedPath), 0o755); err != nil {
			return domain.User{}, fmt.Errorf("create avatar directory: %w", err)
		}
		if err := saveMultipart(input.Avatar, savedPath); err != nil {
			return domain.User{}, err
		}
		avatarPath = &relative
	}

	updated, err := s.repo.UpdateProfile(ctx, input.UserID, input.Username, input.Bio, avatarPath)
	if err != nil {
		if savedPath != "" {
			_ = os.Remove(savedPath)
		}
		return domain.User{}, err
	}
	if avatarPath != nil && current.AvatarURL != "" {
		s.removeMediaPath(strings.TrimPrefix(current.AvatarURL, "/media/"), false)
	}
	return updated, nil
}

func (s *Service) CreatorStats(ctx context.Context, userID int64) (domain.CreatorStats, error) {
	if userID <= 0 {
		return domain.CreatorStats{}, domain.ErrInvalidInput
	}
	if _, err := s.repo.UserByID(ctx, userID); err != nil {
		return domain.CreatorStats{}, err
	}
	stats, err := s.repo.CreatorStats(ctx, userID)
	if err != nil {
		return domain.CreatorStats{}, err
	}
	for index := range stats.RecentVideos {
		stats.RecentVideos[index] = publicVideo(stats.RecentVideos[index])
	}
	return stats, nil
}

func (s *Service) ToggleFollow(ctx context.Context, followerID, followedID int64) (bool, error) {
	if followerID <= 0 || followedID <= 0 {
		return false, domain.ErrInvalidInput
	}
	if followerID == followedID {
		return false, domain.ErrForbidden
	}
	if _, err := s.repo.UserByID(ctx, followedID); err != nil {
		return false, err
	}
	active, err := s.repo.ToggleFollow(ctx, followerID, followedID)
	if err != nil {
		return false, err
	}
	if active {
		if err := s.repo.CreateNotification(ctx, followedID, followerID, "follow", 0, 0, "", ""); err != nil {
			s.logger.Warn("create follow notification", "recipient_id", followedID, "actor_id", followerID, "error", err)
		}
	}
	return active, nil
}
