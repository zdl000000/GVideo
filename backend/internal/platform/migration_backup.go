package platform

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const migrationBackupDirectory = ".migration-backups"

var (
	migrationBackupNow    = time.Now
	migrationBackupSuffix = randomMigrationBackupSuffix
)

func createMigrationBackup(ctx context.Context, db *sql.DB, databasePath string, targetVersion int) (string, error) {
	absoluteDatabasePath, err := filepath.Abs(databasePath)
	if err != nil {
		return "", fmt.Errorf("resolve database path: %w", err)
	}
	backupDirectory := filepath.Join(filepath.Dir(absoluteDatabasePath), migrationBackupDirectory)
	if err := os.MkdirAll(backupDirectory, 0o700); err != nil {
		return "", fmt.Errorf("create migration backup directory: %w", err)
	}
	if err := os.Chmod(backupDirectory, 0o700); err != nil {
		return "", fmt.Errorf("secure migration backup directory: %w", err)
	}

	suffix, err := migrationBackupSuffix()
	if err != nil {
		return "", err
	}
	timestamp := migrationBackupNow().UTC().Format("20060102T150405.000000000Z")
	finalPath := filepath.Join(backupDirectory, fmt.Sprintf("gvideo-pre-migration-v%d-%s-%s.db", targetVersion, timestamp, suffix))
	partialPath := finalPath + ".partial"
	partialCreated := false
	defer func() {
		if partialCreated {
			_ = os.Remove(partialPath)
		}
	}()

	if err := vacuumDatabaseInto(ctx, db, partialPath); err != nil {
		return "", err
	}
	partialCreated = true
	if err := os.Chmod(partialPath, 0o600); err != nil {
		return "", fmt.Errorf("secure migration backup: %w", err)
	}
	if err := verifySQLiteSnapshot(ctx, partialPath); err != nil {
		return "", err
	}
	if err := os.Link(partialPath, finalPath); err != nil {
		return "", fmt.Errorf("publish migration backup without replacement: %w", err)
	}
	if err := os.Remove(partialPath); err != nil {
		return "", fmt.Errorf("remove published migration backup partial: %w", err)
	}
	partialCreated = false
	return finalPath, nil
}

func randomMigrationBackupSuffix() (string, error) {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate migration backup name: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}

func verifySQLiteSnapshot(ctx context.Context, path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve migration backup path: %w", err)
	}
	dsn := "file:" + filepath.ToSlash(absolute) + "?mode=ro&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	backup, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open migration backup for verification: %w", err)
	}
	backup.SetMaxOpenConns(1)
	defer backup.Close()

	var integrity string
	if err := backup.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return fmt.Errorf("verify migration backup integrity: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(integrity), "ok") {
		return fmt.Errorf("verify migration backup integrity: %s", integrity)
	}

	rows, err := backup.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("verify migration backup foreign keys: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("verify migration backup foreign keys: violation found")
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("verify migration backup foreign keys: %w", err)
	}
	return nil
}
