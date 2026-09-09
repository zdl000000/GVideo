package platform

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
)

type databaseMigration struct {
	version  int
	identity string
	apply    func(*sql.Tx) error
}

var databaseMigrations = []databaseMigration{
	{
		version:  1,
		identity: "initial-schema-and-legacy-column-backfills-v1\n" + baselineSchemaV1SQL,
		apply: func(tx *sql.Tx) error {
			return migrateBaselineV1(tx)
		},
	},
}

func migrate(db *sql.DB) error {
	return runMigrations(db, databaseMigrations)
}

func runMigrations(db *sql.DB, migrations []databaseMigration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin database migration: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  version INTEGER PRIMARY KEY,
  applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  checksum TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("create schema migration ledger: %w", err)
	}

	known := make(map[int]string, len(migrations))
	lastVersion := 0
	for _, migration := range migrations {
		if migration.version <= 0 || migration.identity == "" || migration.apply == nil {
			return fmt.Errorf("invalid database migration version %d", migration.version)
		}
		if migration.version <= lastVersion {
			return fmt.Errorf("database migrations must be strictly increasing: version %d follows %d", migration.version, lastVersion)
		}
		known[migration.version] = migrationChecksum(migration)
		lastVersion = migration.version
	}

	rows, err := tx.Query(`SELECT version, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("read schema migration ledger: %w", err)
	}
	applied := make(map[int]string)
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			rows.Close()
			return fmt.Errorf("scan schema migration ledger: %w", err)
		}
		expected, exists := known[version]
		if !exists {
			rows.Close()
			return fmt.Errorf("database schema migration %d is newer than this application", version)
		}
		if checksum != expected {
			rows.Close()
			return fmt.Errorf("database schema migration %d checksum mismatch", version)
		}
		applied[version] = checksum
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close schema migration ledger: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate schema migration ledger: %w", err)
	}
	missingEarlierVersion := false
	for _, migration := range migrations {
		_, exists := applied[migration.version]
		if !exists {
			missingEarlierVersion = true
			continue
		}
		if missingEarlierVersion {
			return fmt.Errorf("database schema migration ledger has a gap before version %d", migration.version)
		}
	}

	for _, migration := range migrations {
		if _, exists := applied[migration.version]; exists {
			continue
		}
		if err := migration.apply(tx); err != nil {
			return fmt.Errorf("apply database migration %d: %w", migration.version, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, checksum) VALUES (?, ?)`, migration.version, known[migration.version]); err != nil {
			return fmt.Errorf("record database migration %d: %w", migration.version, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database migrations: %w", err)
	}
	committed = true
	return nil
}

func migrationChecksum(migration databaseMigration) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s", migration.version, migration.identity)))
	return hex.EncodeToString(sum[:])
}
