package platform

import (
	"context"
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

type migrationState struct {
	hasPending        bool
	hasPersistentData bool
	targetVersion     int
}

type migrationQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

var databaseMigrations = []databaseMigration{
	{
		version:  1,
		identity: "initial-schema-and-legacy-column-backfills-v1\n" + baselineSchemaV1SQL,
		apply: func(tx *sql.Tx) error {
			return migrateBaselineV1(tx)
		},
	},
	{
		version:  2,
		identity: "users-is-admin-flag-v2",
		apply: func(tx *sql.Tx) error {
			_, err := ensureColumn(tx, "users", "is_admin", "INTEGER NOT NULL DEFAULT 0")
			return err
		},
	},
}

func migrate(db *sql.DB) error {
	return runMigrations(db, databaseMigrations)
}

func inspectMigrationState(ctx context.Context, db *sql.DB, migrations []databaseMigration) (migrationState, error) {
	known, targetVersion, err := validateDatabaseMigrations(migrations)
	if err != nil {
		return migrationState{}, err
	}

	var ledgerExists int
	if err := db.QueryRowContext(ctx, `
SELECT EXISTS(
  SELECT 1 FROM sqlite_schema
  WHERE type = 'table' AND name = 'schema_migrations'
)`).Scan(&ledgerExists); err != nil {
		return migrationState{}, fmt.Errorf("inspect schema migration ledger: %w", err)
	}

	applied := make(map[int]string)
	if ledgerExists == 1 {
		applied, err = readAppliedMigrations(ctx, db, known)
		if err != nil {
			return migrationState{}, err
		}
		if err := validateAppliedMigrations(migrations, applied); err != nil {
			return migrationState{}, err
		}
	}

	var persistentTables int
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sqlite_schema
WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&persistentTables); err != nil {
		return migrationState{}, fmt.Errorf("inspect persistent database state: %w", err)
	}

	return migrationState{
		hasPending:        len(applied) < len(migrations),
		hasPersistentData: persistentTables > 0,
		targetVersion:     targetVersion,
	}, nil
}

func runMigrations(db *sql.DB, migrations []databaseMigration) error {
	known, _, err := validateDatabaseMigrations(migrations)
	if err != nil {
		return err
	}

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

	applied, err := readAppliedMigrations(context.Background(), tx, known)
	if err != nil {
		return err
	}
	if err := validateAppliedMigrations(migrations, applied); err != nil {
		return err
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

func validateDatabaseMigrations(migrations []databaseMigration) (map[int]string, int, error) {
	known := make(map[int]string, len(migrations))
	lastVersion := 0
	for _, migration := range migrations {
		if migration.version <= 0 || migration.identity == "" || migration.apply == nil {
			return nil, 0, fmt.Errorf("invalid database migration version %d", migration.version)
		}
		if migration.version <= lastVersion {
			return nil, 0, fmt.Errorf("database migrations must be strictly increasing: version %d follows %d", migration.version, lastVersion)
		}
		known[migration.version] = migrationChecksum(migration)
		lastVersion = migration.version
	}
	return known, lastVersion, nil
}

func readAppliedMigrations(ctx context.Context, queryer migrationQueryer, known map[int]string) (map[int]string, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("read schema migration ledger: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]string)
	for rows.Next() {
		var version int
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("scan schema migration ledger: %w", err)
		}
		expected, exists := known[version]
		if !exists {
			return nil, fmt.Errorf("database schema migration %d is newer than this application", version)
		}
		if checksum != expected {
			return nil, fmt.Errorf("database schema migration %d checksum mismatch", version)
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema migration ledger: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close schema migration ledger: %w", err)
	}
	return applied, nil
}

func validateAppliedMigrations(migrations []databaseMigration, applied map[int]string) error {
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
	return nil
}

func migrationChecksum(migration databaseMigration) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s", migration.version, migration.identity)))
	return hex.EncodeToString(sum[:])
}
