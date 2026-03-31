// Package database: skills storage per docs/tech_specs/skills_storage_and_inference.md.
package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// DefaultSkillID is the reserved UUID for the built-in CyNodeAI interaction skill (REQ-SKILLS-0116).
var DefaultSkillID = uuid.MustParse("00000000-0000-4000-8000-000000000001")

const scopeUser = "user"

const (
	// DefaultSkillPageLimit is the default page size for ListSkillsForUser when limit <= 0.
	DefaultSkillPageLimit = 50
	// MaxSkillPageLimit is the maximum page size for ListSkillsForUser.
	MaxSkillPageLimit = 100
)

// CreateSkill stores a new skill and returns it. Scope must be user|group|project|global; default user. OwnerID required for non-system skills.
func (db *DB) CreateSkill(ctx context.Context, name, content, scope string, ownerID *uuid.UUID, isSystem bool) (*models.Skill, error) {
	if scope == "" {
		scope = scopeUser
	}
	now := time.Now().UTC()
	record := &SkillRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        uuid.New(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		SkillBase: models.SkillBase{
			Name:     name,
			Content:  content,
			Scope:    scope,
			OwnerID:  ownerID,
			IsSystem: isSystem,
		},
	}
	if err := db.db.WithContext(ctx).Create(record).Error; err != nil {
		return nil, wrapErr(err, "create skill")
	}
	return record.ToSkill(), nil
}

// GetSkillByID returns a skill by id, or ErrNotFound.
func (db *DB) GetSkillByID(ctx context.Context, id uuid.UUID) (*models.Skill, error) {
	var record SkillRecord
	if err := db.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		return nil, wrapErr(err, "get skill by id")
	}
	return record.ToSkill(), nil
}

// ListSkillsForUser returns one page of skills visible to the user: own skills (owner_id = userID) plus system default.
// scopeFilter and ownerFilter optional (empty = no filter). limit<=0 uses DefaultSkillPageLimit; limit is clamped to MaxSkillPageLimit.
func (db *DB) ListSkillsForUser(ctx context.Context, userID uuid.UUID, scopeFilter, ownerFilter string, limit, offset int) ([]*models.Skill, int64, error) {
	base := db.db.WithContext(ctx).Model(&SkillRecord{}).
		Where("is_system = ? OR owner_id = ?", true, userID)
	if scopeFilter != "" {
		base = base.Where("scope = ?", scopeFilter)
	}
	if ownerFilter != "" {
		base = base.Where("owner_id::text = ?", ownerFilter)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, wrapErr(err, "count skills for user")
	}
	if limit <= 0 {
		limit = DefaultSkillPageLimit
	}
	if limit > MaxSkillPageLimit {
		limit = MaxSkillPageLimit
	}
	if offset < 0 {
		offset = 0
	}
	var records []SkillRecord
	if err := base.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&records).Error; err != nil {
		return nil, 0, wrapErr(err, "list skills for user")
	}
	out := make([]*models.Skill, len(records))
	for i := range records {
		out[i] = records[i].ToSkill()
	}
	return out, total, nil
}

// UpdateSkill updates name, content, and/or scope by id. Nil pointer means do not update. Returns updated skill or ErrNotFound.
func (db *DB) UpdateSkill(ctx context.Context, id uuid.UUID, name, content, scope *string) (*models.Skill, error) {
	var record SkillRecord
	if err := db.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		return nil, wrapErr(err, "get skill for update")
	}
	if record.IsSystem {
		return nil, wrapErr(errors.New("cannot update system skill"), "update skill")
	}
	updates := make(map[string]interface{})
	updates["updated_at"] = time.Now().UTC()
	if name != nil {
		updates["name"] = *name
	}
	if content != nil {
		updates["content"] = *content
	}
	if scope != nil {
		updates["scope"] = *scope
	}
	if err := db.db.WithContext(ctx).Model(&record).Updates(updates).Error; err != nil {
		return nil, wrapErr(err, "update skill")
	}
	return db.GetSkillByID(ctx, id)
}

// DeleteSkill removes a skill by id. System skill cannot be deleted; returns error.
func (db *DB) DeleteSkill(ctx context.Context, id uuid.UUID) error {
	var record SkillRecord
	if err := db.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		return wrapErr(err, "get skill for delete")
	}
	if record.IsSystem {
		return wrapErr(errors.New("cannot delete system skill"), "delete skill")
	}
	if err := db.db.WithContext(ctx).Delete(&record).Error; err != nil {
		return wrapErr(err, "delete skill")
	}
	return nil
}

// EnsureDefaultSkill creates or updates the single system default skill (id = DefaultSkillID). Used at schema/migration or first use.
func (db *DB) EnsureDefaultSkill(ctx context.Context, content string) error {
	now := time.Now().UTC()
	var record SkillRecord
	err := db.db.WithContext(ctx).Where("id = ?", DefaultSkillID).First(&record).Error
	if err != nil {
		// Create
		record := &SkillRecord{
			GormModelUUID: gormmodel.GormModelUUID{
				ID:        DefaultSkillID,
				CreatedAt: now,
				UpdatedAt: now,
			},
			SkillBase: models.SkillBase{
				Name:     "CyNodeAI interaction",
				Content:  content,
				Scope:    "global",
				OwnerID:  nil,
				IsSystem: true,
			},
		}
		if err := db.db.WithContext(ctx).Create(record).Error; err != nil {
			return wrapErr(err, "create default skill")
		}
		return nil
	}
	// Update content and updated_at
	if err := db.db.WithContext(ctx).Model(&record).Updates(map[string]interface{}{
		"content":    content,
		"updated_at": now,
	}).Error; err != nil {
		return wrapErr(err, "update default skill")
	}
	return nil
}
