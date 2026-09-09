-- Frozen pre-migration schema fixture derived from the earliest published GVideo schema.
-- It intentionally has no schema_migrations ledger and omits columns added later.
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL COLLATE NOCASE UNIQUE,
  password_hash TEXT NOT NULL,
  bio TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE videos (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  category TEXT NOT NULL,
  video_path TEXT NOT NULL,
  cover_path TEXT NOT NULL DEFAULT '',
  mime_type TEXT NOT NULL,
  duration_seconds REAL NOT NULL DEFAULT 0,
  size_bytes INTEGER NOT NULL,
  processing_status TEXT NOT NULL DEFAULT 'ready',
  views_count INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users(id, username, password_hash, bio)
VALUES (41, 'legacy-fixture', 'legacy-hash', 'preserve this profile');

INSERT INTO videos(id, user_id, title, category, video_path, mime_type, size_bytes, processing_status) VALUES
  (101, 41, 'Ready legacy video', 'knowledge', 'videos/legacy-ready.mp4', 'video/mp4', 101, 'ready'),
  (102, 41, 'Pending legacy video', 'knowledge', 'videos/legacy-pending.mp4', 'video/mp4', 102, 'pending'),
  (103, 41, 'Processing legacy video', 'knowledge', 'videos/legacy-processing.mp4', 'video/mp4', 103, 'processing'),
  (104, 41, 'Failed legacy video', 'knowledge', 'videos/legacy-failed.mp4', 'video/mp4', 104, 'failed');
