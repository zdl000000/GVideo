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

func TestServiceReportRulesAndUpsert(t *testing.T) {
	db, err := platform.OpenDatabase(filepath.Join(t.TempDir(), "reports.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	legacy := repository.New(db)
	repo := NewRepository(db)
	svc := NewService(repo)
	ctx := context.Background()
	author, err := legacy.CreateUser(ctx, "report_author", "hash")
	if err != nil {
		t.Fatal(err)
	}
	reporter, err := legacy.CreateUser(ctx, "report_viewer", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := legacy.CreateVideo(ctx, domain.NewVideo{UserID: author.ID, Title: "Reportable video", Category: "科技", Visibility: "public", VideoPath: "videos/report.mp4", MimeType: "video/mp4", SizeBytes: 10})
	if err != nil {
		t.Fatal(err)
	}
	report, err := svc.ReportVideo(ctx, reporter.ID, video.ID, " spam ", " first detail ")
	if err != nil || report.Status != "pending" || report.Detail != "first detail" {
		t.Fatalf("first report = %#v err=%v", report, err)
	}
	updated, err := svc.ReportVideo(ctx, reporter.ID, video.ID, "copyright", "updated detail")
	if err != nil || updated.ID != report.ID || updated.Reason != "copyright" || updated.Detail != "updated detail" {
		t.Fatalf("updated report = %#v err=%v", updated, err)
	}
	if _, err := svc.ReportVideo(ctx, author.ID, video.ID, "spam", "self report"); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("self report error = %v", err)
	}
	if _, err := svc.ReportVideo(ctx, reporter.ID, video.ID, "invalid", "bad reason"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("invalid reason error = %v", err)
	}
	if _, err := svc.ReportVideo(ctx, reporter.ID, video.ID, "spam", string(make([]rune, 1001))); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("long detail error = %v", err)
	}
}

func TestServiceAdminAuthorizationAndValidation(t *testing.T) {
	repo := &serviceRepositoryStub{}
	svc := NewService(repo)
	if _, err := svc.AdminVideoReports(context.Background(), false, "", 1, 20); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("list error = %v", err)
	}
	if _, err := svc.AdminVideoReports(context.Background(), true, "invalid", 1, 20); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("status error = %v", err)
	}
	if _, err := svc.ReviewVideoReport(context.Background(), true, 0, "resolved"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("id error = %v", err)
	}
	if _, err := svc.ReviewVideoReport(context.Background(), true, 1, " resolved "); err != nil {
		t.Fatalf("review error = %v", err)
	}
}

type serviceRepositoryStub struct{}

func (*serviceRepositoryStub) VideoAuthorID(context.Context, int64, int64) (int64, error) {
	return 2, nil
}
func (*serviceRepositoryStub) UpsertVideoReport(context.Context, int64, int64, string, string) (domain.VideoReport, error) {
	return domain.VideoReport{}, nil
}
func (*serviceRepositoryStub) ListVideoReports(context.Context, string, int, int) (domain.VideoReportPage, error) {
	return domain.VideoReportPage{}, nil
}
func (*serviceRepositoryStub) UpdateVideoReportStatus(context.Context, int64, string) (domain.VideoReport, error) {
	return domain.VideoReport{}, nil
}
