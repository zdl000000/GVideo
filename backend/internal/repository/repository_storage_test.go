package repository

import (
	"context"
	"path/filepath"
	"testing"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform"
)

func TestUserStorageUsedCountsVideosCoversAndHLS(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "storage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := New(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "storage_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	video, err := repo.CreateVideo(ctx, domain.NewVideo{
		UserID: user.ID, Title: "Storage", Category: "科技", Visibility: "public",
		VideoPath: "videos/a.mp4", MimeType: "video/mp4", SizeBytes: 100, CoverBytes: 25,
	})
	if err != nil {
		t.Fatal(err)
	}
	used, err := repo.UserStorageUsed(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if used != 125 {
		t.Fatalf("used = %d, want 125 (video + cover)", used)
	}

	if _, err := db.Exec(`UPDATE videos SET hls_size_bytes = 400 WHERE id = ?`, video.ID); err != nil {
		t.Fatal(err)
	}
	used, err = repo.UserStorageUsed(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if used != 525 {
		t.Fatalf("used = %d, want 525 (video + cover + HLS)", used)
	}

	other, err := repo.CreateUser(ctx, "storage_other", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if used, err := repo.UserStorageUsed(ctx, other.ID); err != nil || used != 0 {
		t.Fatalf("other user used = %d err=%v, want 0", used, err)
	}
}
