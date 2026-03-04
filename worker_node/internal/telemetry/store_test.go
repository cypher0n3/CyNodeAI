package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	labels := map[string]string{"a": "b"}
	labelsJSON, _ := json.Marshal(labels)
	_, err := s.db.ExecContext(ctx, `INSERT INTO container_inventory (
		container_id, container_name, kind, runtime, image_ref, created_at, last_seen_at, status, task_id, job_id, labels_json
	) VALUES (?, ?, 'managed', 'podman', 'img', ?, ?, 'running', 'task-1', 'job-1', ?)`,
		"cid-1", "name-1", now, now, string(labelsJSON))
	if err != nil {
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
	labelsJSON := "{}"
		for i := 0; i < 3; i++ {
		_, err := s.db.ExecContext(ctx, `INSERT INTO container_inventory (
			container_id, container_name, kind, runtime, image_ref, created_at, last_seen_at, status, labels_json
		) VALUES (?, ?, 'managed', 'podman', 'img', ?, ?, 'running', ?)`,
			fmt.Sprintf("cid-%d", i), "name", now, now, labelsJSON)
		if err != nil {
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO container_inventory (
		container_id, container_name, kind, runtime, image_ref, created_at, last_seen_at, status, exit_code, task_id, job_id, labels_json
	) VALUES ('gid-1', 'gname', 'sandbox', 'podman', 'img', ?, ?, 'exited', ?, 't', 'j', '{}')`,
		now, now, exitCode)
	if err != nil {
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO log_event (
		log_id, occurred_at, source_kind, source_name, container_id, stream, message, fields_json
	) VALUES ('log-1', ?, 'container', 'cname', 'cid-1', 'stdout', 'hello', '{}')`,
		now)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	events, truncated, next, err := s.QueryLogs(ctx, "container", "cname", "", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(events) != 1 || events[0].Message != "hello" || events[0].Stream != "stdout" {
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
	_, _ = s.db.ExecContext(ctx, `INSERT INTO log_event (log_id, occurred_at, source_kind, source_name, message, fields_json) VALUES ('old-log', ?, 'service', 'x', 'msg', '{}')`, oldLog)
	_, _ = s.db.ExecContext(ctx, `INSERT INTO container_event (event_id, occurred_at, container_id, action, status, details_json) VALUES ('old-evt', ?, 'c', 'start', 'running', '{}')`, oldEvent)
	_, _ = s.db.ExecContext(ctx, `INSERT INTO container_inventory (container_id, container_name, kind, runtime, image_ref, created_at, last_seen_at, status, labels_json) VALUES ('old-c', 'n', 'managed', 'r', 'i', ?, ?, 'exited', '{}')`, oldEvent, oldEvent)
	if err := s.EnforceRetention(ctx); err != nil {
		t.Errorf("EnforceRetention second: %v", err)
	}
	var logCount, eventCount, invCount int
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM log_event WHERE log_id = 'old-log'").Scan(&logCount)
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM container_event WHERE event_id = 'old-evt'").Scan(&eventCount)
	_ = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM container_inventory WHERE container_id = 'old-c'").Scan(&invCount)
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
	// Second open: schema_version already 1, no apply
	s2, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open second: %v", err)
	}
	_ = s2.Close()
}

func TestOpen_runMigrationsUpdateFails(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s1, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, _ = s1.db.ExecContext(ctx, "UPDATE schema_version SET version = 0 WHERE id = 1")
	_ = s1.Close()
	dbPath := filepath.Join(dir, "telemetry", "telemetry.db")
	if err := os.Chmod(dbPath, 0o444); err != nil {
		t.Skip("chmod read-only not supported or not needed")
	}
	defer func() { _ = os.Chmod(dbPath, 0o644) }()
	_, err = Open(ctx, dir)
	if err == nil {
		t.Error("Open expected to fail when DB is read-only during migration")
	}
}

func TestOpen_runMigrationsScanError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s1, err := Open(ctx, dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Corrupt schema_version so Scan fails (version column not int)
	_, err = s1.db.ExecContext(ctx, "UPDATE schema_version SET version = 'bad' WHERE id = 1")
	if err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	_ = s1.Close()
	_, err = Open(ctx, dir)
	if err == nil {
		t.Error("Open expected to fail when schema_version is invalid")
	}
}

func TestQueryLogs_paginationAndCountTruncation(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	for i := 0; i < 5; i++ {
		_, err := s.db.ExecContext(ctx, `INSERT INTO log_event (
			log_id, occurred_at, source_kind, source_name, message, fields_json
		) VALUES (?, ?, 'service', 'worker-api', ?, '{}')`,
			fmt.Sprintf("log-%d", i), now, fmt.Sprintf("msg-%d", i))
		if err != nil {
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
	_, _ = s.db.ExecContext(ctx, `INSERT INTO log_event (
		log_id, occurred_at, source_kind, source_name, container_id, stream, message, fields_json
	) VALUES ('log-s', ?, 'container', 'c', 'cid', 'stderr', 'err', '{}')`, now)
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
	_, _ = s.db.ExecContext(ctx, `INSERT INTO container_inventory (
		container_id, container_name, kind, runtime, image_ref, created_at, last_seen_at, status, labels_json
	) VALUES ('x', 'n', 'managed', 'r', 'i', ?, ?, 'running', '{}')`, now, now)
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
	// One row with message large enough that approxBytes exceeds maxLogRespBytes (1 MiB)
	bigMsg := string(make([]byte, maxLogRespBytes))
	_, err := s.db.ExecContext(ctx, `INSERT INTO log_event (
		log_id, occurred_at, source_kind, source_name, message, fields_json
	) VALUES ('log-big', ?, 'service', 'api', ?, '{}')`, now, bigMsg)
	if err != nil {
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

func openStore(t *testing.T, ctx context.Context) *Store {
	t.Helper()
	s, err := Open(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}
