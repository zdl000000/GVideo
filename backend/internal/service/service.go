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
	"strings"
	"time"
	"unicode/utf8"

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
	Visibility       string
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
	Visibility  string
	Cover       *multipart.FileHeader
}

type UpdateProfileInput struct {
	UserID   int64
	Username string
	Bio      string
	Avatar   *multipart.FileHeader
}

func New(repo *repository.Repository, cfg config.Config, logger *slog.Logger) *Service {
	return &Service{repo: repo, cfg: cfg, logger: logger}
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

func normalizeVisibility(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "public"
	}
	switch value {
	case "public", "unlisted", "private":
		return value
	default:
		return ""
	}
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
		message := strings.ToLower(video.ProcessingError)
		switch {
		case strings.Contains(message, "no such file"),
			strings.Contains(message, "cannot find"),
			strings.Contains(message, "path escapes"),
			strings.Contains(message, "path must be relative"):
			video.ProcessingMessage = "源视频文件不可用，请重新上传视频"
		case strings.Contains(message, "timeout"),
			strings.Contains(message, "deadline"),
			strings.Contains(message, "canceled"):
			video.ProcessingMessage = "视频处理超时，请降低分辨率或缩短时长后重试"
		case strings.Contains(message, "invalid data"),
			strings.Contains(message, "unsupported"),
			strings.Contains(message, "codec"),
			strings.Contains(message, "probe"):
			video.ProcessingMessage = "无法读取视频，请确认文件未损坏且编码格式受支持"
		case strings.Contains(message, "no space"),
			strings.Contains(message, "storage"),
			strings.Contains(message, "disk"):
			video.ProcessingMessage = "服务器存储暂时不足，请稍后重试"
		default:
			video.ProcessingMessage = "视频处理失败，请检查文件格式后重试"
		}
	}
	return video
}
