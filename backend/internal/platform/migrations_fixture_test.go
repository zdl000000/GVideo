package platform

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrationFixturesEmptyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("database exists before opening: %v", err)
	}

	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		db.Close()
		t.Fatalf("database was not created: %v", err)
	}

	wantTables := []string{
		"users", "sessions", "videos", "video_subtitles", "video_likes",
		"video_favorites", "user_follows", "comments", "video_reports",
		"notifications", "transcoding_jobs",
	}
	for _, table := range wantTables {
		var exists int
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?)`, table).Scan(&exists); err != nil {
			db.Close()
			t.Fatal(err)
		}
		if exists != 1 {
			db.Close()
			t.Errorf("expected table %q to exist", table)
		}
	}

	assertMigrationLedger(t, db, []ledgerEntry{{version: 1, checksum: migrationChecksum(databaseMigrations[0])}, {version: 2, checksum: migrationChecksum(databaseMigrations[1])}, {version: 3, checksum: migrationChecksum(databaseMigrations[2])}})
	if _, err := db.Exec(`INSERT INTO users(id, username, password_hash) VALUES (7, 'empty-fixture', 'hash')`); err != nil {
		db.Close()
		t.Fatalf("write to migrated schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = OpenDatabase(path)
	if err != nil {
		t.Fatalf("reopen empty fixture: %v", err)
	}
	defer db.Close()
	assertMigrationLedger(t, db, []ledgerEntry{{version: 1, checksum: migrationChecksum(databaseMigrations[0])}, {version: 2, checksum: migrationChecksum(databaseMigrations[1])}, {version: 3, checksum: migrationChecksum(databaseMigrations[2])}})
	var username string
	if err := db.QueryRow(`SELECT username FROM users WHERE id = 7`).Scan(&username); err != nil {
		t.Fatalf("read data after reopen: %v", err)
	}
	if username != "empty-fixture" {
		t.Fatalf("username after reopen = %q", username)
	}
}

func TestMigrationFixturesLegacyDatabase(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "migrations", "legacy-v0.sql"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy-v0.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(string(fixture)); err != nil {
		legacy.Close()
		t.Fatalf("create legacy fixture: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}

	var username, bio string
	if err := upgraded.QueryRow(`SELECT username, bio FROM users WHERE id = 41`).Scan(&username, &bio); err != nil {
		upgraded.Close()
		t.Fatalf("read preserved legacy user: %v", err)
	}
	if username != "legacy-fixture" || bio != "preserve this profile" {
		upgraded.Close()
		t.Fatalf("legacy user changed: username=%q bio=%q", username, bio)
	}

	wantColumns := map[string][]string{
		"users": {"avatar_path", "is_admin"},
		"videos": {
			"processing_status", "processing_progress", "processing_stage", "visibility",
			"hls_master_path", "source_width", "source_height", "source_bitrate",
			"video_codec", "audio_codec", "processing_error", "processed_at",
			"cover_size_bytes", "hls_size_bytes",
		},
	}
	for table, columns := range wantColumns {
		for _, column := range columns {
			exists, err := databaseColumnExists(t.Context(), upgraded, "main", table, column)
			if err != nil {
				upgraded.Close()
				t.Fatal(err)
			}
			if !exists {
				upgraded.Close()
				t.Errorf("expected migrated column %s.%s", table, column)
			}
		}
	}

	type processingState struct {
		status   string
		progress int
		stage    string
	}
	wantStates := map[int64]processingState{
		101: {status: "ready", progress: 100, stage: "ready"},
		102: {status: "pending", progress: 0, stage: "queued"},
		103: {status: "processing", progress: 35, stage: "transcoding"},
		104: {status: "failed", progress: 0, stage: "failed"},
	}
	for id, want := range wantStates {
		var title, pathValue, status, stage, visibility string
		var size, progress int
		if err := upgraded.QueryRow(`
SELECT title, video_path, size_bytes, processing_status, processing_progress, processing_stage, visibility
FROM videos WHERE id = ?`, id).Scan(&title, &pathValue, &size, &status, &progress, &stage, &visibility); err != nil {
			upgraded.Close()
			t.Fatal(err)
		}
		if title == "" || pathValue == "" || size != int(id) || status != want.status || progress != want.progress || stage != want.stage || visibility != "public" {
			upgraded.Close()
			t.Errorf("video %d changed or backfill is wrong: title=%q path=%q size=%d status=%q progress=%d stage=%q visibility=%q",
				id, title, pathValue, size, status, progress, stage, visibility)
		}
	}
	assertMigrationLedger(t, upgraded, []ledgerEntry{{version: 1, checksum: migrationChecksum(databaseMigrations[0])}, {version: 2, checksum: migrationChecksum(databaseMigrations[1])}, {version: 3, checksum: migrationChecksum(databaseMigrations[2])}})
	if _, err := upgraded.Exec(`UPDATE videos SET processing_progress = 72, processing_stage = 'finalizing' WHERE id = 104`); err != nil {
		upgraded.Close()
		t.Fatal(err)
	}
	if err := upgraded.Close(); err != nil {
		t.Fatal(err)
	}

	upgraded, err = OpenDatabase(path)
	if err != nil {
		t.Fatalf("reopen upgraded fixture: %v", err)
	}
	defer upgraded.Close()
	var progress int
	var stage string
	if err := upgraded.QueryRow(`SELECT processing_progress, processing_stage FROM videos WHERE id = 104`).Scan(&progress, &stage); err != nil {
		t.Fatal(err)
	}
	if progress != 72 || stage != "finalizing" {
		t.Fatalf("reopen repeated backfill: progress=%d stage=%q", progress, stage)
	}
	assertMigrationLedger(t, upgraded, []ledgerEntry{{version: 1, checksum: migrationChecksum(databaseMigrations[0])}, {version: 2, checksum: migrationChecksum(databaseMigrations[1])}, {version: 3, checksum: migrationChecksum(databaseMigrations[2])}})
}

func TestMigrationFixturesFailureRollsBackNextVersion(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "rollback-next.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	v1 := databaseMigration{
		version:  1,
		identity: "fixture-stable-v1",
		apply: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE stable_state (value TEXT NOT NULL); INSERT INTO stable_state(value) VALUES ('preserved')`)
			return err
		},
	}
	if err := runMigrations(db, []databaseMigration{v1}); err != nil {
		t.Fatalf("apply stable migration: %v", err)
	}

	failure := errors.New("fixture migration failure")
	failingV2 := databaseMigration{
		version:  2,
		identity: "fixture-failing-v2",
		apply: func(tx *sql.Tx) error {
			if _, err := tx.Exec(`CREATE TABLE should_rollback (id INTEGER PRIMARY KEY); UPDATE stable_state SET value = 'changed'`); err != nil {
				return err
			}
			return failure
		},
	}
	err = runMigrations(db, []databaseMigration{v1, failingV2})
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "apply database migration 2") {
		t.Fatalf("migration error = %v", err)
	}

	var value string
	if err := db.QueryRow(`SELECT value FROM stable_state`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "preserved" {
		t.Fatalf("stable migration data = %q", value)
	}
	var rolledBackTable int
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = 'should_rollback')`).Scan(&rolledBackTable); err != nil {
		t.Fatal(err)
	}
	if rolledBackTable != 0 {
		t.Fatal("failed migration table was not rolled back")
	}
	assertMigrationLedger(t, db, []ledgerEntry{{version: 1, checksum: migrationChecksum(v1)}})

	successfulV2 := databaseMigration{
		version:  2,
		identity: failingV2.identity,
		apply: func(tx *sql.Tx) error {
			_, err := tx.Exec(`CREATE TABLE retried_state (value TEXT NOT NULL); INSERT INTO retried_state(value) VALUES ('applied')`)
			return err
		},
	}
	if err := runMigrations(db, []databaseMigration{v1, successfulV2}); err != nil {
		t.Fatalf("retry migration after rollback: %v", err)
	}
	assertMigrationLedger(t, db, []ledgerEntry{
		{version: 1, checksum: migrationChecksum(v1)},
		{version: 2, checksum: migrationChecksum(successfulV2)},
	})
}

func TestMigrationFixturesRejectsUnknownVersionWithoutChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "future.db")
	db, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(id, username, password_hash) VALUES (99, 'future-sentinel', 'hash')`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`INSERT INTO schema_migrations(version, checksum) VALUES (999, 'future-checksum')`); err != nil {
		raw.Close()
		t.Fatal(err)
	}
	before := databaseSnapshot(t, raw)
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}

	opened, err := OpenDatabase(path)
	if opened != nil {
		opened.Close()
		t.Fatal("OpenDatabase returned a handle for a future schema")
	}
	if err == nil || !strings.Contains(err.Error(), "database schema migration 999 is newer than this application") {
		t.Fatalf("future schema error = %v", err)
	}

	raw, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	after := databaseSnapshot(t, raw)
	if before != after {
		t.Fatalf("future schema rejection changed database\nbefore: %s\nafter:  %s", before, after)
	}
	assertMigrationLedger(t, raw, []ledgerEntry{
		{version: 1, checksum: migrationChecksum(databaseMigrations[0])},
		{version: 2, checksum: migrationChecksum(databaseMigrations[1])},
		{version: 3, checksum: migrationChecksum(databaseMigrations[2])},
		{version: 999, checksum: "future-checksum"},
	})
	var username string
	if err := raw.QueryRow(`SELECT username FROM users WHERE id = 99`).Scan(&username); err != nil {
		t.Fatal(err)
	}
	if username != "future-sentinel" {
		t.Fatalf("sentinel user changed to %q", username)
	}
}

type ledgerEntry struct {
	version  int
	checksum string
}

func assertMigrationLedger(t *testing.T, db *sql.DB, want []ledgerEntry) {
	t.Helper()
	rows, err := db.Query(`SELECT version, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []ledgerEntry
	for rows.Next() {
		var entry ledgerEntry
		if err := rows.Scan(&entry.version, &entry.checksum); err != nil {
			t.Fatal(err)
		}
		got = append(got, entry)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("migration ledger = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("migration ledger entry %d = %#v, want %#v", index, got[index], want[index])
		}
	}
}

func databaseSnapshot(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query(`
SELECT type, name, COALESCE(sql, '')
FROM sqlite_master
WHERE name NOT LIKE 'sqlite_%'
ORDER BY type, name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var snapshot strings.Builder
	for rows.Next() {
		var objectType, name, definition string
		if err := rows.Scan(&objectType, &name, &definition); err != nil {
			t.Fatal(err)
		}
		snapshot.WriteString(objectType)
		snapshot.WriteByte('|')
		snapshot.WriteString(name)
		snapshot.WriteByte('|')
		snapshot.WriteString(definition)
		snapshot.WriteByte('\n')
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`SELECT printf('%d|%s|%s', version, checksum, applied_at) FROM schema_migrations ORDER BY version`,
		`SELECT printf('%d|%s|%s|%s|%s|%d', id, username, password_hash, bio, avatar_path, is_admin) FROM users ORDER BY id`,
	} {
		dataRows, err := db.Query(query)
		if err != nil {
			t.Fatal(err)
		}
		for dataRows.Next() {
			var value string
			if err := dataRows.Scan(&value); err != nil {
				dataRows.Close()
				t.Fatal(err)
			}
			snapshot.WriteString(value)
			snapshot.WriteByte('\n')
		}
		if err := dataRows.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return snapshot.String()
}
