package service

import (
	"context"
	"strings"

	"gvideo/backend/internal/domain"
)

func (s *Service) Comments(ctx context.Context, videoID, viewerID int64) ([]domain.Comment, error) {
	if _, err := s.repo.VideoByID(ctx, videoID, viewerID); err != nil {
		return nil, err
	}
	return s.repo.ListComments(ctx, videoID)
}

func (s *Service) CreateComment(ctx context.Context, userID, videoID int64, content string) (domain.Comment, error) {
	content = strings.TrimSpace(content)
	if len([]rune(content)) < 1 || len([]rune(content)) > 500 {
		return domain.Comment{}, domain.ErrInvalidInput
	}
	video, err := s.repo.VideoByID(ctx, videoID, userID)
	if err != nil {
		return domain.Comment{}, err
	}
	comment, err := s.repo.CreateComment(ctx, userID, videoID, content)
	if err != nil {
		return domain.Comment{}, err
	}
	if err := s.repo.CreateNotification(ctx, video.UserID, userID, "comment", videoID, comment.ID, video.Title, comment.Content); err != nil {
		s.logger.Warn("create comment notification", "recipient_id", video.UserID, "video_id", videoID, "error", err)
	}
	return comment, nil
}

func (s *Service) DeleteComment(ctx context.Context, userID, videoID, commentID int64) error {
	if userID <= 0 || videoID <= 0 || commentID <= 0 {
		return domain.ErrInvalidInput
	}
	return s.repo.DeleteComment(ctx, userID, videoID, commentID)
}
