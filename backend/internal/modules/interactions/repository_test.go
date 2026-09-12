package interactions

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/modules/notifications"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
)

// Migrated from internal/repository/repository_test.go; interaction toggle
// persistence is now owned by the module repository.
func TestToggleLikeAndFavoritePersistence(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "interactions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	repo := NewRepository(db)
	ctx := context.Background()
	user, err := legacy.CreateUser(ctx, "interaction_toggle_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := legacy.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Interaction video", Category: "knowledge",
		VideoPath: "videos/interactions.mp4", MimeType: "video/mp4", SizeBytes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	active, err := repo.ToggleLike(ctx, user.ID, video.ID)
	if err != nil || !active {
		t.Fatalf("toggle like on: active=%v err=%v", active, err)
	}
	video, err = legacy.VideoByID(ctx, video.ID, user.ID)
	if err != nil || !video.Liked || video.LikesCount != 1 {
		t.Fatalf("liked video = %#v err=%v", video, err)
	}
	active, err = repo.ToggleLike(ctx, user.ID, video.ID)
	if err != nil || active {
		t.Fatalf("toggle like off: active=%v err=%v", active, err)
	}
	video, err = legacy.VideoByID(ctx, video.ID, user.ID)
	if err != nil || video.Liked || video.LikesCount != 0 {
		t.Fatalf("unliked video = %#v err=%v", video, err)
	}

	active, err = repo.ToggleFavorite(ctx, user.ID, video.ID)
	if err != nil || !active {
		t.Fatalf("toggle favorite on: active=%v err=%v", active, err)
	}
	video, err = legacy.VideoByID(ctx, video.ID, user.ID)
	if err != nil || !video.Favorited || video.FavoritesCount != 1 {
		t.Fatalf("favorited video = %#v err=%v", video, err)
	}
	active, err = repo.ToggleFavorite(ctx, user.ID, video.ID)
	if err != nil || active {
		t.Fatalf("toggle favorite off: active=%v err=%v", active, err)
	}
	video, err = legacy.VideoByID(ctx, video.ID, user.ID)
	if err != nil || video.Favorited || video.FavoritesCount != 0 {
		t.Fatalf("unfavorited video = %#v err=%v", video, err)
	}
}

// Migrated from internal/repository/repository_test.go; follow toggle
// persistence is now owned by the module repository.
func TestToggleFollowPersistence(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "follows.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	repo := NewRepository(db)
	ctx := context.Background()
	follower, err := legacy.CreateUser(ctx, "follow_toggle_follower", "hash")
	if err != nil {
		t.Fatal(err)
	}
	followed, err := legacy.CreateUser(ctx, "follow_toggle_followed", "hash")
	if err != nil {
		t.Fatal(err)
	}

	followCount := func() int {
		t.Helper()
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_follows WHERE follower_id = ? AND followed_id = ?`, follower.ID, followed.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	active, err := repo.ToggleFollow(ctx, follower.ID, followed.ID)
	if err != nil || !active {
		t.Fatalf("toggle follow on: active=%v err=%v", active, err)
	}
	if followCount() != 1 {
		t.Fatalf("follow row missing after toggle on")
	}
	active, err = repo.ToggleFollow(ctx, follower.ID, followed.ID)
	if err != nil || active {
		t.Fatalf("toggle follow off: active=%v err=%v", active, err)
	}
	if followCount() != 0 {
		t.Fatalf("follow row remains after toggle off")
	}
}

// The mirrored INSERT must behave like the core CreateNotification write path,
// including the self-interaction suppression, until the event bus lands.
func TestMirroredCreateNotificationSuppressesSelfInteractions(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "interactions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	ctx := context.Background()
	owner, err := legacy.CreateUser(ctx, "interaction_notify_owner", "hash")
	if err != nil {
		t.Fatal(err)
	}
	actor, err := legacy.CreateUser(ctx, "interaction_notify_actor", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := legacy.CreateVideo(ctx, domain.NewVideo{
		UserID: owner.ID, Title: "Interaction notify video", Category: "knowledge",
		VideoPath: "videos/interactions-notify.mp4", MimeType: "video/mp4", SizeBytes: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := notifications.NewRepository(db).Create(ctx, owner.ID, actor.ID, "like", video.ID, 0, video.Title, ""); err != nil {
		t.Fatal(err)
	}
	if err := notifications.NewRepository(db).Create(ctx, owner.ID, actor.ID, "favorite", video.ID, 0, video.Title, ""); err != nil {
		t.Fatal(err)
	}
	if err := notifications.NewRepository(db).Create(ctx, owner.ID, actor.ID, "follow", 0, 0, "", ""); err != nil {
		t.Fatal(err)
	}
	if err := notifications.NewRepository(db).Create(ctx, owner.ID, owner.ID, "like", video.ID, 0, video.Title, "self like"); err != nil {
		t.Fatal(err)
	}

	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE recipient_id = ?`, owner.ID).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != 3 {
		t.Fatalf("notification count = %d, want exactly three (self like suppressed)", total)
	}
	var notificationType, actorUsername, videoTitle string
	var notificationVideoID, notificationCommentID sql.NullInt64
	if err := db.QueryRowContext(ctx, `
SELECT type, actor_username, video_id, video_title, comment_id
FROM notifications WHERE recipient_id = ? AND type = 'like'`, owner.ID).
		Scan(&notificationType, &actorUsername, &notificationVideoID, &videoTitle, &notificationCommentID); err != nil {
		t.Fatal(err)
	}
	if notificationType != "like" || actorUsername != actor.Username || !notificationVideoID.Valid ||
		notificationVideoID.Int64 != video.ID || videoTitle != video.Title || notificationCommentID.Valid {
		t.Fatalf("like notification snapshot = %q %q %v %q %v", notificationType, actorUsername, notificationVideoID, videoTitle, notificationCommentID)
	}
	if err := db.QueryRowContext(ctx, `
SELECT video_id, video_title, comment_id FROM notifications
WHERE recipient_id = ? AND type = 'follow'`, owner.ID).
		Scan(&notificationVideoID, &videoTitle, &notificationCommentID); err != nil {
		t.Fatal(err)
	}
	if notificationVideoID.Valid || notificationCommentID.Valid || videoTitle != "" {
		t.Fatalf("follow notification snapshot = %v %q %v", notificationVideoID, videoTitle, notificationCommentID)
	}
}
