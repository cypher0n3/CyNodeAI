package userapi

import (
	"encoding/json"
	"testing"
)

func TestTaskResponse_ResolveTaskID(t *testing.T) {
	t.Run("prefers task_id", func(t *testing.T) {
		tr := TaskResponse{ID: "id1", TaskID: "task-1"}
		if got := tr.ResolveTaskID(); got != "task-1" {
			t.Errorf("ResolveTaskID() = %q, want task-1", got)
		}
	})
	t.Run("falls back to id", func(t *testing.T) {
		tr := TaskResponse{ID: "id1", TaskID: ""}
		if got := tr.ResolveTaskID(); got != "id1" {
			t.Errorf("ResolveTaskID() = %q, want id1", got)
		}
	})
}

func TestTaskResponse_JSONRoundtrip(t *testing.T) {
	tr := TaskResponse{
		ID:        "id1",
		TaskID:    "task-1",
		Status:    StatusQueued,
		CreatedAt: "2025-01-01T00:00:00Z",
		UpdatedAt: "2025-01-01T00:00:00Z",
	}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out TaskResponse
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ResolveTaskID() != tr.TaskID || out.Status != tr.Status {
		t.Errorf("roundtrip got %+v", out)
	}
}

func TestStatusConstants(t *testing.T) {
	// Ensure status constants are non-empty and stable for API consumers.
	if StatusQueued == "" || StatusRunning == "" || StatusCompleted == "" || StatusFailed == "" || StatusCanceled == "" {
		t.Error("status constants must be non-empty")
	}
}
