package database

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// Plan state for workflow gate (langgraph_mvp.md, project_plans.state).
const planStateActive = "active"

func (db *DB) workflowGateCheckPlan(ctx context.Context, task *models.Task, requestedByPMA bool) (denyReason string, err error) {
	plan, err := db.getPlanByID(ctx, *task.PlanID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "plan not found", nil
		}
		return "", err
	}
	if plan.Archived {
		return "plan is archived", nil
	}
	if plan.State != planStateActive && !requestedByPMA {
		return "plan not active", nil
	}
	return "", nil
}

func (db *DB) workflowGateCheckDeps(ctx context.Context, taskID uuid.UUID) (denyReason string, err error) {
	depIDs, err := db.listTaskDependencyIDs(ctx, taskID)
	if err != nil {
		return "", err
	}
	for _, depID := range depIDs {
		t, err := db.GetTaskByID(ctx, depID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return "dependency task not found", nil
			}
			return "", err
		}
		if t.Status != models.TaskStatusCompleted {
			return "dependencies not satisfied", nil
		}
	}
	return "", nil
}

// EvaluateWorkflowStartGate evaluates the workflow start gate per REQ-ORCHES-0152, REQ-ORCHES-0153
// and langgraph_mvp.md WorkflowStartGatePlanApproved. If the task has no plan_id, allows start.
// Otherwise checks plan archived/state and task dependencies. Returns non-empty denyReason to deny with 409.
func (db *DB) EvaluateWorkflowStartGate(ctx context.Context, task *models.Task, requestedByPMA bool) (denyReason string, err error) {
	// REQ-ORCHES-0178, REQ-ORCHES-0180: only planning_state=ready may start workflow.
	ps := strings.TrimSpace(task.PlanningState)
	if ps != "" && ps != models.PlanningStateReady {
		return "task not ready for workflow", nil
	}
	if task.PlanID == nil {
		return "", nil
	}
	if deny, err := db.workflowGateCheckPlan(ctx, task, requestedByPMA); deny != "" || err != nil {
		return deny, err
	}
	return db.workflowGateCheckDeps(ctx, task.ID)
}

func (db *DB) getPlanByID(ctx context.Context, planID uuid.UUID) (*models.ProjectPlan, error) {
	return getByID[models.ProjectPlan](db, ctx, planID, "get plan by id")
}

func (db *DB) listTaskDependencyIDs(ctx context.Context, taskID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := db.db.WithContext(ctx).Model(&TaskDependencyRecord{}).
		Where("task_id = ?", taskID).
		Pluck("depends_on_task_id", &ids).Error
	if err != nil {
		return nil, wrapErr(err, "list task dependencies")
	}
	return ids, nil
}
