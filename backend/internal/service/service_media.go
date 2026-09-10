package service

import (
	"context"
	"os"
	"strings"

	"gvideo/backend/internal/domain"
)

func (s *Service) AuthorizeMedia(ctx context.Context, storedPath string, viewerID int64) error {
	normalized := strings.TrimPrefix(strings.ReplaceAll(storedPath, `\`, "/"), "/")
	if normalized == "avatars" || strings.HasPrefix(normalized, "avatars/") {
		return nil
	}
	access, err := s.repo.MediaAccessByPath(ctx, normalized)
	if err != nil {
		return err
	}
	if access.Visibility == "private" && access.UserID != viewerID {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Service) removeMediaPath(storedPath string, recursive bool) {
	if storedPath == "" {
		return
	}
	resolved, err := resolveMediaPath(s.cfg.MediaDir, storedPath)
	if err != nil {
		s.logger.Warn("media cleanup skipped", "event", "media_cleanup_failed", "error_class", "invalid_media_path")
		return
	}
	if recursive {
		err = os.RemoveAll(resolved)
	} else {
		err = os.Remove(resolved)
	}
	if err != nil && !os.IsNotExist(err) {
		s.logger.Warn("media cleanup failed", "event", "media_cleanup_failed", "error_class", "filesystem_failed")
	}
}

func (s *Service) removeEmptyMediaParent(storedDir string) {
	resolved, err := resolveMediaPath(s.cfg.MediaDir, storedDir)
	if err == nil {
		_ = os.Remove(resolved)
	}
}
