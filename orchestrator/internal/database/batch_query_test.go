package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// registerQueryCounter increments on each GORM query callback (SELECT path). Returns a snapshot function.
func registerQueryCounter(gdb *gorm.DB, t *testing.T) func() int {
	t.Helper()
	var n int
	name := fmt.Sprintf("batch_query_count_%d", time.Now().UnixNano())
	if err := gdb.Callback().Query().After("gorm:query").Register(name, func(*gorm.DB) {
		n++
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}
	t.Cleanup(func() {
		if err := gdb.Callback().Query().Remove(name); err != nil {
			t.Errorf("remove query callback: %v", err)
		}
	})
	return func() int { return n }
}

func TestIntegration_BatchQuery_WorkflowGateDeps_AtMostTwoQueries(t *testing.T) {
	db, ctx := integrationDB(t)
	now := time.Now().UTC()
	_, planID := workflowGateCreateProjectAndPlan(t, db, ctx, now, "active", false)
	task := workflowGateCreateTaskWithPlan(t, db, ctx, planID, "batch-gate-main")
	for i := 0; i < 5; i++ {
		dep, err := db.CreateTask(ctx, nil, fmt.Sprintf("batch-dep-%d", i), nil, nil)
		if err != nil {
			t.Fatalf("CreateTask: %v", err)
		}
		if err := db.UpdateTaskStatus(ctx, dep.ID, models.TaskStatusCompleted); err != nil {
			t.Fatalf("UpdateTaskStatus: %v", err)
		}
		depRow := &models.TaskDependency{
			TaskDependencyBase: models.TaskDependencyBase{
				TaskID:          task.ID,
				DependsOnTaskID: dep.ID,
			},
			ID: uuid.New(),
		}
		if err := db.GORM().WithContext(ctx).Create(depRow).Error; err != nil {
			t.Fatalf("create dep: %v", err)
		}
	}
	counter := registerQueryCounter(db.GORM(), t)
	if _, err := db.workflowGateCheckDeps(ctx, task.ID); err != nil {
		t.Fatalf("workflowGateCheckDeps: %v", err)
	}
	if n := counter(); n > 2 {
		t.Fatalf("workflowGateCheckDeps issued %d queries, want at most 2", n)
	}
}

func TestIntegration_BatchQuery_EffectivePreferences_AtMostTwoQueries(t *testing.T) {
	db, ctx := integrationDB(t)
	user := integrationEnsureInttestUser(t, db, ctx)
	task, err := db.CreateTask(ctx, &user.ID, "pref-batch", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	counter := registerQueryCounter(db.GORM(), t)
	if _, err := db.GetEffectivePreferencesForTask(ctx, task.ID); err != nil {
		t.Fatalf("GetEffectivePreferencesForTask: %v", err)
	}
	if n := counter(); n > 2 {
		t.Fatalf("GetEffectivePreferencesForTask issued %d queries, want at most 2", n)
	}
}

func TestIntegration_ListPreferenceEntriesForScopes_Empty(t *testing.T) {
	db, ctx := integrationDB(t)
	got, err := db.listPreferenceEntriesForScopes(ctx, nil)
	if err != nil {
		t.Fatalf("listPreferenceEntriesForScopes: %v", err)
	}
	if got != nil {
		t.Fatalf("want nil, got len=%d", len(got))
	}
}

func TestIntegration_GetTasksByIDs_CanceledContext(t *testing.T) {
	db, _ := integrationDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.getTasksByIDs(ctx, []uuid.UUID{uuid.New()})
	if err == nil {
		t.Fatal("getTasksByIDs: expected error when context canceled")
	}
}

func TestIntegration_GetTasksByIDs_EmptyAndLookup(t *testing.T) {
	db, ctx := integrationDB(t)
	m, err := db.getTasksByIDs(ctx, nil)
	if err != nil {
		t.Fatalf("getTasksByIDs(nil): %v", err)
	}
	if len(m) != 0 {
		t.Fatalf("getTasksByIDs(nil): len=%d", len(m))
	}
	user := integrationEnsureInttestUser(t, db, ctx)
	task, err := db.CreateTask(ctx, &user.ID, "lookup-batch", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	m2, err := db.getTasksByIDs(ctx, []uuid.UUID{task.ID})
	if err != nil {
		t.Fatalf("getTasksByIDs: %v", err)
	}
	got, ok := m2[task.ID]
	if !ok || got.ID != task.ID {
		t.Fatalf("getTasksByIDs: missing task %+v", got)
	}
}

func TestIntegration_BatchQuery_CreateTaskDuplicateSummary_BoundedQueries(t *testing.T) {
	db, ctx := integrationDB(t)
	user := integrationEnsureInttestUser(t, db, ctx)
	name := "shared-uniqueness-name"
	if _, err := db.CreateTask(ctx, &user.ID, "p1", &name, nil); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	counter := registerQueryCounter(db.GORM(), t)
	if _, err := db.CreateTask(ctx, &user.ID, "p2", &name, nil); err != nil {
		t.Fatalf("CreateTask second: %v", err)
	}
	if n := counter(); n > 12 {
		t.Fatalf("second CreateTask issued %d queries, want a small bounded number (batched uniqueness)", n)
	}
}
