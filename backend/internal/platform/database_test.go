package platform

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenDatabaseUpgradesExistingVideosTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, bio TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE videos (
  id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL REFERENCES users(id), title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '', category TEXT NOT NULL, video_path TEXT NOT NULL,
  cover_path TEXT NOT NULL DEFAULT '', mime_type TEXT NOT NULL, duration_seconds REAL NOT NULL DEFAULT 0,
  size_bytes INTEGER NOT NULL, views_count INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO users(id, username, password_hash) VALUES (1, 'legacy', 'hash');
INSERT INTO videos(id, user_id, title, category, video_path, mime_type, size_bytes) VALUES (1, 1, 'Legacy video', 'knowledge', 'videos/legacy.mp4', 'video/mp4', 100);
`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	upgraded, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	defer upgraded.Close()
	var status, hlsMasterPath string
	if err := upgraded.QueryRow(`SELECT processing_status, hls_master_path FROM videos WHERE id = 1`).Scan(&status, &hlsMasterPath); err != nil {
		t.Fatal(err)
	}
	if status != "ready" {
		t.Fatalf("legacy video status = %q, want ready", status)
	}
	if hlsMasterPath != "" {
		t.Fatalf("legacy HLS path = %q, want empty", hlsMasterPath)
	}
}

func TestDatabaseStatsAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
INSERT INTO users(id, username, password_hash) VALUES (1, 'backup-user', 'hash');
INSERT INTO users(id, username, password_hash) VALUES (2, 'followed-user', 'hash');
INSERT INTO user_follows(follower_id, followed_id) VALUES (1, 2);`); err != nil {
		t.Fatal(err)
	}
	stats, err := ReadDatabaseStats(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Users != 2 || stats.Videos != 0 || stats.Follows != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}

	backupPath := filepath.Join(t.TempDir(), "backup.db")
	if err := BackupDatabase(context.Background(), db, backupPath); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(backupPath); err != nil || info.Size() == 0 {
		t.Fatalf("backup file: info=%v err=%v", info, err)
	}
	backup, err := OpenDatabase(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	backupStats, err := ReadDatabaseStats(context.Background(), backup)
	if err != nil || backupStats.Users != 2 || backupStats.Follows != 1 {
		t.Fatalf("backup stats: %#v err=%v", backupStats, err)
	}
}

func TestUserFollowsCascadeOnEitherUserDeletion(t *testing.T) {
	db, err := OpenDatabase(filepath.Join(t.TempDir(), "cascade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
INSERT INTO users(id, username, password_hash) VALUES (1, 'follower', 'hash');
INSERT INTO users(id, username, password_hash) VALUES (2, 'followed', 'hash');
INSERT INTO users(id, username, password_hash) VALUES (3, 'another', 'hash');
INSERT INTO user_follows(follower_id, followed_id) VALUES (1, 2);
INSERT INTO user_follows(follower_id, followed_id) VALUES (3, 1);`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM users WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	var follows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM user_follows`).Scan(&follows); err != nil {
		t.Fatal(err)
	}
	if follows != 0 {
		t.Fatalf("follow rows after user deletion = %d, want 0", follows)
	}
}

func TestMergeDatabaseRemapsRelationshipsAndIsRepeatable(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "source.db")
	source, err := OpenDatabase(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = source.Exec(`
INSERT INTO users(id, username, password_hash) VALUES (7, 'source-user', 'source-hash');
INSERT INTO users(id, username, password_hash) VALUES (8, 'source-followed', 'source-hash');
INSERT INTO videos(id, user_id, title, category, video_path, hls_master_path, mime_type, size_bytes) VALUES (9, 7, 'Source video', 'knowledge', 'videos/source.mp4', 'hls/9/master.m3u8', 'video/mp4', 100);
INSERT INTO comments(id, video_id, user_id, content) VALUES (11, 9, 7, 'source comment');
INSERT INTO video_likes(user_id, video_id) VALUES (7, 9);
INSERT INTO user_follows(follower_id, followed_id) VALUES (7, 8);
INSERT INTO transcoding_jobs(video_id, status) VALUES (9, 'completed');`)
	if err != nil {
		t.Fatal(err)
	}
	source.Close()

	destination, err := OpenDatabase(filepath.Join(t.TempDir(), "destination.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	if _, err := destination.Exec(`INSERT INTO users(id, username, password_hash) VALUES (1, 'destination-user', 'hash')`); err != nil {
		t.Fatal(err)
	}
	merged, err := MergeDatabase(ctx, destination, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Users != 2 || merged.Videos != 1 || merged.Comments != 1 || merged.Likes != 1 || merged.Follows != 1 || merged.MediaJobs != 1 {
		t.Fatalf("unexpected merge stats: %#v", merged)
	}
	stats, err := ReadDatabaseStats(ctx, destination)
	if err != nil || stats.Users != 3 || stats.Videos != 1 || stats.Comments != 1 || stats.Follows != 1 {
		t.Fatalf("merged database stats: %#v err=%v", stats, err)
	}
	var mergedHLS string
	if err := destination.QueryRow(`SELECT hls_master_path FROM videos WHERE video_path = 'videos/source.mp4'`).Scan(&mergedHLS); err != nil || mergedHLS != "hls/9/master.m3u8" {
		t.Fatalf("merged HLS path = %q err=%v", mergedHLS, err)
	}
	second, err := MergeDatabase(ctx, destination, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if second.Users != 0 || second.Videos != 0 || second.Comments != 0 || second.Likes != 0 || second.Follows != 0 || second.MediaJobs != 0 {
		t.Fatalf("merge is not repeatable: %#v", second)
	}
}

func TestMergeDatabaseAcceptsLegacySourceWithoutHLSColumn(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "legacy-source.db")
	source, err := sql.Open("sqlite", sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = source.Exec(`
CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, bio TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE sessions (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, csrf_token TEXT NOT NULL, expires_at DATETIME NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE videos (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', category TEXT NOT NULL, video_path TEXT NOT NULL, cover_path TEXT NOT NULL DEFAULT '', mime_type TEXT NOT NULL, duration_seconds REAL NOT NULL DEFAULT 0, size_bytes INTEGER NOT NULL, processing_status TEXT NOT NULL DEFAULT 'ready', source_width INTEGER NOT NULL DEFAULT 0, source_height INTEGER NOT NULL DEFAULT 0, source_bitrate INTEGER NOT NULL DEFAULT 0, video_codec TEXT NOT NULL DEFAULT '', audio_codec TEXT NOT NULL DEFAULT '', processing_error TEXT NOT NULL DEFAULT '', processed_at DATETIME, views_count INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE comments (id INTEGER PRIMARY KEY, video_id INTEGER NOT NULL, user_id INTEGER NOT NULL, content TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE video_likes (user_id INTEGER NOT NULL, video_id INTEGER NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (user_id, video_id));
CREATE TABLE video_favorites (user_id INTEGER NOT NULL, video_id INTEGER NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (user_id, video_id));
CREATE TABLE transcoding_jobs (id INTEGER PRIMARY KEY, video_id INTEGER NOT NULL UNIQUE, status TEXT NOT NULL DEFAULT 'pending', attempts INTEGER NOT NULL DEFAULT 0, last_error TEXT NOT NULL DEFAULT '', available_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, started_at DATETIME, finished_at DATETIME, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
INSERT INTO users(id, username, password_hash) VALUES (1, 'legacy-source', 'hash');
INSERT INTO videos(id, user_id, title, category, video_path, mime_type, size_bytes) VALUES (1, 1, 'Legacy source', 'knowledge', 'videos/legacy-source.mp4', 'video/mp4', 100);`)
	if err != nil {
		t.Fatal(err)
	}
	source.Close()

	destination, err := OpenDatabase(filepath.Join(t.TempDir(), "destination.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	stats, err := MergeDatabase(ctx, destination, sourcePath)
	if err != nil || stats.Videos != 1 {
		t.Fatalf("merge legacy source: stats=%#v err=%v", stats, err)
	}
	var hlsMasterPath string
	if err := destination.QueryRow(`SELECT hls_master_path FROM videos`).Scan(&hlsMasterPath); err != nil || hlsMasterPath != "" {
		t.Fatalf("legacy source HLS path = %q err=%v", hlsMasterPath, err)
	}
}
