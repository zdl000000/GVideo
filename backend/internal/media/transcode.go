package media

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gvideo/backend/internal/domain"
)

type Transcoder interface {
	Enabled() bool
	Transcode(context.Context, string, int64, domain.MediaMetadata) (string, error)
}

type commandRunner interface {
	Run(context.Context, string, ...string) error
}

type execCommandRunner struct{}

func (execCommandRunner) Run(ctx context.Context, executable string, args ...string) error {
	output, err := exec.CommandContext(ctx, executable, args...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if len(message) > 1200 {
			message = message[len(message)-1200:]
		}
		return fmt.Errorf("ffmpeg: %w: %s", err, message)
	}
	return nil
}

type FFmpegTranscoder struct {
	enabled        bool
	executable     string
	mediaDir       string
	timeout        time.Duration
	segmentSeconds int
	runner         commandRunner
}

type hlsRendition struct {
	Name         string
	Width        int
	Height       int
	VideoBitrate int
	AudioBitrate int
}

func NewFFmpegTranscoder(enabled bool, executable, mediaDir string, timeout time.Duration, segmentSeconds int) *FFmpegTranscoder {
	return &FFmpegTranscoder{
		enabled: enabled, executable: executable, mediaDir: mediaDir, timeout: timeout,
		segmentSeconds: segmentSeconds, runner: execCommandRunner{},
	}
}

func (t *FFmpegTranscoder) Enabled() bool { return t.enabled }

func (t *FFmpegTranscoder) Transcode(ctx context.Context, inputPath string, videoID int64, metadata domain.MediaMetadata) (string, error) {
	if !t.enabled {
		return "", nil
	}
	renditions := selectHLSRenditions(metadata)
	if len(renditions) == 0 {
		return "", fmt.Errorf("source dimensions are unavailable")
	}
	hlsRoot := filepath.Join(t.mediaDir, "hls")
	if err := os.MkdirAll(hlsRoot, 0o755); err != nil {
		return "", fmt.Errorf("create HLS directory: %w", err)
	}
	temporaryDir, err := os.MkdirTemp(hlsRoot, fmt.Sprintf(".%d-", videoID))
	if err != nil {
		return "", fmt.Errorf("create temporary HLS directory: %w", err)
	}
	defer func() {
		if temporaryDir != "" {
			_ = os.RemoveAll(temporaryDir)
		}
	}()

	transcodeCtx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	for _, rendition := range renditions {
		variantDir := filepath.Join(temporaryDir, rendition.Name)
		if err := os.MkdirAll(variantDir, 0o755); err != nil {
			return "", fmt.Errorf("create rendition directory: %w", err)
		}
		playlistPath := filepath.Join(variantDir, "index.m3u8")
		segmentPattern := filepath.Join(variantDir, "segment_%05d.ts")
		args := []string{
			"-hide_banner", "-loglevel", "error", "-y", "-i", inputPath,
			"-map", "0:v:0", "-map", "0:a:0?",
			"-vf", fmt.Sprintf("scale=-2:%d", rendition.Height),
			"-c:v", "libx264", "-preset", "veryfast", "-profile:v", "main", "-pix_fmt", "yuv420p",
			"-b:v", fmt.Sprintf("%dk", rendition.VideoBitrate),
			"-maxrate", fmt.Sprintf("%dk", int(float64(rendition.VideoBitrate)*1.07)),
			"-bufsize", fmt.Sprintf("%dk", rendition.VideoBitrate*2),
			"-c:a", "aac", "-b:a", fmt.Sprintf("%dk", rendition.AudioBitrate), "-ac", "2",
			"-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", t.segmentSeconds), "-sc_threshold", "0",
			"-f", "hls", "-hls_time", strconv.Itoa(t.segmentSeconds), "-hls_playlist_type", "vod",
			"-hls_segment_filename", segmentPattern, playlistPath,
		}
		if err := t.runner.Run(transcodeCtx, t.executable, args...); err != nil {
			if transcodeCtx.Err() != nil {
				return "", fmt.Errorf("HLS transcode timeout or cancellation: %w", transcodeCtx.Err())
			}
			return "", fmt.Errorf("transcode %s: %w", rendition.Name, err)
		}
	}
	if err := writeMasterPlaylist(temporaryDir, renditions); err != nil {
		return "", err
	}

	finalDir := filepath.Join(hlsRoot, strconv.FormatInt(videoID, 10))
	backupDir := finalDir + ".previous"
	_ = os.RemoveAll(backupDir)
	if _, err := os.Stat(finalDir); err == nil {
		if err := os.Rename(finalDir, backupDir); err != nil {
			return "", fmt.Errorf("prepare existing HLS output: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect existing HLS output: %w", err)
	}
	if err := os.Rename(temporaryDir, finalDir); err != nil {
		_ = os.Rename(backupDir, finalDir)
		return "", fmt.Errorf("publish HLS output: %w", err)
	}
	temporaryDir = ""
	_ = os.RemoveAll(backupDir)
	return filepath.ToSlash(filepath.Join("hls", strconv.FormatInt(videoID, 10), "master.m3u8")), nil
}

func selectHLSRenditions(metadata domain.MediaMetadata) []hlsRendition {
	if metadata.Width <= 0 || metadata.Height <= 0 {
		return nil
	}
	levels := []struct {
		height       int
		videoBitrate int
		audioBitrate int
	}{{360, 800, 96}, {480, 1400, 128}, {720, 2800, 128}}
	result := make([]hlsRendition, 0, len(levels))
	for _, level := range levels {
		if level.height > metadata.Height {
			continue
		}
		width := int(math.Round(float64(metadata.Width) * float64(level.height) / float64(metadata.Height)))
		if width%2 != 0 {
			width--
		}
		result = append(result, hlsRendition{
			Name: fmt.Sprintf("%dp", level.height), Width: width, Height: level.height,
			VideoBitrate: level.videoBitrate, AudioBitrate: level.audioBitrate,
		})
	}
	if len(result) == 0 {
		height := metadata.Height
		if height%2 != 0 {
			height--
		}
		width := metadata.Width
		if width%2 != 0 {
			width--
		}
		result = append(result, hlsRendition{Name: fmt.Sprintf("%dp", height), Width: width, Height: height, VideoBitrate: 600, AudioBitrate: 96})
	}
	return result
}

func writeMasterPlaylist(directory string, renditions []hlsRendition) error {
	var playlist strings.Builder
	playlist.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	for _, rendition := range renditions {
		bandwidth := (rendition.VideoBitrate + rendition.AudioBitrate) * 1000
		fmt.Fprintf(&playlist, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,NAME=\"%s\"\n%s/index.m3u8\n",
			bandwidth, rendition.Width, rendition.Height, rendition.Name, rendition.Name)
	}
	if err := os.WriteFile(filepath.Join(directory, "master.m3u8"), []byte(playlist.String()), 0o644); err != nil {
		return fmt.Errorf("write HLS master playlist: %w", err)
	}
	return nil
}
