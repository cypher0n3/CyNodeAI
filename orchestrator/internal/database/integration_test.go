// Integration tests for the database package. Run with a real Postgres:
//
//	POSTGRES_TEST_DSN="postgres://..." go test -v -run Integration ./internal/database
package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/google/uuid"
)

const integrationEnv = "POSTGRES_TEST_DSN"

const integrationTestPreferenceValue = `"v1"`

const integrationTestPayloadX1 = `{"x":1}`

func TestIntegration_User(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "inttest-user", nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	got, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Fatalf("GetUserByHandle: %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("GetUserByHandle: id mismatch")
	}
}

func TestIntegration_CreateTaskWithNilCreatedBy(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "prompt-nil-createdby", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask(nil createdBy): %v", err)
	}
	if task.Summary == nil || *task.Summary == "" {
		t.Error("CreateTask with nil createdBy should set task_name_001 style summary")
	}
}

func TestIntegration_TaskAndJob(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first (run TestIntegration_User)")
	}
	task, err := db.CreateTask(ctx, &user.ID, "prompt", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	job, err := db.CreateJob(ctx, task.ID, integrationTestPayloadX1)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job.Payload.Ptr() == nil || *job.Payload.Ptr() != integrationTestPayloadX1 {
		t.Errorf("CreateJob: payload not round-tripped")
	}
	if _, err := db.GetTaskByID(ctx, task.ID); err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	if _, err := db.GetJobByID(ctx, job.ID); err != nil {
		t.Fatalf("GetJobByID: %v", err)
	}
}

func TestIntegration_Node(t *testing.T) {
	db, ctx := integrationDB(t)
	node, err := db.CreateNode(ctx, "inttest-node")
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := db.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive); err != nil {
		t.Fatalf("UpdateNodeStatus: %v", err)
	}
	list, err := db.ListActiveNodes(ctx)
	if err != nil {
		t.Fatalf("ListActiveNodes: %v", err)
	}
	if len(list) < 1 {
		t.Error("ListActiveNodes: expected at least one")
	}
}

func TestIntegration_ListDispatchableNodesAndListTasksByUser(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first (run TestIntegration_User)")
	}
	node, err := db.CreateNode(ctx, "inttest-dispatchable-"+uuid.New().String())
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := db.UpdateNodeStatus(ctx, node.ID, models.NodeStatusActive); err != nil {
		t.Fatalf("UpdateNodeStatus: %v", err)
	}
	if err := db.UpdateNodeWorkerAPIConfig(ctx, node.ID, "http://localhost:12090", "token"); err != nil {
		t.Fatalf("UpdateNodeWorkerAPIConfig: %v", err)
	}
	ackAt := time.Now().UTC()
	if err := db.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil); err != nil {
		t.Fatalf("UpdateNodeConfigAck: %v", err)
	}
	list, err := db.ListDispatchableNodes(ctx)
	if err != nil {
		t.Fatalf("ListDispatchableNodes: %v", err)
	}
	if len(list) < 1 {
		t.Error("ListDispatchableNodes: expected at least one after setup")
	}
	tasks, err := db.ListTasksByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListTasksByUser: %v", err)
	}
	_ = tasks
}

func TestIntegration_NodeConfigVersionAndAck(t *testing.T) {
	db, ctx := integrationDB(t)
	node, err := db.CreateNode(ctx, "inttest-config-node-"+uuid.New().String())
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if err := db.UpdateNodeConfigVersion(ctx, node.ID, "1"); err != nil {
		t.Fatalf("UpdateNodeConfigVersion: %v", err)
	}
	ackAt := time.Now().UTC()
	errMsg := "test error"
	if err := db.UpdateNodeConfigAck(ctx, node.ID, "1", "applied", ackAt, nil); err != nil {
		t.Fatalf("UpdateNodeConfigAck applied: %v", err)
	}
	got, err := db.GetNodeByID(ctx, node.ID)
	if err != nil {
		t.Fatalf("GetNodeByID: %v", err)
	}
	if got.ConfigVersion == nil || *got.ConfigVersion != "1" {
		t.Errorf("ConfigVersion: expected 1, got %v", got.ConfigVersion)
	}
	if got.ConfigAckStatus == nil || *got.ConfigAckStatus != "applied" {
		t.Errorf("ConfigAckStatus: expected applied, got %v", got.ConfigAckStatus)
	}
	if err := db.UpdateNodeConfigAck(ctx, node.ID, "1", "failed", ackAt, &errMsg); err != nil {
		t.Fatalf("UpdateNodeConfigAck failed: %v", err)
	}
	got, _ = db.GetNodeByID(ctx, node.ID)
	if got.ConfigAckError == nil || *got.ConfigAckError != errMsg {
		t.Errorf("ConfigAckError: expected %q, got %v", errMsg, got.ConfigAckError)
	}
}

func TestIntegration_AuthAuditLog(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first (run TestIntegration_User)")
	}
	reason := "test"
	if err := db.CreateAuthAuditLog(ctx, &user.ID, "login_success", true, nil, nil, nil, &reason); err != nil {
		t.Fatalf("CreateAuthAuditLog: %v", err)
	}
}

func TestIntegration_McpToolCallAuditLog(t *testing.T) {
	db, ctx := integrationDB(t)
	rec := &models.McpToolCallAuditLog{
		ToolName: "db.preference.get",
		Decision: "allow",
		Status:   "success",
	}
	if err := db.CreateMcpToolCallAuditLog(ctx, rec); err != nil {
		t.Fatalf("CreateMcpToolCallAuditLog: %v", err)
	}
	if rec.ID == uuid.Nil {
		t.Error("expected ID to be set")
	}
	if rec.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestIntegration_Preferences_GetAndList(t *testing.T) {
	db, ctx := integrationDB(t)
	val := integrationTestPreferenceValue
	ent := &models.PreferenceEntry{
		ID:        uuid.New(),
		ScopeType: "system",
		ScopeID:   nil,
		Key:       "test.key",
		Value:     &val,
		ValueType: "string",
		Version:   1,
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.GORM().WithContext(ctx).Create(ent).Error; err != nil {
		t.Fatalf("create preference entry: %v", err)
	}
	got, err := db.GetPreference(ctx, "system", nil, "test.key")
	if err != nil {
		t.Fatalf("GetPreference: %v", err)
	}
	if got.Key != "test.key" || got.ScopeType != "system" {
		t.Errorf("GetPreference: got %+v", got)
	}
	list, next, err := db.ListPreferences(ctx, "system", nil, "", 10, "")
	if err != nil {
		t.Fatalf("ListPreferences: %v", err)
	}
	if len(list) < 1 || next != "" {
		t.Errorf("ListPreferences: got %d entries, next_cursor %q", len(list), next)
	}
	_, err = db.GetPreference(ctx, "system", nil, "nonexistent.key")
	if err != ErrNotFound {
		t.Errorf("GetPreference nonexistent: got %v, want ErrNotFound", err)
	}
}

func TestIntegration_Preferences_EffectiveAndCursor(t *testing.T) {
	db, ctx := integrationDB(t)
	val := integrationTestPreferenceValue
	ent := &models.PreferenceEntry{
		ID:        uuid.New(),
		ScopeType: "system",
		Key:       "test.key",
		Value:     &val,
		ValueType: "string",
		Version:   1,
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.GORM().WithContext(ctx).Create(ent).Error; err != nil {
		t.Fatalf("create preference: %v", err)
	}
	task, err := db.CreateTask(ctx, nil, "eff-prompt", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	effective, err := db.GetEffectivePreferencesForTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetEffectivePreferencesForTask: %v", err)
	}
	if effective["test.key"] == nil {
		t.Errorf("effective should contain test.key, got %v", effective)
	}
	list2, next2, err := db.ListPreferences(ctx, "system", nil, "test.", 1, "")
	if err != nil {
		t.Fatalf("ListPreferences key_prefix: %v", err)
	}
	if len(list2) < 1 {
		t.Errorf("ListPreferences key_prefix: got %d", len(list2))
	}
	if next2 != "" {
		_, _, err = db.ListPreferences(ctx, "system", nil, "test.", 10, next2)
		if err != nil {
			t.Fatalf("ListPreferences cursor: %v", err)
		}
	}
}

func TestIntegration_Preferences_UserScope(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first (run TestIntegration_User)")
	}
	uv := `"user-val"`
	uent := &models.PreferenceEntry{
		ID:        uuid.New(),
		ScopeType: "user",
		ScopeID:   &user.ID,
		Key:       "user.key",
		Value:     &uv,
		ValueType: "string",
		Version:   1,
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.GORM().WithContext(ctx).Create(uent).Error; err != nil {
		t.Fatalf("create user preference: %v", err)
	}
	got, err := db.GetPreference(ctx, "user", &user.ID, "user.key")
	if err != nil {
		t.Fatalf("GetPreference user scope: %v", err)
	}
	if got.Key != "user.key" {
		t.Errorf("GetPreference: got key %q", got.Key)
	}
	ulist, _, err := db.ListPreferences(ctx, "user", &user.ID, "", 1, "")
	if err != nil {
		t.Fatalf("ListPreferences user scope: %v", err)
	}
	if len(ulist) >= 1 {
		_, nextCur, _ := db.ListPreferences(ctx, "user", &user.ID, "", 1, "0")
		if nextCur != "" {
			_, _, _ = db.ListPreferences(ctx, "user", &user.ID, "", 1, nextCur)
		}
	}
}

func TestIntegration_Preferences_EffectiveWithProject(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first")
	}
	var proj models.Project
	err = db.GORM().WithContext(ctx).Where("slug = ?", "default").First(&proj).Error
	if err != nil {
		pid := uuid.New()
		now := time.Now().UTC()
		_ = db.GORM().WithContext(ctx).Create(&models.Project{ID: pid, Slug: "default", DisplayName: "Default", IsActive: true, CreatedAt: now, UpdatedAt: now}).Error
		task2, _ := db.CreateTask(ctx, &user.ID, "eff2", nil, nil)
		if task2 != nil {
			_ = db.GORM().WithContext(ctx).Model(&models.Task{}).Where("id = ?", task2.ID).Update("project_id", pid).Error
			_, _ = db.GetEffectivePreferencesForTask(ctx, task2.ID)
		}
	} else {
		task2, _ := db.CreateTask(ctx, &user.ID, "eff2", nil, nil)
		if task2 != nil {
			_ = db.GORM().WithContext(ctx).Model(&models.Task{}).Where("id = ?", task2.ID).Update("project_id", proj.ID).Error
			_, _ = db.GetEffectivePreferencesForTask(ctx, task2.ID)
		}
	}
}

// TestIntegration_Preferences_ListLimitAndCursor covers ListPreferences limit cap, invalid cursor, and nextCursor when offset+len < total.
func TestIntegration_Preferences_ListLimitAndCursor(t *testing.T) {
	db, ctx := integrationDB(t)
	val := integrationTestPreferenceValue
	for i := 0; i < 3; i++ {
		ent := &models.PreferenceEntry{
			ID:        uuid.New(),
			ScopeType: "system",
			ScopeID:   nil,
			Key:       fmt.Sprintf("cap.key.%d", i),
			Value:     &val,
			ValueType: "string",
			Version:   1,
			UpdatedAt: time.Now().UTC(),
		}
		if err := db.GORM().WithContext(ctx).Create(ent).Error; err != nil {
			t.Fatalf("create preference: %v", err)
		}
	}
	// Limit cap: request > MaxPreferenceListLimit
	list, _, err := db.ListPreferences(ctx, "system", nil, "", 500, "")
	if err != nil {
		t.Fatalf("ListPreferences limit 500: %v", err)
	}
	if len(list) > MaxPreferenceListLimit {
		t.Errorf("ListPreferences: expected cap at %d, got %d", MaxPreferenceListLimit, len(list))
	}
	// Invalid or negative cursor leaves offset 0; limit <= 0 is capped to MaxPreferenceListLimit
	_, _, err = db.ListPreferences(ctx, "system", nil, "", 10, "x")
	if err != nil {
		t.Fatalf("ListPreferences invalid cursor: %v", err)
	}
	_, _, err = db.ListPreferences(ctx, "system", nil, "", 10, "-1")
	if err != nil {
		t.Fatalf("ListPreferences negative cursor: %v", err)
	}
	_, _, err = db.ListPreferences(ctx, "system", nil, "", 0, "")
	if err != nil {
		t.Fatalf("ListPreferences limit 0: %v", err)
	}
	// Limit 2 with 3+ entries: nextCursor = offset + len(entries) when len(entries) <= limit but more exist
	list2, next2, err := db.ListPreferences(ctx, "system", nil, "cap.key.", 2, "")
	if err != nil {
		t.Fatalf("ListPreferences key_prefix limit 2: %v", err)
	}
	if len(list2) != 2 || next2 == "" {
		t.Errorf("ListPreferences: got %d entries, next_cursor %q", len(list2), next2)
	}
	_, _, _ = db.ListPreferences(ctx, "system", nil, "cap.key.", 2, next2)
}

// Invalid JSON in preference value cannot be stored (value column is JSONB); the skip-on-unmarshal
// path in GetEffectivePreferencesForTask is defensive. Parser behavior is covered by
// TestParsePreferenceValue_InvalidJSON in preferences_test.go.

// TestIntegration_Preferences_EffectiveWithNilValue covers GetEffectivePreferencesForTask when an entry has nil value (skip unmarshal).
func TestIntegration_Preferences_EffectiveWithNilValue(t *testing.T) {
	db, ctx := integrationDB(t)
	ent := &models.PreferenceEntry{
		ID:        uuid.New(),
		ScopeType: "system",
		ScopeID:   nil,
		Key:       "nil.val.key",
		Value:     nil, // nil value: effective still gets the key with nil
		ValueType: "string",
		Version:   1,
		UpdatedAt: time.Now().UTC(),
	}
	if err := db.GORM().WithContext(ctx).Create(ent).Error; err != nil {
		t.Fatalf("create preference: %v", err)
	}
	task, err := db.CreateTask(ctx, nil, "eff-nil-val", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	effective, err := db.GetEffectivePreferencesForTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetEffectivePreferencesForTask: %v", err)
	}
	if effective["nil.val.key"] != nil {
		t.Errorf("expected nil value to yield nil in effective, got %v", effective["nil.val.key"])
	}
}

func TestIntegration_Preferences_CreateUpdateDelete(t *testing.T) {
	db, ctx := integrationDB(t)
	key := "crud.key." + uuid.New().String()
	ent, err := db.CreatePreference(ctx, "system", nil, key, `"v1"`, "string", nil, nil)
	if err != nil {
		t.Fatalf("CreatePreference: %v", err)
	}
	if ent.Key != key || ent.Version < 1 {
		t.Errorf("CreatePreference: got %+v", ent)
	}
	_, err = db.CreatePreference(ctx, "system", nil, key, `"v2"`, "string", nil, nil)
	if err != ErrExists {
		t.Errorf("CreatePreference duplicate: want ErrExists, got %v", err)
	}
	ev := ent.Version
	ent2, err := db.UpdatePreference(ctx, "system", nil, key, `"v1updated"`, "string", &ev, nil, nil)
	if err != nil {
		t.Fatalf("UpdatePreference: %v", err)
	}
	if ent2.Version <= ent.Version {
		t.Errorf("UpdatePreference: version should increase, got %d then %d", ent.Version, ent2.Version)
	}
	evBad := 999
	_, err = db.UpdatePreference(ctx, "system", nil, key, `"x"`, "string", &evBad, nil, nil)
	if err != ErrConflict {
		t.Errorf("UpdatePreference version mismatch: want ErrConflict, got %v", err)
	}
	// Delete using current version (re-fetch in case of shared DB skew)
	cur, err := db.GetPreference(ctx, "system", nil, key)
	if err != nil {
		t.Fatalf("GetPreference before delete: %v", err)
	}
	evDel := cur.Version
	err = db.DeletePreference(ctx, "system", nil, key, &evDel, nil)
	if err != nil {
		t.Fatalf("DeletePreference: %v", err)
	}
	_, err = db.GetPreference(ctx, "system", nil, key)
	if err != ErrNotFound {
		t.Errorf("DeletePreference: want ErrNotFound after delete, got %v", err)
	}
}

func TestIntegration_Preferences_DeleteNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	key := "nonexistent.delete." + uuid.New().String()
	err := db.DeletePreference(ctx, "system", nil, key, nil, nil)
	if err != ErrNotFound {
		t.Errorf("DeletePreference nonexistent: want ErrNotFound, got %v", err)
	}
}

func TestIntegration_Preferences_DeleteVersionConflict(t *testing.T) {
	db, ctx := integrationDB(t)
	key := "conflict.delete." + uuid.New().String()
	ent, err := db.CreatePreference(ctx, "system", nil, key, `"v"`, "string", nil, nil)
	if err != nil {
		t.Fatalf("CreatePreference: %v", err)
	}
	evWrong := ent.Version + 999
	err = db.DeletePreference(ctx, "system", nil, key, &evWrong, nil)
	if err != ErrConflict {
		t.Errorf("DeletePreference version mismatch: want ErrConflict, got %v", err)
	}
	// Clean up
	ev := ent.Version
	_ = db.DeletePreference(ctx, "system", nil, key, &ev, nil)
}

func TestIntegration_Preferences_ListWithKeyPrefixAndCursor(t *testing.T) {
	db, ctx := integrationDB(t)
	prefix := "listpfx." + uuid.New().String()
	for i := 0; i < 3; i++ {
		key := prefix + "." + fmt.Sprintf("%d", i)
		_, err := db.CreatePreference(ctx, "system", nil, key, `"v"`, "string", nil, nil)
		if err != nil {
			t.Fatalf("CreatePreference: %v", err)
		}
	}
	entries, next, err := db.ListPreferences(ctx, "system", nil, prefix, 2, "")
	if err != nil {
		t.Fatalf("ListPreferences: %v", err)
	}
	if len(entries) != 2 || next == "" {
		t.Errorf("expected 2 entries and next cursor, got len=%d next=%q", len(entries), next)
	}
	entries2, next2, err := db.ListPreferences(ctx, "system", nil, prefix, 2, next)
	if err != nil {
		t.Fatalf("ListPreferences page2: %v", err)
	}
	if len(entries2) != 1 {
		t.Errorf("expected 1 entry on second page, got %d", len(entries2))
	}
	_ = next2
}

func TestIntegration_Preferences_CreateWithEmptyValue(t *testing.T) {
	db, ctx := integrationDB(t)
	key := "emptyval." + uuid.New().String()
	ent, err := db.CreatePreference(ctx, "system", nil, key, "", "string", nil, nil)
	if err != nil {
		t.Fatalf("CreatePreference: %v", err)
	}
	if ent.Value != nil {
		t.Errorf("expected nil Value for empty string, got %v", ent.Value)
	}
	got, _ := db.GetPreference(ctx, "system", nil, key)
	if got == nil {
		t.Fatal("GetPreference: not found")
	}
	if got.Value != nil {
		t.Errorf("expected nil Value, got %v", got.Value)
	}
	_ = db.DeletePreference(ctx, "system", nil, key, &ent.Version, nil)
}

func TestIntegration_Preferences_UpdateWithEmptyValue(t *testing.T) {
	db, ctx := integrationDB(t)
	key := "updempty." + uuid.New().String()
	ent, err := db.CreatePreference(ctx, "system", nil, key, `"initial"`, "string", nil, nil)
	if err != nil {
		t.Fatalf("CreatePreference: %v", err)
	}
	updated, err := db.UpdatePreference(ctx, "system", nil, key, "", "string", &ent.Version, nil, nil)
	if err != nil {
		t.Fatalf("UpdatePreference: %v", err)
	}
	if updated.Value != nil {
		t.Errorf("expected nil Value after update to empty, got %v", updated.Value)
	}
	_ = db.DeletePreference(ctx, "system", nil, key, &updated.Version, nil)
}

func TestIntegration_Preferences_ListInvalidCursor(t *testing.T) {
	db, ctx := integrationDB(t)
	key := "invcur." + uuid.New().String()
	_, err := db.CreatePreference(ctx, "system", nil, key, `"v"`, "string", nil, nil)
	if err != nil {
		t.Fatalf("CreatePreference: %v", err)
	}
	entries, next, err := db.ListPreferences(ctx, "system", nil, "", 10, "not-a-number")
	if err != nil {
		t.Fatalf("ListPreferences: %v", err)
	}
	if len(entries) == 0 && next != "" {
		t.Errorf("invalid cursor should be treated as offset 0; got next=%q", next)
	}
	_ = db.DeletePreference(ctx, "system", nil, key, nil, nil)
}

func TestIntegration_Preferences_ListLimitCapped(t *testing.T) {
	db, ctx := integrationDB(t)
	// Request more than MaxPreferenceListLimit; implementation caps to MaxPreferenceListLimit.
	_, _, err := db.ListPreferences(ctx, "system", nil, "", 999, "")
	if err != nil {
		t.Fatalf("ListPreferences(limit 999): %v", err)
	}
}

func TestIntegration_GetEffectivePreferencesForTask_NotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetEffectivePreferencesForTask(ctx, uuid.New())
	if err == nil {
		t.Fatal("GetEffectivePreferencesForTask(nonexistent task) should fail")
	}
}

func TestIntegration_Preferences_CreateDuplicateKey(t *testing.T) {
	db, ctx := integrationDB(t)
	key := "dupkey." + uuid.New().String()
	_, err := db.CreatePreference(ctx, "system", nil, key, `"v"`, "string", nil, nil)
	if err != nil {
		t.Fatalf("CreatePreference: %v", err)
	}
	_, err = db.CreatePreference(ctx, "system", nil, key, `"v2"`, "string", nil, nil)
	if err != ErrExists {
		t.Errorf("CreatePreference duplicate: want ErrExists, got %v", err)
	}
	_ = db.DeletePreference(ctx, "system", nil, key, nil, nil)
}

func TestIntegration_GetArtifactByTaskIDAndPath(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "artifact-task", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	art := &models.TaskArtifact{
		ID:         uuid.New(),
		TaskID:     task.ID,
		Path:       "out/report.md",
		StorageRef: "ref:abc",
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := db.GORM().WithContext(ctx).Create(art).Error; err != nil {
		t.Fatalf("create task artifact: %v", err)
	}
	got, err := db.GetArtifactByTaskIDAndPath(ctx, task.ID, "out/report.md")
	if err != nil {
		t.Fatalf("GetArtifactByTaskIDAndPath: %v", err)
	}
	if got.Path != "out/report.md" || got.StorageRef != "ref:abc" {
		t.Errorf("GetArtifactByTaskIDAndPath: got %+v", got)
	}
	_, err = db.GetArtifactByTaskIDAndPath(ctx, task.ID, "missing")
	if err != ErrNotFound {
		t.Errorf("GetArtifactByTaskIDAndPath missing: want ErrNotFound, got %v", err)
	}
}

func integrationDB(t *testing.T) (*DB, context.Context) {
	t.Helper()
	dsn := os.Getenv(integrationEnv)
	if dsn == "" {
		t.Skipf("set %s to run integration tests", integrationEnv)
	}
	ctx := context.Background()
	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunSchema(ctx, slog.Default()); err != nil {
		t.Fatalf("RunSchema: %v", err)
	}
	if db.GORM() == nil {
		t.Fatal("GORM() must not be nil")
	}
	return db, ctx
}

func TestIntegration_WorkflowLeaseAndCheckpoint(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-lease-test", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	leaseID := uuid.New()
	holderID := "runner-1"
	expiresAt := time.Now().UTC().Add(time.Hour)
	lease, err := db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID, holderID, expiresAt)
	if err != nil {
		t.Fatalf("AcquireTaskWorkflowLease: %v", err)
	}
	if lease.LeaseID != leaseID {
		t.Errorf("lease_id: got %s", lease.LeaseID)
	}
	gotLease, err := db.GetTaskWorkflowLease(ctx, task.ID)
	if err != nil || gotLease.LeaseID != leaseID {
		t.Fatalf("GetTaskWorkflowLease: %v, got %+v", err, gotLease)
	}
	_, err = db.AcquireTaskWorkflowLease(ctx, task.ID, uuid.New(), "runner-2", expiresAt)
	if !errors.Is(err, ErrLeaseHeld) {
		t.Errorf("duplicate acquire: want ErrLeaseHeld, got %v", err)
	}
	if err := db.ReleaseTaskWorkflowLease(ctx, task.ID, leaseID); err != nil {
		t.Fatalf("ReleaseTaskWorkflowLease: %v", err)
	}
	state := `{"task_id":"` + task.ID.String() + `"}`
	cp := &models.WorkflowCheckpoint{TaskID: task.ID, State: &state, LastNodeID: "plan_steps"}
	if err := db.UpsertWorkflowCheckpoint(ctx, cp); err != nil {
		t.Fatalf("UpsertWorkflowCheckpoint: %v", err)
	}
	got, err := db.GetWorkflowCheckpoint(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetWorkflowCheckpoint: %v", err)
	}
	if got.LastNodeID != "plan_steps" {
		t.Errorf("GetWorkflowCheckpoint: last_node_id %q", got.LastNodeID)
	}
	// Update path: upsert again with new last_node_id.
	cp2 := &models.WorkflowCheckpoint{TaskID: task.ID, State: &state, LastNodeID: "dispatch_step"}
	if err := db.UpsertWorkflowCheckpoint(ctx, cp2); err != nil {
		t.Fatalf("UpsertWorkflowCheckpoint update: %v", err)
	}
	got2, _ := db.GetWorkflowCheckpoint(ctx, task.ID)
	if got2.LastNodeID != "dispatch_step" {
		t.Errorf("after update: last_node_id %q", got2.LastNodeID)
	}
}

func TestIntegration_WorkflowLease_ExpiredReacquire(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-lease-expiry", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	leaseID1 := uuid.New()
	_, err = db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID1, "runner-1", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("AcquireTaskWorkflowLease: %v", err)
	}
	// Expire the lease in DB so a different holder can re-acquire.
	past := time.Now().UTC().Add(-time.Minute)
	if err := db.GORM().WithContext(ctx).Model(&models.TaskWorkflowLease{}).Where("task_id = ?", task.ID).Update("expires_at", past).Error; err != nil {
		t.Fatalf("update expires_at: %v", err)
	}
	leaseID2 := uuid.New()
	lease2, err := db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID2, "runner-2", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("Re-acquire after expiry: %v", err)
	}
	if lease2.HolderID == nil || *lease2.HolderID != "runner-2" || lease2.LeaseID != leaseID2 {
		t.Errorf("re-acquired lease: got %+v", lease2)
	}
}

func TestIntegration_WorkflowLease_IdempotentAcquire(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-idempotent", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	leaseID := uuid.New()
	expiresAt := time.Now().UTC().Add(time.Hour)
	lease1, err := db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID, "runner-1", expiresAt)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	lease2, err := db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID, "runner-1", expiresAt)
	if err != nil {
		t.Fatalf("second acquire (idempotent): %v", err)
	}
	if lease2.LeaseID != lease1.LeaseID || lease2.ID != lease1.ID {
		t.Errorf("idempotent acquire: got %+v, want same as %+v", lease2, lease1)
	}
}

func TestIntegration_WorkflowLease_ReleaseThenReacquire(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-release-reacquire", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	leaseID1 := uuid.New()
	_, err = db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID1, "runner-1", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := db.ReleaseTaskWorkflowLease(ctx, task.ID, leaseID1); err != nil {
		t.Fatalf("Release: %v", err)
	}
	leaseID2 := uuid.New()
	lease2, err := db.AcquireTaskWorkflowLease(ctx, task.ID, leaseID2, "runner-2", time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("Re-acquire after release: %v", err)
	}
	if lease2.LeaseID != leaseID2 || lease2.HolderID == nil || *lease2.HolderID != "runner-2" {
		t.Errorf("re-acquired lease: got %+v", lease2)
	}
}

func TestIntegration_WorkflowErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-err-not-found", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	_, err = db.GetWorkflowCheckpoint(ctx, task.ID)
	if err != ErrNotFound {
		t.Errorf("GetWorkflowCheckpoint: want ErrNotFound, got %v", err)
	}
	_, err = db.GetTaskWorkflowLease(ctx, task.ID)
	if err != ErrNotFound {
		t.Errorf("GetTaskWorkflowLease: want ErrNotFound, got %v", err)
	}
}

func TestIntegration_UpsertWorkflowCheckpoint_WithPreSetID(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-checkpoint-preset-id", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	state := integrationTestPayloadX1
	cp := &models.WorkflowCheckpoint{
		ID:         uuid.New(),
		TaskID:     task.ID,
		State:      &state,
		LastNodeID: "step1",
	}
	if err := db.UpsertWorkflowCheckpoint(ctx, cp); err != nil {
		t.Fatalf("UpsertWorkflowCheckpoint: %v", err)
	}
	got, err := db.GetWorkflowCheckpoint(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetWorkflowCheckpoint: %v", err)
	}
	if got.ID != cp.ID || got.LastNodeID != "step1" {
		t.Errorf("checkpoint: got %+v", got)
	}
}

func TestIntegration_GetNextQueuedJob_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	// Drain queue so we can assert ErrNotFound when empty (tests share one DB).
	for {
		job, err := db.GetNextQueuedJob(ctx)
		if err == ErrNotFound {
			break
		}
		if err != nil {
			t.Fatalf("GetNextQueuedJob: %v", err)
		}
		_ = db.CompleteJob(ctx, job.ID, "", models.JobStatusCancelled)
	}
	_, err := db.GetNextQueuedJob(ctx)
	if err != ErrNotFound {
		t.Errorf("GetNextQueuedJob: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_CompleteJobRoundTrip(t *testing.T) {
	db, ctx := integrationDB(t)
	task, _ := db.CreateTask(ctx, nil, "p", nil, nil)
	job, _ := db.CreateJob(ctx, task.ID, "payload")
	result := `{"status":"ok"}`
	if err := db.CompleteJob(ctx, job.ID, result, models.JobStatusCompleted); err != nil {
		t.Fatalf("CompleteJob: %v", err)
	}
	got, err := db.GetJobByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJobByID: %v", err)
	}
	if got.Result.Ptr() == nil || *got.Result.Ptr() != result {
		t.Errorf("result not round-tripped: got %v", got.Result.Ptr())
	}
}

func TestIntegration_CreateJobCompleted(t *testing.T) {
	db, ctx := integrationDB(t)
	task, _ := db.CreateTask(ctx, nil, "prompt-task", nil, nil)
	jobID := uuid.New()
	result := `{"version":1,"task_id":"` + task.ID.String() + `","job_id":"` + jobID.String() + `","status":"completed","stdout":"hi"}`
	job, err := db.CreateJobCompleted(ctx, task.ID, jobID, result)
	if err != nil {
		t.Fatalf("CreateJobCompleted: %v", err)
	}
	if job.Status != models.JobStatusCompleted {
		t.Errorf("status want completed got %s", job.Status)
	}
	if job.Result.Ptr() == nil || *job.Result.Ptr() != result {
		t.Errorf("result not stored: got %v", job.Result.Ptr())
	}
	jobs, _ := db.GetJobsByTaskID(ctx, task.ID)
	if len(jobs) != 1 || jobs[0].ID != jobID {
		t.Errorf("GetJobsByTaskID: got %v", jobs)
	}
}

func storeRoundTripCredentials(t *testing.T, db Store, ctx context.Context, userID uuid.UUID) {
	t.Helper()
	_, err := db.CreatePasswordCredential(ctx, userID, []byte("hash"), "argon2")
	if err != nil {
		t.Fatalf("CreatePasswordCredential: %v", err)
	}
	if _, err := db.GetPasswordCredentialByUserID(ctx, userID); err != nil {
		t.Fatalf("GetPasswordCredentialByUserID: %v", err)
	}
}

func storeRoundTripSessions(t *testing.T, db Store, ctx context.Context, userID uuid.UUID) {
	t.Helper()
	expires := time.Now().Add(time.Hour)
	session, err := db.CreateRefreshSession(ctx, userID, []byte("tokenhash"), expires)
	if err != nil {
		t.Fatalf("CreateRefreshSession: %v", err)
	}
	if _, err := db.GetActiveRefreshSession(ctx, []byte("tokenhash")); err != nil {
		t.Fatalf("GetActiveRefreshSession: %v", err)
	}
	if err := db.InvalidateRefreshSession(ctx, session.ID); err != nil {
		t.Fatalf("InvalidateRefreshSession: %v", err)
	}
}

func storeRoundTripNode(t *testing.T, db Store, ctx context.Context) *models.Node {
	t.Helper()
	node, err := db.CreateNode(ctx, "inttest-roundtrip-node")
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	if _, err := db.GetNodeByID(ctx, node.ID); err != nil {
		t.Fatalf("GetNodeByID: %v", err)
	}
	if _, err := db.GetNodeBySlug(ctx, "inttest-roundtrip-node"); err != nil {
		t.Fatalf("GetNodeBySlug: %v", err)
	}
	return node
}

func storeRoundTripTaskJobNode(t *testing.T, db Store, ctx context.Context, userID uuid.UUID, node *models.Node) {
	t.Helper()
	task, err := db.CreateTask(ctx, &userID, "roundtrip-prompt", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.UpdateTaskStatus(ctx, task.ID, models.TaskStatusRunning); err != nil {
		t.Fatalf("UpdateTaskStatus: %v", err)
	}
	if err := db.UpdateTaskSummary(ctx, task.ID, "summary"); err != nil {
		t.Fatalf("UpdateTaskSummary: %v", err)
	}
	list, err := db.ListTasksByUser(ctx, userID, 10, 0)
	if err != nil || len(list) < 1 {
		t.Fatalf("ListTasksByUser: %v", err)
	}
	job, err := db.CreateJob(ctx, task.ID, `{"r":1}`)
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if _, err := db.GetJobsByTaskID(ctx, task.ID); err != nil {
		t.Fatalf("GetJobsByTaskID: %v", err)
	}
	if err := db.UpdateJobStatus(ctx, job.ID, models.JobStatusRunning); err != nil {
		t.Fatalf("UpdateJobStatus: %v", err)
	}
	if err := db.AssignJobToNode(ctx, job.ID, node.ID); err != nil {
		t.Fatalf("AssignJobToNode: %v", err)
	}
	_ = db.CompleteJob(ctx, job.ID, "{}", models.JobStatusCompleted)
}

// TestIntegration_StoreRoundTrip exercises more Store methods for coverage.
func TestIntegration_StoreRoundTrip(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.GetUserByHandle(ctx, "inttest-user")
	if err != nil {
		t.Skip("create inttest-user first (run TestIntegration_User)")
	}
	if _, err := db.GetUserByID(ctx, user.ID); err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	storeRoundTripCredentials(t, db, ctx, user.ID)
	storeRoundTripSessions(t, db, ctx, user.ID)
	node := storeRoundTripNode(t, db, ctx)
	storeRoundTripTaskJobNode(t, db, ctx, user.ID, node)
	if err := db.InvalidateAllUserSessions(ctx, user.ID); err != nil {
		t.Fatalf("InvalidateAllUserSessions: %v", err)
	}
	_ = db.UpdateNodeLastSeen(ctx, node.ID)
	_ = db.UpdateNodeCapability(ctx, node.ID, "hash")
	_ = db.SaveNodeCapabilitySnapshot(ctx, node.ID, `{}`)
}

func TestIntegration_GetUserByID_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetUserByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("GetUserByID: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetUserByHandle_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetUserByHandle(ctx, "nonexistent-handle-"+uuid.New().String())
	if err != ErrNotFound {
		t.Errorf("GetUserByHandle: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetNodeBySlug_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetNodeBySlug(ctx, "nonexistent-slug-"+uuid.New().String())
	if err != ErrNotFound {
		t.Errorf("GetNodeBySlug: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetNodeByID_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetNodeByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("GetNodeByID: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_CreateUser_DuplicateHandle(t *testing.T) {
	db, ctx := integrationDB(t)
	handle := "dup-handle-" + uuid.New().String()
	_, err := db.CreateUser(ctx, handle, nil)
	if err != nil {
		t.Fatalf("first CreateUser: %v", err)
	}
	_, err = db.CreateUser(ctx, handle, nil)
	if err == nil {
		t.Error("second CreateUser with same handle should fail")
	}
}

func TestIntegration_GetTaskByID_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetTaskByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("GetTaskByID: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetJobByID_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetJobByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("GetJobByID: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetPasswordCredentialByUserID_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "nocred-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	_, err = db.GetPasswordCredentialByUserID(ctx, user.ID)
	if err != ErrNotFound {
		t.Errorf("GetPasswordCredentialByUserID: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetActiveRefreshSession_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	_, err := db.GetActiveRefreshSession(ctx, []byte("nonexistent-token-hash"))
	if err != ErrNotFound {
		t.Errorf("GetActiveRefreshSession: expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_CreateJobWithID_DuplicateID_ReturnsError(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "job-dup-id", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	jobID := uuid.New()
	_, err = db.CreateJobWithID(ctx, task.ID, jobID, integrationTestPayloadX1)
	if err != nil {
		t.Fatalf("first CreateJobWithID: %v", err)
	}
	_, err = db.CreateJobWithID(ctx, task.ID, jobID, integrationTestPayloadX1)
	if err == nil {
		t.Error("second CreateJobWithID with same ID: expected error")
	}
}

func TestIntegration_ListTasksByUser_Empty(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "notasks-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	list, err := db.ListTasksByUser(ctx, user.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListTasksByUser: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("ListTasksByUser: expected empty, got %d", len(list))
	}
}

func TestIntegration_GetJobsByTaskID_Empty(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "no-jobs-task", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	jobs, err := db.GetJobsByTaskID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetJobsByTaskID: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("GetJobsByTaskID: expected empty, got %d", len(jobs))
	}
}

func TestIntegration_ChatThreadsAndMessages(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "chat-user-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	thread, err := db.GetOrCreateActiveChatThread(ctx, user.ID, nil)
	if err != nil {
		t.Fatalf("GetOrCreateActiveChatThread: %v", err)
	}
	if thread.ID == uuid.Nil || thread.UserID != user.ID {
		t.Errorf("thread: id or user_id mismatch")
	}
	msg, err := db.AppendChatMessage(ctx, thread.ID, "user", "hello", nil)
	if err != nil {
		t.Fatalf("AppendChatMessage: %v", err)
	}
	if msg.Role != "user" || msg.Content != "hello" {
		t.Errorf("message: role or content mismatch")
	}
	msg2, err := db.AppendChatMessage(ctx, thread.ID, "assistant", "hi back", nil)
	if err != nil {
		t.Fatalf("AppendChatMessage assistant: %v", err)
	}
	if msg2.Content != "hi back" {
		t.Errorf("assistant message content: got %q", msg2.Content)
	}
	rec := &models.ChatAuditLog{
		UserID:           &user.ID,
		Outcome:          "success",
		RedactionApplied: false,
	}
	if err := db.CreateChatAuditLog(ctx, rec); err != nil {
		t.Fatalf("CreateChatAuditLog: %v", err)
	}
	if rec.ID == uuid.Nil || rec.CreatedAt.IsZero() {
		t.Error("CreateChatAuditLog: expected ID and CreatedAt set")
	}
}

func TestIntegration_ChatThread_WithProjectID(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "chat-proj-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	now := time.Now().UTC()
	proj := &models.Project{
		ID:          uuid.New(),
		Slug:        "chat-proj-" + uuid.New().String()[:8],
		DisplayName: "Chat Project",
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.GORM().WithContext(ctx).Create(proj).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	thread, err := db.GetOrCreateActiveChatThread(ctx, user.ID, &proj.ID)
	if err != nil {
		t.Fatalf("GetOrCreateActiveChatThread(projectID): %v", err)
	}
	if thread.ProjectID == nil || *thread.ProjectID != proj.ID {
		t.Errorf("thread.ProjectID: got %v, want %s", thread.ProjectID, proj.ID)
	}
	thread2, err := db.GetOrCreateActiveChatThread(ctx, user.ID, &proj.ID)
	if err != nil {
		t.Fatalf("second GetOrCreateActiveChatThread: %v", err)
	}
	if thread2.ID != thread.ID {
		t.Errorf("reuse within cutoff: got thread id %s, want %s", thread2.ID, thread.ID)
	}
}

func TestIntegration_CreateRefreshSession_ReturnsSession(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "refresh-sess-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	expires := time.Now().UTC().Add(time.Hour)
	session, err := db.CreateRefreshSession(ctx, user.ID, []byte("tokenhash"), expires)
	if err != nil {
		t.Fatalf("CreateRefreshSession: %v", err)
	}
	if session == nil {
		t.Fatal("CreateRefreshSession: expected non-nil session")
	}
	if session.ID == uuid.Nil || session.UserID != user.ID {
		t.Errorf("CreateRefreshSession: got id=%s user_id=%s", session.ID, session.UserID)
	}
	if !session.IsActive || session.ExpiresAt.Before(time.Now().UTC()) {
		t.Errorf("CreateRefreshSession: IsActive=%v ExpiresAt=%v", session.IsActive, session.ExpiresAt)
	}
}

// workflowGateCreateProjectAndPlan creates a project and plan for workflow gate tests.
func workflowGateCreateProjectAndPlan(t *testing.T, db *DB, ctx context.Context, now time.Time, state string, archived bool) (*models.Project, uuid.UUID) {
	t.Helper()
	proj := &models.Project{
		ID:          uuid.New(),
		Slug:        "gate-proj-" + uuid.New().String()[:8],
		DisplayName: "Gate Project",
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.GORM().WithContext(ctx).Create(proj).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	planID := uuid.New()
	plan := &models.ProjectPlan{
		ID:        planID,
		ProjectID: proj.ID,
		State:     state,
		Archived:  archived,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.GORM().WithContext(ctx).Create(plan).Error; err != nil {
		t.Fatalf("create plan: %v", err)
	}
	return proj, planID
}

// workflowGateCreateTaskWithPlan creates a task with plan_id set.
func workflowGateCreateTaskWithPlan(t *testing.T, db *DB, ctx context.Context, planID uuid.UUID, slug string) *models.Task {
	t.Helper()
	task, err := db.CreateTask(ctx, nil, slug, nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.GORM().WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Update("plan_id", planID).Error; err != nil {
		t.Fatalf("update task plan_id: %v", err)
	}
	task, err = db.GetTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	return task
}

func TestIntegration_WorkflowStartGate_NoPlan(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-gate-noplan", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	deny, err := db.EvaluateWorkflowStartGate(ctx, task, false)
	if err != nil {
		t.Fatalf("EvaluateWorkflowStartGate: %v", err)
	}
	if deny != "" {
		t.Errorf("got deny=%q want allow", deny)
	}
}

func TestIntegration_WorkflowStartGate_PlanNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-gate-badplan", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	nonexistentPlanID := uuid.New()
	if err := db.GORM().WithContext(ctx).Model(&models.Task{}).Where("id = ?", task.ID).Update("plan_id", nonexistentPlanID).Error; err != nil {
		t.Fatalf("update: %v", err)
	}
	task, err = db.GetTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	deny, err := db.EvaluateWorkflowStartGate(ctx, task, false)
	if err != nil {
		t.Fatalf("EvaluateWorkflowStartGate: %v", err)
	}
	if deny != "plan not found" {
		t.Errorf("got deny=%q want plan not found", deny)
	}
}

func TestIntegration_WorkflowStartGate_PlanState(t *testing.T) {
	db, ctx := integrationDB(t)
	now := time.Now().UTC()
	cases := []struct {
		name       string
		state      string
		archived   bool
		pma        bool
		wantDeny   string
	}{
		{"draft_deny", "draft", false, false, "plan not active"},
		{"draft_pma_allow", "draft", false, true, ""},
		{"archived_deny", "active", true, false, "plan is archived"},
		{"active_allow", "active", false, false, ""},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			_, planID := workflowGateCreateProjectAndPlan(t, db, ctx, now, c.state, c.archived)
			task := workflowGateCreateTaskWithPlan(t, db, ctx, planID, "workflow-gate-"+c.name)
			deny, err := db.EvaluateWorkflowStartGate(ctx, task, c.pma)
			if err != nil {
				t.Fatalf("EvaluateWorkflowStartGate: %v", err)
			}
			if deny != c.wantDeny {
				t.Errorf("got deny=%q want %q", deny, c.wantDeny)
			}
		})
	}
}

func TestIntegration_WorkflowStartGate_DepsNotSatisfied(t *testing.T) {
	db, ctx := integrationDB(t)
	now := time.Now().UTC()
	_, planID := workflowGateCreateProjectAndPlan(t, db, ctx, now, "active", false)
	task := workflowGateCreateTaskWithPlan(t, db, ctx, planID, "workflow-gate-deps")
	depTask, err := db.CreateTask(ctx, nil, "gate-dep-task", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask dep: %v", err)
	}
	if err := db.GORM().WithContext(ctx).Model(&models.Task{}).Where("id = ?", depTask.ID).Update("plan_id", planID).Error; err != nil {
		t.Fatalf("update dep plan_id: %v", err)
	}
	depRow := &models.TaskDependency{ID: uuid.New(), TaskID: task.ID, DependsOnTaskID: depTask.ID}
	if err := db.GORM().WithContext(ctx).Create(depRow).Error; err != nil {
		t.Fatalf("create task_dependency: %v", err)
	}
	task, err = db.GetTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	deny, err := db.EvaluateWorkflowStartGate(ctx, task, false)
	if err != nil {
		t.Fatalf("EvaluateWorkflowStartGate: %v", err)
	}
	if deny != "dependencies not satisfied" {
		t.Errorf("got deny=%q want dependencies not satisfied", deny)
	}
}

func TestIntegration_WorkflowStartGate_DepTaskNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	now := time.Now().UTC()
	_, planID := workflowGateCreateProjectAndPlan(t, db, ctx, now, "active", false)
	task := workflowGateCreateTaskWithPlan(t, db, ctx, planID, "workflow-gate-dep-missing")
	missingDepID := uuid.New()
	depRow := &models.TaskDependency{ID: uuid.New(), TaskID: task.ID, DependsOnTaskID: missingDepID}
	if err := db.GORM().WithContext(ctx).Create(depRow).Error; err != nil {
		t.Fatalf("create task_dependency: %v", err)
	}
	deny, err := db.EvaluateWorkflowStartGate(ctx, task, false)
	if err != nil {
		t.Fatalf("EvaluateWorkflowStartGate: %v", err)
	}
	if deny != "dependency task not found" {
		t.Errorf("got deny=%q want dependency task not found", deny)
	}
}

func TestIntegration_HasAnyActiveApiCredential_CancelledContext(t *testing.T) {
	db, _ := integrationDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.HasAnyActiveApiCredential(ctx)
	if err == nil {
		t.Error("HasAnyActiveApiCredential with cancelled context: expected error")
	}
}
