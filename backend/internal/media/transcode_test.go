package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gvideo/backend/internal/domain"
)

func TestSelectHLSRenditionsNeverUpscales(t *testing.T) {
	tests := []struct {
		name     string
		metadata domain.MediaMetadata
		want     []string
	}{
		{"360 source", domain.MediaMetadata{Width: 640, Height: 360}, []string{"360p"}},
		{"480 source", domain.MediaMetadata{Width: 854, Height: 480}, []string{"360p", "480p"}},
		{"720 source", domain.MediaMetadata{Width: 1280, Height: 720}, []string{"360p", "480p", "720p"}},
		{"1080 source", domain.MediaMetadata{Width: 1920, Height: 1080}, []string{"360p", "480p", "720p"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			renditions := selectHLSRenditions(test.metadata)
			names := make([]string, len(renditions))
			for i, rendition := range renditions {
				names[i] = rendition.Name
				if rendition.Height > test.metadata.Height || rendition.Width > test.metadata.Width {
					t.Fatalf("rendition upscales source: %#v", rendition)
				}
			}
			if !reflect.DeepEqual(names, test.want) {
				t.Fatalf("levels = %#v, want %#v", names, test.want)
			}
		})
	}
}

type fakeCommandRunner struct{ err error }

func (r fakeCommandRunner) Run(_ context.Context, _ string, args ...string) error {
	if r.err != nil {
		return r.err
	}
	playlistPath := args[len(args)-1]
	if err := os.WriteFile(playlistPath, []byte("#EXTM3U\n#EXTINF:4,\nsegment_00000.ts\n"), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(filepath.Dir(playlistPath), "segment_00000.ts"), []byte("segment"), 0o644)
}

func TestFFmpegTranscoderPublishesMasterAndVariants(t *testing.T) {
	mediaDir := t.TempDir()
	transcoder := NewFFmpegTranscoder(true, "ffmpeg", mediaDir, time.Minute, 4)
	transcoder.runner = fakeCommandRunner{}
	path, err := transcoder.Transcode(context.Background(), filepath.Join(mediaDir, "source.mp4"), 42, domain.MediaMetadata{Width: 1280, Height: 720})
	if err != nil {
		t.Fatal(err)
	}
	if path != "hls/42/master.m3u8" {
		t.Fatalf("path = %q", path)
	}
	master, err := os.ReadFile(filepath.Join(mediaDir, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"360p/index.m3u8", "480p/index.m3u8", "720p/index.m3u8"} {
		if !strings.Contains(string(master), expected) {
			t.Fatalf("master playlist missing %q: %s", expected, master)
		}
	}
}

func TestFFmpegTranscoderCleansTemporaryOutputOnFailure(t *testing.T) {
	mediaDir := t.TempDir()
	transcoder := NewFFmpegTranscoder(true, "ffmpeg", mediaDir, time.Minute, 4)
	transcoder.runner = fakeCommandRunner{err: errors.New("encode failed")}
	if _, err := transcoder.Transcode(context.Background(), filepath.Join(mediaDir, "source.mp4"), 7, domain.MediaMetadata{Width: 640, Height: 360}); err == nil {
		t.Fatal("expected transcode failure")
	}
	entries, err := os.ReadDir(filepath.Join(mediaDir, "hls"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary output was not cleaned: %#v", entries)
	}
}
