package mcpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestToolCallHandler_TaskGet_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(context.Background(), nil, "prompt", nil)
	body := `{"tool_name":"task.get","arguments":{"task_id":"` + task.ID.String() + `"}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d", code)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["status"] != "pending" {
		t.Errorf("expected status pending, got %v", out["status"])
	}
}

func TestToolCallHandler_TaskGet_NotFound(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"task.get","arguments":{"task_id":"`+uuid.New().String()+`"}}`, http.StatusNotFound)
}

func TestToolCallHandler_JobGet_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(context.Background(), nil, "p", nil)
	job, _ := mock.CreateJob(context.Background(), task.ID, `{"cmd":"x"}`)
	body := `{"tool_name":"job.get","arguments":{"job_id":"` + job.ID.String() + `"}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d", code)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["status"] != "queued" {
		t.Errorf("expected status queued, got %v", out["status"])
	}
}

func TestToolCallHandler_JobGet_NotFound(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"job.get","arguments":{"job_id":"`+uuid.New().String()+`"}}`, http.StatusNotFound)
}
func TestToolCallHandler_TaskGet_BadArgs(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"task.get","arguments":{}}`, http.StatusBadRequest)
}
func TestToolCallHandler_TaskList_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	_, _ = mock.CreateTask(context.Background(), &user.ID, "p", nil)
	body := `{"tool_name":"task.list","arguments":{"user_id":"` + user.ID.String() + `"}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d body %s", code, respBody)
	}
	var out struct {
		Tasks []map[string]interface{} `json:"tasks"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Tasks) < 1 {
		t.Errorf("expected at least one task")
	}
}

func TestToolCallHandler_TaskResultAndLogs_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(context.Background(), nil, "p", nil)
	for _, tc := range []struct {
		name string
		tool string
	}{
		{"task.result", "task.result"},
		{"task.logs", "task.logs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"tool_name":"` + tc.tool + `","arguments":{"task_id":"` + task.ID.String() + `"}}`
			code, _ := callToolHandlerWithStoreAndBody(t, mock, body)
			if code != http.StatusOK {
				t.Fatalf("got status %d", code)
			}
		})
	}
}

func TestToolCallHandler_TaskCancel_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(context.Background(), nil, "p", nil)
	body := `{"tool_name":"task.cancel","arguments":{"task_id":"` + task.ID.String() + `"}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d", code)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["canceled"] != true {
		t.Errorf("expected canceled true")
	}
}

func TestToolCallHandler_ProjectGet_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	proj, _ := mock.GetOrCreateDefaultProjectForUser(context.Background(), user.ID)
	body := `{"tool_name":"project.get","arguments":{"user_id":"` + user.ID.String() + `","project_id":"` + proj.ID.String() + `"}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d %s", code, respBody)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["slug"] == nil {
		t.Error("expected slug")
	}
}
func TestToolCallHandler_JobGet_BadArgs(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"job.get","arguments":{}}`, http.StatusBadRequest)
}
func TestToolCallHandler_TaskGet_InternalError(t *testing.T) {
	mock, taskID := mockWithTask(t)
	mock.GetTaskByIDErr = errors.New("db error")
	callToolHandlerWithStore(t, mock, `{"tool_name":"task.get","arguments":{"task_id":"`+taskID.String()+`"}}`, http.StatusInternalServerError)
}

func TestToolCallHandler_JobGet_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(context.Background(), nil, "p", nil)
	job, _ := mock.CreateJob(context.Background(), task.ID, "{}")
	mock.GetJobByIDErr = errors.New("db error")
	body := `{"tool_name":"job.get","arguments":{"job_id":"` + job.ID.String() + `"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusInternalServerError)
}
func TestHandleTaskAndProjectTools_DirectValidation(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	rec := &models.McpToolCallAuditLog{}

	code, _, _ := handleTaskList(ctx, mock, map[string]interface{}{}, rec)
	if code != http.StatusBadRequest {
		t.Errorf("task.list no user_id: %d", code)
	}
	uid := uuid.New()
	code, _, _ = handleTaskList(ctx, mock, map[string]interface{}{"user_id": uid.String(), "cursor": "not-int"}, rec)
	if code != http.StatusBadRequest {
		t.Errorf("task.list bad cursor: %d", code)
	}

	code, _, _ = handleTaskResult(ctx, mock, map[string]interface{}{}, rec)
	if code != http.StatusBadRequest {
		t.Errorf("task.result no task_id: %d", code)
	}
	code, _, _ = handleTaskLogs(ctx, mock, map[string]interface{}{}, rec)
	if code != http.StatusBadRequest {
		t.Errorf("task.logs no task_id: %d", code)
	}
	code, _, _ = handleTaskCancel(ctx, mock, map[string]interface{}{}, rec)
	if code != http.StatusBadRequest {
		t.Errorf("task.cancel no task_id: %d", code)
	}
	code, _, _ = handleTaskCancel(ctx, mock, map[string]interface{}{"task_id": uuid.New().String()}, rec)
	if code != http.StatusNotFound {
		t.Errorf("task.cancel missing task: %d", code)
	}

	code, _, _ = handleProjectGet(ctx, mock, map[string]interface{}{"user_id": uid.String(), "project_id": uuid.New().String(), "slug": "x"}, rec)
	if code != http.StatusBadRequest {
		t.Errorf("project.get both id and slug: %d", code)
	}
	code, _, _ = handleProjectList(ctx, mock, map[string]interface{}{}, rec)
	if code != http.StatusBadRequest {
		t.Errorf("project.list no user_id: %d", code)
	}

	code, _, _ = handleTaskResult(ctx, mock, map[string]interface{}{"task_id": uuid.New().String()}, rec)
	if code != http.StatusNotFound {
		t.Errorf("task.result not found: %d", code)
	}
	code, _, _ = handleTaskLogs(ctx, mock, map[string]interface{}{"task_id": uuid.New().String()}, rec)
	if code != http.StatusNotFound {
		t.Errorf("task.logs not found: %d", code)
	}
	code, _, _ = handleTaskLogs(ctx, mock, map[string]interface{}{"task_id": uuid.New().String(), "stream": "stdout"}, rec)
	if code != http.StatusNotFound {
		t.Errorf("task.logs stream not found: %d", code)
	}
}
func TestHandleTaskGet_DirectOK(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	task, err := mock.CreateTask(ctx, nil, "prompt", nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := &models.McpToolCallAuditLog{}
	code, body, _ := handleTaskGet(ctx, mock, map[string]interface{}{"task_id": task.ID.String()}, rec)
	if code != http.StatusOK {
		t.Fatalf("handleTaskGet: %d %s", code, body)
	}
	if len(body) == 0 {
		t.Fatal("empty body")
	}
}

func TestHandleJobGet_DirectOK(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	task, err := mock.CreateTask(ctx, nil, "prompt", nil)
	if err != nil {
		t.Fatal(err)
	}
	job, err := mock.CreateJob(ctx, task.ID, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	rec := &models.McpToolCallAuditLog{}
	code, body, _ := handleJobGet(ctx, mock, map[string]interface{}{"job_id": job.ID.String()}, rec)
	if code != http.StatusOK {
		t.Fatalf("handleJobGet: %d %s", code, body)
	}
	if len(body) == 0 {
		t.Fatal("empty body")
	}
}
func TestHandleTaskCancel_CancelTaskError(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	task, err := mock.CreateTask(ctx, nil, "prompt", nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.UpdateTaskStatusErr = errors.New("injected update failure")
	rec := &models.McpToolCallAuditLog{}
	code, body, _ := handleTaskCancel(ctx, mock, map[string]interface{}{"task_id": task.ID.String()}, rec)
	if code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", code, body)
	}
}

func TestHandleProjectList_StoreError(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	mock.GetOrCreateDefaultProjectForUserErr = errors.New("injected project list failure")
	rec := &models.McpToolCallAuditLog{}
	code, body, _ := handleProjectList(ctx, mock, map[string]interface{}{"user_id": uuid.New().String()}, rec)
	if code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", code, body)
	}
}
func TestHandleProjectGet_DirectBadArgs(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	rec := &models.McpToolCallAuditLog{}
	code, body, _ := handleProjectGet(ctx, mock, map[string]interface{}{"project_id": uuid.New().String()}, rec)
	if code != http.StatusBadRequest || !bytes.Contains(body, []byte("user_id")) {
		t.Errorf("missing user_id: %d %s", code, body)
	}
	uid := uuid.New()
	code, body, _ = handleProjectGet(ctx, mock, map[string]interface{}{"user_id": uid.String()}, rec)
	if code != http.StatusBadRequest || !bytes.Contains(body, []byte("exactly one")) {
		t.Errorf("missing project id/slug: %d %s", code, body)
	}
	code, body, _ = handleProjectGet(ctx, mock, map[string]interface{}{
		"user_id": uid.String(), "project_id": uuid.New().String(), "slug": "x",
	}, rec)
	if code != http.StatusBadRequest {
		t.Errorf("both id and slug: %d %s", code, body)
	}
}

func TestHandleProjectList_DirectLimitCap(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(ctx, "u-lim", nil)
	mock.AddUser(user)
	rec := &models.McpToolCallAuditLog{}
	code, _, _ := handleProjectList(ctx, mock, map[string]interface{}{
		"user_id": user.ID.String(),
		"limit":   999,
	}, rec)
	if code != http.StatusOK {
		t.Errorf("project.list cap: %d", code)
	}
}

func TestHandleTaskListAndProjectList_DirectInternalError(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(ctx, "u-terr", nil)
	mock.AddUser(user)
	mock.ForceError = errors.New("db")
	rec := &models.McpToolCallAuditLog{}
	code, _, _ := handleTaskList(ctx, mock, map[string]interface{}{"user_id": user.ID.String()}, rec)
	mock.ForceError = nil
	if code != http.StatusInternalServerError {
		t.Errorf("task.list internal: %d", code)
	}
	mock.ForceError = errors.New("db")
	code, _, _ = handleProjectList(ctx, mock, map[string]interface{}{"user_id": user.ID.String()}, rec)
	mock.ForceError = nil
	if code != http.StatusInternalServerError {
		t.Errorf("project.list internal: %d", code)
	}
}
func TestHandleTaskCancel_DirectCancelFails(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(ctx, nil, "p", nil)
	mock.UpdateTaskStatusErr = errors.New("db")
	rec := &models.McpToolCallAuditLog{}
	code, _, _ := handleTaskCancel(ctx, mock, map[string]interface{}{"task_id": task.ID.String()}, rec)
	mock.UpdateTaskStatusErr = nil
	if code != http.StatusInternalServerError {
		t.Errorf("task.cancel internal: %d", code)
	}
}

func TestHandleTaskResultAndLogs_DirectInternalError(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(ctx, nil, "p", nil)
	mock.GetTaskByIDErr = errors.New("db")
	rec := &models.McpToolCallAuditLog{}
	code, _, _ := handleTaskResult(ctx, mock, map[string]interface{}{"task_id": task.ID.String()}, rec)
	mock.GetTaskByIDErr = nil
	if code != http.StatusInternalServerError {
		t.Errorf("task.result internal: %d", code)
	}
	mock.GetTaskByIDErr = errors.New("db")
	code, _, _ = handleTaskLogs(ctx, mock, map[string]interface{}{"task_id": task.ID.String()}, rec)
	mock.GetTaskByIDErr = nil
	if code != http.StatusInternalServerError {
		t.Errorf("task.logs internal: %d", code)
	}
	mock.GetJobsByTaskIDErr = errors.New("db")
	code, _, _ = handleTaskResult(ctx, mock, map[string]interface{}{"task_id": task.ID.String()}, rec)
	mock.GetJobsByTaskIDErr = nil
	if code != http.StatusInternalServerError {
		t.Errorf("task.result jobs err: %d", code)
	}
	mock.GetJobsByTaskIDErr = errors.New("db")
	code, _, _ = handleTaskLogs(ctx, mock, map[string]interface{}{"task_id": task.ID.String()}, rec)
	mock.GetJobsByTaskIDErr = nil
	if code != http.StatusInternalServerError {
		t.Errorf("task.logs jobs err: %d", code)
	}
}
func TestHandleProjectGet_DirectGetOrCreateFails(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(ctx, "u-goc", nil)
	mock.AddUser(user)
	def, err := mock.GetOrCreateDefaultProjectForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	mock.GetOrCreateDefaultProjectForUserErr = errors.New("db")
	rec := &models.McpToolCallAuditLog{}
	code, _, _ := handleProjectGet(ctx, mock, map[string]interface{}{
		"user_id":    user.ID.String(),
		"project_id": def.ID.String(),
	}, rec)
	mock.GetOrCreateDefaultProjectForUserErr = nil
	if code != http.StatusInternalServerError {
		t.Errorf("project.get GetOrCreate error: %d", code)
	}
}

func TestHandleProjectGet_DirectNotAuthorizedAndSlug(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(ctx, "u-pg", nil)
	mock.AddUser(user)
	def, err := mock.GetOrCreateDefaultProjectForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherID := uuid.New()
	other := &models.Project{
		ProjectBase: models.ProjectBase{
			Slug:        "other-proj",
			DisplayName: "Other",
			IsActive:    true,
		},
		ID:        otherID,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mock.Projects[otherID] = other
	rec := &models.McpToolCallAuditLog{}
	code, _, _ := handleProjectGet(ctx, mock, map[string]interface{}{
		"user_id":    user.ID.String(),
		"project_id": otherID.String(),
	}, rec)
	if code != http.StatusNotFound {
		t.Errorf("project.get non-default project: %d", code)
	}
	code, body, _ := handleProjectGet(ctx, mock, map[string]interface{}{
		"user_id": user.ID.String(),
		"slug":    def.Slug,
	}, rec)
	if code != http.StatusOK {
		t.Fatalf("project.get by slug: %d %s", code, body)
	}
	code, _, _ = handleProjectGet(ctx, mock, map[string]interface{}{
		"user_id":    user.ID.String(),
		"project_id": uuid.New().String(),
	}, rec)
	if code != http.StatusNotFound {
		t.Errorf("project.get missing project id: %d", code)
	}
}
func TestProjectResponseMap_DescriptionOptional(t *testing.T) {
	desc := "about"
	pid := uuid.MustParse("00000000-0000-4000-8000-0000000000aa")
	ts := time.Unix(100, 0).UTC()
	withDesc := projectResponseMap(&models.Project{
		ProjectBase: models.ProjectBase{
			Slug:        "slug",
			DisplayName: "Name",
			IsActive:    true,
			Description: &desc,
		},
		ID:        pid,
		CreatedAt: ts,
		UpdatedAt: ts,
	})
	if withDesc["description"] != "about" {
		t.Fatalf("description: %v", withDesc["description"])
	}
	without := projectResponseMap(&models.Project{
		ProjectBase: models.ProjectBase{
			Slug:        "slug",
			DisplayName: "Name",
			IsActive:    true,
		},
		ID:        pid,
		CreatedAt: ts,
		UpdatedAt: ts,
	})
	if _, ok := without["description"]; ok {
		t.Fatal("expected no description when nil")
	}
}

func TestHandleTaskAndProjectList_ListError(t *testing.T) {
	ctx := context.Background()
	cases := []mcpHandlerToolCase{
		{
			"task_list",
			func(m *testutil.MockDB) { m.ListTasksByUserErr = errors.New("db") },
			handleTaskList,
		},
		{
			"project_list",
			func(m *testutil.MockDB) { m.ListAuthorizedProjectsForUserErr = errors.New("db") },
			handleProjectList,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := testutil.NewMockDB()
			user, _ := mock.CreateUser(ctx, "list-err-"+tc.name, nil)
			tc.prep(mock)
			rec := &models.McpToolCallAuditLog{}
			code, _, _ := tc.call(ctx, mock, map[string]interface{}{"user_id": user.ID.String()}, rec)
			if code != http.StatusInternalServerError {
				t.Fatalf("got %d", code)
			}
		})
	}
}
func TestHandleTaskResult_JobsError(t *testing.T) {
	testMCPHandleTaskWithPrep(t, func(m *testutil.MockDB) { m.GetJobsByTaskIDErr = errors.New("db") }, handleTaskResult)
}

func TestHandleTaskCancel_UpdateStatusError(t *testing.T) {
	testMCPHandleTaskWithPrep(t, func(m *testutil.MockDB) { m.UpdateTaskStatusErr = errors.New("db") }, handleTaskCancel)
}

func TestHandleTaskLogs_JobsError(t *testing.T) {
	testMCPHandleTaskWithPrep(t, func(m *testutil.MockDB) { m.GetJobsByTaskIDErr = errors.New("db") }, handleTaskLogs)
}

func TestHandleJobAndTaskGet_InternalError(t *testing.T) {
	cases := []struct {
		name  string
		idKey string
		prep  func(*testutil.MockDB)
		call  func(context.Context, database.Store, map[string]interface{}, *models.McpToolCallAuditLog) (int, []byte, *models.McpToolCallAuditLog)
	}{
		{"job_get", "job_id", func(m *testutil.MockDB) { m.GetJobByIDErr = errors.New("db") }, handleJobGet},
		{"task_get", "task_id", func(m *testutil.MockDB) { m.GetTaskByIDErr = errors.New("db") }, handleTaskGet},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			id := uuid.New().String()
			testMCPHandlerSimpleStoreError(t, tc.prep, tc.call, map[string]interface{}{tc.idKey: id})
		})
	}
}
