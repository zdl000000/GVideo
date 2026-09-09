package service

import (
	"context"
)

func (s *Service) ToggleLike(ctx context.Context, userID, videoID int64) (bool, error) {
	video, err := s.repo.VideoByID(ctx, videoID, userID)
	if err != nil {
		return false, err
	}
	active, err := s.repo.ToggleLike(ctx, userID, videoID)
	if err != nil {
		return false, err
	}
	if active {
		if err := s.repo.CreateNotification(ctx, video.UserID, userID, "like", videoID, 0, video.Title, ""); err != nil {
			s.logger.Warn("create like notification", "recipient_id", video.UserID, "video_id", videoID, "error", err)
		}
	}
	return active, nil
}

func (s *Service) ToggleFavorite(ctx context.Context, userID, videoID int64) (bool, error) {
	video, err := s.repo.VideoByID(ctx, videoID, userID)
	if err != nil {
		return false, err
	}
	active, err := s.repo.ToggleFavorite(ctx, userID, videoID)
	if err != nil {
		return false, err
	}
	if active {
		if err := s.repo.CreateNotification(ctx, video.UserID, userID, "favorite", videoID, 0, video.Title, ""); err != nil {
			s.logger.Warn("create favorite notification", "recipient_id", video.UserID, "video_id", videoID, "error", err)
		}
	}
	return active, nil
}
