package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"gvideo/backend/internal/config"
	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform"
	"gvideo/backend/internal/repository"
)

// A minimal MP4 signature the standard library's sniffer recognizes.
var minimalMP4 = []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom<\x06t\xbfmdat")

func TestUploadStorageQuota(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "quota.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(repository.New(db), config.Config{
		SessionTTL:            time.Hour,
		MediaDir:              filepath.Join(dir, "media"),
		MaxUploadBytes:        1 << 20,
		UserStorageQuotaBytes: int64(len(minimalMP4)) + 5,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	user, err := svc.Register(ctx, "quota_user", "password123")
	if err != nil {
		t.Fatal(err)
	}
	upload := func(userID int64, title string) error {
		_, err := svc.UploadVideo(ctx, UploadInput{
			UserID: userID, Title: title, Category: Categories[0], Visibility: "public",
			Video: multipartFileHeader(t, "video", "sample.mp4", "video/mp4", minimalMP4),
		})
		return err
	}

	if err := upload(user.User.ID, "配额内视频"); err != nil {
		t.Fatalf("first upload: %v", err)
	}
	if err := upload(user.User.ID, "超出配额"); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("quota error = %v, want ErrQuotaExceeded", err)
	}

	// Every user keeps an independent budget.
	other, err := svc.Register(ctx, "quota_other", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if err := upload(other.User.ID, "另一个用户"); err != nil {
		t.Fatalf("other user upload: %v", err)
	}
}

// A minimal PNG signature the image sniffer recognizes.
var quotaCoverPNG = []byte("\x89PNG\r\n\x1a\ncover")

// Covers count against the quota: a tiny video with a large cover cannot be
// used to store more than the quota allows.
func TestUploadQuotaCountsCoverBytes(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "quota-cover.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	videoBytes := int64(len(minimalMP4))
	coverBytes := int64(len(quotaCoverPNG))
	svc := New(repository.New(db), config.Config{
		SessionTTL:     time.Hour,
		MediaDir:       filepath.Join(dir, "media"),
		MaxUploadBytes: 1 << 20,
		// Fits two bare videos (2×24) but not video+cover followed by another
		// video: only cover accounting makes the second upload fail.
		UserStorageQuotaBytes: 2*videoBytes + 2,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	user, err := svc.Register(ctx, "quota_cover", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UploadVideo(ctx, UploadInput{
		UserID: user.User.ID, Title: "带封面视频", Category: Categories[0], Visibility: "public",
		Video: multipartFileHeader(t, "video", "sample.mp4", "video/mp4", minimalMP4),
		Cover: multipartFileHeader(t, "cover", "cover.png", "image/png", quotaCoverPNG),
	}); err != nil {
		t.Fatalf("upload with cover: %v (cover bytes=%d)", err, coverBytes)
	}
	if _, err := svc.UploadVideo(ctx, UploadInput{
		UserID: user.User.ID, Title: "第二个视频", Category: Categories[0], Visibility: "public",
		Video: multipartFileHeader(t, "video", "sample.mp4", "video/mp4", minimalMP4),
	}); !errors.Is(err, domain.ErrQuotaExceeded) {
		t.Fatalf("cover-adjusted quota error = %v, want ErrQuotaExceeded", err)
	}
}

func TestUploadWithoutQuotaIsUnlimited(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "quota-off.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := New(repository.New(db), config.Config{
		SessionTTL:     time.Hour,
		MediaDir:       filepath.Join(dir, "media"),
		MaxUploadBytes: 1 << 20,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx := context.Background()

	user, err := svc.Register(ctx, "quota_disabled", "password123")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := svc.UploadVideo(ctx, UploadInput{
			UserID: user.User.ID, Title: "无配额视频", Category: Categories[0], Visibility: "public",
			Video: multipartFileHeader(t, "video", "sample.mp4", "video/mp4", minimalMP4),
		}); err != nil {
			t.Fatalf("upload %d: %v", i+1, err)
		}
	}
}
