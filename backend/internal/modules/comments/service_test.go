package comments

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
)

func TestServiceValidationAndDelegation(t *testing.T) {
	repo := &serviceRepositoryStub{}
	svc := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()
	for name, content := range map[string]string{
		"empty":    "",
		"blanks":   "   ",
		"too long": strings.Repeat("界", 501),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.CreateComment(ctx, 7, 5, content); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("create comment error = %v, want invalid input", err)
			}
		})
	}
	if repo.videoRequested {
		t.Fatalf("invalid content reached repository: %#v", repo)
	}
	if err := svc.DeleteComment(ctx, 0, 5, 3); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("delete invalid user error = %v", err)
	}
	if err := svc.DeleteComment(ctx, 7, 0, 3); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("delete invalid video error = %v", err)
	}
	if err := svc.DeleteComment(ctx, 7, 5, 0); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("delete invalid comment error = %v", err)
	}
	if repo.deleteCalled {
		t.Fatalf("invalid delete reached repository: %#v", repo)
	}
	comment := domain.Comment{ID: 9, UserID: 7, Content: "hello"}
	repo.comment = comment
	created, err := svc.CreateComment(ctx, 7, 5, "  hello  ")
	if err != nil || created.ID != 9 || repo.createUserID != 7 || repo.createVideoID != 5 || repo.createContent != "hello" {
		t.Fatalf("create = %#v err=%v repo=%#v", created, err, repo)
	}
	if !repo.notificationCalled || repo.notificationRecipient != 4 || repo.notificationActor != 7 ||
		repo.notificationType != "comment" || repo.notificationVideoID != 5 || repo.notificationCommentID != 9 ||
		repo.notificationVideoTitle != "Video title" || repo.notificationPreview != "hello" {
		t.Fatalf("author notification = %#v", repo)
	}
	if err := svc.DeleteComment(ctx, 7, 5, 3); err != nil || !repo.deleteCalled ||
		repo.deleteUserID != 7 || repo.deleteVideoID != 5 || repo.deleteCommentID != 3 {
		t.Fatalf("delete = %v repo=%#v", err, repo)
	}
	repo.comments = []domain.Comment{comment}
	listed, err := svc.Comments(ctx, 5, 2)
	if err != nil || len(listed) != 1 || repo.authVideoID != 5 || repo.authViewerID != 2 || repo.listVideoID != 5 {
		t.Fatalf("list = %#v err=%v repo=%#v", listed, err, repo)
	}
}

func TestServiceNotificationFailureDoesNotFailComment(t *testing.T) {
	repo := &serviceRepositoryStub{notificationErr: errors.New("notification write failed"), comment: domain.Comment{ID: 9, Content: "hello"}}
	svc := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	comment, err := svc.CreateComment(context.Background(), 7, 5, "hello")
	if err != nil || comment.ID != 9 {
		t.Fatalf("comment = %#v err=%v; notification failures must only be logged", comment, err)
	}
}

type serviceRepositoryStub struct {
	comments                                    []domain.Comment
	comment                                     domain.Comment
	videoRequested                              bool
	videoErr                                    error
	authVideoID                                 int64
	authViewerID                                int64
	listCalled                                  bool
	listVideoID                                 int64
	createCalled                                bool
	createUserID                                int64
	createVideoID                               int64
	createContent                               string
	deleteCalled                                bool
	deleteUserID                                int64
	deleteVideoID                               int64
	deleteCommentID                             int64
	notificationCalled                          bool
	notificationErr                             error
	notificationRecipient, notificationActor    int64
	notificationType                            string
	notificationVideoID, notificationCommentID  int64
	notificationVideoTitle, notificationPreview string
}

func (s *serviceRepositoryStub) VideoAuthorID(_ context.Context, videoID, viewerID int64) (int64, string, error) {
	s.videoRequested, s.authVideoID, s.authViewerID = true, videoID, viewerID
	if s.videoErr != nil {
		return 0, "", s.videoErr
	}
	return 4, "Video title", nil
}

func (s *serviceRepositoryStub) ListComments(_ context.Context, videoID int64) ([]domain.Comment, error) {
	s.listCalled, s.listVideoID = true, videoID
	return s.comments, nil
}

func (s *serviceRepositoryStub) CreateComment(_ context.Context, userID, videoID int64, content string) (domain.Comment, error) {
	s.createCalled, s.createUserID, s.createVideoID, s.createContent = true, userID, videoID, content
	return s.comment, nil
}

func (s *serviceRepositoryStub) DeleteComment(_ context.Context, userID, videoID, commentID int64) error {
	s.deleteCalled, s.deleteUserID, s.deleteVideoID, s.deleteCommentID = true, userID, videoID, commentID
	return nil
}

func (s *serviceRepositoryStub) createNotification(_ context.Context, recipientID, actorID int64, notificationType string, videoID, commentID int64, videoTitle, commentPreview string) error {
	s.notificationCalled, s.notificationRecipient, s.notificationActor, s.notificationType = true, recipientID, actorID, notificationType
	s.notificationVideoID, s.notificationCommentID, s.notificationVideoTitle, s.notificationPreview = videoID, commentID, videoTitle, commentPreview
	return s.notificationErr
}

// Migrated from internal/service/service_test.go; the module keeps enforcing
// the core video visibility predicate for comment reads and writes.
func TestPrivateVideoCommentsAuthorization(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	ctx := context.Background()
	owner, err := legacy.CreateUser(ctx, "private_owner", "hash")
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := legacy.CreateUser(ctx, "private_viewer", "hash")
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

	if _, err := svc.Comments(ctx, privateVideo.ID, 0); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("anonymous private comments error = %v, want not found", err)
	}
	if _, err := svc.CreateComment(ctx, viewer.ID, privateVideo.ID, "no access"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("viewer private comment error = %v, want not found", err)
	}
	comment, err := svc.CreateComment(ctx, owner.ID, privateVideo.ID, "owner comment")
	if err != nil || comment.UserID != owner.ID {
		t.Fatalf("owner private comment = %#v err=%v", comment, err)
	}
	comments, err := svc.Comments(ctx, privateVideo.ID, owner.ID)
	if err != nil || len(comments) != 1 {
		t.Fatalf("owner private comments = %#v err=%v", comments, err)
	}
}
