package interactions

import (
	"context"
	"log/slog"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform/bus"
)

type interactionRepository interface {
	VideoAuthorID(context.Context, int64, int64) (int64, string, error)
	UserExists(context.Context, int64) error
	ToggleLike(context.Context, int64, int64) (bool, error)
	ToggleFavorite(context.Context, int64, int64) (bool, error)
	ToggleFollow(context.Context, int64, int64) (bool, error)
}

// EventPublisher 由消费方定义：互动成功后发布通知事件，写路径统一由
// notifications 模块的订阅者接管（事件总线为同步分发，nil 时静默跳过）。
type EventPublisher interface {
	Publish(ctx context.Context, event bus.NotificationEvent)
}

// Service owns the interaction toggle rules and the interaction-notification
// write path.
type Service struct {
	repo      interactionRepository
	publisher EventPublisher
	logger    *slog.Logger
}

func NewService(repo interactionRepository, publisher EventPublisher, logger *slog.Logger) *Service {
	return &Service{repo: repo, publisher: publisher, logger: logger}
}

// publish 在已接入事件总线时同步分发通知事件；nil 发布方（部分测试）静默跳过。
func (s *Service) publish(ctx context.Context, event bus.NotificationEvent) {
	if s.publisher == nil {
		return
	}
	s.publisher.Publish(ctx, event)
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
		s.publish(ctx, bus.NotificationEvent{RecipientID: authorID, ActorID: userID, Kind: "like", VideoID: videoID, VideoTitle: videoTitle})
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
		s.publish(ctx, bus.NotificationEvent{RecipientID: authorID, ActorID: userID, Kind: "favorite", VideoID: videoID, VideoTitle: videoTitle})
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
		s.publish(ctx, bus.NotificationEvent{RecipientID: followedID, ActorID: followerID, Kind: "follow"})
	}
	return active, nil
}
