package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestInsertContainerEvent(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	err := s.InsertContainerEvent(ctx, "evt-1", now, "cid-1", "start", "running", nil, "task-1", "job-1", map[string]interface{}{"foo": "bar"})
	if err != nil {
		t.Fatalf("InsertContainerEvent: %v", err)
	}
	var count int64
	s.db.WithContext(ctx).Model(&ContainerEvent{}).Where("event_id = ?", "evt-1").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}
}

func TestInsertContainerEvent_withExitCode(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	now := time.Now().UTC().Format(time.RFC3339)
	exitCode := 1
	err := s.InsertContainerEvent(ctx, "evt-2", now, "cid-2", "stop", "exited", &exitCode, "", "", nil)
	if err != nil {
		t.Fatalf("InsertContainerEvent: %v", err)
	}
	var c int64
	s.db.WithContext(ctx).Model(&ContainerEvent{}).Where("event_id = ?", "evt-2").Count(&c)
	if c != 1 {
		t.Errorf("count: got %d", c)
	}
}

func TestInsertLogEvent(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	in := LogEventInput{
		LogID:      "log-1",
		SourceKind: "service",
		SourceName: "worker_api",
		Message:    "test message",
	}
	if err := s.InsertLogEvent(ctx, &in); err != nil {
		t.Fatalf("InsertLogEvent: %v", err)
	}
	events, _, _, err := s.QueryLogs(ctx, "service", "worker_api", "", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(events) != 1 || events[0].Message != "test message" {
		t.Errorf("QueryLogs: %+v", events)
	}
	if events[0].OccurredAt == "" {
		t.Error("OccurredAt should be set by default")
	}
}

func TestInsertLogEvent_container(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	in := LogEventInput{
		LogID:       "log-2",
		SourceKind:  "container",
		SourceName:  "sbox-1",
		ContainerID: "cid-1",
		Stream:      StreamStdout,
		Message:     "container line",
		Fields:      map[string]string{"job_id": "j1"},
	}
	if err := s.InsertLogEvent(ctx, &in); err != nil {
		t.Fatalf("InsertLogEvent: %v", err)
	}
	events, _, _, err := s.QueryLogs(ctx, "container", "", "cid-1", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	if len(events) != 1 || events[0].Message != "container line" || events[0].Stream != StreamStdout {
		t.Errorf("QueryLogs: %+v", events)
	}
}
