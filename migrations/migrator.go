package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
)

const advisoryLockKey int64 = 647348721451091244
const trackingTableName = "luck_mcp_schema_migrations"

type migrationFile struct {
	Version  string
	SQL      string
	Checksum string
}

type Result struct {
	Applied []string
	Skipped int
}

type Runner struct {
	db         *sql.DB
	logger     *slog.Logger
	filesystem fs.FS
}

func NewRunner(db *sql.DB, logger *slog.Logger) *Runner {
	return NewRunnerWithFS(db, logger, files)
}

func NewRunnerWithFS(db *sql.DB, logger *slog.Logger, filesystem fs.FS) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	if filesystem == nil {
		filesystem = files
	}
	return &Runner{
		db:         db,
		logger:     logger,
		filesystem: filesystem,
	}
}

func (r *Runner) Run(ctx context.Context) (Result, error) {
	if r.db == nil {
		return Result{}, fmt.Errorf("migration runner requires a database")
	}

	if err := r.lock(ctx); err != nil {
		return Result{}, err
	}
	defer func() {
		if err := r.unlock(context.Background()); err != nil {
			r.logger.Error("failed to release migration lock", slog.String("error", err.Error()))
		}
	}()

	if err := r.ensureTrackingTable(ctx); err != nil {
		return Result{}, err
	}

	migrations, err := loadMigrationFiles(r.filesystem)
	if err != nil {
		return Result{}, err
	}

	applied, err := r.listApplied(ctx)
	if err != nil {
		return Result{}, err
	}

	result := Result{Applied: make([]string, 0, len(migrations))}
	for _, migration := range migrations {
		if checksum, ok := applied[migration.Version]; ok {
			if checksum != migration.Checksum {
				return result, fmt.Errorf("migration %s checksum mismatch: migration file changed after apply", migration.Version)
			}
			result.Skipped++
			continue
		}

		if err := r.applyOne(ctx, migration); err != nil {
			return result, err
		}
		result.Applied = append(result.Applied, migration.Version)
	}

	r.logger.Info("schema migrations checked",
		slog.Int("applied", len(result.Applied)),
		slog.Int("skipped", result.Skipped),
	)
	return result, nil
}

func (r *Runner) lock(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, advisoryLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	return nil
}

func (r *Runner) unlock(ctx context.Context) error {
	if _, err := r.db.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, advisoryLockKey); err != nil {
		return fmt.Errorf("release migration lock: %w", err)
	}
	return nil
}

func (r *Runner) ensureTrackingTable(ctx context.Context) error {
	const q = `
CREATE TABLE IF NOT EXISTS ` + trackingTableName + ` (
	version TEXT PRIMARY KEY,
	checksum TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`
	if _, err := r.db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func (r *Runner) listApplied(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT version, checksum
FROM `+trackingTableName+`
ORDER BY version`)
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]string)
	for rows.Next() {
		var version string
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}
	return applied, nil
}

func (r *Runner) applyOne(ctx context.Context, migration migrationFile) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", migration.Version, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("apply migration %s: %w", migration.Version, err)
	}

	if _, err := tx.ExecContext(ctx, `
INSERT INTO `+trackingTableName+` (version, checksum)
VALUES ($1, $2)`, migration.Version, migration.Checksum); err != nil {
		return fmt.Errorf("record migration %s: %w", migration.Version, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", migration.Version, err)
	}
	return nil
}

func loadMigrationFiles(filesystem fs.FS) ([]migrationFile, error) {
	paths, err := fs.Glob(filesystem, "*.up.sql")
	if err != nil {
		return nil, fmt.Errorf("glob migration files: %w", err)
	}
	sort.Strings(paths)

	migrations := make([]migrationFile, 0, len(paths))
	for _, path := range paths {
		body, err := fs.ReadFile(filesystem, path)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", path, err)
		}
		checksum := sha256.Sum256(body)
		migrations = append(migrations, migrationFile{
			Version:  path,
			SQL:      string(body),
			Checksum: hex.EncodeToString(checksum[:]),
		})
	}
	return migrations, nil
}
