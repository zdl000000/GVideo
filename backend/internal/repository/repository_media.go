package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strings"

	"gvideo/backend/internal/domain"
)

func (r *Repository) MediaAccessByPath(ctx context.Context, storedPath string) (domain.MediaAccess, error) {
	storedPath = strings.TrimPrefix(strings.ReplaceAll(storedPath, `\`, "/"), "/")
	var access domain.MediaAccess
	err := r.db.QueryRowContext(ctx, `
SELECT v.id, v.user_id, v.visibility
FROM videos v
WHERE replace(v.video_path, '\', '/') = ?
   OR replace(v.cover_path, '\', '/') = ?
   OR EXISTS (
     SELECT 1 FROM video_subtitles s
     WHERE s.video_id = v.id AND replace(s.subtitle_path, '\', '/') = ?
   )
LIMIT 1`, storedPath, storedPath, storedPath).Scan(&access.VideoID, &access.UserID, &access.Visibility)
	if err == nil {
		return access, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return domain.MediaAccess{}, fmt.Errorf("find media owner: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT id, user_id, visibility, replace(hls_master_path, '\', '/')
FROM videos WHERE hls_master_path <> ''`)
	if err != nil {
		return domain.MediaAccess{}, fmt.Errorf("list HLS owners: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var masterPath string
		if err := rows.Scan(&access.VideoID, &access.UserID, &access.Visibility, &masterPath); err != nil {
			return domain.MediaAccess{}, fmt.Errorf("scan HLS owner: %w", err)
		}
		masterPath = strings.TrimPrefix(strings.ReplaceAll(masterPath, `\`, "/"), "/")
		dir := path.Dir(masterPath)
		if storedPath == masterPath || (dir != "." && strings.HasPrefix(storedPath, dir+"/")) {
			return access, nil
		}
	}
	if err := rows.Err(); err != nil {
		return domain.MediaAccess{}, fmt.Errorf("iterate HLS owners: %w", err)
	}
	return domain.MediaAccess{}, domain.ErrNotFound
}
