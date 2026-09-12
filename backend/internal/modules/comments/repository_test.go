package comments

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/modules/notifications"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
)

// Migrated from internal/repository/repository_test.go; comment persistence and
// ownership rules are now owned by the module repository.
func TestDeleteCommentAuthorization(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "comments.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	repo := NewRepository(db)
	ctx := context.Background()
	owner, err := legacy.CreateUser(ctx, "comment_video_owner", "hash")
	if err != nil {
		t.Fatal(err)
	}
	author, err := legacy.CreateUser(ctx, "comment_author", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := legacy.CreateUser(ctx, "comment_other", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := legacy.CreateVideo(ctx, domain.NewVideo{UserID: owner.ID, Title: "Comments", Category: "知识", VideoPath: "videos/comments.mp4", MimeType: "video/mp4", SizeBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	comment, err := repo.CreateComment(ctx, author.ID, video.ID, "remove me")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteComment(ctx, other.ID, video.ID, comment.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("other delete error = %v, want forbidden", err)
	}
	if err := repo.DeleteComment(ctx, owner.ID, video.ID, comment.ID); err != nil {
		t.Fatalf("video owner delete: %v", err)
	}
	comments, err := repo.ListComments(ctx, video.ID)
	if err != nil || len(comments) != 0 {
		t.Fatalf("comments after delete = %#v err=%v", comments, err)
	}
	comment, err = repo.CreateComment(ctx, author.ID, video.ID, "author removes")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteComment(ctx, author.ID, video.ID, comment.ID); err != nil {
		t.Fatalf("comment author delete: %v", err)
	}
}

func TestCreateCommentReadsBackSnapshot(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "comments.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	repo := NewRepository(db)
	ctx := context.Background()
	user, err := legacy.CreateUser(ctx, "comment_snapshot_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := legacy.CreateVideo(ctx, domain.NewVideo{UserID: user.ID, Title: "Snapshot", Category: "knowledge", VideoPath: "videos/snapshot.mp4", MimeType: "video/mp4", SizeBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	comment, err := repo.CreateComment(ctx, user.ID, video.ID, "useful")
	if err != nil || comment.Content != "useful" {
		t.Fatalf("create comment: %#v err=%v", comment, err)
	}
	if comment.VideoID != video.ID || comment.UserID != user.ID || comment.Username != user.Username {
		t.Fatalf("created comment snapshot = %#v", comment)
	}
	listed, err := repo.ListComments(ctx, video.ID)
	if err != nil || len(listed) != 1 || listed[0].ID != comment.ID {
		t.Fatalf("list comments = %#v err=%v", listed, err)
	}
}

// The mirrored INSERT must behave like the core CreateNotification write path,
// including the self-comment suppression, until the event bus lands.
func TestMirroredCreateNotificationSuppressesSelfComment(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "comments.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	repo := NewRepository(db)
	ctx := context.Background()
	owner, err := legacy.CreateUser(ctx, "comment_notify_owner", "hash")
	if err != nil {
		t.Fatal(err)
	}
	actor, err := legacy.CreateUser(ctx, "comment_notify_actor", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := legacy.CreateVideo(ctx, domain.NewVideo{UserID: owner.ID, Title: "Notify video", Category: "knowledge", VideoPath: "videos/notify.mp4", MimeType: "video/mp4", SizeBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	comment, err := repo.CreateComment(ctx, actor.ID, video.ID, "notify me")
	if err != nil {
		t.Fatal(err)
	}
	if err := notifications.NewRepository(db).Create(ctx, owner.ID, actor.ID, "comment", video.ID, comment.ID, video.Title, comment.Content); err != nil {
		t.Fatal(err)
	}
	if err := notifications.NewRepository(db).Create(ctx, owner.ID, owner.ID, "comment", video.ID, comment.ID, video.Title, "self comment"); err != nil {
		t.Fatal(err)
	}

	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE recipient_id = ?`, owner.ID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("notification count = %d, want exactly one (self comment suppressed)", total)
	}
	var notificationType, actorUsername, videoTitle, commentPreview string
	var notificationVideoID, notificationCommentID int64
	if err := db.QueryRowContext(ctx, `
SELECT type, actor_username, video_id, video_title, comment_id, comment_preview
FROM notifications WHERE recipient_id = ?`, owner.ID).
		Scan(&notificationType, &actorUsername, &notificationVideoID, &videoTitle, &notificationCommentID, &commentPreview); err != nil {
		t.Fatal(err)
	}
	if notificationType != "comment" || actorUsername != actor.Username || notificationVideoID != video.ID ||
		videoTitle != video.Title || notificationCommentID != comment.ID || commentPreview != comment.Content {
		t.Fatalf("comment notification snapshot = %q %q %d %q %d %q", notificationType, actorUsername, notificationVideoID, videoTitle, notificationCommentID, commentPreview)
	}
}
