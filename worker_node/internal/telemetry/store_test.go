package telemetry

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)


func TestOpenClose(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestOpen_mkdirFails(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	filePath := dir + "/file"
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Opening with stateDir = file path: telemetry dir becomes file/telemetry, MkdirAll fails because file is not a directory
	_, err := Open(ctx, filePath)
	if err == nil {
		t.Error("Open expected to fail when stateDir is a file")
	}
}

func TestOpen_emptyStateDirUsesDefault(t *testing.T) {
	ctx := context.Background()
	// Open with empty string uses default path which may not be writable; just ensure we don't panic
	s, err := Open(ctx, "")
	if err != nil {
		return
	}
	_ = s.Close()
}

func TestListContainers_empty(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	list, next, err := s.ListContainers(ctx, "", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(list) != 0 || next != "" {
		t.Errorf("expected empty list, got %d items, next=%q", len(list), next)
	}
}

func TestListContainers_limitBoundary(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	// limit 0 or >1000 defaults to 100
	list, _, _ := s.ListContainers(ctx, "", "", "", "", "", 0)
	if len(list) != 0 {
		t.Errorf("limit 0: expected 0 cap")
	}
	list, _, _ = s.ListContainers(ctx, "", "", "", "", "", 1001)
	if len(list) != 0 {
		t.Errorf("limit 1001: should be capped")
	}
}

func TestListContainers_withData(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	labelsJSON := `{"a":"b"}`
	if err := s.db.WithContext(ctx).Create(&ContainerInventory{
		ContainerID: "cid-1", ContainerName: "name-1", Kind: "managed", Runtime: "podman", ImageRef: "img",
		CreatedAt: now, LastSeenAt: now, Status: "running", TaskID: "task-1", JobID: "job-1", LabelsJSON: labelsJSON,
	}).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	list, nextToken, err := s.ListContainers(ctx, "", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(list) != 1 || nextToken != "" {
		t.Errorf("len=%d next=%q", len(list), nextToken)
	}
	if list[0].ContainerID != "cid-1" || list[0].TaskID != "task-1" || list[0].Labels["a"] != "b" {
		t.Errorf("row: %+v", list[0])
	}
	// filters
	list, _, _ = s.ListContainers(ctx, "managed", "", "", "", "", 10)
	if len(list) != 1 {
		t.Errorf("kind filter: %d", len(list))
	}
	list, _, _ = s.ListContainers(ctx, "sandbox", "", "", "", "", 10)
	if len(list) != 0 {
		t.Errorf("kind sandbox should be empty: %d", len(list))
	}
	list, _, _ = s.ListContainers(ctx, "", "", "task-1", "job-1", "", 10)
	if len(list) != 1 {
		t.Errorf("task/job filter: %d", len(list))
	}
}

func TestListContainers_pagination(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	for i := 0; i < 3; i++ {
		if err := s.db.WithContext(ctx).Create(&ContainerInventory{
			ContainerID: fmt.Sprintf("cid-%d", i), ContainerName: "name", Kind: "managed", Runtime: "podman", ImageRef: "img",
			CreatedAt: now, LastSeenAt: now, Status: "running", LabelsJSON: "{}",
		}).Error; err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	list, next, err := s.ListContainers(ctx, "", "", "", "", "", 2)
	if err != nil || len(list) != 2 || next != "2" {
		t.Errorf("first page: err=%v len=%d next=%q", err, len(list), next)
	}
	list2, next2, _ := s.ListContainers(ctx, "", "", "", "", "2", 2)
	if len(list2) != 1 || next2 != "" {
		t.Errorf("second page: len=%d next=%q", len(list2), next2)
	}
}

func TestGetContainer_notFound(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	c, err := s.GetContainer(ctx, "nonexistent")
	if err != nil || c != nil {
		t.Errorf("GetContainer not found: err=%v c=%v", err, c)
	}
}

func TestInsertTestContainer(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	if err := s.InsertTestContainer(ctx, "tid-1", "tname", "managed", "running", "task-1", "job-1"); err != nil {
		t.Fatalf("InsertTestContainer: %v", err)
	}
	c, err := s.GetContainer(ctx, "tid-1")
	if err != nil || c == nil || c.ContainerName != "tname" || c.TaskID != "task-1" {
		t.Errorf("GetContainer after InsertTestContainer: err=%v c=%+v", err, c)
	}
}

func TestGetContainer_found(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	exitCode := 0
	if err := s.db.WithContext(ctx).Create(&ContainerInventory{
		ContainerID: "gid-1", ContainerName: "gname", Kind: "sandbox", Runtime: "podman", ImageRef: "img",
		CreatedAt: now, LastSeenAt: now, Status: "exited", ExitCode: &exitCode, TaskID: "t", JobID: "j", LabelsJSON: "{}",
	}).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	c, err := s.GetContainer(ctx, "gid-1")
	if err != nil || c == nil {
		t.Fatalf("GetContainer: err=%v c=%v", err, c)
	}
	if c.ContainerName != "gname" || c.Kind != "sandbox" || c.TaskID != "t" || c.JobID != "j" {
		t.Errorf("row: %+v", c)
	}
	if c.ExitCode == nil || *c.ExitCode != 0 {
		t.Errorf("exit_code: %v", c.ExitCode)
	}
}

func TestQueryLogs_validation(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	_, _, _, err := s.QueryLogs(ctx, "", "", "", "", "", "", "", 10)
	if err == nil {
		t.Error("expected error when source_kind and container_id both empty")
	}
}

func TestQueryLogs_empty(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	events, truncated, next, err := s.QueryLogs(ctx, "container", "", "cid-x", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(events) != 0 || truncated.LimitedBy != limitedByNone || next != "" {
		t.Errorf("empty: events=%d limited=%s next=%q", len(events), truncated.LimitedBy, next)
	}
}

func TestQueryLogs_withData(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	if err := s.db.WithContext(ctx).Create(&LogEvent{
		LogID: "log-1", OccurredAt: now, SourceKind: "container", SourceName: "cname", ContainerID: "cid-1",
		Stream: StreamStdout, Message: "hello", FieldsJSON: "{}",
	}).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	events, truncated, next, err := s.QueryLogs(ctx, "container", "cname", "", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(events) != 1 || events[0].Message != "hello" || events[0].Stream != StreamStdout {
		t.Errorf("events: %+v", events)
	}
	if truncated.LimitedBy != limitedByNone || next != "" {
		t.Errorf("truncated=%+v next=%q", truncated, next)
	}
	// container_id filter
	events2, _, _, _ := s.QueryLogs(ctx, "container", "", "cid-1", "", "", "", "", 10)
	if len(events2) != 1 {
		t.Errorf("container_id filter: %d", len(events2))
	}
}

func TestQueryLogs_limitBoundary(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	_, _, _, err := s.QueryLogs(ctx, "service", "worker-api", "", "", "", "", "", 0)
	if err != nil {
		t.Fatalf("limit 0: %v", err)
	}
}

func TestQueryLogs_contextCanceled(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	_, _, _, err := s.QueryLogs(cctx, "service", "worker-api", "", "", "", "", "", 10)
	if err == nil {
		t.Error("expected error when context canceled")
	}
}

func TestEnforceRetention(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	if err := s.EnforceRetention(ctx); err != nil {
		t.Errorf("EnforceRetention: %v", err)
	}
	// Insert old rows and ensure EnforceRetention deletes them
	oldLog := time.Now().UTC().AddDate(0, 0, -8).Format(time.RFC3339)
	oldEvent := time.Now().UTC().AddDate(0, 0, -31).Format(time.RFC3339)
	_ = s.db.WithContext(ctx).Create(&LogEvent{LogID: "old-log", OccurredAt: oldLog, SourceKind: "service", SourceName: "x", Message: "msg", FieldsJSON: "{}"})
	_ = s.db.WithContext(ctx).Create(&ContainerEvent{EventID: "old-evt", OccurredAt: oldEvent, ContainerID: "c", Action: "start", Status: "running", DetailsJSON: "{}"})
	_ = s.db.WithContext(ctx).Create(&ContainerInventory{ContainerID: "old-c", ContainerName: "n", Kind: "managed", Runtime: "r", ImageRef: "i", CreatedAt: oldEvent, LastSeenAt: oldEvent, Status: "exited", LabelsJSON: "{}"})
	if err := s.EnforceRetention(ctx); err != nil {
		t.Errorf("EnforceRetention second: %v", err)
	}
	var logCount, eventCount, invCount int64
	_ = s.db.WithContext(ctx).Model(&LogEvent{}).Where("log_id = ?", "old-log").Count(&logCount)
	_ = s.db.WithContext(ctx).Model(&ContainerEvent{}).Where("event_id = ?", "old-evt").Count(&eventCount)
	_ = s.db.WithContext(ctx).Model(&ContainerInventory{}).Where("container_id = ?", "old-c").Count(&invCount)
	if logCount != 0 || eventCount != 0 || invCount != 0 {
		t.Errorf("old rows should be deleted: log=%d event=%d inv=%d", logCount, eventCount, invCount)
	}
}

func TestVacuum(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	if err := s.Vacuum(ctx); err != nil {
		t.Errorf("Vacuum: %v", err)
	}
}

func TestOpen_idempotentMigrations(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s1, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_ = s1.Close()
	s2, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open second: %v", err)
	}
	_ = s2.Close()
}

func TestQueryLogs_paginationAndCountTruncation(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	for i := 0; i < 5; i++ {
		if err := s.db.WithContext(ctx).Create(&LogEvent{
			LogID: fmt.Sprintf("log-%d", i), OccurredAt: now, SourceKind: "service", SourceName: "worker-api",
			Message: fmt.Sprintf("msg-%d", i), FieldsJSON: "{}",
		}).Error; err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	// limit 2 -> get 2, nextToken set, limited_by count
	events, truncated, next, err := s.QueryLogs(ctx, "service", "worker-api", "", "", "", "", "", 2)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(events) != 2 || truncated.LimitedBy != "count" || next != "2" {
		t.Errorf("events=%d limited=%s next=%q", len(events), truncated.LimitedBy, next)
	}
	events2, _, next2, _ := s.QueryLogs(ctx, "service", "worker-api", "", "", "", "", "2", 10)
	if len(events2) != 3 || next2 != "" {
		t.Errorf("page2: len=%d next=%q", len(events2), next2)
	}
}

func TestQueryLogs_sinceUntilStream(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	_ = s.db.WithContext(ctx).Create(&LogEvent{
		LogID: "log-s", OccurredAt: now, SourceKind: "container", SourceName: "c", ContainerID: "cid",
		Stream: "stderr", Message: "err", FieldsJSON: "{}",
	})
	events, _, _, err := s.QueryLogs(ctx, "container", "", "cid", "stderr", now, "", "", 10)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(events) != 1 || events[0].Stream != "stderr" {
		t.Errorf("events: %+v", events)
	}
}

func TestListContainers_contextCanceled(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	_ = s.db.WithContext(ctx).Create(&ContainerInventory{
		ContainerID: "x", ContainerName: "n", Kind: "managed", Runtime: "r", ImageRef: "i",
		CreatedAt: now, LastSeenAt: now, Status: "running", LabelsJSON: "{}",
	})
	cctx, cancel := context.WithCancel(ctx)
	cancel() // cancel immediately so QueryContext or iteration sees it
	_, _, err := s.ListContainers(cctx, "", "", "", "", "", 10)
	if err == nil {
		t.Error("expected error when context canceled")
	}
}

func TestListContainers_invalidPageToken(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	list, next, err := s.ListContainers(ctx, "", "", "", "", "bad", 10)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	if len(list) != 0 || next != "" {
		t.Errorf("invalid token: list=%d next=%q", len(list), next)
	}
	// Negative offset can cause DB error on some drivers
	list2, _, err2 := s.ListContainers(ctx, "", "", "", "", "-1", 10)
	if err2 != nil {
		return // error path covered
	}
	if len(list2) != 0 {
		t.Errorf("negative token: expected empty or error, got %d", len(list2))
	}
}

func TestQueryLogs_bytesTruncation(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	bigMsg := string(make([]byte, maxLogRespBytes))
	if err := s.db.WithContext(ctx).Create(&LogEvent{
		LogID: "log-big", OccurredAt: now, SourceKind: "service", SourceName: "api", Message: bigMsg, FieldsJSON: "{}",
	}).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	events, truncated, _, err := s.QueryLogs(ctx, "service", "api", "", "", "", "", "", 5000)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if truncated.LimitedBy != limitedByBytes {
		t.Errorf("expected bytes truncation, got %s", truncated.LimitedBy)
	}
	// First row exceeds byte limit so we break without appending any event
	if truncated.LimitedBy != limitedByBytes || len(events) != 0 {
		t.Errorf("bytes truncation: limited_by=%s events=%d", truncated.LimitedBy, len(events))
	}
}

func TestUpsertContainerInventory_insertAndUpdate(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	in := ContainerRow{
		ContainerID:   "upsert-1",
		ContainerName: "name1",
		Kind:          "sandbox",
		Runtime:       "podman",
		ImageRef:      "img:latest",
		CreatedAt:     now,
		LastSeenAt:    now,
		Status:        "running",
		TaskID:        "t1",
		JobID:         "j1",
		Labels:        map[string]string{"a": "b"},
	}
	if err := s.UpsertContainerInventory(ctx, &in); err != nil {
		t.Fatalf("UpsertContainerInventory: %v", err)
	}
	c, err := s.GetContainer(ctx, "upsert-1")
	if err != nil || c == nil {
		t.Fatalf("GetContainer: err=%v c=%v", err, c)
	}
	if c.Kind != "sandbox" || c.TaskID != "t1" || c.Labels["a"] != "b" {
		t.Errorf("row: %+v", c)
	}
	// Update same container
	in.Status = "exited"
	exitCode := 0
	in.ExitCode = &exitCode
	in.LastSeenAt = now
	if err := s.UpsertContainerInventory(ctx, &in); err != nil {
		t.Fatalf("UpsertContainerInventory update: %v", err)
	}
	c2, _ := s.GetContainer(ctx, "upsert-1")
	if c2 == nil || c2.Status != "exited" || c2.ExitCode == nil || *c2.ExitCode != 0 {
		t.Errorf("after update: %+v", c2)
	}
}

func openStore(t *testing.T, ctx context.Context) *Store {
	t.Helper()
	s, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}
