// Integration tests: chat threads, workflow start gates, access-control listing, API-credential context.
// Run with a real Postgres:
//
//	POSTGRES_TEST_DSN="postgres://..." go test -v -run Integration ./internal/database
package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func TestIntegration_ChatThread_WithProjectID(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "chat-proj-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	now := time.Now().UTC()
	proj := &models.Project{
		ProjectBase: models.ProjectBase{
			Slug:        "chat-proj-" + uuid.New().String()[:8],
			DisplayName: "Chat Project",
			IsActive:    true,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
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
		ProjectBase: models.ProjectBase{
			Slug:        "gate-proj-" + uuid.New().String()[:8],
			DisplayName: "Gate Project",
			IsActive:    true,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.GORM().WithContext(ctx).Create(proj).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	planID := uuid.New()
	plan := &models.ProjectPlan{
		ProjectPlanBase: models.ProjectPlanBase{
			ProjectID: proj.ID,
			State:     state,
			Archived:  archived,
		},
		ID:        planID,
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
	if err := db.GORM().WithContext(ctx).Model(&TaskRecord{}).Where("id = ?", task.ID).Update("plan_id", planID).Error; err != nil {
		t.Fatalf("update task plan_id: %v", err)
	}
	if err := db.UpdateTaskPlanningState(ctx, task.ID, models.PlanningStateReady); err != nil {
		t.Fatalf("UpdateTaskPlanningState: %v", err)
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
	if err := db.UpdateTaskPlanningState(ctx, task.ID, models.PlanningStateReady); err != nil {
		t.Fatalf("UpdateTaskPlanningState: %v", err)
	}
	task, err = db.GetTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	deny, err := db.EvaluateWorkflowStartGate(ctx, task, false)
	if err != nil {
		t.Fatalf("EvaluateWorkflowStartGate: %v", err)
	}
	if deny != "" {
		t.Errorf("got deny=%q want allow", deny)
	}
}

func TestIntegration_WorkflowStartGate_PlanningDraftDenied(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-gate-draft", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	deny, err := db.EvaluateWorkflowStartGate(ctx, task, false)
	if err != nil {
		t.Fatalf("EvaluateWorkflowStartGate: %v", err)
	}
	if deny != "task not ready for workflow" {
		t.Errorf("got deny=%q want task not ready for workflow", deny)
	}
}

func TestIntegration_WorkflowStartGate_PlanNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	task, err := db.CreateTask(ctx, nil, "workflow-gate-badplan", nil, nil)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := db.UpdateTaskPlanningState(ctx, task.ID, models.PlanningStateReady); err != nil {
		t.Fatalf("UpdateTaskPlanningState: %v", err)
	}
	task, err = db.GetTaskByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTaskByID: %v", err)
	}
	nonexistentPlanID := uuid.New()
	if err := db.GORM().WithContext(ctx).Model(&TaskRecord{}).Where("id = ?", task.ID).Update("plan_id", nonexistentPlanID).Error; err != nil {
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
		name     string
		state    string
		archived bool
		pma      bool
		wantDeny string
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
	if err := db.GORM().WithContext(ctx).Model(&TaskRecord{}).Where("id = ?", depTask.ID).Update("plan_id", planID).Error; err != nil {
		t.Fatalf("update dep plan_id: %v", err)
	}
	depRow := &models.TaskDependency{
		TaskDependencyBase: models.TaskDependencyBase{
			TaskID:          task.ID,
			DependsOnTaskID: depTask.ID,
		},
		ID: uuid.New(),
	}
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
	depRow := &models.TaskDependency{
		TaskDependencyBase: models.TaskDependencyBase{
			TaskID:          task.ID,
			DependsOnTaskID: missingDepID,
		},
		ID: uuid.New(),
	}
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

func TestIntegration_HasAnyActiveApiCredential_CanceledContext(t *testing.T) {
	db, _ := integrationDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.HasAnyActiveApiCredential(ctx)
	if err == nil {
		t.Error("HasAnyActiveApiCredential with canceled context: expected error")
	}
}

func TestIntegration_CreateChatThread(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "ct-user-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	t1, err := db.CreateChatThread(ctx, user.ID, nil, nil)
	if err != nil {
		t.Fatalf("CreateChatThread: %v", err)
	}
	if t1.ID == uuid.Nil || t1.UserID != user.ID {
		t.Errorf("thread1: id or user_id mismatch")
	}
	// Second call must create a distinct thread even within the inactivity window.
	t2, err := db.CreateChatThread(ctx, user.ID, nil, nil)
	if err != nil {
		t.Fatalf("CreateChatThread second: %v", err)
	}
	if t1.ID == t2.ID {
		t.Error("CreateChatThread returned the same ID twice; expected distinct threads")
	}
	// GetOrCreateActive should still return t2 (the newest one) since it is within the window.
	active, err := db.GetOrCreateActiveChatThread(ctx, user.ID, nil)
	if err != nil {
		t.Fatalf("GetOrCreateActiveChatThread after CreateChatThread: %v", err)
	}
	if active.ID != t2.ID {
		t.Errorf("active thread after CreateChatThread = %s, want %s", active.ID, t2.ID)
	}
}

func assertListChatMessages(t *testing.T, db *DB, ctx context.Context, threadID uuid.UUID) {
	t.Helper()
	msgs, err := db.ListChatMessages(ctx, threadID, 0)
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("ListChatMessages: expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Errorf("ListChatMessages order: got [%s, %s]", msgs[0].Role, msgs[1].Role)
	}
	limited, err := db.ListChatMessages(ctx, threadID, 1)
	if err != nil {
		t.Fatalf("ListChatMessages (limit 1): %v", err)
	}
	if len(limited) != 1 {
		t.Errorf("ListChatMessages (limit 1): expected 1, got %d", len(limited))
	}
}

func TestIntegration_ListChatThreads_GetChatThread_UpdateTitle_GetThreadByResponseID(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "lct-user-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	thread, err := db.CreateChatThread(ctx, user.ID, nil, nil)
	if err != nil {
		t.Fatalf("CreateChatThread: %v", err)
	}
	list, err := db.ListChatThreads(ctx, user.ID, nil, 10, 0)
	if err != nil {
		t.Fatalf("ListChatThreads: %v", err)
	}
	if len(list) < 1 {
		t.Fatalf("ListChatThreads: expected at least 1, got %d", len(list))
	}
	got, err := db.GetChatThreadByID(ctx, thread.ID, user.ID)
	if err != nil {
		t.Fatalf("GetChatThreadByID: %v", err)
	}
	if got.ID != thread.ID {
		t.Errorf("GetChatThreadByID: got %s, want %s", got.ID, thread.ID)
	}
	err = db.UpdateChatThreadTitle(ctx, thread.ID, user.ID, "New Title")
	if err != nil {
		t.Fatalf("UpdateChatThreadTitle: %v", err)
	}
	got, _ = db.GetChatThreadByID(ctx, thread.ID, user.ID)
	if got.Title == nil || *got.Title != "New Title" {
		t.Errorf("UpdateChatThreadTitle: got title %v", got.Title)
	}
	// GetThreadByResponseID (GORM jsonb containment) so integration can assert it.
	respID := uuid.New().String()
	meta := fmt.Sprintf(`{"response_id":%q}`, respID)
	_, err = db.AppendChatMessage(ctx, thread.ID, "assistant", "reply", &meta)
	if err != nil {
		t.Fatalf("AppendChatMessage(metadata): %v", err)
	}
	resolved, err := db.GetThreadByResponseID(ctx, respID, user.ID)
	if err != nil {
		t.Fatalf("GetThreadByResponseID: %v", err)
	}
	if resolved.ID != thread.ID {
		t.Errorf("GetThreadByResponseID: got thread %s, want %s", resolved.ID, thread.ID)
	}
}

func TestIntegration_CreateChatThread_WithTitle(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "ct-title-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	title := "My Chat"
	thread, err := db.CreateChatThread(ctx, user.ID, nil, &title)
	if err != nil {
		t.Fatalf("CreateChatThread(with title): %v", err)
	}
	if thread.Title == nil || *thread.Title != title {
		t.Errorf("CreateChatThread title: got %v", thread.Title)
	}
}

func TestIntegration_ListAccessControlRulesForApiCall(t *testing.T) {
	db, ctx := integrationDB(t)
	now := time.Now().UTC()
	rule := &models.AccessControlRule{
		AccessControlRuleBase: models.AccessControlRuleBase{
			SubjectType:     "user",
			SubjectID:       nil,
			Action:          ActionApiCall,
			ResourceType:    ResourceTypeProviderOperation,
			ResourcePattern: "*",
			Effect:          "allow",
			Priority:        10,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.GORM().WithContext(ctx).Create(rule).Error; err != nil {
		t.Fatalf("create access control rule: %v", err)
	}
	rules, err := db.ListAccessControlRulesForApiCall(ctx, "user", nil, ActionApiCall, ResourceTypeProviderOperation)
	if err != nil {
		t.Fatalf("ListAccessControlRulesForApiCall: %v", err)
	}
	if len(rules) < 1 {
		t.Errorf("ListAccessControlRulesForApiCall: expected at least 1 rule")
	}
	user, _ := db.CreateUser(ctx, "acr-user-"+uuid.New().String(), nil)
	rule2 := &models.AccessControlRule{
		AccessControlRuleBase: models.AccessControlRuleBase{
			SubjectType:     "user",
			SubjectID:       &user.ID,
			Action:          ActionApiCall,
			ResourceType:    ResourceTypeProviderOperation,
			ResourcePattern: "*",
			Effect:          "allow",
			Priority:        5,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.GORM().WithContext(ctx).Create(rule2).Error; err != nil {
		t.Fatalf("create rule with subject_id: %v", err)
	}
	rules2, err := db.ListAccessControlRulesForApiCall(ctx, "user", &user.ID, ActionApiCall, ResourceTypeProviderOperation)
	if err != nil {
		t.Fatalf("ListAccessControlRulesForApiCall(subjectID): %v", err)
	}
	if len(rules2) < 1 {
		t.Errorf("ListAccessControlRulesForApiCall(subjectID): expected at least 1")
	}
}

func TestIntegration_GetThreadByResponseID_EmptyID(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "gtr-user-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	_, err = db.GetThreadByResponseID(ctx, "", user.ID)
	if err == nil {
		t.Fatal("expected error for empty response id")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_GetChatThreadByID_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "gct-user-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	_, err = db.GetChatThreadByID(ctx, uuid.New(), user.ID)
	if err == nil {
		t.Fatal("expected error for non-existent thread")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_UpdateChatThreadTitle_ErrNotFound(t *testing.T) {
	db, ctx := integrationDB(t)
	user1, _ := db.CreateUser(ctx, "uct-u1-"+uuid.New().String(), nil)
	user2, _ := db.CreateUser(ctx, "uct-u2-"+uuid.New().String(), nil)
	thread, _ := db.CreateChatThread(ctx, user1.ID, nil, nil)
	err := db.UpdateChatThreadTitle(ctx, thread.ID, user2.ID, "Other")
	if err == nil {
		t.Fatal("expected error when updating another user's thread")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_ListChatThreads_WithProjectAndOffset(t *testing.T) {
	db, ctx := integrationDB(t)
	user, err := db.CreateUser(ctx, "lct-p-"+uuid.New().String(), nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	now := time.Now().UTC()
	proj := &models.Project{
		ProjectBase: models.ProjectBase{
			Slug:        "lct-proj-" + uuid.New().String()[:8],
			DisplayName: "List Threads Project",
			IsActive:    true,
		},
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.GORM().WithContext(ctx).Create(proj).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	_, _ = db.CreateChatThread(ctx, user.ID, &proj.ID, nil)
	_, _ = db.CreateChatThread(ctx, user.ID, &proj.ID, nil)
	list, err := db.ListChatThreads(ctx, user.ID, &proj.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListChatThreads(projectID): %v", err)
	}
	if len(list) < 2 {
		t.Errorf("ListChatThreads(projectID): expected at least 2, got %d", len(list))
	}
	list1, err := db.ListChatThreads(ctx, user.ID, &proj.ID, 1, 0)
	if err != nil {
		t.Fatalf("ListChatThreads(limit 1): %v", err)
	}
	if len(list1) != 1 {
		t.Errorf("ListChatThreads(limit 1): expected 1, got %d", len(list1))
	}
	listOffset, err := db.ListChatThreads(ctx, user.ID, &proj.ID, 10, 1)
	if err != nil {
		t.Fatalf("ListChatThreads(offset 1): %v", err)
	}
	if len(listOffset) != len(list)-1 {
		t.Errorf("ListChatThreads(offset 1): expected %d, got %d", len(list)-1, len(listOffset))
	}
}
