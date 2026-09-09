package moderation

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
)

func TestRepositoryVideoReportLifecycle(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "reports.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	repo := NewRepository(db)
	ctx := context.Background()
	author, err := legacy.CreateUser(ctx, "report_review_author", "hash")
	if err != nil {
		t.Fatal(err)
	}
	reporter, err := legacy.CreateUser(ctx, "report_review_viewer", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := legacy.CreateVideo(ctx, domain.NewVideo{UserID: author.ID, Title: "Review target", Category: "知识", VideoPath: "videos/review.mp4", MimeType: "video/mp4", SizeBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	authorID, err := repo.VideoAuthorID(ctx, video.ID, reporter.ID)
	if err != nil || authorID != author.ID {
		t.Fatalf("author = %d err=%v", authorID, err)
	}
	report, err := repo.UpsertVideoReport(ctx, video.ID, reporter.ID, "spam", "review this")
	if err != nil {
		t.Fatal(err)
	}
	page, err := repo.ListVideoReports(ctx, "pending", 1, 20)
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].VideoTitle != video.Title || page.Items[0].ReporterUsername != reporter.Username {
		t.Fatalf("pending reports = %#v err=%v", page, err)
	}
	updated, err := repo.UpdateVideoReportStatus(ctx, report.ID, "resolved")
	if err != nil || updated.Status != "resolved" || updated.VideoAuthor != author.Username {
		t.Fatalf("updated report = %#v err=%v", updated, err)
	}
	pending, err := repo.ListVideoReports(ctx, "pending", 1, 20)
	if err != nil || pending.Total != 0 || len(pending.Items) != 0 {
		t.Fatalf("pending after review = %#v err=%v", pending, err)
	}
	if _, err := repo.UpdateVideoReportStatus(ctx, report.ID+1000, "resolved"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing update error = %v", err)
	}
}

func TestRepositoryPrivateVideoVisibility(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "private.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	repo := NewRepository(db)
	ctx := context.Background()
	author, _ := legacy.CreateUser(ctx, "private_report_author", "hash")
	reporter, _ := legacy.CreateUser(ctx, "private_reporter", "hash")
	video, err := legacy.CreateVideo(ctx, domain.NewVideo{UserID: author.ID, Title: "Private", Category: "知识", Visibility: "private", VideoPath: "videos/private.mp4", MimeType: "video/mp4", SizeBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.VideoAuthorID(ctx, video.ID, reporter.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("private lookup error = %v", err)
	}
	if got, err := repo.VideoAuthorID(ctx, video.ID, author.ID); err != nil || got != author.ID {
		t.Fatalf("owner lookup = %d err=%v", got, err)
	}
}
