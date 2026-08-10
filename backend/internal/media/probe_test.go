package media

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestParseProbeOutput(t *testing.T) {
	metadata, err := parseProbeOutput([]byte(`{
  "streams": [
    {"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"bit_rate":"3200000"},
    {"codec_type":"audio","codec_name":"aac"}
  ],
  "format": {"duration":"12.345","bit_rate":"3500000"}
}`))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Width != 1920 || metadata.Height != 1080 || metadata.VideoCodec != "h264" || metadata.AudioCodec != "aac" {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}
	if metadata.DurationSeconds != 12.345 || metadata.Bitrate != 3500000 {
		t.Fatalf("unexpected numeric metadata: %#v", metadata)
	}
}

func TestParseProbeOutputAllowsMissingAudioAndBitrate(t *testing.T) {
	metadata, err := parseProbeOutput([]byte(`{
  "streams": [{"codec_type":"video","codec_name":"vp9","width":1280,"height":720}],
  "format": {"duration":"4.5"}
}`))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.AudioCodec != "" || metadata.Bitrate != 0 || metadata.VideoCodec != "vp9" {
		t.Fatalf("unexpected optional metadata: %#v", metadata)
	}
}

func TestParseProbeOutputRejectsFilesWithoutVideo(t *testing.T) {
	if _, err := parseProbeOutput([]byte(`{"streams":[{"codec_type":"audio","codec_name":"aac"}],"format":{}}`)); err == nil {
		t.Fatal("expected missing video stream error")
	}
}

func TestFFprobeIntegration(t *testing.T) {
	path := os.Getenv("FFPROBE_INTEGRATION_FILE")
	executable := os.Getenv("FFPROBE_INTEGRATION_PATH")
	if path == "" || executable == "" {
		t.Skip("set FFPROBE_INTEGRATION_FILE and FFPROBE_INTEGRATION_PATH to run the real tool test")
	}
	metadata, err := NewFFprobe(executable, 15*time.Second).Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Width <= 0 || metadata.Height <= 0 || metadata.DurationSeconds <= 0 || metadata.VideoCodec == "" {
		t.Fatalf("incomplete real metadata: %#v", metadata)
	}
	t.Logf("duration=%.3fs resolution=%dx%d bitrate=%d video=%s audio=%s", metadata.DurationSeconds, metadata.Width, metadata.Height, metadata.Bitrate, metadata.VideoCodec, metadata.AudioCodec)
}
