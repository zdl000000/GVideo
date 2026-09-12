package interactions

import (
	"context"
	"log/slog"

	"gvideo/backend/internal/domain"
)

type interactionRepository interface {
	VideoAuthorID(context.Context, int64, int64) (int64, string, error)
	UserExists(context.Context, int64) error
	ToggleLike(context.Context, int64, int64) (bool, error)
	ToggleFavorite(context.Context, int64, int64) (bool, error)
	ToggleFollow(context.Context, int64, int64) (bool, error)
	createNotification(context.Context, int64, int64, string, int64, int64, string, string) error
}

// Service owns the interaction toggle rules and the interaction-notification
// write path.
type Service struct {
	repo   interactionRepository
	logger *slog.Logger
}

func NewService(repo interactionRepository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) ToggleLike(ctx context.Context, userID, videoID int64) (bool, error) {
	authorID, videoTitle, err := s.repo.VideoAuthorID(ctx, videoID, userID)
	if err != nil {
		return false, err
	}
	active, err := s.repo.ToggleLike(ctx, userID, videoID)
	if err != nil {
		return false, err
	}
	if active {
		if err := s.repo.createNotification(ctx, authorID, userID, "like", videoID, 0, videoTitle, ""); err != nil {
			s.logger.Warn("create like notification", "recipient_id", authorID, "video_id", videoID, "error", err)
		}
	}
	return active, nil
}

func (s *Service) ToggleFavorite(ctx context.Context, userID, videoID int64) (bool, error) {
	authorID, videoTitle, err := s.repo.VideoAuthorID(ctx, videoID, userID)
	if err != nil {
		return false, err
	}
	active, err := s.repo.ToggleFavorite(ctx, userID, videoID)
	if err != nil {
		return false, err
	}
	if active {
		if err := s.repo.createNotification(ctx, authorID, userID, "favorite", videoID, 0, videoTitle, ""); err != nil {
			s.logger.Warn("create favorite notification", "recipient_id", authorID, "video_id", videoID, "error", err)
		}
	}
	return active, nil
}

func (s *Service) ToggleFollow(ctx context.Context, followerID, followedID int64) (bool, error) {
	if followerID <= 0 || followedID <= 0 {
		return false, domain.ErrInvalidInput
	}
	if followerID == followedID {
		return false, domain.ErrForbidden
	}
	if err := s.repo.UserExists(ctx, followedID); err != nil {
		return false, err
	}
	active, err := s.repo.ToggleFollow(ctx, followerID, followedID)
	if err != nil {
		return false, err
	}
	if active {
		if err := s.repo.createNotification(ctx, followedID, followerID, "follow", 0, 0, "", ""); err != nil {
			s.logger.Warn("create follow notification", "recipient_id", followedID, "actor_id", followerID, "error", err)
		}
	}
	return active, nil
}
