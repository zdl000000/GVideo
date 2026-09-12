package interactions

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
)

func TestServiceValidationAndDelegation(t *testing.T) {
	repo := &serviceRepositoryStub{authorID: 4, videoTitle: "Video title"}
	svc := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	if _, err := svc.ToggleFollow(ctx, 0, 4); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("follow invalid follower error = %v", err)
	}
	if _, err := svc.ToggleFollow(ctx, 7, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("follow invalid followed error = %v", err)
	}
	if _, err := svc.ToggleFollow(ctx, 7, 7); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("self follow error = %v", err)
	}
	if repo.userExistsCalled || repo.followCalled {
		t.Fatalf("invalid follow reached repository: %#v", repo)
	}
	repo.userErr = domain.ErrNotFound
	if _, err := svc.ToggleFollow(ctx, 7, 99999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing user follow error = %v, want not found", err)
	}
	repo.userErr = nil
	repo.active = true
	active, err := svc.ToggleFollow(ctx, 7, 4)
	if err != nil || !active || !repo.userExistsCalled || repo.userExistsID != 4 ||
		!repo.followCalled || repo.followFollowerID != 7 || repo.followFollowedID != 4 {
		t.Fatalf("follow = %v err=%v repo=%#v", active, err, repo)
	}
	if !repo.notificationCalled || repo.notificationRecipient != 4 || repo.notificationActor != 7 ||
		repo.notificationType != "follow" || repo.notificationVideoID != 0 || repo.notificationCommentID != 0 ||
		repo.notificationVideoTitle != "" || repo.notificationPreview != "" {
		t.Fatalf("follow notification = %#v", repo)
	}

	active, err = svc.ToggleLike(ctx, 7, 5)
	if err != nil || !active || !repo.likeCalled || repo.likeUserID != 7 || repo.likeVideoID != 5 ||
		repo.authVideoID != 5 || repo.authViewerID != 7 {
		t.Fatalf("like = %v err=%v repo=%#v", active, err, repo)
	}
	if !repo.notificationCalled || repo.notificationRecipient != 4 || repo.notificationActor != 7 ||
		repo.notificationType != "like" || repo.notificationVideoID != 5 || repo.notificationCommentID != 0 ||
		repo.notificationVideoTitle != "Video title" || repo.notificationPreview != "" {
		t.Fatalf("like notification = %#v", repo)
	}

	active, err = svc.ToggleFavorite(ctx, 7, 5)
	if err != nil || !active || !repo.favoriteCalled || repo.favoriteUserID != 7 || repo.favoriteVideoID != 5 {
		t.Fatalf("favorite = %v err=%v repo=%#v", active, err, repo)
	}
	if repo.notificationType != "favorite" {
		t.Fatalf("favorite notification type = %q", repo.notificationType)
	}
}

func TestServiceNotificationFailureDoesNotFailToggle(t *testing.T) {
	repo := &serviceRepositoryStub{authorID: 4, videoTitle: "Video title", active: true, notificationErr: errors.New("notification write failed")}
	svc := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	active, err := svc.ToggleLike(context.Background(), 7, 5)
	if err != nil || !active {
		t.Fatalf("like = %v err=%v; notification failures must only be logged", active, err)
	}
}

// Migrated from internal/service/service_test.go; the module keeps enforcing
// the core video visibility predicate and the follow rules.
func TestPrivateVideoInteractionAndFollowRules(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	ctx := context.Background()
	owner, err := legacy.CreateUser(ctx, "interaction_owner", "hash")
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := legacy.CreateUser(ctx, "interaction_viewer", "hash")
	if err != nil {
		t.Fatal(err)
	}
	privateVideo, err := legacy.CreateVideo(ctx, domain.NewVideo{
		UserID: owner.ID, Title: "Private video", Category: "knowledge", Visibility: "private",
		VideoPath: "videos/private.mp4", MimeType: "video/mp4", SizeBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(NewRepository(db), slog.New(slog.NewTextHandler(io.Discard, nil)))

	if _, err := svc.ToggleLike(ctx, viewer.ID, privateVideo.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("viewer private like error = %v, want not found", err)
	}
	if _, err := svc.ToggleFavorite(ctx, viewer.ID, privateVideo.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("viewer private favorite error = %v, want not found", err)
	}
	if _, err := svc.ToggleFollow(ctx, viewer.ID, viewer.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("self follow error = %v, want forbidden", err)
	}
	if _, err := svc.ToggleFollow(ctx, viewer.ID, 99999); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing user follow error = %v, want not found", err)
	}
	active, err := svc.ToggleFollow(ctx, viewer.ID, owner.ID)
	if err != nil || !active {
		t.Fatalf("follow user: active=%v err=%v", active, err)
	}
	active, err = svc.ToggleLike(ctx, owner.ID, privateVideo.ID)
	if err != nil || !active {
		t.Fatalf("owner private like: active=%v err=%v", active, err)
	}
	var follows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_follows WHERE follower_id = ? AND followed_id = ?`, viewer.ID, owner.ID).Scan(&follows); err != nil || follows != 1 {
		t.Fatalf("follow rows=%d err=%v", follows, err)
	}
	var likes int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM video_likes WHERE user_id = ? AND video_id = ?`, owner.ID, privateVideo.ID).Scan(&likes); err != nil || likes != 1 {
		t.Fatalf("like rows=%d err=%v", likes, err)
	}
	profile, err := legacy.CreatorProfile(ctx, owner.ID, viewer.ID)
	if err != nil || !profile.Followed || profile.FollowersCount != 1 {
		t.Fatalf("creator profile after follow: %#v err=%v", profile, err)
	}
	active, err = svc.ToggleFollow(ctx, viewer.ID, owner.ID)
	if err != nil || active {
		t.Fatalf("unfollow user: active=%v err=%v", active, err)
	}
	profile, err = legacy.CreatorProfile(ctx, owner.ID, viewer.ID)
	if err != nil || profile.Followed || profile.FollowersCount != 0 {
		t.Fatalf("profile after unfollow: %#v err=%v", profile, err)
	}
}

type serviceRepositoryStub struct {
	authorID                                    int64
	videoTitle                                  string
	active                                      bool
	userErr                                     error
	notificationErr                             error
	userExistsCalled                            bool
	userExistsID                                int64
	followCalled                                bool
	followFollowerID, followFollowedID          int64
	likeCalled                                  bool
	likeUserID, likeVideoID                     int64
	favoriteCalled                              bool
	favoriteUserID, favoriteVideoID             int64
	authVideoID, authViewerID                   int64
	notificationCalled                          bool
	notificationRecipient, notificationActor    int64
	notificationType                            string
	notificationVideoID, notificationCommentID  int64
	notificationVideoTitle, notificationPreview string
}

func (s *serviceRepositoryStub) VideoAuthorID(_ context.Context, videoID, viewerID int64) (int64, string, error) {
	s.authVideoID, s.authViewerID = videoID, viewerID
	return s.authorID, s.videoTitle, nil
}

func (s *serviceRepositoryStub) UserExists(_ context.Context, userID int64) error {
	s.userExistsCalled, s.userExistsID = true, userID
	return s.userErr
}

func (s *serviceRepositoryStub) ToggleLike(_ context.Context, userID, videoID int64) (bool, error) {
	s.likeCalled, s.likeUserID, s.likeVideoID = true, userID, videoID
	return s.active, nil
}

func (s *serviceRepositoryStub) ToggleFavorite(_ context.Context, userID, videoID int64) (bool, error) {
	s.favoriteCalled, s.favoriteUserID, s.favoriteVideoID = true, userID, videoID
	return s.active, nil
}

func (s *serviceRepositoryStub) ToggleFollow(_ context.Context, followerID, followedID int64) (bool, error) {
	s.followCalled, s.followFollowerID, s.followFollowedID = true, followerID, followedID
	return s.active, nil
}

func (s *serviceRepositoryStub) createNotification(_ context.Context, recipientID, actorID int64, notificationType string, videoID, commentID int64, videoTitle, commentPreview string) error {
	s.notificationCalled, s.notificationRecipient, s.notificationActor, s.notificationType = true, recipientID, actorID, notificationType
	s.notificationVideoID, s.notificationCommentID, s.notificationVideoTitle, s.notificationPreview = videoID, commentID, videoTitle, commentPreview
	return s.notificationErr
}
