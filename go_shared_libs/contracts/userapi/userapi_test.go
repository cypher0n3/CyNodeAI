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

// TestSSEEventConstantsAndPayloads locks the CyNodeAI streaming SSE event names and payload shapes
// per CYNAI.USRGWY.OpenAIChatApi.StreamingPerEndpointSSEFormat.
func TestSSEEventConstantsAndPayloads(t *testing.T) {
	t.Run("cynodeai_event_names", testSSEEventNamesHavePrefix)
	t.Run("amendment_payloads", testSSEAmendmentPayloadRoundtrips)
	t.Run("heartbeat_payload", testSSEHeartbeatPayloadRoundtrip)
	t.Run("responses_stream_events", testResponsesStreamEventConstants)
}

func testSSEEventNamesHavePrefix(t *testing.T) {
	t.Helper()
	eventNames := []string{
		SSEEventThinkingDelta, SSEEventToolCall, SSEEventToolProgress,
		SSEEventIterationStart, SSEEventAmendment, SSEEventHeartbeat,
	}
	for _, name := range eventNames {
		if name == "" || len(name) < 5 {
			t.Errorf("SSE event constant empty or too short: %q", name)
		}
		if len(name) < 9 || name[:9] != "cynodeai." {
			t.Errorf("SSE event must have cynodeai. prefix: %q", name)
		}
	}
}

func testSSEAmendmentPayloadRoundtrips(t *testing.T) {
	t.Helper()
	var out SSEAmendmentPayload
	secretPayload := SSEAmendmentPayload{Type: "secret_redaction", Content: "redacted", RedactionKinds: []string{"api_key"}}
	b, _ := json.Marshal(secretPayload)
	if err := json.Unmarshal(b, &out); err != nil || out.Type != secretPayload.Type {
		t.Errorf("amendment secret_redaction roundtrip: %v", err)
	}
	iter := 1
	overwritePayload := SSEAmendmentPayload{Type: "overwrite", Content: "x", Scope: "iteration", Iteration: &iter}
	b, _ = json.Marshal(overwritePayload)
	if err := json.Unmarshal(b, &out); err != nil || out.Scope != "iteration" || out.Iteration == nil || *out.Iteration != 1 {
		t.Errorf("amendment overwrite roundtrip: %v", err)
	}
}

func testSSEHeartbeatPayloadRoundtrip(t *testing.T) {
	t.Helper()
	hb := SSEHeartbeatPayload{ElapsedS: 5, Status: "processing"}
	b, _ := json.Marshal(hb)
	var hbOut SSEHeartbeatPayload
	if err := json.Unmarshal(b, &hbOut); err != nil || hbOut.ElapsedS != 5 {
		t.Errorf("heartbeat roundtrip: %v", err)
	}
}

func testResponsesStreamEventConstants(t *testing.T) {
	t.Helper()
	if SSEEventResponseOutputTextDelta == "" || SSEEventResponseCompleted == "" {
		t.Error("responses SSE event constants must be non-empty")
	}
}
