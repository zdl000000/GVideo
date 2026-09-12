package notifications

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/modules/comments"
	"gvideo/backend/internal/modules/interactions"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/platform/bus"
	"gvideo/backend/internal/repository"
)

func TestServiceValidationAndDelegation(t *testing.T) {
	repo := &serviceRepositoryStub{}
	svc := NewService(repo)
	ctx := context.Background()
	if _, err := svc.Notifications(ctx, 0, 1, 20); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("list invalid user error = %v", err)
	}
	if repo.listUserID != 0 {
		t.Fatalf("invalid user reached repository: %d", repo.listUserID)
	}
	if err := svc.MarkNotificationRead(ctx, 0, 1); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("mark read invalid user error = %v", err)
	}
	if err := svc.MarkNotificationRead(ctx, 7, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("mark read invalid id error = %v", err)
	}
	if err := svc.MarkAllNotificationsRead(ctx, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("mark all invalid user error = %v", err)
	}
	if repo.markUserID != 0 || repo.markAllUserID != 0 {
		t.Fatalf("invalid input reached repository: %#v", repo)
	}
	page := domain.NotificationPage{Total: 3, UnreadCount: 2}
	repo.page = page
	result, err := svc.Notifications(ctx, 7, 2, 20)
	if err != nil || result.Total != 3 || repo.listUserID != 7 || repo.listPage != 2 || repo.listPageSize != 20 {
		t.Fatalf("list = %#v err=%v repo=%#v", result, err, repo)
	}
	if err := svc.MarkNotificationRead(ctx, 7, 5); err != nil || repo.markUserID != 7 || repo.markNotificationID != 5 {
		t.Fatalf("mark read = %v repo=%#v", err, repo)
	}
	if err := svc.MarkAllNotificationsRead(ctx, 7); err != nil || repo.markAllUserID != 7 {
		t.Fatalf("mark all = %v repo=%#v", err, repo)
	}
}

type serviceRepositoryStub struct {
	page                                          domain.NotificationPage
	listUserID                                    int64
	listPage, listPageSize                        int
	markUserID, markNotificationID, markAllUserID int64
}

func (s *serviceRepositoryStub) ListNotifications(_ context.Context, userID int64, page, pageSize int) (domain.NotificationPage, error) {
	s.listUserID, s.listPage, s.listPageSize = userID, page, pageSize
	return s.page, nil
}

func (s *serviceRepositoryStub) MarkNotificationRead(_ context.Context, userID, notificationID int64) error {
	s.markUserID, s.markNotificationID = userID, notificationID
	return nil
}

func (s *serviceRepositoryStub) MarkAllNotificationsRead(_ context.Context, userID int64) error {
	s.markAllUserID = userID
	return nil
}

// Interaction, follow, and comment flows write notifications through their
// modules' mirrored INSERTs (the core write path stays until the event bus
// lands). This test pins them to the module read side.
func TestInteractionNotificationsAndSelfSuppression(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "notifications.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repository.New(db)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// 真实总线接线：本测试即「发布方事件 → notifications 写路径」的进程内
	// 端到端验证（同步分发，与生产 main.go 的装配一致）。
	eventBus := bus.New()
	notificationsRepo := NewRepository(db)
	eventBus.Subscribe(func(ctx context.Context, event bus.NotificationEvent) {
		if err := notificationsRepo.Create(ctx, event.RecipientID, event.ActorID, event.Kind, event.VideoID, event.CommentID, event.VideoTitle, event.CommentPreview); err != nil {
			logger.Warn("record notification", "kind", event.Kind, "error", err)
		}
	})
	commentSvc := comments.NewService(comments.NewRepository(db), eventBus, logger)
	interactionsSvc := interactions.NewService(interactions.NewRepository(db), eventBus, logger)
	svc := NewService(notificationsRepo)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "notification_owner", "hash")
	if err != nil {
		t.Fatal(err)
	}
	actor, err := repo.CreateUser(ctx, "notification_actor", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: owner.ID, Title: "Service notifications", Category: "knowledge",
		VideoPath: "videos/service-notifications.mp4", MimeType: "video/mp4", SizeBytes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	if active, err := interactionsSvc.ToggleFollow(ctx, actor.ID, owner.ID); err != nil || !active {
		t.Fatalf("follow: active=%v err=%v", active, err)
	}
	if active, err := interactionsSvc.ToggleLike(ctx, actor.ID, video.ID); err != nil || !active {
		t.Fatalf("like: active=%v err=%v", active, err)
	}
	if active, err := interactionsSvc.ToggleFavorite(ctx, actor.ID, video.ID); err != nil || !active {
		t.Fatalf("favorite: active=%v err=%v", active, err)
	}
	comment, err := commentSvc.CreateComment(ctx, actor.ID, video.ID, "service notification comment")
	if err != nil {
		t.Fatal(err)
	}
	page, err := svc.Notifications(ctx, owner.ID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 || page.UnreadCount != 4 {
		t.Fatalf("notification totals = %#v, want four unread", page)
	}
	types := make(map[string]domain.Notification, len(page.Items))
	for _, item := range page.Items {
		types[item.Type] = item
	}
	for _, notificationType := range []string{"follow", "like", "favorite", "comment"} {
		if _, ok := types[notificationType]; !ok {
			t.Fatalf("missing %q notification in %#v", notificationType, page.Items)
		}
	}
	if types["comment"].CommentID != comment.ID || types["comment"].CommentPreview != comment.Content {
		t.Fatalf("comment notification = %#v", types["comment"])
	}

	if active, err := interactionsSvc.ToggleFollow(ctx, actor.ID, owner.ID); err != nil || active {
		t.Fatalf("unfollow: active=%v err=%v", active, err)
	}
	if active, err := interactionsSvc.ToggleLike(ctx, actor.ID, video.ID); err != nil || active {
		t.Fatalf("unlike: active=%v err=%v", active, err)
	}
	if active, err := interactionsSvc.ToggleFavorite(ctx, actor.ID, video.ID); err != nil || active {
		t.Fatalf("unfavorite: active=%v err=%v", active, err)
	}
	if _, err := interactionsSvc.ToggleLike(ctx, owner.ID, video.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := interactionsSvc.ToggleFavorite(ctx, owner.ID, video.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := commentSvc.CreateComment(ctx, owner.ID, video.ID, "owner comment"); err != nil {
		t.Fatal(err)
	}
	page, err = svc.Notifications(ctx, owner.ID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 {
		t.Fatalf("toggle-off or self interaction created notifications: %#v", page.Items)
	}

	if err := svc.MarkNotificationRead(ctx, actor.ID, page.Items[0].ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other user mark read error = %v, want not found", err)
	}
	if err := svc.MarkNotificationRead(ctx, owner.ID, page.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkAllNotificationsRead(ctx, owner.ID); err != nil {
		t.Fatal(err)
	}
	page, err = svc.Notifications(ctx, owner.ID, 1, 20)
	if err != nil || page.UnreadCount != 0 {
		t.Fatalf("mark all result = %#v err=%v", page, err)
	}
}
