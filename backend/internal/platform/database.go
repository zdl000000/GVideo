package platform

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type DatabaseStats struct {
	Users     int64
	Sessions  int64
	Videos    int64
	Subtitles int64
	Comments  int64
	Likes     int64
	Favorites int64
	Follows   int64
	MediaJobs int64
}

type MergeStats struct {
	Users        int64
	RenamedUsers int64
	Sessions     int64
	Videos       int64
	Subtitles    int64
	Comments     int64
	Likes        int64
	Favorites    int64
	Follows      int64
	MediaJobs    int64
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

func migrate(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL COLLATE NOCASE UNIQUE,
  password_hash TEXT NOT NULL,
  bio TEXT NOT NULL DEFAULT '',
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
		name       string
		definition string
	}{
		{"processing_status", "TEXT NOT NULL DEFAULT 'ready'"},
		{"hls_master_path", "TEXT NOT NULL DEFAULT ''"},
		{"source_width", "INTEGER NOT NULL DEFAULT 0"},
		{"source_height", "INTEGER NOT NULL DEFAULT 0"},
		{"source_bitrate", "INTEGER NOT NULL DEFAULT 0"},
		{"video_codec", "TEXT NOT NULL DEFAULT ''"},
		{"audio_codec", "TEXT NOT NULL DEFAULT ''"},
		{"processing_error", "TEXT NOT NULL DEFAULT ''"},
		{"processed_at", "DATETIME"},
	}
	for _, column := range columns {
		if err := ensureColumn(db, "videos", column.name, column.definition); err != nil {
			return fmt.Errorf("migrate videos.%s: %w", column.name, err)
		}
	}
	return nil
}

func ensureColumn(db *sql.DB, table, name, definition string) error {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var columnName, columnType string
		var notNull, primaryKey int
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if columnName == name {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + name + ` ` + definition)
	return err
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
	for _, query := range queries {
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+query.name).Scan(query.target); err != nil {
			return DatabaseStats{}, fmt.Errorf("count %s: %w", query.name, err)
		}
	}
	return stats, nil
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
	if _, err := tx.ExecContext(ctx, `
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
INSERT OR IGNORE INTO users(username, password_hash, bio, created_at)
SELECT su.username, su.password_hash, su.bio, su.created_at
FROM source_db.users su
WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.username = su.username);
INSERT OR IGNORE INTO merge_user_map(source_id, target_id, added)
SELECT su.id, u.id, 1 FROM source_db.users su
JOIN users u ON u.username = su.username AND u.password_hash = su.password_hash;
INSERT OR IGNORE INTO users(username, password_hash, bio, created_at)
SELECT su.username || '-local-' || su.id, su.password_hash, su.bio, su.created_at
FROM source_db.users su
WHERE NOT EXISTS (SELECT 1 FROM merge_user_map m WHERE m.source_id = su.id);
INSERT OR IGNORE INTO merge_user_map(source_id, target_id, renamed, added)
SELECT su.id, u.id, 1, 1 FROM source_db.users su
JOIN users u ON u.username = su.username || '-local-' || su.id
WHERE NOT EXISTS (SELECT 1 FROM merge_user_map m WHERE m.source_id = su.id);`); err != nil {
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
	sourceHasSubtitles, err := databaseTableExists(ctx, tx, "source_db", "video_subtitles")
	if err != nil {
		return MergeStats{}, fmt.Errorf("inspect source subtitle table: %w", err)
	}
	sourceHasFollows, err := databaseTableExists(ctx, tx, "source_db", "user_follows")
	if err != nil {
		return MergeStats{}, fmt.Errorf("inspect source follows table: %w", err)
	}

	statements := []struct {
		name  string
		query string
	}{
		{"sessions", `INSERT OR IGNORE INTO sessions(token_hash, user_id, csrf_token, expires_at, created_at)
SELECT s.token_hash, m.target_id, s.csrf_token, s.expires_at, s.created_at
FROM source_db.sessions s JOIN merge_user_map m ON m.source_id = s.user_id`},
		{"videos", fmt.Sprintf(`INSERT INTO videos(
  user_id, title, description, category, video_path, hls_master_path, cover_path, mime_type, duration_seconds, size_bytes,
  processing_status, source_width, source_height, source_bitrate, video_codec, audio_codec,
  processing_error, processed_at, views_count, created_at)
SELECT m.target_id, v.title, v.description, v.category, v.video_path, %s, v.cover_path, v.mime_type, v.duration_seconds, v.size_bytes,
  v.processing_status, v.source_width, v.source_height, v.source_bitrate, v.video_codec, v.audio_codec,
  v.processing_error, v.processed_at, v.views_count, v.created_at
FROM source_db.videos v JOIN merge_user_map m ON m.source_id = v.user_id
WHERE NOT EXISTS (SELECT 1 FROM videos existing WHERE existing.video_path = v.video_path)`, sourceHLSExpression)},
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
