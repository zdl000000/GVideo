package platform

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestMigrationBackupNewDatabaseAndCurrentVersionDoNotBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gvideo.db")
	calls := 0
	backup := func(context.Context, *sql.DB, string, int) (string, error) {
		calls++
		return "", errors.New("unexpected backup")
	}
	db, err := openDatabase(path, databaseMigrations, backup)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("new database backups = %d", calls)
	}
	assertNoMigrationBackupDirectory(t, path)
	db, err = openDatabase(path, databaseMigrations, backup)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("current database backups = %d", calls)
	}
	assertNoMigrationBackupDirectory(t, path)
}

func TestMigrationBackupLegacyDatabaseBeforeUpgrade(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "migrations", "legacy-v0.sql"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(string(fixture)); err != nil {
		legacy.Close()
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	upgraded, err := OpenDatabase(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := upgraded.Close(); err != nil {
		t.Fatal(err)
	}
	backups := migrationBackupFiles(t, path)
	if len(backups) != 1 {
		t.Fatalf("migration backups = %v", backups)
	}
	backup, err := OpenDatabaseReadOnly(backups[0])
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var ledger int
	if err := backup.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='schema_migrations')`).Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	if ledger != 0 {
		t.Fatal("pre-migration backup contains migration ledger")
	}
	var user string
	if err := backup.QueryRow(`SELECT username FROM users WHERE id = 41`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	if user != "legacy-fixture" {
		t.Fatalf("backup username = %q", user)
	}
	var avatarColumn int
	if err := backup.QueryRow(`SELECT EXISTS(SELECT 1 FROM pragma_table_info('users') WHERE name='avatar_path')`).Scan(&avatarColumn); err != nil {
		t.Fatal(err)
	}
	if avatarColumn != 0 {
		t.Fatal("pre-migration backup contains upgraded avatar_path column")
	}
}

func TestMigrationBackupIncludesCommittedWALData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.db")
	db := openRawMigrationDatabase(t, path)
	if _, err := db.Exec(`PRAGMA wal_autocheckpoint=0; CREATE TABLE legacy_items(id INTEGER PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO legacy_items(id, value) VALUES (1, 'committed-in-wal')`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	var busy, logFrames, checkpointed int
	if err := db.QueryRow(`PRAGMA wal_checkpoint(PASSIVE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO legacy_items(id, value) VALUES (2, 'after-checkpoint')`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	backupPath, err := createMigrationBackup(t.Context(), db, path, 1)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	backup, err := OpenDatabaseReadOnly(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	var value string
	if err := backup.QueryRow(`SELECT value FROM legacy_items WHERE id = 2`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "after-checkpoint" {
		t.Fatalf("WAL value = %q", value)
	}
}

func TestMigrationBackupFailurePreventsMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "failure.db")
	seedPersistentDatabase(t, path)
	migrationCalls := 0
	migrations := []databaseMigration{{version: 1, identity: "backup-failure-gate", apply: func(*sql.Tx) error { migrationCalls++; return nil }}}
	backupError := errors.New("disk full")
	opened, err := openDatabase(path, migrations, func(context.Context, *sql.DB, string, int) (string, error) { return "", backupError })
	if opened != nil {
		opened.Close()
		t.Fatal("openDatabase returned a handle after backup failure")
	}
	if !errors.Is(err, backupError) {
		t.Fatalf("error = %v", err)
	}
	if migrationCalls != 0 {
		t.Fatalf("migration calls = %d", migrationCalls)
	}
	assertTableAbsent(t, path, "schema_migrations")
}

func TestMigrationBackupMigrationFailureKeepsPublishedBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migration-failure.db")
	seedPersistentDatabase(t, path)
	migrationError := errors.New("migration failed")
	migrations := []databaseMigration{{version: 1, identity: "failing-migration", apply: func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE should_rollback(id INTEGER)`); err != nil {
			return err
		}
		return migrationError
	}}}
	opened, err := openDatabase(path, migrations, createMigrationBackup)
	if opened != nil {
		opened.Close()
		t.Fatal("openDatabase returned a handle after migration failure")
	}
	if !errors.Is(err, migrationError) {
		t.Fatalf("error = %v", err)
	}
	backups := migrationBackupFiles(t, path)
	if len(backups) != 1 {
		t.Fatalf("published backups = %v", backups)
	}
	if err := verifySQLiteSnapshot(t.Context(), backups[0]); err != nil {
		t.Fatalf("verify retained backup: %v", err)
	}
	assertTableAbsent(t, path, "should_rollback")
	assertTableAbsent(t, path, "schema_migrations")
}

func TestMigrationBackupRejectsInvalidLedgersWithoutBackupOrChange(t *testing.T) {
	migrations := []databaseMigration{
		{version: 1, identity: "one", apply: func(*sql.Tx) error { return nil }},
		{version: 2, identity: "two", apply: func(*sql.Tx) error { return nil }},
	}
	tests := []struct{ name, ledgerRows, wantError string }{
		{name: "checksum", ledgerRows: `INSERT INTO schema_migrations(version, checksum) VALUES (1, 'wrong')`, wantError: "checksum mismatch"},
		{name: "future", ledgerRows: `INSERT INTO schema_migrations(version, checksum) VALUES (999, 'future')`, wantError: "newer than this application"},
		{name: "gap", ledgerRows: fmt.Sprintf(`INSERT INTO schema_migrations(version, checksum) VALUES (2, '%s')`, migrationChecksum(migrations[1])), wantError: "ledger has a gap"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "invalid.db")
			db := openRawMigrationDatabase(t, path)
			if _, err := db.Exec(`CREATE TABLE persistent(id INTEGER PRIMARY KEY, value TEXT); INSERT INTO persistent VALUES (1, 'sentinel'); CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, checksum TEXT NOT NULL); ` + test.ledgerRows); err != nil {
				db.Close()
				t.Fatal(err)
			}
			before := invalidLedgerSnapshot(t, db)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			backupCalls := 0
			opened, err := openDatabase(path, migrations, func(context.Context, *sql.DB, string, int) (string, error) { backupCalls++; return "", nil })
			if opened != nil {
				opened.Close()
				t.Fatal("openDatabase returned a handle for invalid ledger")
			}
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v", err)
			}
			if backupCalls != 0 {
				t.Fatalf("backup calls = %d", backupCalls)
			}
			db = openRawMigrationDatabase(t, path)
			after := invalidLedgerSnapshot(t, db)
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Fatalf("database changed: before=%q after=%q", before, after)
			}
			assertNoMigrationBackupDirectory(t, path)
		})
	}
}

func TestMigrationBackupCollisionNeverReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "collision.db")
	db := openRawMigrationDatabase(t, path)
	if _, err := db.Exec(`CREATE TABLE persistent(id INTEGER PRIMARY KEY); INSERT INTO persistent VALUES (1)`); err != nil {
		db.Close()
		t.Fatal(err)
	}

	previousNow := migrationBackupNow
	previousSuffix := migrationBackupSuffix
	migrationBackupNow = func() time.Time {
		return time.Date(2026, time.September, 10, 0, 47, 0, 123456789, time.UTC)
	}
	migrationBackupSuffix = func() (string, error) {
		return "fixed-collision", nil
	}
	defer func() {
		migrationBackupNow = previousNow
		migrationBackupSuffix = previousSuffix
	}()

	backupDirectory := filepath.Join(filepath.Dir(path), migrationBackupDirectory)
	if err := os.MkdirAll(backupDirectory, 0o700); err != nil {
		db.Close()
		t.Fatal(err)
	}
	finalPath := filepath.Join(backupDirectory, "gvideo-pre-migration-v2-20260910T004700.123456789Z-fixed-collision.db")
	original := []byte("existing backup must remain unchanged")
	if err := os.WriteFile(finalPath, original, 0o600); err != nil {
		db.Close()
		t.Fatal(err)
	}

	published, err := createMigrationBackup(t.Context(), db, path, 2)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err == nil {
		t.Fatalf("collision unexpectedly published %q", published)
	}
	if !strings.Contains(err.Error(), "publish migration backup without replacement") {
		t.Fatalf("collision error = %v", err)
	}
	got, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("existing backup changed: got %q want %q", got, original)
	}
	if _, err := os.Stat(finalPath + ".partial"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("collision partial remains or cannot be inspected: %v", err)
	}
}
func TestMigrationBackupNamesDoNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "names.db")
	db := openRawMigrationDatabase(t, path)
	if _, err := db.Exec(`CREATE TABLE persistent(id INTEGER PRIMARY KEY); INSERT INTO persistent VALUES (1)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	first, err := createMigrationBackup(t.Context(), db, path, 2)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	second, err := createMigrationBackup(t.Context(), db, path, 2)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("backup path reused: %s", first)
	}
	for _, path := range []string{first, second} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() == 0 {
			t.Fatalf("empty backup: %s", path)
		}
		if strings.ContainsAny(filepath.Base(path), `:<>"/\\|?*`) {
			t.Fatalf("Windows-incompatible backup name: %s", filepath.Base(path))
		}
	}
}

func openRawMigrationDatabase(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		t.Fatal(err)
	}
	return db
}

func seedPersistentDatabase(t *testing.T, path string) {
	t.Helper()
	db := openRawMigrationDatabase(t, path)
	if _, err := db.Exec(`CREATE TABLE persistent(id INTEGER PRIMARY KEY, value TEXT); INSERT INTO persistent VALUES (1, 'sentinel')`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertTableAbsent(t *testing.T, path, table string) {
	t.Helper()
	db := openRawMigrationDatabase(t, path)
	defer db.Close()
	var exists int
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name=?)`, table).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists != 0 {
		t.Fatalf("table %q exists", table)
	}
}

func migrationBackupFiles(t *testing.T, databasePath string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(filepath.Dir(databasePath), migrationBackupDirectory, "*.db"))
	if err != nil {
		t.Fatal(err)
	}
	partials, err := filepath.Glob(filepath.Join(filepath.Dir(databasePath), migrationBackupDirectory, "*.partial"))
	if err != nil {
		t.Fatal(err)
	}
	if len(partials) != 0 {
		t.Fatalf("partial backups remain: %v", partials)
	}
	return files
}

func assertNoMigrationBackupDirectory(t *testing.T, databasePath string) {
	t.Helper()
	_, err := os.Stat(filepath.Join(filepath.Dir(databasePath), migrationBackupDirectory))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("migration backup directory exists or cannot be inspected: %v", err)
	}
}

func invalidLedgerSnapshot(t *testing.T, db *sql.DB) string {
	t.Helper()
	var persistent, ledger string
	if err := db.QueryRow(`SELECT group_concat(printf('%d|%s', id, value), ';') FROM persistent`).Scan(&persistent); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT group_concat(printf('%d|%s', version, checksum), ';') FROM schema_migrations`).Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	return persistent + "#" + ledger
}
