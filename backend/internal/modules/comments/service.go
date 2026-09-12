package comments

import (
	"context"
	"log/slog"
	"strings"

	"gvideo/backend/internal/domain"
)

type commentRepository interface {
	VideoAuthorID(context.Context, int64, int64) (int64, string, error)
	ListComments(context.Context, int64) ([]domain.Comment, error)
	CreateComment(context.Context, int64, int64, string) (domain.Comment, error)
	DeleteComment(context.Context, int64, int64, int64) error
	createNotification(context.Context, int64, int64, string, int64, int64, string, string) error
}

// Service owns comment validation, video visibility checks, and the
// comment-notification write path.
type Service struct {
	repo   commentRepository
	logger *slog.Logger
}

func NewService(repo commentRepository, logger *slog.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) Comments(ctx context.Context, videoID, viewerID int64) ([]domain.Comment, error) {
	if _, _, err := s.repo.VideoAuthorID(ctx, videoID, viewerID); err != nil {
		return nil, err
	}
	return s.repo.ListComments(ctx, videoID)
}

func (s *Service) CreateComment(ctx context.Context, userID, videoID int64, content string) (domain.Comment, error) {
	content = strings.TrimSpace(content)
	if len([]rune(content)) < 1 || len([]rune(content)) > 500 {
		return domain.Comment{}, domain.ErrInvalidInput
	}
	authorID, videoTitle, err := s.repo.VideoAuthorID(ctx, videoID, userID)
	if err != nil {
		return domain.Comment{}, err
	}
	comment, err := s.repo.CreateComment(ctx, userID, videoID, content)
	if err != nil {
		return domain.Comment{}, err
	}
	if err := s.repo.createNotification(ctx, authorID, userID, "comment", videoID, comment.ID, videoTitle, comment.Content); err != nil {
		s.logger.Warn("create comment notification", "recipient_id", authorID, "video_id", videoID, "error", err)
	}
	return comment, nil
}

func (s *Service) DeleteComment(ctx context.Context, userID, videoID, commentID int64) error {
	if userID <= 0 || videoID <= 0 || commentID <= 0 {
		return domain.ErrInvalidInput
	}
	return s.repo.DeleteComment(ctx, userID, videoID, commentID)
}
