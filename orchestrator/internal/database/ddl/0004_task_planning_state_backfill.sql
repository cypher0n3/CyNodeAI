-- Backfill planning_state after GORM adds the column (default draft for existing rows).
-- See docs/tech_specs/orchestrator.md Task planning_state Migration.

UPDATE tasks
SET planning_state = 'ready'
WHERE planning_state = 'draft'
  AND (
    status IN ('running', 'completed', 'failed', 'canceled', 'superseded')
    OR EXISTS (SELECT 1 FROM jobs j WHERE j.task_id::uuid = tasks.id)
  );
