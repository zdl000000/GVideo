package moderation

import (
	"context"
	"strings"

	"gvideo/backend/internal/domain"
)

type reportRepository interface {
	VideoAuthorID(context.Context, int64, int64) (int64, error)
	UpsertVideoReport(context.Context, int64, int64, string, string) (domain.VideoReport, error)
	ListVideoReports(context.Context, string, int, int) (domain.VideoReportPage, error)
	UpdateVideoReportStatus(context.Context, int64, string) (domain.VideoReport, error)
}

// Service owns report validation and moderation authorization.
type Service struct {
	repo reportRepository
}

func NewService(repo reportRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ReportVideo(ctx context.Context, userID, videoID int64, reason, detail string) (domain.VideoReport, error) {
	reason = strings.TrimSpace(reason)
	detail = strings.TrimSpace(detail)
	if userID <= 0 || videoID <= 0 || !validReportReason(reason) || len([]rune(detail)) > 1000 {
		return domain.VideoReport{}, domain.ErrInvalidInput
	}
	authorID, err := s.repo.VideoAuthorID(ctx, videoID, userID)
	if err != nil {
		return domain.VideoReport{}, err
	}
	if authorID == userID {
		return domain.VideoReport{}, domain.ErrForbidden
	}
	return s.repo.UpsertVideoReport(ctx, videoID, userID, reason, detail)
}

func (s *Service) AdminVideoReports(ctx context.Context, isAdmin bool, status string, page, pageSize int) (domain.VideoReportPage, error) {
	if !isAdmin {
		return domain.VideoReportPage{}, domain.ErrForbidden
	}
	status = strings.TrimSpace(status)
	if status != "" && !validReportStatus(status) {
		return domain.VideoReportPage{}, domain.ErrInvalidInput
	}
	return s.repo.ListVideoReports(ctx, status, page, pageSize)
}

func (s *Service) ReviewVideoReport(ctx context.Context, isAdmin bool, reportID int64, status string) (domain.VideoReport, error) {
	if !isAdmin {
		return domain.VideoReport{}, domain.ErrForbidden
	}
	status = strings.TrimSpace(status)
	if reportID <= 0 || !validReportStatus(status) {
		return domain.VideoReport{}, domain.ErrInvalidInput
	}
	return s.repo.UpdateVideoReportStatus(ctx, reportID, status)
}

func validReportReason(reason string) bool {
	switch reason {
	case "spam", "inappropriate", "copyright", "other":
		return true
	default:
		return false
	}
}

func validReportStatus(status string) bool {
	switch status {
	case "pending", "reviewed", "resolved", "dismissed":
		return true
	default:
		return false
	}
}
