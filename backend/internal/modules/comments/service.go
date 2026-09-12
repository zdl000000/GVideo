package comments

import (
	"context"
	"log/slog"
	"strings"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform/bus"
)

type commentRepository interface {
	VideoAuthorID(context.Context, int64, int64) (int64, string, error)
	ListComments(context.Context, int64) ([]domain.Comment, error)
	CreateComment(context.Context, int64, int64, string) (domain.Comment, error)
	DeleteComment(context.Context, int64, int64, int64) error
}

// EventPublisher 由消费方定义：评论成功后发布通知事件，写路径统一由
// notifications 模块的订阅者接管（事件总线为同步分发，nil 时静默跳过）。
type EventPublisher interface {
	Publish(ctx context.Context, event bus.NotificationEvent)
}

// Service owns comment validation, video visibility checks, and the
// comment-notification write path.
type Service struct {
	repo      commentRepository
	publisher EventPublisher
	logger    *slog.Logger
}

func NewService(repo commentRepository, publisher EventPublisher, logger *slog.Logger) *Service {
	return &Service{repo: repo, publisher: publisher, logger: logger}
}

// publish 在已接入事件总线时同步分发通知事件；nil 发布方（部分测试）静默跳过。
func (s *Service) publish(ctx context.Context, event bus.NotificationEvent) {
	if s.publisher == nil {
		return
	}
	s.publisher.Publish(ctx, event)
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
	s.publish(ctx, bus.NotificationEvent{
		RecipientID:    authorID,
		ActorID:        userID,
		Kind:           "comment",
		VideoID:        videoID,
		CommentID:      comment.ID,
		VideoTitle:     videoTitle,
		CommentPreview: comment.Content,
	})
	return comment, nil
}

func (s *Service) DeleteComment(ctx context.Context, userID, videoID, commentID int64) error {
	if userID <= 0 || videoID <= 0 || commentID <= 0 {
		return domain.ErrInvalidInput
	}
	return s.repo.DeleteComment(ctx, userID, videoID, commentID)
}
