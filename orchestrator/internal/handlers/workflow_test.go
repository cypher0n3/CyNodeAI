package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"log/slog"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestWorkflowHandler_Start_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	taskID := uuid.New()
	mock.AddTask(&models.Task{
		TaskBase: models.TaskBase{Status: models.TaskStatusPending},
		ID:       taskID,
	})
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	body := map[string]interface{}{"task_id": taskID.String(), "holder_id": "runner-1"}
	req := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Start(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Start: got status %d: %s", rec.Code, rec.Body.String())
	}
	var resp StartWorkflowResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RunID == "" || resp.Status != "started" {
		t.Errorf("Start response: %+v", resp)
	}
}

func TestWorkflowHandler_Start_WithExpiresInAndIdempotencyKey(t *testing.T) {
	mock := testutil.NewMockDB()
	taskID := uuid.New()
	mock.AddTask(&models.Task{
		TaskBase: models.TaskBase{Status: models.TaskStatusPending},
		ID:       taskID,
	})
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	exp := 120
	body := map[string]interface{}{
		"task_id": taskID.String(), "holder_id": "runner-1",
		"expires_in_sec": exp, "idempotency_key": uuid.New().String(),
	}
	req := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Start(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Start with expires_in_sec: got %d", rec.Code)
	}
}

func TestWorkflowHandler_Start_BadRequest(t *testing.T) {
	mock := testutil.NewMockDB()
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	body := map[string]interface{}{"task_id": uuid.New().String()}
	req := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Start(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Start missing holder_id: got %d", rec.Code)
	}

	req2 := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, map[string]interface{}{"task_id": "not-a-uuid", "holder_id": "r1"}))
	rec2 := httptest.NewRecorder()
	h.Start(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("Start invalid task_id: got %d", rec2.Code)
	}
}

func TestWorkflowHandler_Start_TaskNotFound(t *testing.T) {
	mock := testutil.NewMockDB()
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	body := map[string]interface{}{"task_id": uuid.New().String(), "holder_id": "runner-1"}
	req := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Start(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Start task not found: got %d", rec.Code)
	}
}

func TestWorkflowHandler_Start_GateDenied(t *testing.T) {
	mock := testutil.NewMockDB()
	taskID := uuid.New()
	planID := uuid.New()
	mock.AddTask(&models.Task{
		TaskBase: models.TaskBase{Status: models.TaskStatusPending, PlanID: &planID},
		ID:       taskID,
	})
	mock.EvaluateWorkflowStartGateDenyReason = "plan not active"
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	body := map[string]interface{}{"task_id": taskID.String(), "holder_id": "runner-1"}
	req := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Start(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("Start gate denied: got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWorkflowHandler_Start_GateError(t *testing.T) {
	mock := testutil.NewMockDB()
	taskID := uuid.New()
	mock.AddTask(&models.Task{
		TaskBase: models.TaskBase{Status: models.TaskStatusPending},
		ID:       taskID,
	})
	mock.EvaluateWorkflowStartGateErr = errors.New("db unavailable")
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	body := map[string]interface{}{"task_id": taskID.String(), "holder_id": "runner-1"}
	req := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Start(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Start gate error: got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWorkflowHandler_Start_DuplicateDifferentHolder_Conflict(t *testing.T) {
	mock := testutil.NewMockDB()
	taskID := uuid.New()
	mock.AddTask(&models.Task{
		TaskBase: models.TaskBase{Status: models.TaskStatusPending},
		ID:       taskID,
	})
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	body1 := map[string]interface{}{"task_id": taskID.String(), "holder_id": "runner-1"}
	req1 := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, body1))
	rec1 := httptest.NewRecorder()
	h.Start(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first Start: got %d", rec1.Code)
	}

	body2 := map[string]interface{}{"task_id": taskID.String(), "holder_id": "runner-2"}
	req2 := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, body2))
	rec2 := httptest.NewRecorder()
	h.Start(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Errorf("second Start (different holder): want 409, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestWorkflowHandler_Start_IdempotentAlreadyRunning(t *testing.T) {
	mock := testutil.NewMockDB()
	taskID := uuid.New()
	mock.AddTask(&models.Task{
		TaskBase: models.TaskBase{Status: models.TaskStatusPending},
		ID:       taskID,
	})
	leaseID := uuid.New()
	holder := "runner-1"
	exp := time.Now().UTC().Add(time.Hour)
	mock.TaskWorkflowLeases[taskID] = &models.TaskWorkflowLease{
		TaskWorkflowLeaseBase: models.TaskWorkflowLeaseBase{
			TaskID:    taskID,
			LeaseID:   leaseID,
			HolderID:  &holder,
			ExpiresAt: &exp,
		},
		ID: uuid.New(),
	}
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	body := map[string]interface{}{
		"task_id":         taskID.String(),
		"holder_id":       holder,
		"idempotency_key": leaseID.String(),
	}
	req := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Start(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Start idempotent: got %d: %s", rec.Code, rec.Body.String())
	}
	var resp StartWorkflowResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "already_running" {
		t.Errorf("want already_running, got %+v", resp)
	}
}

//nolint:dupl // same arrange/act/assert shape as SaveCheckpoint store error test (different mock + handler).
func TestWorkflowHandler_Start_GetWorkflowLeaseError_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	taskID := uuid.New()
	mock.AddTask(&models.Task{
		TaskBase: models.TaskBase{Status: models.TaskStatusPending},
		ID:       taskID,
	})
	mock.GetTaskWorkflowLeaseErr = errors.New("lease read failed")
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	body := map[string]interface{}{"task_id": taskID.String(), "holder_id": "runner-1"}
	req := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Start(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Start on GetTaskWorkflowLease error: got %d", rec.Code)
	}
}

func TestWorkflowHandler_Resume_NotFound(t *testing.T) {
	mock := testutil.NewMockDB()
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	body := map[string]interface{}{"task_id": uuid.New().String()}
	req := httptest.NewRequest("POST", "/v1/workflow/resume", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Resume(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("Resume no checkpoint: got %d", rec.Code)
	}
}

func TestWorkflowHandler_SaveCheckpoint_Release(t *testing.T) {
	mock := testutil.NewMockDB()
	taskID := uuid.New()
	mock.AddTask(&models.Task{
		TaskBase: models.TaskBase{Status: models.TaskStatusPending},
		ID:       taskID,
	})
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))

	cpBody := map[string]interface{}{"task_id": taskID.String(), "last_node_id": "plan_steps", "state": "{}"}
	reqCP := httptest.NewRequest("POST", "/v1/workflow/checkpoint", jsonBody(t, cpBody))
	recCP := httptest.NewRecorder()
	h.SaveCheckpoint(recCP, reqCP)
	if recCP.Code != http.StatusNoContent {
		t.Errorf("SaveCheckpoint: got %d", recCP.Code)
	}

	resumeBody := map[string]interface{}{"task_id": taskID.String()}
	reqResume := httptest.NewRequest("POST", "/v1/workflow/resume", jsonBody(t, resumeBody))
	recResume := httptest.NewRecorder()
	h.Resume(recResume, reqResume)
	if recResume.Code != http.StatusOK {
		t.Fatalf("Resume: got %d: %s", recResume.Code, recResume.Body.String())
	}
	var resp ResumeWorkflowResponse
	if err := json.NewDecoder(recResume.Body).Decode(&resp); err != nil {
		t.Fatalf("decode resume: %v", err)
	}
	if resp.LastNodeID != "plan_steps" {
		t.Errorf("Resume: got last_node_id %q", resp.LastNodeID)
	}

	startBody := map[string]interface{}{"task_id": taskID.String(), "holder_id": "r1"}
	reqStart := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, startBody))
	recStart := httptest.NewRecorder()
	h.Start(recStart, reqStart)
	if recStart.Code != http.StatusOK {
		t.Fatalf("Start: %d", recStart.Code)
	}
	var startResp StartWorkflowResponse
	_ = json.NewDecoder(recStart.Body).Decode(&startResp)

	relBody := map[string]interface{}{"task_id": taskID.String(), "lease_id": startResp.LeaseID}
	reqRel := httptest.NewRequest("POST", "/v1/workflow/release", jsonBody(t, relBody))
	recRel := httptest.NewRecorder()
	h.Release(recRel, reqRel)
	if recRel.Code != http.StatusNoContent {
		t.Errorf("Release: got %d", recRel.Code)
	}
}

func TestWorkflowHandler_Start_GetTaskError_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.GetTaskByIDErr = errors.New("db error")
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	body := map[string]interface{}{"task_id": uuid.New().String(), "holder_id": "r1"}
	req := httptest.NewRequest("POST", "/v1/workflow/start", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Start(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Start on GetTaskByID error: got %d", rec.Code)
	}
}

func TestWorkflowHandler_Resume_GetCheckpointError_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("db error")
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	body := map[string]interface{}{"task_id": uuid.New().String()}
	req := httptest.NewRequest("POST", "/v1/workflow/resume", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Resume(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Resume on GetWorkflowCheckpoint error: got %d", rec.Code)
	}
}

//nolint:dupl // same arrange/act/assert shape as Start lease error test (different mock + handler).
func TestWorkflowHandler_SaveCheckpoint_StoreError_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	taskID := uuid.New()
	mock.AddTask(&models.Task{
		TaskBase: models.TaskBase{Status: models.TaskStatusPending},
		ID:       taskID,
	})
	mock.ForceError = errors.New("db error")
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	body := map[string]interface{}{"task_id": taskID.String(), "last_node_id": "x"}
	req := httptest.NewRequest("POST", "/v1/workflow/checkpoint", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.SaveCheckpoint(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("SaveCheckpoint on Upsert error: got %d", rec.Code)
	}
}

func TestWorkflowHandler_Release_DBError_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("release failed")
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	body := map[string]interface{}{"task_id": uuid.New().String(), "lease_id": uuid.New().String()}
	req := httptest.NewRequest("POST", "/v1/workflow/release", jsonBody(t, body))
	rec := httptest.NewRecorder()
	h.Release(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Release on DB error: got %d", rec.Code)
	}
}

func TestWorkflowHandler_Release_BadRequest(t *testing.T) {
	mock := testutil.NewMockDB()
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	req := httptest.NewRequest("POST", "/v1/workflow/release", jsonBody(t, map[string]interface{}{"task_id": "bad", "lease_id": uuid.New().String()}))
	rec := httptest.NewRecorder()
	h.Release(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Release bad task_id: got %d", rec.Code)
	}
	req2 := httptest.NewRequest("POST", "/v1/workflow/release", jsonBody(t, map[string]interface{}{"task_id": uuid.New().String(), "lease_id": "bad"}))
	rec2 := httptest.NewRecorder()
	h.Release(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("Release bad lease_id: got %d", rec2.Code)
	}
}

func TestWorkflowHandler_Resume_BadRequest(t *testing.T) {
	mock := testutil.NewMockDB()
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	req := httptest.NewRequest("POST", "/v1/workflow/resume", jsonBody(t, map[string]interface{}{"task_id": "not-uuid"}))
	rec := httptest.NewRecorder()
	h.Resume(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Resume bad task_id: got %d", rec.Code)
	}
}

func TestWorkflowHandler_WrongMethod(t *testing.T) {
	mock := testutil.NewMockDB()
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	for _, method := range []string{"GET", "PUT", "PATCH", "DELETE"} {
		req := httptest.NewRequest(method, "/v1/workflow/start", http.NoBody)
		rec := httptest.NewRecorder()
		h.Start(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("Start %s: got %d", method, rec.Code)
		}
		reqResume := httptest.NewRequest(method, "/v1/workflow/resume", jsonBody(t, map[string]string{"task_id": uuid.New().String()}))
		recResume := httptest.NewRecorder()
		h.Resume(recResume, reqResume)
		if recResume.Code != http.StatusMethodNotAllowed {
			t.Errorf("Resume %s: got %d", method, recResume.Code)
		}
		reqCP := httptest.NewRequest(method, "/v1/workflow/checkpoint", http.NoBody)
		recCP := httptest.NewRecorder()
		h.SaveCheckpoint(recCP, reqCP)
		if recCP.Code != http.StatusMethodNotAllowed {
			t.Errorf("SaveCheckpoint %s: got %d", method, recCP.Code)
		}
		reqRel := httptest.NewRequest(method, "/v1/workflow/release", jsonBody(t, map[string]string{"task_id": uuid.New().String(), "lease_id": uuid.New().String()}))
		recRel := httptest.NewRecorder()
		h.Release(recRel, reqRel)
		if recRel.Code != http.StatusMethodNotAllowed {
			t.Errorf("Release %s: got %d", method, recRel.Code)
		}
	}
}

func TestWorkflowHandler_SaveCheckpoint_BadRequest(t *testing.T) {
	mock := testutil.NewMockDB()
	h := NewWorkflowHandler(mock, slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})))
	req := httptest.NewRequest("POST", "/v1/workflow/checkpoint", bytes.NewBufferString("not json"))
	rec := httptest.NewRecorder()
	h.SaveCheckpoint(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("SaveCheckpoint invalid JSON: got %d", rec.Code)
	}
	req2 := httptest.NewRequest("POST", "/v1/workflow/checkpoint", jsonBody(t, map[string]interface{}{"task_id": "not-uuid", "last_node_id": "n1"}))
	rec2 := httptest.NewRecorder()
	h.SaveCheckpoint(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("SaveCheckpoint invalid task_id: got %d", rec2.Code)
	}
}

func jsonBody(t *testing.T, v interface{}) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewBuffer(b)
}
