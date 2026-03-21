// Package database: project read helpers for MCP gateway and handlers.
package database

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// GetProjectByID returns a project by primary key or ErrNotFound.
func (db *DB) GetProjectByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	return getDomainByID(db, ctx, id, "get project by id", (*ProjectRecord).ToProject)
}

// GetProjectBySlug returns a project by slug or ErrNotFound.
func (db *DB) GetProjectBySlug(ctx context.Context, slug string) (*models.Project, error) {
	var record ProjectRecord
	err := db.db.WithContext(ctx).Where("slug = ?", strings.TrimSpace(slug)).First(&record).Error
	if err != nil {
		return nil, wrapErr(err, "get project by slug")
	}
	return record.ToProject(), nil
}

// ListAuthorizedProjectsForUser returns projects visible to the user.
// MVP: the authorized set is the per-user default project (see GetOrCreateDefaultProjectForUser).
// Optional q filters by substring match on slug, display_name, or description (case-insensitive).
func (db *DB) ListAuthorizedProjectsForUser(ctx context.Context, userID uuid.UUID, q string, limit, offset int) ([]*models.Project, error) {
	_ = limit // accepted for API parity; MVP returns at most one project (pagination reserved).
	if offset < 0 {
		offset = 0
	}
	def, err := db.GetOrCreateDefaultProjectForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if offset > 0 {
		// Single-project MVP: offset beyond the one row yields empty.
		return []*models.Project{}, nil
	}
	q = strings.TrimSpace(strings.ToLower(q))
	if q != "" {
		sl := strings.ToLower(def.Slug)
		dn := strings.ToLower(def.DisplayName)
		desc := ""
		if def.Description != nil {
			desc = strings.ToLower(*def.Description)
		}
		if !strings.Contains(sl, q) && !strings.Contains(dn, q) && !strings.Contains(desc, q) {
			return []*models.Project{}, nil
		}
	}
	return []*models.Project{def}, nil
}
