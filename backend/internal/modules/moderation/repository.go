package moderation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gvideo/backend/internal/domain"
)

// Repository owns moderation persistence and its read model.
type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) VideoAuthorID(ctx context.Context, videoID, viewerID int64) (int64, error) {
	var authorID int64
	err := r.db.QueryRowContext(ctx, `
SELECT user_id FROM videos
WHERE id = ? AND (visibility <> 'private' OR user_id = ?)`, videoID, viewerID).Scan(&authorID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, domain.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("find reportable video: %w", err)
	}
	return authorID, nil
}

func (r *Repository) UpsertVideoReport(ctx context.Context, videoID, userID int64, reason, detail string) (domain.VideoReport, error) {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO video_reports(video_id, user_id, reason, detail, status)
VALUES (?, ?, ?, ?, 'pending')
ON CONFLICT(video_id, user_id) DO UPDATE SET reason = excluded.reason, detail = excluded.detail,
  status = 'pending', updated_at = CURRENT_TIMESTAMP`, videoID, userID, reason, detail)
	if err != nil {
		return domain.VideoReport{}, fmt.Errorf("upsert video report: %w", err)
	}
	var report domain.VideoReport
	err = r.db.QueryRowContext(ctx, `
SELECT id, video_id, user_id, reason, detail, status, created_at, updated_at
FROM video_reports WHERE video_id = ? AND user_id = ?`, videoID, userID).
		Scan(&report.ID, &report.VideoID, &report.UserID, &report.Reason, &report.Detail, &report.Status, &report.CreatedAt, &report.UpdatedAt)
	if err != nil {
		return domain.VideoReport{}, fmt.Errorf("read video report: %w", err)
	}
	return report, nil
}

func (r *Repository) ListVideoReports(ctx context.Context, status string, page, pageSize int) (domain.VideoReportPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	result := domain.VideoReportPage{Page: page, PageSize: pageSize, Items: []domain.VideoReport{}}
	where := ""
	args := make([]any, 0, 3)
	if status != "" {
		where = " WHERE r.status = ?"
		args = append(args, status)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM video_reports r`+where, args...).Scan(&result.Total); err != nil {
		return domain.VideoReportPage{}, fmt.Errorf("count video reports: %w", err)
	}
	args = append(args, pageSize, offset)
	rows, err := r.db.QueryContext(ctx, `
SELECT r.id, r.video_id, v.title, v.user_id, author.username, r.user_id, reporter.username,
       r.reason, r.detail, r.status, r.created_at, r.updated_at
FROM video_reports r
JOIN videos v ON v.id = r.video_id
JOIN users author ON author.id = v.user_id
JOIN users reporter ON reporter.id = r.user_id`+where+`
ORDER BY CASE r.status WHEN 'pending' THEN 0 WHEN 'reviewed' THEN 1 ELSE 2 END,
         r.updated_at DESC, r.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return domain.VideoReportPage{}, fmt.Errorf("list video reports: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.VideoReport
		if err := rows.Scan(&item.ID, &item.VideoID, &item.VideoTitle, &item.VideoAuthorID, &item.VideoAuthor,
			&item.UserID, &item.ReporterUsername, &item.Reason, &item.Detail, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return domain.VideoReportPage{}, fmt.Errorf("scan video report: %w", err)
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.VideoReportPage{}, fmt.Errorf("iterate video reports: %w", err)
	}
	result.HasNext = int64(offset+len(result.Items)) < result.Total
	return result, nil
}

func (r *Repository) UpdateVideoReportStatus(ctx context.Context, reportID int64, status string) (domain.VideoReport, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE video_reports SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, status, reportID)
	if err != nil {
		return domain.VideoReport{}, fmt.Errorf("update video report status: %w", err)
	}
	changed, _ := result.RowsAffected()
	if changed != 1 {
		return domain.VideoReport{}, domain.ErrNotFound
	}
	var report domain.VideoReport
	err = r.db.QueryRowContext(ctx, `
SELECT r.id, r.video_id, v.title, v.user_id, author.username, r.user_id, reporter.username,
       r.reason, r.detail, r.status, r.created_at, r.updated_at
FROM video_reports r
JOIN videos v ON v.id = r.video_id
JOIN users author ON author.id = v.user_id
JOIN users reporter ON reporter.id = r.user_id
WHERE r.id = ?`, reportID).Scan(&report.ID, &report.VideoID, &report.VideoTitle, &report.VideoAuthorID,
		&report.VideoAuthor, &report.UserID, &report.ReporterUsername, &report.Reason, &report.Detail,
		&report.Status, &report.CreatedAt, &report.UpdatedAt)
	if err != nil {
		return domain.VideoReport{}, fmt.Errorf("read updated video report: %w", err)
	}
	return report, nil
}
