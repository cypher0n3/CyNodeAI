package database

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
)

// MigrationsFS is set at runtime by the main package with embedded migrations.
var MigrationsFS fs.FS

// RunMigrations runs all SQL migrations in order.
func (db *DB) RunMigrations(ctx context.Context, logger *slog.Logger) error {
	if MigrationsFS == nil {
		return fmt.Errorf("migrations filesystem not set")
	}

	if err := db.createMigrationsTable(ctx); err != nil {
		return err
	}

	applied, err := db.getAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	files, err := db.getMigrationFiles()
	if err != nil {
		return err
	}

	return db.applyPendingMigrations(ctx, logger, files, applied)
}

func (db *DB) createMigrationsTable(ctx context.Context) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}
	return nil
}

func (db *DB) getAppliedMigrations(ctx context.Context) (map[string]bool, error) {
	applied := make(map[string]bool)
	rows, err := db.QueryContext(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("query migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan migration: %w", err)
		}
		applied[version] = true
	}
	return applied, nil
}

func (db *DB) getMigrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(MigrationsFS, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func (db *DB) applyPendingMigrations(ctx context.Context, logger *slog.Logger, files []string, applied map[string]bool) error {
	for _, file := range files {
		version := strings.TrimSuffix(file, ".sql")
		if applied[version] {
			logger.Debug("migration already applied", "version", version)
			continue
		}

		if err := db.applySingleMigration(ctx, logger, file, version); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) applySingleMigration(ctx context.Context, logger *slog.Logger, file, version string) error {
	content, err := fs.ReadFile(MigrationsFS, file)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", file, err)
	}

	logger.Info("applying migration", "version", version)

	_, err = db.ExecContext(ctx, string(content))
	if err != nil {
		return fmt.Errorf("apply migration %s: %w", file, err)
	}

	_, err = db.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version)
	if err != nil {
		return fmt.Errorf("record migration %s: %w", file, err)
	}

	logger.Info("migration applied", "version", version)
	return nil
}
