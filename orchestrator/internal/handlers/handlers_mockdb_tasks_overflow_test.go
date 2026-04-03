package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

//nolint:dupl // node registration body struct repeated across tests
func TestNodeHandler_handleExistingNodeDBError(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-1234567890123456", 15*time.Minute, 7*24*time.Hour, 24*time.Hour)
	logger := newTestLogger()

	node := &models.Node{
		NodeBase: models.NodeBase{
			NodeSlug: "test-node",
			Status:   "offline",
		},
		ID:        uuid.New(),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	mockDB := testutil.NewMockDB()
	mockDB.AddNode(node)
	mockDB.ForceError = errors.New("db error on update")

	handler := NewNodeHandler(mockDB, jwtMgr, "test-psk", testOrchestratorURL, "", "", "", nil, "", "", logger)

	body := nodepayloads.RegistrationRequest{
		PSK: "test-psk",
		Capability: nodepayloads.CapabilityReport{
			Version: 1,
			Node:    nodepayloads.CapabilityNode{NodeSlug: "test-node"},
			Platform: nodepayloads.Platform{
				OS:   "linux",
				Arch: "amd64",
			},
			Compute: nodepayloads.Compute{
				CPUCores: 4,
				RAMMB:    8192,
			},
		},
	}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/v1/nodes/register", bytes.NewBuffer(jsonBody))
	rec := httptest.NewRecorder()

	handler.Register(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

func TestTaskHandler_GetTaskResultJobsDBError(t *testing.T) {
	logger := newTestLogger()
	userID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&userID, models.TaskStatusCompleted, &prompt)

	mockDB := &errorOnJobsMockDB{
		MockDB: testutil.NewMockDB(),
	}
	mockDB.AddTask(task)

	handler := NewTaskHandler(mockDB, logger, "", "")

	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String()+"/result", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, userID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()

	handler.GetTaskResult(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}
}

// errorOnJobsMockDB wraps MockDB to return error only on GetJobsByTaskID
type errorOnJobsMockDB struct {
	*testutil.MockDB
}

func (m *errorOnJobsMockDB) GetJobsByTaskID(_ context.Context, _ uuid.UUID) ([]*models.Job, error) {
	return nil, errors.New("jobs query error")
}

func (m *errorOnJobsMockDB) ListJobsForTask(_ context.Context, _ uuid.UUID, _, _ int) ([]*models.Job, int64, error) {
	return nil, 0, errors.New("jobs query error")
}

func TestTaskHandler_GetTaskForbidden(t *testing.T) {
	mockDB := testutil.NewMockDB()
	logger := newTestLogger()
	handler := NewTaskHandler(mockDB, logger, "", "")
	ownerID := uuid.New()
	otherID := uuid.New()
	prompt := testPrompt
	task := newMockTask(&ownerID, models.TaskStatusPending, &prompt)
	mockDB.AddTask(task)
	req := httptest.NewRequest("GET", "/v1/tasks/"+task.ID.String(), http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, otherID))
	req.SetPathValue("id", task.ID.String())
	rec := httptest.NewRecorder()
	handler.GetTask(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}
