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

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

const integrationEnv = "POSTGRES_TEST_DSN"

const integrationTestPreferenceValue = `"v1"`

const integrationTestPayloadX1 = `{"x":1}`

func TestIntegration_User(t *testing.T) {
	db, ctx := integrationDB(t)
	user := integrationEnsureInttestUser(t, db, ctx)
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
	user := integrationEnsureInttestUser(t, db, ctx)
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
	all, err := db.ListNodes(ctx, 50, 0)
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	found := false
	for _, n := range all {
		if n.NodeSlug == node.NodeSlug {
			found = true
			break
		}
	}
	if !found {
		t.Error("ListNodes: expected created node in registry")
	}
}

func TestIntegration_SystemSettings(t *testing.T) {
	db, ctx := integrationDB(t)
	key := "inttest.setting." + uuid.New().String()
	integrationSystemSettingsGetCreate(t, db, ctx, key)
	keyA, keyB := integrationSystemSettingsListAndPaging(t, db, ctx)
	integrationSystemSettingsUpdateDelete(t, db, ctx, key, keyA, keyB)
}

func integrationSystemSettingsGetCreate(t *testing.T, db *DB, ctx context.Context, key string) {
	t.Helper()
	_, err := db.GetSystemSetting(ctx, key)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSystemSetting missing key: %v", err)
	}
	s, err := db.CreateSystemSetting(ctx, key, `"v"`, "string", nil, nil)
	if err != nil {
		t.Fatalf("CreateSystemSetting: %v", err)
	}
	if s.Version != 1 {
		t.Errorf("version = %d", s.Version)
	}
	got, err := db.GetSystemSetting(ctx, key)
	if err != nil {
		t.Fatalf("GetSystemSetting: %v", err)
	}
	if got.Key != key {
		t.Errorf("key = %q", got.Key)
	}
}

func integrationSystemSettingsListAndPaging(t *testing.T, db *DB, ctx context.Context) (keyA, keyB string) {
	t.Helper()
	keyA = "inttest.list.a." + uuid.New().String()
	keyB = "inttest.list.b." + uuid.New().String()
	if _, err := db.CreateSystemSetting(ctx, keyA, `"1"`, "string", nil, nil); err != nil {
		t.Fatalf("CreateSystemSetting: %v", err)
	}
	if _, err := db.CreateSystemSetting(ctx, keyB, `"2"`, "string", nil, nil); err != nil {
		t.Fatalf("CreateSystemSetting: %v", err)
	}
	listed, next, err := db.ListSystemSettings(ctx, "inttest.list.", 10, "")
	if err != nil {
		t.Fatalf("ListSystemSettings: %v", err)
	}
	if len(listed) < 2 {
		t.Fatalf("ListSystemSettings: want >=2 rows, got %d", len(listed))
	}
	if next != "" {
		t.Logf("next cursor: %q", next)
	}
	page1, nextCur, err := db.ListSystemSettings(ctx, "inttest.list.", 1, "")
	if err != nil || len(page1) != 1 || nextCur == "" {
		t.Fatalf("ListSystemSettings page1: err=%v len=%d next=%q", err, len(page1), nextCur)
	}
	if _, _, err := db.ListSystemSettings(ctx, "inttest.list.", 1, nextCur); err != nil {
		t.Fatalf("ListSystemSettings page2: %v", err)
	}
	if _, _, err := db.ListSystemSettings(ctx, "", 10, "not-a-number"); err != nil {
		t.Fatalf("ListSystemSettings invalid cursor: %v", err)
	}
	if rows, _, err := db.ListSystemSettings(ctx, "", 200, ""); err != nil {
		t.Fatalf("ListSystemSettings high limit: %v", err)
	} else if len(rows) > MaxSystemSettingListLimit {
		t.Fatalf("ListSystemSettings: got %d rows, cap %d", len(rows), MaxSystemSettingListLimit)
	}
	return keyA, keyB
}

func integrationSystemSettingsUpdateDelete(t *testing.T, db *DB, ctx context.Context, key, keyA, keyB string) {
	t.Helper()
	ev := 1
	updated, err := db.UpdateSystemSetting(ctx, key, `"w"`, "string", &ev, nil, nil)
	if err != nil || updated.Version != 2 {
		t.Fatalf("UpdateSystemSetting: %v %+v", err, updated)
	}
	if _, err := db.UpdateSystemSetting(ctx, key, `"z"`, "string", &ev, nil, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("UpdateSystemSetting want ErrConflict, got %v", err)
	}

	if err := db.DeleteSystemSetting(ctx, key, &ev, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("DeleteSystemSetting want ErrConflict, got %v", err)
	}
	if err := db.DeleteSystemSetting(ctx, key, &updated.Version, nil); err != nil {
		t.Fatalf("DeleteSystemSetting: %v", err)
	}
	if err := db.DeleteSystemSetting(ctx, keyA, nil, nil); err != nil {
		t.Fatalf("DeleteSystemSetting: %v", err)
	}
	if err := db.DeleteSystemSetting(ctx, keyB, nil, nil); err != nil {
		t.Fatalf("DeleteSystemSetting: %v", err)
	}
}

func TestIntegration_CreateSystemSetting_duplicateKey(t *testing.T) {
	db, ctx := integrationDB(t)
	k := "inttest.dup." + uuid.New().String()
	if _, err := db.CreateSystemSetting(ctx, k, `"v"`, "string", nil, nil); err != nil {
		t.Fatalf("CreateSystemSetting: %v", err)
	}
	if _, err := db.CreateSystemSetting(ctx, k, `"v2"`, "string", nil, nil); !errors.Is(err, ErrExists) {
		t.Fatalf("want ErrExists, got %v", err)
	}
}

func TestIntegration_ListDispatchableNodesAndListTasksByUser(t *testing.T) {
	db, ctx := integrationDB(t)
	user := integrationEnsureInttestUser(t, db, ctx)
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
	user := integrationEnsureInttestUser(t, db, ctx)
	reason := "test"
	if err := db.CreateAuthAuditLog(ctx, &user.ID, "login_success", true, nil, nil, nil, &reason); err != nil {
		t.Fatalf("CreateAuthAuditLog: %v", err)
	}
}

func TestIntegration_McpToolCallAuditLog(t *testing.T) {
	db, ctx := integrationDB(t)
	rec := &models.McpToolCallAuditLog{
		McpToolCallAuditLogBase: models.McpToolCallAuditLogBase{
			ToolName: "db.preference.get",
			Decision: "allow",
			Status:   "success",
		},
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
		PreferenceEntryBase: models.PreferenceEntryBase{
			ScopeType: "system",
			ScopeID:   nil,
			Key:       "test.key",
			Value:     &val,
			ValueType: "string",
			Version:   1,
		},
		ID:        uuid.New(),
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
		PreferenceEntryBase: models.PreferenceEntryBase{
			ScopeType: "system",
			Key:       "test.key",
			Value:     &val,
			ValueType: "string",
			Version:   1,
		},
		ID:        uuid.New(),
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
	user := integrationEnsureInttestUser(t, db, ctx)
	uv := `"user-val"`
	uent := &models.PreferenceEntry{
		PreferenceEntryBase: models.PreferenceEntryBase{
			ScopeType: "user",
			ScopeID:   &user.ID,
			Key:       "user.key",
			Value:     &uv,
			ValueType: "string",
			Version:   1,
		},
		ID:        uuid.New(),
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
	user := integrationEnsureInttestUser(t, db, ctx)
	var proj models.Project
	err := db.GORM().WithContext(ctx).Where("slug = ?", "default").First(&proj).Error
	if err != nil {
		pid := uuid.New()
		now := time.Now().UTC()
		_ = db.GORM().WithContext(ctx).Create(&ProjectRecord{
			GormModelUUID: gormmodel.GormModelUUID{ID: pid, CreatedAt: now, UpdatedAt: now},
			ProjectBase:   models.ProjectBase{Slug: "default", DisplayName: "Default", IsActive: true},
		}).Error
		task2, _ := db.CreateTask(ctx, &user.ID, "eff2", nil, nil)
		if task2 != nil {
			_ = db.GORM().WithContext(ctx).Model(&TaskRecord{}).Where("id = ?", task2.ID).Update("project_id", pid).Error
			_, _ = db.GetEffectivePreferencesForTask(ctx, task2.ID)
		}
	} else {
		task2, _ := db.CreateTask(ctx, &user.ID, "eff2", nil, nil)
		if task2 != nil {
			_ = db.GORM().WithContext(ctx).Model(&TaskRecord{}).Where("id = ?", task2.ID).Update("project_id", proj.ID).Error
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
			PreferenceEntryBase: models.PreferenceEntryBase{
				ScopeType: "system",
				ScopeID:   nil,
				Key:       fmt.Sprintf("cap.key.%d", i),
				Value:     &val,
				ValueType: "string",
				Version:   1,
			},
			ID:        uuid.New(),
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
		PreferenceEntryBase: models.PreferenceEntryBase{
			ScopeType: "system",
			ScopeID:   nil,
			Key:       "nil.val.key",
			Value:     nil, // nil value: effective still gets the key with nil
			ValueType: "string",
			Version:   1,
		},
		ID:        uuid.New(),
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
		TaskArtifactBase: models.TaskArtifactBase{
			TaskID:     task.ID,
			Path:       "out/report.md",
			StorageRef: "ref:abc",
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
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

// integrationEnsureInttestUser returns the shared integration user, creating it if missing.
// Tests must not rely on alphabetical order relative to TestIntegration_User.
func integrationEnsureInttestUser(t *testing.T, db *DB, ctx context.Context) *models.User {
	t.Helper()
	u, err := db.GetUserByHandle(ctx, "inttest-user")
	if err == nil {
		return u
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetUserByHandle: %v", err)
	}
	u, err = db.CreateUser(ctx, "inttest-user", nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return u
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
