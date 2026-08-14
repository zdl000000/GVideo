package platform

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type DatabaseStats struct {
	Users         int64
	Sessions      int64
	Videos        int64
	Subtitles     int64
	Comments      int64
	Likes         int64
	Favorites     int64
	Follows       int64
	MediaJobs     int64
	Notifications int64
}

type MergeStats struct {
	Users         int64
	RenamedUsers  int64
	Sessions      int64
	Videos        int64
	Subtitles     int64
	Comments      int64
	Likes         int64
	Favorites     int64
	Follows       int64
	MediaJobs     int64
	Notifications int64
}

func OpenDatabase(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func OpenDatabaseReadOnly(path string) (*sql.DB, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve sqlite path: %w", err)
	}
	if _, err := os.Stat(absolute); err != nil {
		return nil, fmt.Errorf("inspect sqlite database: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(absolute) + "?mode=ro&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite read-only: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite read-only: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL COLLATE NOCASE UNIQUE,
  password_hash TEXT NOT NULL,
  bio TEXT NOT NULL DEFAULT '',
  avatar_path TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  csrf_token TEXT NOT NULL,
  expires_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
CREATE TABLE IF NOT EXISTS videos (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL,
  video_path TEXT NOT NULL,
	hls_master_path TEXT NOT NULL DEFAULT '',
  cover_path TEXT NOT NULL DEFAULT '',
  mime_type TEXT NOT NULL,
  duration_seconds REAL NOT NULL DEFAULT 0,
	size_bytes INTEGER NOT NULL,
	processing_status TEXT NOT NULL DEFAULT 'ready',
	processing_progress INTEGER NOT NULL DEFAULT 100,
	processing_stage TEXT NOT NULL DEFAULT 'ready',
	visibility TEXT NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'unlisted', 'private')),
	source_width INTEGER NOT NULL DEFAULT 0,
	source_height INTEGER NOT NULL DEFAULT 0,
	source_bitrate INTEGER NOT NULL DEFAULT 0,
	video_codec TEXT NOT NULL DEFAULT '',
	audio_codec TEXT NOT NULL DEFAULT '',
	processing_error TEXT NOT NULL DEFAULT '',
	processed_at DATETIME,
  views_count INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_videos_created_at ON videos(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_videos_category ON videos(category);
CREATE TABLE IF NOT EXISTS video_subtitles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  video_id INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  language TEXT NOT NULL,
  label TEXT NOT NULL,
  subtitle_path TEXT NOT NULL,
  is_default INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(video_id, language)
);
CREATE INDEX IF NOT EXISTS idx_video_subtitles_video_id ON video_subtitles(video_id, id);
CREATE TABLE IF NOT EXISTS video_likes (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  video_id INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, video_id)
);
CREATE TABLE IF NOT EXISTS video_favorites (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  video_id INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, video_id)
);
CREATE TABLE IF NOT EXISTS user_follows (
  follower_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  followed_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (follower_id, followed_id),
  CHECK (follower_id <> followed_id)
);
CREATE INDEX IF NOT EXISTS idx_user_follows_followed ON user_follows(followed_id, created_at DESC);
CREATE TABLE IF NOT EXISTS comments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  video_id INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  content TEXT NOT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_comments_video_id ON comments(video_id, created_at DESC);
CREATE TABLE IF NOT EXISTS video_reports (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  video_id INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  reason TEXT NOT NULL CHECK (reason IN ('spam', 'inappropriate', 'copyright', 'other')),
  detail TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'reviewed', 'resolved', 'dismissed')),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(video_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_video_reports_status ON video_reports(status, updated_at DESC);
CREATE TABLE IF NOT EXISTS notifications (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  recipient_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  actor_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
	actor_username TEXT NOT NULL DEFAULT '',
	actor_avatar_path TEXT NOT NULL DEFAULT '',
  type TEXT NOT NULL CHECK (type IN ('follow', 'like', 'favorite', 'comment', 'processing_ready', 'processing_failed')),
  video_id INTEGER REFERENCES videos(id) ON DELETE SET NULL,
  video_title TEXT NOT NULL DEFAULT '',
  comment_id INTEGER REFERENCES comments(id) ON DELETE SET NULL,
  comment_preview TEXT NOT NULL DEFAULT '',
  read_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_created ON notifications(recipient_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_unread ON notifications(recipient_id, read_at);
CREATE TABLE IF NOT EXISTS transcoding_jobs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  video_id INTEGER NOT NULL UNIQUE REFERENCES videos(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending',
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT '',
  available_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  started_at DATETIME,
  finished_at DATETIME,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_transcoding_jobs_claim ON transcoding_jobs(status, available_at, id);
`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	columns := []struct {
		table      string
		name       string
		definition string
	}{
		{"users", "avatar_path", "TEXT NOT NULL DEFAULT ''"},
		{"videos", "processing_status", "TEXT NOT NULL DEFAULT 'ready'"},
		{"videos", "processing_progress", "INTEGER NOT NULL DEFAULT 100"},
		{"videos", "processing_stage", "TEXT NOT NULL DEFAULT 'ready'"},
		{"videos", "visibility", "TEXT NOT NULL DEFAULT 'public'"},
		{"videos", "hls_master_path", "TEXT NOT NULL DEFAULT ''"},
		{"videos", "source_width", "INTEGER NOT NULL DEFAULT 0"},
		{"videos", "source_height", "INTEGER NOT NULL DEFAULT 0"},
		{"videos", "source_bitrate", "INTEGER NOT NULL DEFAULT 0"},
		{"videos", "video_codec", "TEXT NOT NULL DEFAULT ''"},
		{"videos", "audio_codec", "TEXT NOT NULL DEFAULT ''"},
		{"videos", "processing_error", "TEXT NOT NULL DEFAULT ''"},
		{"videos", "processed_at", "DATETIME"},
		{"notifications", "actor_username", "TEXT NOT NULL DEFAULT ''"},
		{"notifications", "actor_avatar_path", "TEXT NOT NULL DEFAULT ''"},
	}
	addedColumns := make(map[string]bool)
	for _, column := range columns {
		added, err := ensureColumn(db, column.table, column.name, column.definition)
		if err != nil {
			return fmt.Errorf("migrate %s.%s: %w", column.table, column.name, err)
		}
		addedColumns[column.table+"."+column.name] = added
	}
	if addedColumns["videos.processing_progress"] {
		if _, err := db.Exec(`
UPDATE videos SET processing_progress = CASE processing_status
  WHEN 'ready' THEN 100
  WHEN 'processing' THEN 35
  ELSE 0
END`); err != nil {
			return fmt.Errorf("backfill videos.processing_progress: %w", err)
		}
	}
	if addedColumns["videos.processing_stage"] {
		if _, err := db.Exec(`
UPDATE videos SET processing_stage = CASE processing_status
  WHEN 'ready' THEN 'ready'
  WHEN 'processing' THEN 'transcoding'
  WHEN 'failed' THEN 'failed'
  ELSE 'queued'
END`); err != nil {
			return fmt.Errorf("backfill videos.processing_stage: %w", err)
		}
	}
	return nil
}

func ensureColumn(db *sql.DB, table, name, definition string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var columnName, columnType string
		var notNull, primaryKey int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if columnName == name {
			return false, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	_, err = db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + name + ` ` + definition)
	return err == nil, err
}

func ReadDatabaseStats(ctx context.Context, db *sql.DB) (DatabaseStats, error) {
	queries := []struct {
		name   string
		target *int64
	}{
		{"users", nil},
		{"sessions", nil},
		{"videos", nil},
		{"video_subtitles", nil},
		{"comments", nil},
		{"video_likes", nil},
		{"video_favorites", nil},
		{"user_follows", nil},
		{"transcoding_jobs", nil},
		{"notifications", nil},
	}
	var stats DatabaseStats
	queries[0].target = &stats.Users
	queries[1].target = &stats.Sessions
	queries[2].target = &stats.Videos
	queries[3].target = &stats.Subtitles
	queries[4].target = &stats.Comments
	queries[5].target = &stats.Likes
	queries[6].target = &stats.Favorites
	queries[7].target = &stats.Follows
	queries[8].target = &stats.MediaJobs
	queries[9].target = &stats.Notifications
	for _, query := range queries {
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+query.name).Scan(query.target); err != nil {
			return DatabaseStats{}, fmt.Errorf("count %s: %w", query.name, err)
		}
	}
	return stats, nil
}

func VerifyDatabase(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return fmt.Errorf("check sqlite integrity: %w", err)
	}
	defer rows.Close()
	var problems []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("read sqlite integrity result: %w", err)
		}
		if result != "ok" && len(problems) < 5 {
			problems = append(problems, result)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite integrity results: %w", err)
	}
	if len(problems) > 0 {
		return fmt.Errorf("sqlite integrity check failed: %s", strings.Join(problems, "; "))
	}
	foreignKeyRows, err := db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("check sqlite foreign keys: %w", err)
	}
	defer foreignKeyRows.Close()
	problems = problems[:0]
	for foreignKeyRows.Next() {
		var table, parent string
		var rowID sql.NullInt64
		var foreignKeyID int64
		if err := foreignKeyRows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
			return fmt.Errorf("read sqlite foreign key result: %w", err)
		}
		if len(problems) < 5 {
			problems = append(problems, fmt.Sprintf("%s row %d references %s (fk %d)", table, rowID.Int64, parent, foreignKeyID))
		}
	}
	if err := foreignKeyRows.Err(); err != nil {
		return fmt.Errorf("iterate sqlite foreign key results: %w", err)
	}
	if len(problems) > 0 {
		return fmt.Errorf("sqlite foreign key check failed: %s", strings.Join(problems, "; "))
	}
	return nil
}

func VerifyMediaFiles(ctx context.Context, db *sql.DB, mediaDir string) error {
	root, err := filepath.Abs(mediaDir)
	if err != nil {
		return fmt.Errorf("resolve media directory: %w", err)
	}
	verifyFile := func(kind string, id int64, relative string, expectedSize int64) error {
		if relative == "" {
			return nil
		}
		absolute, err := secureMediaPath(root, relative)
		if err != nil {
			return fmt.Errorf("%s %d: %w", kind, id, err)
		}
		info, err := os.Stat(absolute)
		if err != nil {
			return fmt.Errorf("%s %d file %q: %w", kind, id, relative, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s %d file %q is not regular", kind, id, relative)
		}
		if expectedSize >= 0 && info.Size() != expectedSize {
			return fmt.Errorf("%s %d file %q size is %d, want %d", kind, id, relative, info.Size(), expectedSize)
		}
		return nil
	}

	rows, err := db.QueryContext(ctx, `SELECT id, avatar_path FROM users WHERE avatar_path <> ''`)
	if err != nil {
		return fmt.Errorf("read avatar references: %w", err)
	}
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			rows.Close()
			return fmt.Errorf("scan avatar reference: %w", err)
		}
		if err := verifyFile("avatar", id, path, -1); err != nil {
			rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate avatar references: %w", err)
	}
	rows.Close()

	rows, err = db.QueryContext(ctx, `SELECT id, video_path, size_bytes, cover_path, hls_master_path FROM videos`)
	if err != nil {
		return fmt.Errorf("read video media references: %w", err)
	}
	var playlists []struct {
		id   int64
		path string
	}
	for rows.Next() {
		var id, size int64
		var videoPath, coverPath, hlsPath string
		if err := rows.Scan(&id, &videoPath, &size, &coverPath, &hlsPath); err != nil {
			rows.Close()
			return fmt.Errorf("scan video media reference: %w", err)
		}
		if err := verifyFile("video", id, videoPath, size); err != nil {
			rows.Close()
			return err
		}
		if err := verifyFile("cover", id, coverPath, -1); err != nil {
			rows.Close()
			return err
		}
		if err := verifyFile("HLS master", id, hlsPath, -1); err != nil {
			rows.Close()
			return err
		}
		if hlsPath != "" {
			playlists = append(playlists, struct {
				id   int64
				path string
			}{id: id, path: hlsPath})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate video media references: %w", err)
	}
	rows.Close()

	rows, err = db.QueryContext(ctx, `SELECT id, subtitle_path FROM video_subtitles WHERE subtitle_path <> ''`)
	if err != nil {
		return fmt.Errorf("read subtitle references: %w", err)
	}
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			rows.Close()
			return fmt.Errorf("scan subtitle reference: %w", err)
		}
		if err := verifyFile("subtitle", id, path, -1); err != nil {
			rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate subtitle references: %w", err)
	}
	rows.Close()

	visited := make(map[string]bool)
	for _, playlist := range playlists {
		if err := verifyHLSPlaylist(root, playlist.path, visited); err != nil {
			return fmt.Errorf("video %d HLS: %w", playlist.id, err)
		}
	}
	return nil
}

func secureMediaPath(root, relative string) (string, error) {
	portable := strings.ReplaceAll(strings.TrimSpace(relative), `\`, "/")
	clean := filepath.Clean(filepath.FromSlash(portable))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("unsafe media path %q", relative)
	}
	absolute := filepath.Join(root, clean)
	rootPrefix := root + string(filepath.Separator)
	if absolute != root && !strings.HasPrefix(absolute, rootPrefix) {
		return "", fmt.Errorf("media path escapes root: %q", relative)
	}
	return absolute, nil
}

func verifyHLSPlaylist(root, relative string, visited map[string]bool) error {
	portable := strings.ReplaceAll(relative, `\`, "/")
	normalized := filepath.ToSlash(filepath.Clean(filepath.FromSlash(portable)))
	if visited[normalized] {
		return nil
	}
	visited[normalized] = true
	absolute, err := secureMediaPath(root, normalized)
	if err != nil {
		return err
	}
	file, err := os.Open(absolute)
	if err != nil {
		return fmt.Errorf("open playlist %q: %w", relative, err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		reference, err := url.Parse(line)
		if err != nil || reference.IsAbs() || reference.Host != "" || strings.HasPrefix(reference.Path, "/") {
			return fmt.Errorf("playlist %q has unsupported reference %q", relative, line)
		}
		referencedPath, err := url.PathUnescape(reference.Path)
		if err != nil {
			return fmt.Errorf("playlist %q has invalid escaped path %q", relative, line)
		}
		child := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(filepath.FromSlash(normalized)), filepath.FromSlash(referencedPath))))
		childAbsolute, err := secureMediaPath(root, child)
		if err != nil {
			return fmt.Errorf("playlist %q reference %q: %w", relative, line, err)
		}
		info, err := os.Stat(childAbsolute)
		if err != nil {
			return fmt.Errorf("playlist %q reference %q: %w", relative, line, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("playlist %q reference %q is not regular", relative, line)
		}
		if strings.EqualFold(filepath.Ext(child), ".m3u8") {
			if err := verifyHLSPlaylist(root, child, visited); err != nil {
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read playlist %q: %w", relative, err)
	}
	return nil
}

func BackupDatabase(ctx context.Context, db *sql.DB, destination string) error {
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("resolve backup path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	if _, err := os.Stat(absolute); err == nil {
		return fmt.Errorf("backup destination already exists: %s", absolute)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect backup destination: %w", err)
	}
	quoted := "'" + strings.ReplaceAll(filepath.ToSlash(absolute), "'", "''") + "'"
	if _, err := db.ExecContext(ctx, `VACUUM INTO `+quoted); err != nil {
		return fmt.Errorf("backup sqlite database: %w", err)
	}
	return nil
}

func MergeDatabase(ctx context.Context, db *sql.DB, source string) (MergeStats, error) {
	absolute, err := filepath.Abs(source)
	if err != nil {
		return MergeStats{}, fmt.Errorf("resolve merge source: %w", err)
	}
	if _, err := os.Stat(absolute); err != nil {
		return MergeStats{}, fmt.Errorf("inspect merge source: %w", err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return MergeStats{}, fmt.Errorf("acquire merge connection: %w", err)
	}
	defer conn.Close()
	quoted := "'" + strings.ReplaceAll(filepath.ToSlash(absolute), "'", "''") + "'"
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE `+quoted+` AS source_db`); err != nil {
		return MergeStats{}, fmt.Errorf("attach merge source: %w", err)
	}
	defer conn.ExecContext(context.Background(), `DETACH DATABASE source_db`)

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return MergeStats{}, fmt.Errorf("begin database merge: %w", err)
	}
	defer tx.Rollback()
	sourceHasAvatar, err := databaseColumnExists(ctx, tx, "source_db", "users", "avatar_path")
	if err != nil {
		return MergeStats{}, fmt.Errorf("inspect source user columns: %w", err)
	}
	sourceAvatarExpression := "''"
	if sourceHasAvatar {
		sourceAvatarExpression = "su.avatar_path"
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
DROP TABLE IF EXISTS merge_user_map;
CREATE TEMP TABLE merge_user_map (
  source_id INTEGER PRIMARY KEY,
  target_id INTEGER NOT NULL,
  renamed INTEGER NOT NULL DEFAULT 0,
  added INTEGER NOT NULL DEFAULT 0
);
INSERT INTO merge_user_map(source_id, target_id)
SELECT su.id, u.id FROM source_db.users su
JOIN users u ON u.username = su.username AND u.password_hash = su.password_hash;
INSERT OR IGNORE INTO users(username, password_hash, bio, avatar_path, created_at)
SELECT su.username, su.password_hash, su.bio, %s, su.created_at
FROM source_db.users su
WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.username = su.username);
INSERT OR IGNORE INTO merge_user_map(source_id, target_id, added)
SELECT su.id, u.id, 1 FROM source_db.users su
JOIN users u ON u.username = su.username AND u.password_hash = su.password_hash;
INSERT OR IGNORE INTO users(username, password_hash, bio, avatar_path, created_at)
SELECT su.username || '-local-' || su.id, su.password_hash, su.bio, %s, su.created_at
FROM source_db.users su
WHERE NOT EXISTS (SELECT 1 FROM merge_user_map m WHERE m.source_id = su.id);
INSERT OR IGNORE INTO merge_user_map(source_id, target_id, renamed, added)
SELECT su.id, u.id, 1, 1 FROM source_db.users su
JOIN users u ON u.username = su.username || '-local-' || su.id
WHERE NOT EXISTS (SELECT 1 FROM merge_user_map m WHERE m.source_id = su.id);`, sourceAvatarExpression, sourceAvatarExpression)); err != nil {
		return MergeStats{}, fmt.Errorf("map merge users: %w", err)
	}
	sourceHasHLS, err := databaseColumnExists(ctx, tx, "source_db", "videos", "hls_master_path")
	if err != nil {
		return MergeStats{}, fmt.Errorf("inspect source video columns: %w", err)
	}
	sourceHLSExpression := "''"
	if sourceHasHLS {
		sourceHLSExpression = "v.hls_master_path"
	}
	sourceHasVisibility, err := databaseColumnExists(ctx, tx, "source_db", "videos", "visibility")
	if err != nil {
		return MergeStats{}, fmt.Errorf("inspect source video visibility: %w", err)
	}
	sourceVisibilityExpression := "'public'"
	if sourceHasVisibility {
		sourceVisibilityExpression = `CASE
  WHEN v.visibility IN ('public', 'unlisted', 'private') THEN v.visibility
  ELSE 'private'
END`
	}
	sourceHasProcessingProgress, err := databaseColumnExists(ctx, tx, "source_db", "videos", "processing_progress")
	if err != nil {
		return MergeStats{}, fmt.Errorf("inspect source video processing progress: %w", err)
	}
	sourceProcessingProgressExpression := `CASE v.processing_status
  WHEN 'ready' THEN 100
  WHEN 'processing' THEN 35
  ELSE 0
END`
	if sourceHasProcessingProgress {
		sourceProcessingProgressExpression = "v.processing_progress"
	}
	sourceHasProcessingStage, err := databaseColumnExists(ctx, tx, "source_db", "videos", "processing_stage")
	if err != nil {
		return MergeStats{}, fmt.Errorf("inspect source video processing stage: %w", err)
	}
	sourceProcessingStageExpression := `CASE v.processing_status
  WHEN 'ready' THEN 'ready'
  WHEN 'processing' THEN 'transcoding'
  WHEN 'failed' THEN 'failed'
  ELSE 'queued'
END`
	if sourceHasProcessingStage {
		sourceProcessingStageExpression = "v.processing_stage"
	}
	sourceVideoExpressions := map[string]string{
		"source_width":     "0",
		"source_height":    "0",
		"source_bitrate":   "0",
		"video_codec":      "''",
		"audio_codec":      "''",
		"processing_error": "''",
		"processed_at":     "NULL",
	}
	for column, fallback := range sourceVideoExpressions {
		exists, err := databaseColumnExists(ctx, tx, "source_db", "videos", column)
		if err != nil {
			return MergeStats{}, fmt.Errorf("inspect source video %s: %w", column, err)
		}
		if exists {
			sourceVideoExpressions[column] = "v." + column
		}
		_ = fallback
	}
	sourceHasSubtitles, err := databaseTableExists(ctx, tx, "source_db", "video_subtitles")
	if err != nil {
		return MergeStats{}, fmt.Errorf("inspect source subtitle table: %w", err)
	}
	sourceHasFollows, err := databaseTableExists(ctx, tx, "source_db", "user_follows")
	if err != nil {
		return MergeStats{}, fmt.Errorf("inspect source follows table: %w", err)
	}
	sourceHasTranscodingJobs, err := databaseTableExists(ctx, tx, "source_db", "transcoding_jobs")
	if err != nil {
		return MergeStats{}, fmt.Errorf("inspect source transcoding jobs table: %w", err)
	}
	sourceHasNotifications, err := databaseTableExists(ctx, tx, "source_db", "notifications")
	if err != nil {
		return MergeStats{}, fmt.Errorf("inspect source notifications table: %w", err)
	}
	sourceNotificationActorUsername := "''"
	sourceNotificationActorAvatar := "''"
	if sourceHasNotifications {
		if exists, err := databaseColumnExists(ctx, tx, "source_db", "notifications", "actor_username"); err != nil {
			return MergeStats{}, fmt.Errorf("inspect source notification actor username: %w", err)
		} else if exists {
			sourceNotificationActorUsername = "sn.actor_username"
		}
		if exists, err := databaseColumnExists(ctx, tx, "source_db", "notifications", "actor_avatar_path"); err != nil {
			return MergeStats{}, fmt.Errorf("inspect source notification actor avatar: %w", err)
		} else if exists {
			sourceNotificationActorAvatar = "sn.actor_avatar_path"
		}
	}

	statements := []struct {
		name  string
		query string
	}{
		{"sessions", `INSERT OR IGNORE INTO sessions(token_hash, user_id, csrf_token, expires_at, created_at)
SELECT s.token_hash, m.target_id, s.csrf_token, s.expires_at, s.created_at
FROM source_db.sessions s JOIN merge_user_map m ON m.source_id = s.user_id`},
		{"videos", fmt.Sprintf(`INSERT INTO videos(
  user_id, title, description, category, visibility, video_path, hls_master_path, cover_path, mime_type, duration_seconds, size_bytes,
  processing_status, processing_progress, processing_stage, source_width, source_height, source_bitrate, video_codec, audio_codec,
  processing_error, processed_at, views_count, created_at)
SELECT m.target_id, v.title, v.description, v.category, %s, v.video_path, %s, v.cover_path, v.mime_type, v.duration_seconds, v.size_bytes,
  v.processing_status, %s, %s, %s, %s, %s, %s, %s,
  %s, %s, v.views_count, v.created_at
FROM source_db.videos v JOIN merge_user_map m ON m.source_id = v.user_id
WHERE NOT EXISTS (SELECT 1 FROM videos existing WHERE existing.video_path = v.video_path)`,
			sourceVisibilityExpression, sourceHLSExpression, sourceProcessingProgressExpression, sourceProcessingStageExpression,
			sourceVideoExpressions["source_width"], sourceVideoExpressions["source_height"], sourceVideoExpressions["source_bitrate"],
			sourceVideoExpressions["video_codec"], sourceVideoExpressions["audio_codec"], sourceVideoExpressions["processing_error"],
			sourceVideoExpressions["processed_at"])},
		{"subtitles", `INSERT OR IGNORE INTO video_subtitles(video_id, language, label, subtitle_path, is_default, created_at)
SELECT v.id, s.language, s.label, s.subtitle_path, s.is_default, s.created_at
FROM source_db.video_subtitles s
JOIN source_db.videos sv ON sv.id = s.video_id
JOIN videos v ON v.video_path = sv.video_path`},
		{"likes", `INSERT OR IGNORE INTO video_likes(user_id, video_id, created_at)
SELECT m.target_id, v.id, l.created_at FROM source_db.video_likes l
JOIN merge_user_map m ON m.source_id = l.user_id
JOIN source_db.videos sv ON sv.id = l.video_id JOIN videos v ON v.video_path = sv.video_path`},
		{"favorites", `INSERT OR IGNORE INTO video_favorites(user_id, video_id, created_at)
SELECT m.target_id, v.id, f.created_at FROM source_db.video_favorites f
JOIN merge_user_map m ON m.source_id = f.user_id
JOIN source_db.videos sv ON sv.id = f.video_id JOIN videos v ON v.video_path = sv.video_path`},
		{"follows", `INSERT OR IGNORE INTO user_follows(follower_id, followed_id, created_at)
SELECT follower.target_id, followed.target_id, f.created_at FROM source_db.user_follows f
JOIN merge_user_map follower ON follower.source_id = f.follower_id
JOIN merge_user_map followed ON followed.source_id = f.followed_id
WHERE follower.target_id <> followed.target_id`},
		{"comments", `INSERT INTO comments(video_id, user_id, content, created_at)
SELECT v.id, m.target_id, c.content, c.created_at FROM source_db.comments c
JOIN merge_user_map m ON m.source_id = c.user_id
JOIN source_db.videos sv ON sv.id = c.video_id JOIN videos v ON v.video_path = sv.video_path
WHERE NOT EXISTS (SELECT 1 FROM comments existing
  WHERE existing.video_id = v.id AND existing.user_id = m.target_id
    AND existing.content = c.content AND existing.created_at = c.created_at)`},
		{"media_jobs", `INSERT OR IGNORE INTO transcoding_jobs(
  video_id, status, attempts, last_error, available_at, started_at, finished_at, created_at, updated_at)
SELECT v.id, j.status, j.attempts, j.last_error, j.available_at, j.started_at, j.finished_at, j.created_at, j.updated_at
FROM source_db.transcoding_jobs j
JOIN source_db.videos sv ON sv.id = j.video_id JOIN videos v ON v.video_path = sv.video_path`},
		{"notifications", fmt.Sprintf(`INSERT INTO notifications(
  recipient_id, actor_id, actor_username, actor_avatar_path, type, video_id, video_title,
  comment_id, comment_preview, read_at, created_at)
SELECT recipient.target_id, actor.target_id, %s, %s, sn.type, v.id, sn.video_title,
       mapped_comment.id, sn.comment_preview, sn.read_at, sn.created_at
FROM source_db.notifications sn
JOIN merge_user_map recipient ON recipient.source_id = sn.recipient_id
LEFT JOIN merge_user_map actor ON actor.source_id = sn.actor_id
LEFT JOIN source_db.videos sv ON sv.id = sn.video_id
LEFT JOIN videos v ON v.video_path = sv.video_path
LEFT JOIN source_db.comments sc ON sc.id = sn.comment_id
LEFT JOIN comments mapped_comment ON mapped_comment.video_id = v.id
  AND mapped_comment.user_id = (SELECT target_id FROM merge_user_map WHERE source_id = sc.user_id)
  AND mapped_comment.content = sc.content AND mapped_comment.created_at = sc.created_at
WHERE NOT EXISTS (SELECT 1 FROM notifications existing
  WHERE existing.recipient_id = recipient.target_id AND existing.type = sn.type
    AND existing.created_at = sn.created_at
    AND COALESCE(existing.actor_id, 0) = COALESCE(actor.target_id, 0)
    AND COALESCE(existing.video_id, 0) = COALESCE(v.id, 0)
    AND COALESCE(existing.comment_id, 0) = COALESCE(mapped_comment.id, 0)
    AND COALESCE(existing.comment_preview, '') = COALESCE(sn.comment_preview, ''))`, sourceNotificationActorUsername, sourceNotificationActorAvatar)},
	}
	if !sourceHasSubtitles {
		statements = append(statements[:2], statements[3:]...)
	}
	if !sourceHasFollows {
		for index, statement := range statements {
			if statement.name == "follows" {
				statements = append(statements[:index], statements[index+1:]...)
				break
			}
		}
	}
	if !sourceHasTranscodingJobs {
		for index, statement := range statements {
			if statement.name == "media_jobs" {
				statements = append(statements[:index], statements[index+1:]...)
				break
			}
		}
	}
	if !sourceHasNotifications {
		for index, statement := range statements {
			if statement.name == "notifications" {
				statements = append(statements[:index], statements[index+1:]...)
				break
			}
		}
	}
	var stats MergeStats
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM merge_user_map WHERE added = 1`).Scan(&stats.Users); err != nil {
		return MergeStats{}, fmt.Errorf("count added merge users: %w", err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM merge_user_map WHERE renamed = 1`).Scan(&stats.RenamedUsers); err != nil {
		return MergeStats{}, fmt.Errorf("count renamed merge users: %w", err)
	}
	for _, statement := range statements {
		result, err := tx.ExecContext(ctx, statement.query)
		if err != nil {
			return MergeStats{}, fmt.Errorf("merge %s: %w", statement.name, err)
		}
		changed, _ := result.RowsAffected()
		switch statement.name {
		case "sessions":
			stats.Sessions = changed
		case "videos":
			stats.Videos = changed
		case "subtitles":
			stats.Subtitles = changed
		case "comments":
			stats.Comments = changed
		case "likes":
			stats.Likes = changed
		case "favorites":
			stats.Favorites = changed
		case "follows":
			stats.Follows = changed
		case "media_jobs":
			stats.MediaJobs = changed
		case "notifications":
			stats.Notifications = changed
		}
	}
	if err := tx.Commit(); err != nil {
		return MergeStats{}, fmt.Errorf("commit database merge: %w", err)
	}
	return stats, nil
}

type queryContext interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func databaseColumnExists(ctx context.Context, db queryContext, schema, table, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA `+schema+`.table_info(`+table+`)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var columnName, columnType string
		var notNull, primaryKey int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if columnName == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func databaseTableExists(ctx context.Context, db queryContext, schema, table string) (bool, error) {
	var exists int
	err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM `+schema+`.sqlite_master WHERE type = 'table' AND name = ?)`, table).Scan(&exists)
	return exists == 1, err
}
