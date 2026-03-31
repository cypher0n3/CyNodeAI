package mcptaskbridge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestTaskToResponseAndJobToResponse(t *testing.T) {
	tid := uuid.New()
	prompt := "p"
	summary := "s"
	task := &models.Task{
		TaskBase: models.TaskBase{
			Status:        models.TaskStatusRunning,
			Prompt:        &prompt,
			Summary:       &summary,
			PlanningState: models.PlanningStateDraft,
		},
		ID:        tid,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	tr := TaskToResponse(task, userapi.StatusQueued, []string{"/a"})
	if tr.TaskID != tid.String() || tr.Status != userapi.StatusQueued || len(tr.Attachments) != 1 {
		t.Errorf("TaskToResponse: %+v", tr)
	}
	if tr.PlanningState != models.PlanningStateDraft {
		t.Errorf("PlanningState: got %q", tr.PlanningState)
	}
	jid := uuid.New()
	st := time.Now()
	en := st.Add(time.Minute)
	res := `{"x":1}`
	job := &models.Job{
		JobBase: models.JobBase{
			Status:    models.JobStatusCompleted,
			Result:    models.NewJSONBString(&res),
			StartedAt: &st,
			EndedAt:   &en,
		},
		ID: jid,
	}
	jr := JobToResponse(job)
	if jr.ID != jid.String() || jr.Result == nil || *jr.Result != res {
		t.Errorf("JobToResponse: %+v", jr)
	}
	if jr.StartedAt == nil || jr.EndedAt == nil {
		t.Error("JobToResponse timestamps")
	}
}

func TestTaskStatusToSpec(t *testing.T) {
	if got := TaskStatusToSpec(models.TaskStatusPending); got != userapi.StatusQueued {
		t.Errorf("pending: %s", got)
	}
	if got := TaskStatusToSpec(models.TaskStatusCanceled); got != userapi.StatusCanceled {
		t.Errorf("canceled: %s", got)
	}
	if got := TaskStatusToSpec(models.TaskStatusSuperseded); got != userapi.StatusSuperseded {
		t.Errorf("superseded: %s", got)
	}
	if got := TaskStatusToSpec("running"); got != "running" {
		t.Errorf("passthrough: %s", got)
	}
}

func TestParseListLimitOffset_Basics(t *testing.T) {
	l, o, st, c, err := ParseListLimitOffset(nil)
	if err != "" || l != 50 || o != 0 || st != "" || c != "" {
		t.Errorf("nil: limit=%d offset=%d status=%q cursor=%q err=%q", l, o, st, c, err)
	}
	args := map[string]interface{}{
		"limit": float64(10), "offset": 5, "status": "canceled", "cursor": "12",
	}
	l, o, st, c, err = ParseListLimitOffset(args)
	if err != "" || l != 10 || o != 12 || st != userapi.StatusCanceled || c != "12" {
		t.Errorf("cursor: limit=%d offset=%d status=%q cursor=%q err=%q", l, o, st, c, err)
	}
	lim, off, _, _, err2 := ParseListLimitOffset(map[string]interface{}{"limit": float64(500), "offset": -3})
	if err2 != "" || lim != 200 || off != 0 {
		t.Errorf("cap and offset clamp: limit=%d offset=%d err=%q", lim, off, err2)
	}
}

func TestParseListLimitOffset_InvalidCursor(t *testing.T) {
	const wantInvalidCursor = "invalid cursor"
	if _, _, _, _, err := ParseListLimitOffset(map[string]interface{}{"cursor": "x"}); err != wantInvalidCursor {
		t.Errorf("bad cursor: %q", err)
	}
	if _, _, _, _, err := ParseListLimitOffset(map[string]interface{}{"cursor": "-1"}); err != wantInvalidCursor {
		t.Errorf("negative cursor: %q", err)
	}
}

func TestListTasksForUser_Pagination(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	for i := 0; i < 3; i++ {
		tid := uuid.New()
		db.Tasks[tid] = &models.Task{
			TaskBase:  models.TaskBase{Status: models.TaskStatusRunning, CreatedBy: &uid},
			ID:        tid,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
	}
	resp, err := ListTasksForUser(ctx, db, uid, 2, 0, "", "")
	if err != nil {
		t.Fatalf("ListTasksForUser: %v", err)
	}
	if len(resp.Tasks) != 2 || resp.NextCursor == "" || resp.NextOffset == nil {
		t.Errorf("pagination: %+v", resp)
	}
}

func TestTaskResultForUser_OK(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	tid := uuid.New()
	task := &models.Task{
		TaskBase:  models.TaskBase{Status: models.TaskStatusCompleted},
		ID:        tid,
		CreatedAt: time.Now(),
	}
	db.Tasks[tid] = task
	jid := uuid.New()
	job := &models.Job{
		JobBase:   models.JobBase{TaskID: tid, Status: models.JobStatusCompleted},
		ID:        jid,
		CreatedAt: time.Now(),
	}
	db.Jobs[jid] = job
	db.JobsByTask[tid] = []*models.Job{job}
	out, err := TaskResultForUser(ctx, db, tid, 0, 0)
	if err != nil || len(out.Jobs) != 1 {
		t.Fatalf("TaskResultForUser: %+v err=%v", out, err)
	}
	if out.TotalCount != 1 || out.NextCursor != "" || out.NextOffset != nil {
		t.Errorf("single-page metadata: %+v", out)
	}
}

func TestTaskResultForUser_PaginationNext(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	tid := uuid.New()
	task := &models.Task{
		TaskBase:  models.TaskBase{Status: models.TaskStatusCompleted},
		ID:        tid,
		CreatedAt: time.Now(),
	}
	db.Tasks[tid] = task
	for i := 0; i < 3; i++ {
		jid := uuid.New()
		job := &models.Job{
			JobBase:   models.JobBase{TaskID: tid, Status: models.JobStatusCompleted},
			ID:        jid,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Millisecond),
		}
		db.Jobs[jid] = job
		db.JobsByTask[tid] = append(db.JobsByTask[tid], job)
	}
	out, err := TaskResultForUser(ctx, db, tid, 1, 0)
	if err != nil {
		t.Fatalf("TaskResultForUser: %v", err)
	}
	if out.TotalCount != 3 || len(out.Jobs) != 1 || out.NextCursor == "" || out.NextOffset == nil {
		t.Fatalf("expected page + next: %+v", out)
	}
	if *out.NextOffset != 1 {
		t.Errorf("NextOffset: %v", *out.NextOffset)
	}
}

func TestTaskResultForUser_ListJobsError(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	tid := uuid.New()
	db.Tasks[tid] = &models.Task{TaskBase: models.TaskBase{Status: models.TaskStatusRunning}, ID: tid, CreatedAt: time.Now()}
	db.GetJobsByTaskIDErr = errors.New("db")
	defer func() { db.GetJobsByTaskIDErr = nil }()
	_, err := TaskResultForUser(ctx, db, tid, 0, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTaskLogsForUser_NextPage(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	tid := uuid.New()
	task := &models.Task{
		TaskBase:  models.TaskBase{Status: models.TaskStatusRunning},
		ID:        tid,
		CreatedAt: time.Now(),
	}
	db.Tasks[tid] = task
	res := `{"stdout":"a","stderr":"b"}`
	for i := 0; i < 2; i++ {
		jid := uuid.New()
		job := &models.Job{
			JobBase: models.JobBase{
				TaskID: tid, Status: models.JobStatusCompleted,
				Result: models.NewJSONBString(&res),
			},
			ID:        jid,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Millisecond),
		}
		db.Jobs[jid] = job
		db.JobsByTask[tid] = append(db.JobsByTask[tid], job)
	}
	out, err := TaskLogsForUser(ctx, db, tid, streamParamAll, 1, 0)
	if err != nil {
		t.Fatalf("TaskLogsForUser: %v", err)
	}
	if out.TotalCount != 2 || out.NextCursor == "" {
		t.Fatalf("logs pagination: %+v", out)
	}
}

func TestListTasksForUser_StoreError(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	db.ListTasksByUserErr = errors.New("db")
	defer func() { db.ListTasksByUserErr = nil }()
	_, err := ListTasksForUser(ctx, db, uuid.New(), 10, 0, "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCancelTask_Errors(t *testing.T) {
	ctx := context.Background()
	tid := uuid.New()
	errCases := []struct {
		name string
		prep func(*testutil.MockDB)
	}{
		{"update task", func(db *testutil.MockDB) { db.UpdateTaskStatusErr = errors.New("fail") }},
		{"list jobs", func(db *testutil.MockDB) { db.GetJobsByTaskIDErr = errors.New("fail") }},
	}
	for _, c := range errCases {
		t.Run(c.name, func(t *testing.T) {
			db := testutil.NewMockDB()
			db.Tasks[tid] = &models.Task{TaskBase: models.TaskBase{Status: models.TaskStatusRunning}, ID: tid, CreatedAt: time.Now()}
			c.prep(db)
			defer func() {
				db.UpdateTaskStatusErr = nil
				db.GetJobsByTaskIDErr = nil
			}()
			if err := CancelTask(ctx, db, tid); err == nil {
				t.Fatal("expected error")
			}
		})
	}
	t.Run("update job", func(t *testing.T) {
		db := testutil.NewMockDB()
		db.Tasks[tid] = &models.Task{TaskBase: models.TaskBase{Status: models.TaskStatusRunning}, ID: tid, CreatedAt: time.Now()}
		jid := uuid.New()
		job := &models.Job{JobBase: models.JobBase{TaskID: tid, Status: models.JobStatusRunning}, ID: jid, CreatedAt: time.Now()}
		db.Jobs[jid] = job
		db.JobsByTask[tid] = []*models.Job{job}
		db.UpdateJobStatusErr = errors.New("fail")
		defer func() { db.UpdateJobStatusErr = nil }()
		if err := CancelTask(ctx, db, tid); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAggregateLogsFromJobs(t *testing.T) {
	res := `{"stdout":"out","stderr":"err"}`
	jobs := []*models.Job{
		{JobBase: models.JobBase{Result: models.NewJSONBString(&res)}},
	}
	stdout, stderr := AggregateLogsFromJobs(jobs, streamParamAll)
	if stdout != "out" || stderr != "err" {
		t.Errorf("all: stdout=%q stderr=%q", stdout, stderr)
	}
	stdout, stderr = AggregateLogsFromJobs(jobs, "stdout")
	if stdout != "out" || stderr != "" {
		t.Errorf("stdout: stdout=%q stderr=%q", stdout, stderr)
	}
	stdout, stderr = AggregateLogsFromJobs(jobs, "stderr")
	if stdout != "" || stderr != "err" {
		t.Errorf("stderr: stdout=%q stderr=%q", stdout, stderr)
	}
	bad := `not-json`
	jobsBad := []*models.Job{{JobBase: models.JobBase{Result: models.NewJSONBString(&bad)}}}
	stdout, stderr = AggregateLogsFromJobs(jobsBad, streamParamAll)
	if stdout != "" || stderr != "" {
		t.Errorf("bad json: stdout=%q stderr=%q", stdout, stderr)
	}
	var wr workerapi.RunJobResponse
	inner, _ := json.Marshal(wr)
	s := string(inner)
	jobsEmpty := []*models.Job{{JobBase: models.JobBase{Result: models.NewJSONBString(&s)}}}
	AggregateLogsFromJobs(jobsEmpty, streamParamAll)
}

func TestListTasksForUser_StatusFilter(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	uid := uuid.New()
	t1 := uuid.New()
	task := &models.Task{
		TaskBase: models.TaskBase{
			Status:    models.TaskStatusRunning,
			CreatedBy: &uid,
		},
		ID:        t1,
		CreatedAt: time.Now(),
	}
	db.Tasks[t1] = task
	resp, err := ListTasksForUser(ctx, db, uid, 50, 0, "", "")
	if err != nil || len(resp.Tasks) != 1 {
		t.Fatalf("list: %v err=%v", resp, err)
	}
	resp, err = ListTasksForUser(ctx, db, uid, 50, 0, userapi.StatusQueued, "")
	if err != nil || len(resp.Tasks) != 0 {
		t.Errorf("filter mismatch: got %d tasks", len(resp.Tasks))
	}
}

func TestTaskResultForUser_NotFound(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	_, err := TaskResultForUser(ctx, db, uuid.New(), 0, 0)
	if err != database.ErrNotFound {
		t.Errorf("want ErrNotFound: %v", err)
	}
}

func TestCancelTask_UpdatesJobs(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	tid := uuid.New()
	task := &models.Task{
		TaskBase:  models.TaskBase{Status: models.TaskStatusRunning},
		ID:        tid,
		CreatedAt: time.Now(),
	}
	db.Tasks[tid] = task
	jid := uuid.New()
	jdone := uuid.New()
	job := &models.Job{
		JobBase: models.JobBase{
			TaskID: tid,
			Status: models.JobStatusRunning,
		},
		ID:        jid,
		CreatedAt: time.Now(),
	}
	doneJob := &models.Job{
		JobBase: models.JobBase{
			TaskID: tid,
			Status: models.JobStatusCompleted,
		},
		ID:        jdone,
		CreatedAt: time.Now(),
	}
	db.Jobs[jid] = job
	db.Jobs[jdone] = doneJob
	db.JobsByTask[tid] = []*models.Job{job, doneJob}
	if err := CancelTask(ctx, db, tid); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	if db.Tasks[tid].Status != models.TaskStatusCanceled {
		t.Errorf("task status: %s", db.Tasks[tid].Status)
	}
	if db.Jobs[jid].Status != models.JobStatusCanceled {
		t.Errorf("job status: %s", db.Jobs[jid].Status)
	}
	if db.Jobs[jdone].Status != models.JobStatusCompleted {
		t.Errorf("completed job should stay: %s", db.Jobs[jdone].Status)
	}
}

func TestTaskLogsForUser_DefaultStream(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewMockDB()
	tid := uuid.New()
	task := &models.Task{
		TaskBase:  models.TaskBase{Status: models.TaskStatusRunning},
		ID:        tid,
		CreatedAt: time.Now(),
	}
	db.Tasks[tid] = task
	res := `{"stdout":"hello","stderr":"e"}`
	jid := uuid.New()
	job := &models.Job{
		JobBase: models.JobBase{
			TaskID: tid,
			Status: models.JobStatusCompleted,
			Result: models.NewJSONBString(&res),
		},
		ID:        jid,
		CreatedAt: time.Now(),
	}
	db.Jobs[jid] = job
	db.JobsByTask[tid] = []*models.Job{job}
	out, err := TaskLogsForUser(ctx, db, tid, "", 0, 0)
	if err != nil {
		t.Fatalf("TaskLogsForUser: %v", err)
	}
	if out.Stdout != "hello" || out.Stderr != "e" {
		t.Errorf("logs: stdout=%q stderr=%q", out.Stdout, out.Stderr)
	}
	out, err = TaskLogsForUser(ctx, db, tid, "stdout", 0, 0)
	if err != nil || out.Stdout != "hello" || out.Stderr != "" {
		t.Errorf("stdout only: %+v err=%v", out, err)
	}
}
