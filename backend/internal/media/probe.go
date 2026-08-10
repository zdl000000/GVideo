package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gvideo/backend/internal/domain"
)

type Prober interface {
	Probe(context.Context, string) (domain.MediaMetadata, error)
}

type FFprobe struct {
	executable string
	timeout    time.Duration
}

func NewFFprobe(executable string, timeout time.Duration) *FFprobe {
	return &FFprobe{executable: executable, timeout: timeout}
}

func (p *FFprobe) Probe(ctx context.Context, path string) (domain.MediaMetadata, error) {
	probeCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	command := exec.CommandContext(probeCtx, p.executable,
		"-v", "error", "-print_format", "json", "-show_format", "-show_streams", path)
	output, err := command.Output()
	if err != nil {
		if probeCtx.Err() != nil {
			return domain.MediaMetadata{}, fmt.Errorf("ffprobe timed out or was canceled: %w", probeCtx.Err())
		}
		message := ""
		if exitError, ok := err.(*exec.ExitError); ok {
			message = strings.TrimSpace(string(exitError.Stderr))
		}
		if len(message) > 1200 {
			message = message[len(message)-1200:]
		}
		if message == "" {
			return domain.MediaMetadata{}, fmt.Errorf("run ffprobe: %w", err)
		}
		return domain.MediaMetadata{}, fmt.Errorf("run ffprobe: %w: %s", err, message)
	}
	metadata, err := parseProbeOutput(output)
	if err != nil {
		return domain.MediaMetadata{}, fmt.Errorf("parse ffprobe output: %w", err)
	}
	return metadata, nil
}

func parseProbeOutput(output []byte) (domain.MediaMetadata, error) {
	var payload struct {
		Format struct {
			Duration string `json:"duration"`
			Bitrate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Bitrate   string `json:"bit_rate"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return domain.MediaMetadata{}, err
	}

	metadata := domain.MediaMetadata{}
	metadata.DurationSeconds, _ = strconv.ParseFloat(payload.Format.Duration, 64)
	metadata.Bitrate, _ = strconv.ParseInt(payload.Format.Bitrate, 10, 64)
	foundVideo := false
	for _, stream := range payload.Streams {
		switch stream.CodecType {
		case "video":
			if foundVideo {
				continue
			}
			foundVideo = true
			metadata.Width = stream.Width
			metadata.Height = stream.Height
			metadata.VideoCodec = stream.CodecName
			if metadata.Bitrate == 0 {
				metadata.Bitrate, _ = strconv.ParseInt(stream.Bitrate, 10, 64)
			}
		case "audio":
			if metadata.AudioCodec == "" {
				metadata.AudioCodec = stream.CodecName
			}
		}
	}
	if !foundVideo {
		return domain.MediaMetadata{}, fmt.Errorf("no video stream found")
	}
	return metadata, nil
}
