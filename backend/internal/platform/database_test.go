package platform

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpenDatabaseCreatesAndReusesMigrationLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	var version int
	var checksum string
	if err := db.QueryRow(`SELECT version, checksum FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version, &checksum); err != nil {
		db.Close()
		t.Fatal(err)
	}
	latest := databaseMigrations[len(databaseMigrations)-1]
	if version != latest.version || checksum != migrationChecksum(latest) {
		db.Close()
		t.Fatalf("unexpected ledger row: version=%d checksum=%q", version, checksum)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = OpenDatabase(path)
	if err != nil {
		t.Fatalf("reopen migrated database: %v", err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil || count != len(databaseMigrations) {
		t.Fatalf("migration ledger is not idempotent: count=%d err=%v", count, err)
	}
}

func TestOpenDatabaseRejectsMigrationChecksumMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checksum.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE schema_migrations SET checksum = 'tampered' WHERE version = 1`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDatabase(path); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestMigrationFailureRollsBackLedgerAndChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrations := []databaseMigration{
		{
			version:  1,
			identity: "failing-migration",
			apply: func(tx *sql.Tx) error {
				if _, err := tx.Exec(`CREATE TABLE should_rollback (id INTEGER PRIMARY KEY)`); err != nil {
					return err
				}
				return fmt.Errorf("intentional failure")
			},
		},
	}
	if err := runMigrations(db, migrations); err == nil {
		t.Fatal("expected migration failure")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('schema_migrations', 'should_rollback')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed migration left %d tables behind", count)
	}
}

func TestMigrationRejectsUnknownVersionAndLedgerGap(t *testing.T) {
	newMigration := func(version int) databaseMigration {
		return databaseMigration{
			version:  version,
			identity: fmt.Sprintf("migration-%d", version),
			apply: func(*sql.Tx) error {
				return nil
			},
		}
	}

	t.Run("unknown version", func(t *testing.T) {
		db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "unknown.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, checksum TEXT NOT NULL)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version, checksum) VALUES (2, 'future')`); err != nil {
			t.Fatal(err)
		}
		if err := runMigrations(db, []databaseMigration{newMigration(1)}); err == nil || !strings.Contains(err.Error(), "newer than this application") {
			t.Fatalf("expected unknown migration rejection, got %v", err)
		}
	})

	t.Run("ledger gap", func(t *testing.T) {
		db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "gap.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		migrations := []databaseMigration{newMigration(1), newMigration(2)}
		if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, checksum TEXT NOT NULL)`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations(version, checksum) VALUES (2, ?)`, migrationChecksum(migrations[1])); err != nil {
			t.Fatal(err)
		}
		if err := runMigrations(db, migrations); err == nil || !strings.Contains(err.Error(), "ledger has a gap") {
			t.Fatalf("expected migration ledger gap rejection, got %v", err)
		}
	})
}

func TestMigrationDefinitionsMustBeStrictlyIncreasing(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "order.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrations := []databaseMigration{
		{version: 2, identity: "second", apply: func(*sql.Tx) error { return nil }},
		{version: 1, identity: "first", apply: func(*sql.Tx) error { return nil }},
	}
	if err := runMigrations(db, migrations); err == nil || !strings.Contains(err.Error(), "strictly increasing") {
		t.Fatalf("expected migration order rejection, got %v", err)
	}
}

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
	var avatarPath, status, visibility, hlsMasterPath string
	if err := upgraded.QueryRow(`
SELECT u.avatar_path, v.processing_status, v.visibility, v.hls_master_path
FROM videos v JOIN users u ON u.id = v.user_id WHERE v.id = 1`).
		Scan(&avatarPath, &status, &visibility, &hlsMasterPath); err != nil {
		t.Fatal(err)
	}
	if avatarPath != "" {
		t.Fatalf("legacy avatar path = %q, want empty", avatarPath)
	}
	if status != "ready" {
		t.Fatalf("legacy video status = %q, want ready", status)
	}
	if visibility != "public" {
		t.Fatalf("legacy video visibility = %q, want public", visibility)
	}
	if hlsMasterPath != "" {
		t.Fatalf("legacy HLS path = %q, want empty", hlsMasterPath)
	}
}

func TestOpenDatabaseBackfillsProcessingProgressIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-progress.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, bio TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE videos (
  id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL, video_path TEXT NOT NULL, cover_path TEXT NOT NULL DEFAULT '', mime_type TEXT NOT NULL,
  duration_seconds REAL NOT NULL DEFAULT 0, size_bytes INTEGER NOT NULL, processing_status TEXT NOT NULL DEFAULT 'ready',
  views_count INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO users(id, username, password_hash) VALUES (1, 'legacy-progress', 'hash');
INSERT INTO videos(id, user_id, title, category, video_path, mime_type, size_bytes, processing_status) VALUES
  (1, 1, 'Ready', 'knowledge', 'videos/ready.mp4', 'video/mp4', 100, 'ready'),
  (2, 1, 'Pending', 'knowledge', 'videos/pending.mp4', 'video/mp4', 100, 'pending'),
  (3, 1, 'Processing', 'knowledge', 'videos/processing.mp4', 'video/mp4', 100, 'processing'),
  (4, 1, 'Failed', 'knowledge', 'videos/failed.mp4', 'video/mp4', 100, 'failed');`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	upgraded, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[int64]struct {
		progress int
		stage    string
	}{
		1: {progress: 100, stage: "ready"},
		2: {progress: 0, stage: "queued"},
		3: {progress: 35, stage: "transcoding"},
		4: {progress: 0, stage: "failed"},
	}
	for id, want := range expected {
		var progress int
		var stage string
		if err := upgraded.QueryRow(`SELECT processing_progress, processing_stage FROM videos WHERE id = ?`, id).Scan(&progress, &stage); err != nil {
			t.Fatal(err)
		}
		if progress != want.progress || stage != want.stage {
			t.Fatalf("video %d progress=%d stage=%q, want %d/%q", id, progress, stage, want.progress, want.stage)
		}
	}
	if _, err := upgraded.Exec(`UPDATE videos SET processing_progress = 72, processing_stage = 'finalizing' WHERE id = 4`); err != nil {
		t.Fatal(err)
	}
	upgraded.Close()

	reopened, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var progress int
	var stage string
	if err := reopened.QueryRow(`SELECT processing_progress, processing_stage FROM videos WHERE id = 4`).Scan(&progress, &stage); err != nil {
		t.Fatal(err)
	}
	if progress != 72 || stage != "finalizing" {
		t.Fatalf("idempotent migration overwrote existing values: %d/%q", progress, stage)
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
INSERT INTO user_follows(follower_id, followed_id) VALUES (1, 2);
INSERT INTO notifications(recipient_id, actor_id, actor_username, type, created_at)
VALUES (2, 1, 'backup-user', 'follow', '2026-08-12 10:00:00');`); err != nil {
		t.Fatal(err)
	}
	stats, err := ReadDatabaseStats(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Users != 2 || stats.Videos != 0 || stats.Follows != 1 || stats.Notifications != 1 {
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
	if err != nil || backupStats.Users != 2 || backupStats.Follows != 1 || backupStats.Notifications != 1 {
		t.Fatalf("backup stats: %#v err=%v", backupStats, err)
	}
	if err := VerifyDatabase(context.Background(), backup); err != nil {
		t.Fatalf("verify backup database: %v", err)
	}
	readOnly, err := OpenDatabaseReadOnly(backupPath)
	if err != nil {
		t.Fatalf("open backup read-only: %v", err)
	}
	defer readOnly.Close()
	if err := VerifyDatabase(context.Background(), readOnly); err != nil {
		t.Fatalf("verify read-only backup database: %v", err)
	}
}

func TestVerifyDatabaseRejectsForeignKeyViolations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foreign-key-violation.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO comments(video_id, user_id, content) VALUES (999, 999, 'orphan')`); err != nil {
		t.Fatal(err)
	}
	if err := VerifyDatabase(context.Background(), db); err == nil {
		t.Fatal("VerifyDatabase() accepted a foreign key violation")
	}
}

func TestVerifyMediaFilesChecksReferencesAndHLSChildren(t *testing.T) {
	root := t.TempDir()
	db, err := OpenDatabase(filepath.Join(root, "data", "gvideo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mediaDir := filepath.Join(root, "media")
	files := map[string]string{
		"avatars/user.png":          "avatar",
		"videos/source.mp4":         "video",
		"covers/source.jpg":         "cover",
		"subtitles/source.vtt":      "WEBVTT",
		"hls/1/master.m3u8":         "#EXTM3U\n360p/index.m3u8\n",
		"hls/1/360p/index.m3u8":     "#EXTM3U\nsegment-000.ts\n",
		"hls/1/360p/segment-000.ts": "segment",
	}
	for name, content := range files {
		path := filepath.Join(mediaDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`
INSERT INTO users(id, username, password_hash, avatar_path) VALUES (1, 'media-user', 'hash', 'avatars/user.png');
INSERT INTO videos(id, user_id, title, category, video_path, hls_master_path, cover_path, mime_type, size_bytes)
VALUES (1, 1, 'Media', 'knowledge', 'videos\source.mp4', 'hls\1\master.m3u8', 'covers/source.jpg', 'video/mp4', 5);
INSERT INTO video_subtitles(id, video_id, language, label, subtitle_path) VALUES (1, 1, 'en', 'English', 'subtitles/source.vtt');`); err != nil {
		t.Fatal(err)
	}
	if err := VerifyMediaFiles(context.Background(), db, mediaDir); err != nil {
		t.Fatalf("VerifyMediaFiles() error = %v", err)
	}
	if err := os.Remove(filepath.Join(mediaDir, "hls", "1", "360p", "segment-000.ts")); err != nil {
		t.Fatal(err)
	}
	if err := VerifyMediaFiles(context.Background(), db, mediaDir); err == nil {
		t.Fatal("VerifyMediaFiles() accepted a missing HLS segment")
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
INSERT INTO users(id, username, password_hash, avatar_path) VALUES (7, 'source-user', 'source-hash', 'avatars/source.png');
INSERT INTO users(id, username, password_hash) VALUES (8, 'source-followed', 'source-hash');
UPDATE users SET is_admin = 1 WHERE id = 7;
INSERT INTO videos(id, user_id, title, category, visibility, video_path, hls_master_path, mime_type, size_bytes, processing_status, processing_progress, processing_stage, processing_error)
VALUES (9, 7, 'Source video', 'knowledge', 'private', 'videos/source.mp4', 'hls/9/master.m3u8', 'video/mp4', 100, 'failed', 72, 'failed', 'internal failure');
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
	var mergedAvatar, mergedVisibility, mergedHLS, mergedStage string
	var mergedProgress int
	if err := destination.QueryRow(`
SELECT u.avatar_path, v.visibility, v.hls_master_path, v.processing_progress, v.processing_stage
FROM videos v JOIN users u ON u.id = v.user_id
WHERE v.video_path = 'videos/source.mp4'`).Scan(&mergedAvatar, &mergedVisibility, &mergedHLS, &mergedProgress, &mergedStage); err != nil {
		t.Fatal(err)
	}
	if mergedAvatar != "avatars/source.png" || mergedVisibility != "private" || mergedHLS != "hls/9/master.m3u8" ||
		mergedProgress != 72 || mergedStage != "failed" {
		t.Fatalf("merged fields: avatar=%q visibility=%q hls=%q progress=%d stage=%q",
			mergedAvatar, mergedVisibility, mergedHLS, mergedProgress, mergedStage)
	}
	var mergedAdmin int
	if err := destination.QueryRow(`SELECT is_admin FROM users WHERE username = 'source-user'`).Scan(&mergedAdmin); err != nil {
		t.Fatal(err)
	}
	if mergedAdmin != 1 {
		t.Fatalf("merged admin flag = %d, want 1", mergedAdmin)
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
INSERT INTO videos(id, user_id, title, category, video_path, mime_type, size_bytes, processing_status)
VALUES (1, 1, 'Legacy source', 'knowledge', 'videos/legacy-source.mp4', 'video/mp4', 100, 'processing');`)
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
	var avatarPath, visibility, hlsMasterPath, processingStage string
	var processingProgress int
	if err := destination.QueryRow(`
SELECT u.avatar_path, v.visibility, v.hls_master_path, v.processing_progress, v.processing_stage
FROM videos v JOIN users u ON u.id = v.user_id`).
		Scan(&avatarPath, &visibility, &hlsMasterPath, &processingProgress, &processingStage); err != nil {
		t.Fatal(err)
	}
	if avatarPath != "" || visibility != "public" || hlsMasterPath != "" || processingProgress != 35 || processingStage != "transcoding" {
		t.Fatalf("legacy merged fields: avatar=%q visibility=%q hls=%q progress=%d stage=%q",
			avatarPath, visibility, hlsMasterPath, processingProgress, processingStage)
	}
}

func TestMergeDatabaseNormalizesInvalidVisibilityToPrivate(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "invalid-visibility-source.db")
	source, err := OpenDatabase(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec(`
PRAGMA ignore_check_constraints = ON;
INSERT INTO users(id, username, password_hash) VALUES (1, 'invalid-visibility-user', 'hash');
INSERT INTO videos(id, user_id, title, category, visibility, video_path, mime_type, size_bytes)
VALUES (1, 1, 'Invalid visibility', 'knowledge', 'friends', 'videos/invalid-visibility.mp4', 'video/mp4', 10);`); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	destination, err := OpenDatabase(filepath.Join(t.TempDir(), "destination.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	merged, err := MergeDatabase(ctx, destination, sourcePath)
	if err != nil || merged.Videos != 1 {
		t.Fatalf("merge invalid visibility: stats=%#v err=%v", merged, err)
	}
	var visibility string
	if err := destination.QueryRow(`SELECT visibility FROM videos WHERE video_path = 'videos/invalid-visibility.mp4'`).Scan(&visibility); err != nil {
		t.Fatal(err)
	}
	if visibility != "private" {
		t.Fatalf("merged visibility = %q, want private", visibility)
	}
	second, err := MergeDatabase(ctx, destination, sourcePath)
	if err != nil || second.Videos != 0 {
		t.Fatalf("repeat invalid visibility merge: stats=%#v err=%v", second, err)
	}
}

func TestMergeDatabaseAcceptsSourceWithoutTranscodingJobsAndIsRepeatable(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "source-without-jobs.db")
	source, err := sql.Open("sqlite", sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = source.Exec(`
CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, bio TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE sessions (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, csrf_token TEXT NOT NULL, expires_at DATETIME NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE videos (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', category TEXT NOT NULL, video_path TEXT NOT NULL, cover_path TEXT NOT NULL DEFAULT '', mime_type TEXT NOT NULL, duration_seconds REAL NOT NULL DEFAULT 0, size_bytes INTEGER NOT NULL, processing_status TEXT NOT NULL DEFAULT 'ready', views_count INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE comments (id INTEGER PRIMARY KEY, video_id INTEGER NOT NULL, user_id INTEGER NOT NULL, content TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE video_likes (user_id INTEGER NOT NULL, video_id INTEGER NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (user_id, video_id));
CREATE TABLE video_favorites (user_id INTEGER NOT NULL, video_id INTEGER NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (user_id, video_id));
INSERT INTO users(id, username, password_hash) VALUES (1, 'legacy-no-jobs', 'hash');
INSERT INTO users(id, username, password_hash) VALUES (2, 'legacy-no-jobs-followed', 'hash');
INSERT INTO videos(id, user_id, title, category, video_path, mime_type, size_bytes) VALUES
  (1, 1, 'Legacy without jobs', 'knowledge', 'videos/no-jobs.mp4', 'video/mp4', 100);
INSERT INTO comments(id, video_id, user_id, content) VALUES (1, 1, 1, 'legacy comment');
INSERT INTO video_likes(user_id, video_id) VALUES (2, 1);`)
	if err != nil {
		source.Close()
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	destination, err := OpenDatabase(filepath.Join(t.TempDir(), "destination.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	merged, err := MergeDatabase(ctx, destination, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Users != 2 || merged.Videos != 1 || merged.Comments != 1 || merged.Likes != 1 || merged.MediaJobs != 0 {
		t.Fatalf("unexpected merge stats without jobs: %#v", merged)
	}
	stats, err := ReadDatabaseStats(ctx, destination)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Users != 2 || stats.Videos != 1 || stats.Comments != 1 || stats.Likes != 1 || stats.MediaJobs != 0 {
		t.Fatalf("unexpected database stats without jobs: %#v", stats)
	}

	second, err := MergeDatabase(ctx, destination, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if second.Users != 0 || second.Videos != 0 || second.Comments != 0 || second.Likes != 0 || second.MediaJobs != 0 {
		t.Fatalf("merge without jobs is not repeatable: %#v", second)
	}
}

func TestMergeDatabaseAcceptsLegacySourceWithoutNotifications(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "legacy-without-notifications.db")
	source, err := sql.Open("sqlite", sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec(`
CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, bio TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE sessions (token_hash TEXT PRIMARY KEY, user_id INTEGER NOT NULL, csrf_token TEXT NOT NULL, expires_at DATETIME NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE videos (id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', category TEXT NOT NULL, video_path TEXT NOT NULL, cover_path TEXT NOT NULL DEFAULT '', mime_type TEXT NOT NULL, duration_seconds REAL NOT NULL DEFAULT 0, size_bytes INTEGER NOT NULL, processing_status TEXT NOT NULL DEFAULT 'ready', views_count INTEGER NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE comments (id INTEGER PRIMARY KEY, video_id INTEGER NOT NULL, user_id INTEGER NOT NULL, content TEXT NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE video_likes (user_id INTEGER NOT NULL, video_id INTEGER NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (user_id, video_id));
CREATE TABLE video_favorites (user_id INTEGER NOT NULL, video_id INTEGER NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (user_id, video_id));
INSERT INTO users(id, username, password_hash) VALUES (4, 'legacy-without-notifications', 'hash');
INSERT INTO videos(id, user_id, title, category, video_path, mime_type, size_bytes)
VALUES (5, 4, 'Legacy video', 'knowledge', 'videos/legacy-without-notifications.mp4', 'video/mp4', 100);`); err != nil {
		source.Close()
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	destination, err := OpenDatabase(filepath.Join(t.TempDir(), "destination.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	merged, err := MergeDatabase(ctx, destination, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Users != 1 || merged.Videos != 1 || merged.Notifications != 0 {
		t.Fatalf("unexpected legacy merge stats: %#v", merged)
	}
	stats, err := ReadDatabaseStats(ctx, destination)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Notifications != 0 {
		t.Fatalf("notification count after legacy merge = %d, want 0", stats.Notifications)
	}
}

func TestMergeDatabaseRemapsNotificationsAndIsRepeatable(t *testing.T) {
	ctx := context.Background()
	sourcePath := filepath.Join(t.TempDir(), "notification-source.db")
	source, err := OpenDatabase(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := source.Exec(`
INSERT INTO users(id, username, password_hash, avatar_path) VALUES
  (10, 'notification-recipient', 'hash', 'avatars/recipient.png'),
  (20, 'notification-actor', 'hash', 'avatars/actor.png');
INSERT INTO videos(id, user_id, title, category, video_path, mime_type, size_bytes)
VALUES (30, 10, 'Notification video', 'knowledge', 'videos/notification.mp4', 'video/mp4', 100);
INSERT INTO comments(id, video_id, user_id, content, created_at)
VALUES (40, 30, 20, 'mapped comment', '2026-08-12 10:01:00');
INSERT INTO notifications(
  id, recipient_id, actor_id, actor_username, actor_avatar_path, type, video_id,
  video_title, comment_id, comment_preview, read_at, created_at)
VALUES (50, 10, 20, 'notification-actor', 'avatars/actor.png', 'comment', 30,
  'Notification video', 40, 'mapped comment', '2026-08-12 10:03:00', '2026-08-12 10:02:00');`); err != nil {
		source.Close()
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}

	destination, err := OpenDatabase(filepath.Join(t.TempDir(), "notification-destination.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	if _, err := destination.Exec(`
INSERT INTO users(id, username, password_hash) VALUES
  (10, 'destination-user-10', 'hash'),
  (20, 'destination-user-20', 'hash');
INSERT INTO videos(id, user_id, title, category, video_path, mime_type, size_bytes)
VALUES (30, 10, 'Destination video', 'knowledge', 'videos/destination.mp4', 'video/mp4', 10);
INSERT INTO comments(id, video_id, user_id, content) VALUES (40, 30, 20, 'destination comment');`); err != nil {
		t.Fatal(err)
	}

	merged, err := MergeDatabase(ctx, destination, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Users != 2 || merged.Videos != 1 || merged.Comments != 1 || merged.Notifications != 1 {
		t.Fatalf("unexpected notification merge stats: %#v", merged)
	}

	var notificationID, recipientID, actorID, videoID, commentID int64
	var actorUsername, actorAvatarPath, videoTitle, commentPreview, readAt string
	if err := destination.QueryRow(`
SELECT id, recipient_id, actor_id, actor_username, actor_avatar_path, video_id,
       video_title, comment_id, comment_preview, read_at
FROM notifications`).Scan(
		&notificationID, &recipientID, &actorID, &actorUsername, &actorAvatarPath, &videoID,
		&videoTitle, &commentID, &commentPreview, &readAt); err != nil {
		t.Fatal(err)
	}
	if notificationID == 50 {
		t.Fatalf("notification retained source id %d; destination should allocate its own id", notificationID)
	}
	if recipientID == 10 || actorID == 20 || videoID == 30 || commentID == 40 {
		t.Fatalf("notification relationships were not remapped: recipient=%d actor=%d video=%d comment=%d",
			recipientID, actorID, videoID, commentID)
	}
	if actorUsername != "notification-actor" || actorAvatarPath != "avatars/actor.png" ||
		videoTitle != "Notification video" || commentPreview != "mapped comment" || readAt == "" {
		t.Fatalf("notification snapshots not preserved: actor=%q avatar=%q video=%q preview=%q read_at=%q",
			actorUsername, actorAvatarPath, videoTitle, commentPreview, readAt)
	}
	var mappedRecipient, mappedActor, mappedVideo, mappedComment int64
	if err := destination.QueryRow(`SELECT id FROM users WHERE username = 'notification-recipient'`).Scan(&mappedRecipient); err != nil {
		t.Fatal(err)
	}
	if err := destination.QueryRow(`SELECT id FROM users WHERE username = 'notification-actor'`).Scan(&mappedActor); err != nil {
		t.Fatal(err)
	}
	if err := destination.QueryRow(`SELECT id FROM videos WHERE video_path = 'videos/notification.mp4'`).Scan(&mappedVideo); err != nil {
		t.Fatal(err)
	}
	if err := destination.QueryRow(`SELECT id FROM comments WHERE video_id = ? AND content = 'mapped comment'`, mappedVideo).Scan(&mappedComment); err != nil {
		t.Fatal(err)
	}
	if recipientID != mappedRecipient || actorID != mappedActor || videoID != mappedVideo || commentID != mappedComment {
		t.Fatalf("notification mappings = %d/%d/%d/%d, want %d/%d/%d/%d",
			recipientID, actorID, videoID, commentID, mappedRecipient, mappedActor, mappedVideo, mappedComment)
	}

	stats, err := ReadDatabaseStats(ctx, destination)
	if err != nil || stats.Notifications != 1 {
		t.Fatalf("notification stats after merge: %#v err=%v", stats, err)
	}
	second, err := MergeDatabase(ctx, destination, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if second.Users != 0 || second.Videos != 0 || second.Comments != 0 || second.Notifications != 0 {
		t.Fatalf("notification merge is not repeatable: %#v", second)
	}
}
