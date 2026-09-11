package repository

import (
	"context"
	"path/filepath"
	"testing"

	"gvideo/backend/internal/domain"
	"gvideo/backend/internal/platform"
)

func TestSearchTreatsLikeWildcardsLiterally(t *testing.T) {
	dir := t.TempDir()
	db, err := platform.OpenDatabase(filepath.Join(dir, "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := New(db)
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "search_user", "hash")
	if err != nil {
		t.Fatal(err)
	}
	titles := []string{"100% 折扣", "100x 折扣", "a_b 标题", "axb 标题", `100\% 原价`}
	for index, title := range titles {
		if _, err := repo.CreateVideo(ctx, domain.NewVideo{
			UserID: user.ID, Title: title, Category: "科技", Visibility: "public",
			VideoPath: "videos/search.mp4", MimeType: "video/mp4", SizeBytes: int64(index + 1),
		}); err != nil {
			t.Fatal(err)
		}
	}

	search := func(query string) []string {
		videos, err := repo.ListVideos(ctx, domain.VideoFilter{Query: query, Limit: 10}, 0)
		if err != nil {
			t.Fatal(err)
		}
		found := make([]string, 0, len(videos))
		for _, video := range videos {
			found = append(found, video.Title)
		}
		return found
	}

	if got := search("100%"); len(got) != 1 || got[0] != "100% 折扣" {
		t.Fatalf("literal %% search matched %#v, want only the percent title", got)
	}
	if got := search("a_b"); len(got) != 1 || got[0] != "a_b 标题" {
		t.Fatalf("underscore search matched %#v, want only the underscore title", got)
	}
	// The escaped backslash must survive a single replacement pass: a doubled
	// escape would match nothing here.
	if got := search(`\%`); len(got) != 1 || got[0] != `100\% 原价` {
		t.Fatalf("escaped backslash-percent search matched %#v", got)
	}
	if got := search("标题"); len(got) != 2 {
		t.Fatalf("plain search matched %#v, want both underscore titles", got)
	}
}
