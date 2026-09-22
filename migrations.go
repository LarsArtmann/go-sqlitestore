package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/larsartmann/go-error-family"
)

// Migration represents a single versioned schema migration. Each migration
// has a monotonically increasing version number and an Up function that
// applies the schema change. The runner guarantees each migration runs at
// most once per database; the version is recorded in schema_migrations
// after success.
//
// Migrations must be idempotent within themselves as a defense-in-depth
// measure (CREATE TABLE IF NOT EXISTS, AddColumnIfNotExists, etc.),
// because the schema_migrations record is written AFTER Up returns. If the
// process crashes mid-migration, re-running applies the migration again
// safely.
type Migration struct {
	Version     int
	Description string
	Up          func(ctx context.Context, db *sql.DB) error
}

// schemaMigrationsSQL creates the metadata table that tracks applied
// migrations. Created before any migration runs; it is infrastructure,
// not a migration itself.
const schemaMigrationsSQL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    INTEGER PRIMARY KEY,
    applied_at TEXT    NOT NULL
);`

// RunMigrations applies every migration whose Version is NOT yet recorded
// in schema_migrations (membership-based), in ascending version order,
// recording each version AFTER its Up succeeds.
//
// Membership semantics (never "version > max"): an unrecorded version
// applies regardless of where it sits numerically — combined legacy
// databases shared by multiple consumers (state migrations to v10, cache
// migrations owning v11) stay correct even when consumers run in either
// order. A recorded version never re-runs: history is immutable.
//
// To add a migration: append a Migration with the next version number.
// Never modify, reorder, or remove existing migrations.
func RunMigrations(ctx context.Context, db *sql.DB, migrations []Migration) error {
	if _, err := db.ExecContext(ctx, schemaMigrationsSQL); err != nil {
		return errorfamily.WrapInfrastructure(err, "sqlitestore.migrate_meta",
			"failed to create schema_migrations table")
	}

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	ordered := make([]Migration, len(migrations))
	copy(ordered, migrations)
	sortByVersion(ordered)

	for _, migration := range ordered {
		if _, done := applied[migration.Version]; done {
			continue
		}

		slog.Info("sqlitestore: applying migration",
			"version", migration.Version,
			"description", migration.Description,
		)

		if err := migration.Up(ctx, db); err != nil {
			return errorfamily.WrapTransient(err, "sqlitestore.migrate_up",
				fmt.Sprintf("migration %d (%s) failed", migration.Version, migration.Description))
		}

		if _, err := db.ExecContext(ctx,
			"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)",
			migration.Version, time.Now().UTC().Format(time.RFC3339)); err != nil {
			return errorfamily.WrapTransient(err, "sqlitestore.migrate_record",
				fmt.Sprintf("failed to record migration %d", migration.Version))
		}

		slog.Debug("sqlitestore: migration applied",
			"version", migration.Version,
			"description", migration.Description,
		)
	}

	return nil
}

// appliedVersions returns the set of recorded migration versions.
func appliedVersions(ctx context.Context, db *sql.DB) (map[int]struct{}, error) {
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, errorfamily.WrapInfrastructure(err, "sqlitestore.migrate_query",
			"failed to read applied migration versions")
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Debug("sqlitestore: failed to close schema_migrations rows", "error", closeErr)
		}
	}()

	applied := make(map[int]struct{})

	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, errorfamily.WrapTransient(err, "sqlitestore.migrate_scan",
				"failed to scan applied migration version")
		}

		applied[version] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, errorfamily.WrapTransient(err, "sqlitestore.migrate_rows",
			"failed to iterate applied migration versions")
	}

	return applied, nil
}

// sortByVersion orders migrations ascending by version. In-place insertion
// sort: migration lists are tiny (tens), and stability across equal
// versions is meaningless (versions are unique by contract).
func sortByVersion(migrations []Migration) {
	for i := 1; i < len(migrations); i++ {
		for j := i; j > 0 && migrations[j].Version < migrations[j-1].Version; j-- {
			migrations[j], migrations[j-1] = migrations[j-1], migrations[j]
		}
	}
}

// TableExists checks whether a table exists in sqlite_master.
func TableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var count int

	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// ColumnExists checks whether a column exists on a table via PRAGMA
// table_info.
func ColumnExists(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false, err
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Debug("sqlitestore: failed to close PRAGMA table_info rows", "table", table, "error", closeErr)
		}
	}()

	for rows.Next() {
		var (
			cid         int
			name, typ   string
			notNull, pk int
			dflt        sql.NullString
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return false, err
		}

		if name == column {
			return true, nil
		}
	}

	return false, rows.Err()
}

// AddColumnIfNotExists adds a column to an existing table if absent.
// SQLite has no ALTER TABLE ADD COLUMN IF NOT EXISTS, so PRAGMA table_info
// is checked first and "duplicate column name" errors are ignored as a
// race-condition safety net.
func AddColumnIfNotExists(ctx context.Context, db *sql.DB, table, column, defn string) error {
	exists, err := ColumnExists(ctx, db, table, column)
	if err != nil {
		return errorfamily.WrapTransient(err, "sqlitestore.migrate_column",
			fmt.Sprintf("failed to check column %s.%s", table, column))
	}

	if exists {
		return nil
	}

	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, defn)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		if IsDuplicateColumnError(err) {
			return nil
		}

		return errorfamily.WrapTransient(err, "sqlitestore.migrate_column",
			fmt.Sprintf("failed to add column %s.%s", table, column))
	}

	return nil
}

// IsDuplicateColumnError reports whether an ALTER TABLE error is the
// column-already-exists case.
func IsDuplicateColumnError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate column name")
}
