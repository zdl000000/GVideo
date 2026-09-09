package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"

	"gvideo/backend/internal/domain"
)

func (s *Service) AddSubtitle(ctx context.Context, userID, videoID int64, header *multipart.FileHeader, language, label string) (domain.SubtitleTrack, error) {
	video, err := s.repo.VideoByID(ctx, videoID, userID)
	if err != nil {
		return domain.SubtitleTrack{}, err
	}
	if video.UserID != userID {
		return domain.SubtitleTrack{}, domain.ErrForbidden
	}
	subtitle, err := inspectAndConvertSubtitle(header, language, label)
	if err != nil {
		return domain.SubtitleTrack{}, err
	}
	token, err := randomToken(12)
	if err != nil {
		return domain.SubtitleTrack{}, err
	}
	subtitle.path = filepath.Join("subtitles", fmt.Sprintf("%d-%s", videoID, token), subtitle.language+".vtt")
	path := filepath.Join(s.cfg.MediaDir, subtitle.path)
	if err := saveBytes(path, subtitle.data); err != nil {
		return domain.SubtitleTrack{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(path)
		}
	}()
	tracks, err := s.repo.ListSubtitles(ctx, videoID)
	if err != nil {
		return domain.SubtitleTrack{}, err
	}
	track, err := s.repo.CreateSubtitle(ctx, videoID, subtitle.language, subtitle.label, subtitle.path, len(tracks) == 0)
	if err != nil {
		return domain.SubtitleTrack{}, err
	}
	cleanup = false
	return track, nil
}

func (s *Service) SetDefaultSubtitle(ctx context.Context, userID, videoID, subtitleID int64) ([]domain.SubtitleTrack, error) {
	video, err := s.repo.VideoByID(ctx, videoID, userID)
	if err != nil {
		return nil, err
	}
	if video.UserID != userID {
		return nil, domain.ErrForbidden
	}
	return s.repo.SetDefaultSubtitle(ctx, videoID, subtitleID)
}

func (s *Service) DeleteSubtitle(ctx context.Context, userID, videoID, subtitleID int64) ([]domain.SubtitleTrack, error) {
	video, err := s.repo.VideoByID(ctx, videoID, userID)
	if err != nil {
		return nil, err
	}
	if video.UserID != userID {
		return nil, domain.ErrForbidden
	}
	subtitlePath, tracks, err := s.repo.DeleteSubtitle(ctx, videoID, subtitleID)
	if err != nil {
		return nil, err
	}
	s.removeMediaPath(subtitlePath, false)
	s.removeEmptyMediaParent(filepath.Dir(subtitlePath))
	return tracks, nil
}
