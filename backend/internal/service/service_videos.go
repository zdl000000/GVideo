package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gvideo/backend/internal/domain"
)

func (s *Service) ListVideos(ctx context.Context, filter domain.VideoFilter, viewerID int64) (domain.VideoPage, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Category = strings.TrimSpace(filter.Category)
	if filter.Sort != "popular" {
		filter.Sort = "latest"
	}
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 24
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	items, err := s.repo.ListVideos(ctx, filter, viewerID)
	if err != nil {
		return domain.VideoPage{}, err
	}
	total, err := s.repo.CountVideos(ctx, filter)
	if err != nil {
		return domain.VideoPage{}, err
	}
	if items == nil {
		items = []domain.Video{}
	}
	for index := range items {
		items[index] = publicVideo(items[index])
	}
	return domain.VideoPage{
		Items: items, Page: filter.Offset/filter.Limit + 1, PageSize: filter.Limit,
		Total: total, HasNext: int64(filter.Offset+len(items)) < total,
	}, nil
}

func (s *Service) Video(ctx context.Context, id, viewerID int64, countView bool) (domain.Video, error) {
	video, err := s.repo.VideoByID(ctx, id, viewerID)
	if err != nil {
		return domain.Video{}, err
	}
	if countView {
		if err := s.repo.IncrementViews(ctx, id); err != nil {
			s.logger.Warn("increment video views", "video_id", id, "error", err)
		} else {
			video.ViewsCount++
		}
	}
	return publicVideo(video), nil
}

// enforceStorageQuota rejects uploads that would push a user's stored source
// bytes past the configured quota; zero disables the check. Accounting is best
// effort: concurrent uploads can momentarily overshoot.
func (s *Service) enforceStorageQuota(ctx context.Context, userID, incomingBytes int64) error {
	if s.cfg.UserStorageQuotaBytes <= 0 {
		return nil
	}
	used, err := s.repo.UserStorageUsed(ctx, userID)
	if err != nil {
		return err
	}
	if used+incomingBytes > s.cfg.UserStorageQuotaBytes {
		return domain.ErrQuotaExceeded
	}
	return nil
}

func (s *Service) UploadVideo(ctx context.Context, input UploadInput) (domain.Video, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.Visibility = normalizeVisibility(input.Visibility)
	if len([]rune(input.Title)) < 2 || len([]rune(input.Title)) > 80 || len([]rune(input.Description)) > 2000 || !validCategory(input.Category) || input.Visibility == "" || input.Video == nil {
		return domain.Video{}, domain.ErrInvalidInput
	}
	if input.Video.Size <= 0 || input.Video.Size > s.cfg.MaxUploadBytes {
		return domain.Video{}, domain.ErrInvalidInput
	}
	if err := s.enforceStorageQuota(ctx, input.UserID, input.Video.Size); err != nil {
		return domain.Video{}, err
	}
	subtitle, err := inspectAndConvertSubtitle(input.Subtitle, input.SubtitleLanguage, input.SubtitleLabel)
	if err != nil {
		return domain.Video{}, err
	}

	id, err := randomToken(12)
	if err != nil {
		return domain.Video{}, err
	}
	if subtitle.data != nil {
		subtitle.path = filepath.Join("subtitles", id, subtitle.language+".vtt")
	}
	videoDir := filepath.Join(s.cfg.MediaDir, "videos")
	coverDir := filepath.Join(s.cfg.MediaDir, "covers")
	if err := os.MkdirAll(videoDir, 0o755); err != nil {
		return domain.Video{}, fmt.Errorf("create video directory: %w", err)
	}
	if err := os.MkdirAll(coverDir, 0o755); err != nil {
		return domain.Video{}, fmt.Errorf("create cover directory: %w", err)
	}

	mimeType, ext, err := inspectVideo(input.Video)
	if err != nil {
		return domain.Video{}, err
	}
	videoName := id + ext
	videoPath := filepath.Join(videoDir, videoName)
	if err := saveMultipart(input.Video, videoPath); err != nil {
		return domain.Video{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(videoPath)
			if subtitle.path != "" {
				_ = os.Remove(filepath.Join(s.cfg.MediaDir, subtitle.path))
			}
		}
	}()
	if subtitle.path != "" {
		if err := saveBytes(filepath.Join(s.cfg.MediaDir, subtitle.path), subtitle.data); err != nil {
			return domain.Video{}, err
		}
	}

	coverName := ""
	coverBytes := int64(0)
	if input.Cover != nil && input.Cover.Size > 0 {
		coverExt, err := inspectImage(input.Cover)
		if err != nil {
			return domain.Video{}, err
		}
		coverName = id + coverExt
		if err := saveMultipart(input.Cover, filepath.Join(coverDir, coverName)); err != nil {
			return domain.Video{}, err
		}
		coverBytes = input.Cover.Size
	} else if generateCover(ctx, s.cfg.FFmpegPath, videoPath, filepath.Join(coverDir, id+".jpg")) == nil {
		coverName = id + ".jpg"
		if info, err := os.Stat(filepath.Join(coverDir, coverName)); err == nil {
			coverBytes = info.Size()
		}
	}

	newVideo := domain.NewVideo{
		UserID: input.UserID, Title: input.Title, Description: input.Description, Category: input.Category,
		Visibility: input.Visibility,
		VideoPath:  filepath.Join("videos", videoName), CoverPath: relativeCover(coverName), MimeType: mimeType,
		SizeBytes: input.Video.Size, CoverBytes: coverBytes,
	}
	var video domain.Video
	if subtitle.path == "" {
		video, err = s.repo.CreateVideo(ctx, newVideo)
	} else {
		video, err = s.repo.CreateVideoWithSubtitle(ctx, newVideo, domain.NewSubtitle{
			Language: subtitle.language, Label: subtitle.label, Path: subtitle.path, IsDefault: true,
		})
	}
	if err != nil {
		if coverName != "" {
			_ = os.Remove(filepath.Join(coverDir, coverName))
		}
		return domain.Video{}, err
	}
	cleanup = false
	return video, nil
}

func (s *Service) UpdateVideo(ctx context.Context, input UpdateVideoInput) (domain.Video, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.Visibility = normalizeVisibility(input.Visibility)
	if len([]rune(input.Title)) < 2 || len([]rune(input.Title)) > 80 || len([]rune(input.Description)) > 2000 || !validCategory(input.Category) || input.Visibility == "" {
		return domain.Video{}, domain.ErrInvalidInput
	}
	current, err := s.repo.VideoByID(ctx, input.VideoID, input.UserID)
	if err != nil {
		return domain.Video{}, err
	}
	if current.UserID != input.UserID {
		return domain.Video{}, domain.ErrForbidden
	}

	var newCoverPath *string
	var savedCoverPath string
	coverBytes := int64(0)
	if input.Cover != nil && input.Cover.Size > 0 {
		ext, err := inspectImage(input.Cover)
		if err != nil {
			return domain.Video{}, err
		}
		token, err := randomToken(12)
		if err != nil {
			return domain.Video{}, err
		}
		relative := filepath.Join("covers", token+ext)
		savedCoverPath = filepath.Join(s.cfg.MediaDir, relative)
		if err := os.MkdirAll(filepath.Dir(savedCoverPath), 0o755); err != nil {
			return domain.Video{}, fmt.Errorf("create cover directory: %w", err)
		}
		if err := saveMultipart(input.Cover, savedCoverPath); err != nil {
			return domain.Video{}, err
		}
		newCoverPath = &relative
		coverBytes = input.Cover.Size
	}

	updated, err := s.repo.UpdateVideo(ctx, input.VideoID, input.UserID, domain.UpdateVideo{
		Title: input.Title, Description: input.Description, Category: input.Category, Visibility: input.Visibility, CoverPath: newCoverPath,
		CoverBytes: coverBytes,
	})
	if err != nil {
		if savedCoverPath != "" {
			_ = os.Remove(savedCoverPath)
		}
		return domain.Video{}, err
	}
	if newCoverPath != nil && current.CoverURL != "" {
		s.removeMediaPath(strings.TrimPrefix(current.CoverURL, "/media/"), false)
	}
	return publicVideo(updated), nil
}

func (s *Service) DeleteVideo(ctx context.Context, userID, videoID int64) error {
	video, err := s.repo.VideoByID(ctx, videoID, userID)
	if err != nil {
		return err
	}
	if video.UserID != userID {
		return domain.ErrForbidden
	}
	assets, err := s.repo.DeleteVideo(ctx, videoID, userID)
	if err != nil {
		return err
	}
	s.removeMediaPath(assets.VideoPath, false)
	s.removeMediaPath(assets.CoverPath, false)
	if assets.HLSMasterPath != "" {
		s.removeMediaPath(filepath.Dir(assets.HLSMasterPath), true)
	} else {
		s.removeMediaPath(filepath.Join("hls", strconv.FormatInt(videoID, 10)), true)
	}
	for _, subtitlePath := range assets.SubtitlePaths {
		s.removeMediaPath(subtitlePath, false)
		s.removeEmptyMediaParent(filepath.Dir(subtitlePath))
	}
	return nil
}

func (s *Service) RetryTranscoding(ctx context.Context, userID, videoID int64) error {
	video, err := s.repo.VideoByID(ctx, videoID, userID)
	if err != nil {
		return err
	}
	if video.UserID != userID {
		return domain.ErrForbidden
	}
	if err := s.repo.RetryTranscoding(ctx, videoID, userID); err != nil {
		return err
	}
	s.removeMediaPath(filepath.Join("hls", strconv.FormatInt(videoID, 10)), true)
	return nil
}
