package notifications

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/modules/comments"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
)

func TestNotificationsLifecycleAndOwnership(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "notifications.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	repo := NewRepository(db)
	ctx := context.Background()
	recipient, err := legacy.CreateUser(ctx, "notification_recipient", "hash")
	if err != nil {
		t.Fatal(err)
	}
	actor, err := legacy.CreateUser(ctx, "notification_actor", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := legacy.CreateUser(ctx, "notification_other", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := legacy.CreateVideo(ctx, domain.NewVideo{
		UserID: recipient.ID, Title: "Notification video", Category: "knowledge",
		VideoPath: "videos/notification.mp4", MimeType: "video/mp4", SizeBytes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	comment, err := comments.NewRepository(db).CreateComment(ctx, actor.ID, video.ID, "notification comment")
	if err != nil {
		t.Fatal(err)
	}

	if err := repo.Create(ctx, recipient.ID, actor.ID, "comment", video.ID, comment.ID, video.Title, comment.Content); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, recipient.ID, actor.ID, "like", video.ID, 0, video.Title, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, recipient.ID, recipient.ID, "favorite", video.ID, 0, video.Title, ""); err != nil {
		t.Fatal(err)
	}

	page, err := repo.ListNotifications(ctx, recipient.ID, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || page.UnreadCount != 2 || len(page.Items) != 1 || !page.HasNext {
		t.Fatalf("unexpected first notification page: %#v", page)
	}
	first := page.Items[0]
	if first.ActorID != actor.ID || first.ActorUsername != actor.Username || first.VideoID != video.ID || first.VideoTitle != video.Title {
		t.Fatalf("notification snapshots not populated: %#v", first)
	}

	if err := repo.MarkNotificationRead(ctx, other.ID, first.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other user mark read error = %v, want not found", err)
	}
	if err := repo.MarkNotificationRead(ctx, recipient.ID, first.ID); err != nil {
		t.Fatal(err)
	}
	page, err = repo.ListNotifications(ctx, recipient.ID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if page.UnreadCount != 1 || len(page.Items) != 2 || page.Items[0].ReadAt == nil {
		t.Fatalf("single read state not persisted: %#v", page)
	}
	if err := repo.MarkAllNotificationsRead(ctx, recipient.ID); err != nil {
		t.Fatal(err)
	}
	page, err = repo.ListNotifications(ctx, recipient.ID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if page.UnreadCount != 0 {
		t.Fatalf("unread count = %d, want 0", page.UnreadCount)
	}
	otherPage, err := repo.ListNotifications(ctx, other.ID, 1, 20)
	if err != nil || otherPage.Total != 0 {
		t.Fatalf("other user notifications = %#v err=%v", otherPage, err)
	}
}
