package media

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestHLSDirectorySize(t *testing.T) {
	dir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// A missing output directory reports zero instead of failing.
	if got := hlsDirectorySize(dir, 7, logger); got != 0 {
		t.Fatalf("missing directory size = %d, want 0", got)
	}

	hlsDir := filepath.Join(dir, "hls", "7")
	if err := os.MkdirAll(filepath.Join(hlsDir, "720p"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hlsDir, "master.m3u8"), []byte("master"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hlsDir, "720p", "index.m3u8"), []byte("playlist"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := int64(len("master") + len("playlist"))
	if got := hlsDirectorySize(dir, 7, logger); got != want {
		t.Fatalf("hls directory size = %d, want %d", got, want)
	}
}
