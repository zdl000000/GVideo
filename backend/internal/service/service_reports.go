package service

import (
	"context"
	"strings"

	"gvideo/backend/internal/domain"
)

func (s *Service) ReportVideo(ctx context.Context, userID, videoID int64, reason, detail string) (domain.VideoReport, error) {
	reason = strings.TrimSpace(reason)
	detail = strings.TrimSpace(detail)
	if userID <= 0 || videoID <= 0 || !validReportReason(reason) || len([]rune(detail)) > 1000 {
		return domain.VideoReport{}, domain.ErrInvalidInput
	}
	video, err := s.repo.VideoByID(ctx, videoID, userID)
	if err != nil {
		return domain.VideoReport{}, err
	}
	if video.UserID == userID {
		return domain.VideoReport{}, domain.ErrForbidden
	}
	return s.repo.UpsertVideoReport(ctx, videoID, userID, reason, detail)
}

func validReportReason(reason string) bool {
	switch reason {
	case "spam", "inappropriate", "copyright", "other":
		return true
	default:
		return false
	}
}

func (s *Service) AdminVideoReports(ctx context.Context, username, status string, page, pageSize int) (domain.VideoReportPage, error) {
	if !s.IsAdmin(username) {
		return domain.VideoReportPage{}, domain.ErrForbidden
	}
	status = strings.TrimSpace(status)
	if status != "" && !validReportStatus(status) {
		return domain.VideoReportPage{}, domain.ErrInvalidInput
	}
	return s.repo.ListVideoReports(ctx, status, page, pageSize)
}

func (s *Service) ReviewVideoReport(ctx context.Context, username string, reportID int64, status string) (domain.VideoReport, error) {
	if !s.IsAdmin(username) {
		return domain.VideoReport{}, domain.ErrForbidden
	}
	status = strings.TrimSpace(status)
	if reportID <= 0 || !validReportStatus(status) {
		return domain.VideoReport{}, domain.ErrInvalidInput
	}
	return s.repo.UpdateVideoReportStatus(ctx, reportID, status)
}
