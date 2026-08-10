package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/repository"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_\p{Han}]{3,24}$`)
var subtitleLanguagePattern = regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z0-9]{2,8})?$`)
var srtTimestampPattern = regexp.MustCompile(`(\d{2}:\d{2}:\d{2}),(\d{3})`)

var Categories = []string{"动画", "游戏", "音乐", "知识", "科技", "生活", "影视", "运动"}

type Service struct {
	repo   *repository.Repository
	cfg    config.Config
	logger *slog.Logger
}

type CreatedSession struct {
	Token     string
	CSRFToken string
	User      domain.User
}

type UploadInput struct {
	UserID           int64
	Title            string
	Description      string
	Category         string
	Video            *multipart.FileHeader
	Cover            *multipart.FileHeader
	Subtitle         *multipart.FileHeader
	SubtitleLanguage string
	SubtitleLabel    string
}

type UpdateVideoInput struct {
	UserID      int64
	VideoID     int64
	Title       string
	Description string
	Category    string
	Cover       *multipart.FileHeader
}

func New(repo *repository.Repository, cfg config.Config, logger *slog.Logger) *Service {
	return &Service{repo: repo, cfg: cfg, logger: logger}
}

func (s *Service) Register(ctx context.Context, username, password string) (CreatedSession, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) || len(password) < 8 || len(password) > 72 {
		return CreatedSession{}, domain.ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return CreatedSession{}, fmt.Errorf("hash password: %w", err)
	}
	user, err := s.repo.CreateUser(ctx, username, string(hash))
	if err != nil {
		return CreatedSession{}, err
	}
	return s.newSession(ctx, user)
}

func (s *Service) Login(ctx context.Context, username, password string) (CreatedSession, error) {
	user, hash, err := s.repo.UserAuthByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return CreatedSession{}, domain.ErrUnauthorized
		}
		return CreatedSession{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return CreatedSession{}, domain.ErrUnauthorized
	}
	return s.newSession(ctx, user)
}

func (s *Service) newSession(ctx context.Context, user domain.User) (CreatedSession, error) {
	token, err := randomToken(32)
	if err != nil {
		return CreatedSession{}, err
	}
	csrf, err := randomToken(24)
	if err != nil {
		return CreatedSession{}, err
	}
	if err := s.repo.CreateSession(ctx, hashToken(token), user.ID, csrf, time.Now().Add(s.cfg.SessionTTL)); err != nil {
		return CreatedSession{}, err
	}
	return CreatedSession{Token: token, CSRFToken: csrf, User: user}, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (domain.Session, error) {
	if token == "" {
		return domain.Session{}, domain.ErrInvalidSession
	}
	return s.repo.SessionByHash(ctx, hashToken(token))
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.repo.DeleteSession(ctx, hashToken(token))
}

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

func (s *Service) CreatorProfile(ctx context.Context, userID, viewerID int64) (domain.CreatorProfile, error) {
	if userID <= 0 {
		return domain.CreatorProfile{}, domain.ErrInvalidInput
	}
	return s.repo.CreatorProfile(ctx, userID, viewerID)
}

func (s *Service) ToggleFollow(ctx context.Context, followerID, followedID int64) (bool, error) {
	if followerID <= 0 || followedID <= 0 {
		return false, domain.ErrInvalidInput
	}
	if followerID == followedID {
		return false, domain.ErrForbidden
	}
	if _, err := s.repo.UserByID(ctx, followedID); err != nil {
		return false, err
	}
	return s.repo.ToggleFollow(ctx, followerID, followedID)
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

func (s *Service) UploadVideo(ctx context.Context, input UploadInput) (domain.Video, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if len([]rune(input.Title)) < 2 || len([]rune(input.Title)) > 80 || len([]rune(input.Description)) > 2000 || !validCategory(input.Category) || input.Video == nil {
		return domain.Video{}, domain.ErrInvalidInput
	}
	if input.Video.Size <= 0 || input.Video.Size > s.cfg.MaxUploadBytes {
		return domain.Video{}, domain.ErrInvalidInput
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
	if input.Cover != nil && input.Cover.Size > 0 {
		coverExt, err := inspectImage(input.Cover)
		if err != nil {
			return domain.Video{}, err
		}
		coverName = id + coverExt
		if err := saveMultipart(input.Cover, filepath.Join(coverDir, coverName)); err != nil {
			return domain.Video{}, err
		}
	} else if generateCover(ctx, s.cfg.FFmpegPath, videoPath, filepath.Join(coverDir, id+".jpg")) == nil {
		coverName = id + ".jpg"
	}

	newVideo := domain.NewVideo{
		UserID: input.UserID, Title: input.Title, Description: input.Description, Category: input.Category,
		VideoPath: filepath.Join("videos", videoName), CoverPath: relativeCover(coverName), MimeType: mimeType,
		SizeBytes: input.Video.Size,
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

func (s *Service) UpdateVideo(ctx context.Context, input UpdateVideoInput) (domain.Video, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if len([]rune(input.Title)) < 2 || len([]rune(input.Title)) > 80 || len([]rune(input.Description)) > 2000 || !validCategory(input.Category) {
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
	}

	updated, err := s.repo.UpdateVideo(ctx, input.VideoID, input.UserID, domain.UpdateVideo{
		Title: input.Title, Description: input.Description, Category: input.Category, CoverPath: newCoverPath,
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

func (s *Service) ToggleLike(ctx context.Context, userID, videoID int64) (bool, error) {
	if _, err := s.repo.VideoByID(ctx, videoID, userID); err != nil {
		return false, err
	}
	return s.repo.ToggleLike(ctx, userID, videoID)
}

func (s *Service) ToggleFavorite(ctx context.Context, userID, videoID int64) (bool, error) {
	if _, err := s.repo.VideoByID(ctx, videoID, userID); err != nil {
		return false, err
	}
	return s.repo.ToggleFavorite(ctx, userID, videoID)
}

func (s *Service) Comments(ctx context.Context, videoID int64) ([]domain.Comment, error) {
	return s.repo.ListComments(ctx, videoID)
}

func (s *Service) CreateComment(ctx context.Context, userID, videoID int64, content string) (domain.Comment, error) {
	content = strings.TrimSpace(content)
	if len([]rune(content)) < 1 || len([]rune(content)) > 500 {
		return domain.Comment{}, domain.ErrInvalidInput
	}
	if _, err := s.repo.VideoByID(ctx, videoID, userID); err != nil {
		return domain.Comment{}, err
	}
	return s.repo.CreateComment(ctx, userID, videoID, content)
}

func randomToken(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func validCategory(value string) bool {
	for _, category := range Categories {
		if category == value {
			return true
		}
	}
	return false
}

func inspectVideo(header *multipart.FileHeader) (string, string, error) {
	file, err := header.Open()
	if err != nil {
		return "", "", fmt.Errorf("open uploaded video: %w", err)
	}
	defer file.Close()
	buffer := make([]byte, 512)
	n, err := io.ReadFull(file, buffer)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", "", domain.ErrInvalidInput
	}
	mimeType := http.DetectContentType(buffer[:n])
	switch mimeType {
	case "video/mp4":
		return mimeType, ".mp4", nil
	case "video/webm":
		return mimeType, ".webm", nil
	case "video/ogg", "application/ogg":
		return "video/ogg", ".ogv", nil
	default:
		return "", "", domain.ErrInvalidInput
	}
}

func inspectImage(header *multipart.FileHeader) (string, error) {
	if header.Size > 10<<20 {
		return "", domain.ErrInvalidInput
	}
	file, err := header.Open()
	if err != nil {
		return "", fmt.Errorf("open cover: %w", err)
	}
	defer file.Close()
	buffer := make([]byte, 512)
	n, _ := io.ReadFull(file, buffer)
	switch http.DetectContentType(buffer[:n]) {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/webp":
		return ".webp", nil
	default:
		return "", domain.ErrInvalidInput
	}
}

func saveMultipart(header *multipart.FileHeader, destination string) error {
	source, err := header.Open()
	if err != nil {
		return fmt.Errorf("open multipart file: %w", err)
	}
	defer source.Close()
	temp := destination + ".upload"
	target, err := os.OpenFile(temp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create media file: %w", err)
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("save media file: %w", errors.Join(copyErr, closeErr))
	}
	if err := os.Rename(temp, destination); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("publish media file: %w", err)
	}
	return nil
}

type subtitleUpload struct {
	data     []byte
	path     string
	language string
	label    string
}

func inspectAndConvertSubtitle(header *multipart.FileHeader, language, label string) (subtitleUpload, error) {
	if header == nil {
		return subtitleUpload{}, nil
	}
	if header.Size <= 0 || header.Size > 2<<20 {
		return subtitleUpload{}, domain.ErrInvalidInput
	}
	language = strings.TrimSpace(language)
	if language == "" {
		language = "zh-CN"
	}
	if !subtitleLanguagePattern.MatchString(language) {
		return subtitleUpload{}, domain.ErrInvalidInput
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = language
	}
	if len([]rune(label)) > 40 {
		return subtitleUpload{}, domain.ErrInvalidInput
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".vtt" && ext != ".srt" {
		return subtitleUpload{}, domain.ErrInvalidInput
	}
	file, err := header.Open()
	if err != nil {
		return subtitleUpload{}, fmt.Errorf("open subtitle: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (2<<20)+1))
	if err != nil || len(data) > 2<<20 || bytesContainsNUL(data) || !utf8.Valid(data) {
		return subtitleUpload{}, domain.ErrInvalidInput
	}
	data = bytesTrimUTF8BOM(data)
	text := strings.ReplaceAll(strings.ReplaceAll(string(data), "\r\n", "\n"), "\r", "\n")
	if ext == ".srt" {
		text = "WEBVTT\n\n" + srtTimestampPattern.ReplaceAllString(text, "$1.$2")
	} else if !strings.HasPrefix(strings.TrimSpace(text), "WEBVTT") {
		return subtitleUpload{}, domain.ErrInvalidInput
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return subtitleUpload{data: []byte(text), language: language, label: label}, nil
}

func saveBytes(destination string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create subtitle directory: %w", err)
	}
	temp := destination + ".upload"
	target, err := os.OpenFile(temp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create subtitle file: %w", err)
	}
	_, writeErr := target.Write(data)
	closeErr := target.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("save subtitle file: %w", errors.Join(writeErr, closeErr))
	}
	if err := os.Rename(temp, destination); err != nil {
		_ = os.Remove(temp)
		return fmt.Errorf("publish subtitle file: %w", err)
	}
	return nil
}

func bytesContainsNUL(data []byte) bool { return strings.IndexByte(string(data), 0) >= 0 }
func bytesTrimUTF8BOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xef && data[1] == 0xbb && data[2] == 0xbf {
		return data[3:]
	}
	return data
}

func generateCover(ctx context.Context, executable, videoPath, coverPath string) error {
	coverCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return exec.CommandContext(coverCtx, executable, "-y", "-ss", "00:00:01", "-i", videoPath, "-frames:v", "1", "-vf", "scale=1280:-2", coverPath).Run()
}

func relativeCover(name string) string {
	if name == "" {
		return ""
	}
	return filepath.Join("covers", name)
}

func (s *Service) removeMediaPath(storedPath string, recursive bool) {
	if storedPath == "" {
		return
	}
	resolved, err := resolveMediaPath(s.cfg.MediaDir, storedPath)
	if err != nil {
		s.logger.Warn("skip unsafe media cleanup", "path", storedPath, "error", err)
		return
	}
	if recursive {
		err = os.RemoveAll(resolved)
	} else {
		err = os.Remove(resolved)
	}
	if err != nil && !os.IsNotExist(err) {
		s.logger.Warn("cleanup media asset", "path", storedPath, "error", err)
	}
}

func (s *Service) removeEmptyMediaParent(storedDir string) {
	resolved, err := resolveMediaPath(s.cfg.MediaDir, storedDir)
	if err == nil {
		_ = os.Remove(resolved)
	}
}

func resolveMediaPath(root, storedPath string) (string, error) {
	normalized := filepath.FromSlash(strings.ReplaceAll(storedPath, `\`, "/"))
	if filepath.IsAbs(normalized) {
		return "", fmt.Errorf("media path must be relative")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Abs(filepath.Join(absoluteRoot, normalized))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(absoluteRoot, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("media path escapes media directory")
	}
	return resolved, nil
}

func publicVideo(video domain.Video) domain.Video {
	if video.ProcessingStatus == "failed" {
		video.ProcessingMessage = "媒体处理失败，请检查视频编码后重新转码"
	}
	return video
}
