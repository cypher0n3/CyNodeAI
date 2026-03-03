package database

import (
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

const scopeUser = "user"

//nolint:gocyclo // integration test exercises full CRUD flow in one place
func TestSkills_CreateGetListUpdateDelete_Integration(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "skills-user-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	skill, err := db.CreateSkill(ctx, "My Skill", "# Content", scopeUser, &user.ID, false)
	if err != nil {
		t.Fatalf("CreateSkill: %v", err)
	}
	if skill.ID == uuid.Nil {
		t.Error("CreateSkill: id empty")
	}
	got, err := db.GetSkillByID(ctx, skill.ID)
	if err != nil {
		t.Fatalf("GetSkillByID: %v", err)
	}
	if got.Name != "My Skill" || got.Content != "# Content" {
		t.Errorf("GetSkillByID: got name=%q content=%q", got.Name, got.Content)
	}
	list, err := db.ListSkillsForUser(ctx, user.ID, "", "")
	if err != nil {
		t.Fatalf("ListSkillsForUser: %v", err)
	}
	if len(list) < 1 {
		t.Errorf("ListSkillsForUser: want at least 1 (own + default), got %d", len(list))
	}
	content2 := "# Updated"
	updated, err := db.UpdateSkill(ctx, skill.ID, nil, &content2, nil)
	if err != nil {
		t.Fatalf("UpdateSkill: %v", err)
	}
	if updated.Content != content2 {
		t.Errorf("UpdateSkill: content = %q", updated.Content)
	}
	name2 := "Renamed"
	scope2 := "project"
	updated2, err := db.UpdateSkill(ctx, skill.ID, &name2, nil, &scope2)
	if err != nil {
		t.Fatalf("UpdateSkill name/scope: %v", err)
	}
	if updated2.Name != name2 || updated2.Scope != scope2 {
		t.Errorf("UpdateSkill name/scope: got name=%q scope=%q", updated2.Name, updated2.Scope)
	}
	if err := db.DeleteSkill(ctx, skill.ID); err != nil {
		t.Fatalf("DeleteSkill: %v", err)
	}
	_, err = db.GetSkillByID(ctx, skill.ID)
	if err != ErrNotFound {
		t.Errorf("after DeleteSkill GetSkillByID: want ErrNotFound, got %v", err)
	}
}

func TestSkills_EnsureDefaultSkill_Integration(t *testing.T) {
	db, ctx := integrationDB(t)
	content := "# Default CyNodeAI skill"
	if err := db.EnsureDefaultSkill(ctx, content); err != nil {
		t.Fatalf("EnsureDefaultSkill: %v", err)
	}
	got, err := db.GetSkillByID(ctx, DefaultSkillID)
	if err != nil {
		t.Fatalf("GetSkillByID default: %v", err)
	}
	if !got.IsSystem || got.Content != content {
		t.Errorf("default skill: IsSystem=%v content=%q", got.IsSystem, got.Content)
	}
}

func TestSkills_EnsureDefaultSkill_UpdatePath(t *testing.T) {
	db, ctx := integrationDB(t)
	if err := db.EnsureDefaultSkill(ctx, "# First"); err != nil {
		t.Fatalf("EnsureDefaultSkill first: %v", err)
	}
	if err := db.EnsureDefaultSkill(ctx, "# Updated content"); err != nil {
		t.Fatalf("EnsureDefaultSkill update: %v", err)
	}
	got, err := db.GetSkillByID(ctx, DefaultSkillID)
	if err != nil {
		t.Fatalf("GetSkillByID: %v", err)
	}
	if got.Content != "# Updated content" {
		t.Errorf("content = %q", got.Content)
	}
}

func TestSkills_ListSkillsForUser_WithFilters(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "list-filter-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	_, err = db.CreateSkill(ctx, "S1", "# C1", scopeUser, &user.ID, false)
	if err != nil {
		t.Fatalf("CreateSkill: %v", err)
	}
	_, err = db.CreateSkill(ctx, "S2", "# C2", "group", &user.ID, false)
	if err != nil {
		t.Fatalf("CreateSkill group: %v", err)
	}
	list, err := db.ListSkillsForUser(ctx, user.ID, scopeUser, "")
	if err != nil {
		t.Fatalf("ListSkillsForUser scope filter: %v", err)
	}
	if len(list) < 1 {
		t.Errorf("ListSkillsForUser(scope=user): want at least 1, got %d", len(list))
	}
	listGroup, err := db.ListSkillsForUser(ctx, user.ID, "group", "")
	if err != nil {
		t.Fatalf("ListSkillsForUser scope=group: %v", err)
	}
	if len(listGroup) < 1 {
		t.Errorf("ListSkillsForUser(scope=group): want at least 1, got %d", len(listGroup))
	}
	list2, err := db.ListSkillsForUser(ctx, user.ID, "", "")
	if err != nil {
		t.Fatalf("ListSkillsForUser no filter: %v", err)
	}
	if len(list2) < 1 {
		t.Errorf("ListSkillsForUser(no filter): want at least 1, got %d", len(list2))
	}
	list3, err := db.ListSkillsForUser(ctx, user.ID, scopeUser, user.ID.String())
	if err != nil {
		t.Fatalf("ListSkillsForUser owner filter: %v", err)
	}
	if len(list3) < 1 {
		t.Errorf("ListSkillsForUser(owner filter): want at least 1, got %d", len(list3))
	}
}

func TestSkills_CreateSkill_ExplicitScope(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "scope-user-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	skill, err := db.CreateSkill(ctx, "Group Skill", "# Group", "group", &user.ID, false)
	if err != nil {
		t.Fatalf("CreateSkill group: %v", err)
	}
	if skill.Scope != "group" {
		t.Errorf("scope = %q", skill.Scope)
	}
	skillDefaultScope, err := db.CreateSkill(ctx, "Default Scope", "# X", "", &user.ID, false)
	if err != nil {
		t.Fatalf("CreateSkill default scope: %v", err)
	}
	if skillDefaultScope.Scope != scopeUser {
		t.Errorf("default scope = %q", skillDefaultScope.Scope)
	}
}

func TestSkills_UpdateSystemSkill_ReturnsError(t *testing.T) {
	db, ctx := integrationDB(t)
	_ = db.EnsureDefaultSkill(ctx, "# Default")
	_, err := db.UpdateSkill(ctx, DefaultSkillID, nil, strPtr("# Changed"), nil)
	if err == nil {
		t.Error("UpdateSkill on system skill: expected error")
	}
}

func TestSkills_DeleteSystemSkill_ReturnsError(t *testing.T) {
	db, ctx := integrationDB(t)
	_ = db.EnsureDefaultSkill(ctx, "# Default")
	err := db.DeleteSkill(ctx, DefaultSkillID)
	if err == nil {
		t.Error("DeleteSkill on system skill: expected error")
	}
}

func TestSkills_GetSkillByID_NotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetSkillByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("GetSkillByID missing id: got %v", err)
	}
}

func TestSkills_UpdateSkill_NotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	content := "# x"
	_, err := db.UpdateSkill(ctx, uuid.New(), nil, &content, nil)
	if err != ErrNotFound {
		t.Errorf("UpdateSkill missing id: got %v", err)
	}
}

// Ensure we don't break models.Skill reference (compile-time check).
var _ = models.Skill{}
