package database

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

//go:embed ddl/*.sql
var ddlFS embed.FS

// RunSchema runs GORM AutoMigrate for all models then the idempotent DDL bootstrap (extensions, etc.).
// Per postgres_schema.md: schema is represented as GORM models; a separate DDL step manages extensions and non-ORM DDL.
func (db *DB) RunSchema(ctx context.Context, logger *slog.Logger) error {
	if err := db.runAutoMigrate(ctx, logger); err != nil {
		return err
	}
	return db.runDDLBootstrap(ctx, logger)
}

// runAutoMigrate runs GORM AutoMigrate for all tables used by the Store.
func (db *DB) runAutoMigrate(ctx context.Context, logger *slog.Logger) error {
	logger.Info("running GORM AutoMigrate")
	err := db.db.WithContext(ctx).AutoMigrate(
		&models.User{},
		&models.PasswordCredential{},
		&models.RefreshSession{},
		&models.AuthAuditLog{},
		&models.Task{},
		&models.Job{},
		&models.Node{},
		&models.NodeCapability{},
	)
	if err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	logger.Info("AutoMigrate complete")
	return nil
}

// runDDLBootstrap runs idempotent DDL from embedded ddl/*.sql (e.g. CREATE EXTENSION).
func (db *DB) runDDLBootstrap(ctx context.Context, logger *slog.Logger) error {
	entries, err := ddlFS.ReadDir("ddl")
	if err != nil {
		return fmt.Errorf("read ddl dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	sqlDB, err := db.db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}

	for _, file := range files {
		path := "ddl/" + file
		content, err := ddlFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		logger.Info("applying DDL", "file", file)
		_, err = sqlDB.ExecContext(ctx, string(content))
		if err != nil {
			return fmt.Errorf("execute %s: %w", file, err)
		}
		logger.Info("DDL applied", "file", file)
	}
	return nil
}
