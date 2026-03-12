package telemetry

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestLogHandler_Enabled_nilInner(t *testing.T) {
	h := &LogHandler{Inner: nil, Store: nil, Source: "api"}
	ctx := context.Background()
	if h.Enabled(ctx, slog.LevelInfo) {
		t.Error("Enabled with nil Inner should be false")
	}
}

func TestLogHandler_Enabled_withInner(t *testing.T) {
	inner := slog.Default().Handler()
	h := &LogHandler{Inner: inner, Store: nil, Source: "api"}
	ctx := context.Background()
	if !h.Enabled(ctx, slog.LevelInfo) {
		t.Error("Enabled with Inner should delegate to Inner")
	}
}

func TestLogHandler_Handle_noStoreOrSource(t *testing.T) {
	ctx := context.Background()
	h := &LogHandler{Inner: slog.Default().Handler(), Store: nil, Source: "api"}
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0)
	if err := h.Handle(ctx, r); err != nil {
		t.Errorf("Handle: %v", err)
	}

	h2 := &LogHandler{Inner: slog.Default().Handler(), Store: nil, Source: ""}
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	h2.Store = s
	if err := h2.Handle(ctx, r); err != nil {
		t.Errorf("Handle empty Source: %v", err)
	}
	// Empty Source skips insert, so no log written
}

func TestLogHandler_Handle_writesToStore(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	h := &LogHandler{Inner: slog.Default().Handler(), Store: s, Source: "worker_api"}
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "sloghandler-test-msg", 0)
	r.AddAttrs(slog.String("k", "v"))
	if err := h.Handle(ctx, r); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// Insert runs in goroutine; short wait then query
	time.Sleep(50 * time.Millisecond)
	events, _, _, err := s.QueryLogs(ctx, "service", "worker_api", "", "", "", "", "", 10)
	if err != nil {
		t.Fatalf("QueryLogs: %v", err)
	}
	var found bool
	for _, e := range events {
		if e.Message == "sloghandler-test-msg" && e.SourceName == "worker_api" {
			found = true
			if e.Fields["k"] != "v" {
				t.Errorf("expected field k=v, got %q", e.Fields["k"])
			}
			break
		}
	}
	if !found {
		t.Error("expected one log_event from Handle, not found")
	}
}

func TestLogHandler_Handle_zeroTime(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	h := &LogHandler{Inner: nil, Store: s, Source: "api"}
	r := slog.NewRecord(time.Time{}, slog.LevelInfo, "zero-time-msg", 0)
	if err := h.Handle(ctx, r); err != nil {
		t.Errorf("Handle: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	events, _, _, _ := s.QueryLogs(ctx, "service", "api", "", "", "", "", "", 10)
	for _, e := range events {
		if e.Message == "zero-time-msg" {
			return
		}
	}
	t.Error("expected log with zero time to still be stored (occurredAt set to now)")
}

func TestLogHandler_WithAttrs(t *testing.T) {
	inner := slog.Default().Handler()
	h := &LogHandler{Inner: inner, Store: nil, Source: "x"}
	out := h.WithAttrs([]slog.Attr{slog.String("a", "b")})
	if out == h {
		t.Error("WithAttrs should return new handler when Inner is set")
	}
	if out.(*LogHandler).Source != "x" {
		t.Errorf("WithAttrs should preserve Source: %q", out.(*LogHandler).Source)
	}
	// nil Inner returns same handler
	hNil := &LogHandler{Inner: nil, Store: nil, Source: "x"}
	outNil := hNil.WithAttrs([]slog.Attr{slog.String("a", "b")})
	if outNil != hNil {
		t.Error("WithAttrs with nil Inner should return same handler")
	}
}

func TestLogHandler_WithGroup(t *testing.T) {
	inner := slog.Default().Handler()
	h := &LogHandler{Inner: inner, Store: nil, Source: "x"}
	out := h.WithGroup("g")
	if out == h {
		t.Error("WithGroup should return new handler when Inner is set")
	}
	hNil := &LogHandler{Inner: nil, Store: nil, Source: "x"}
	outNil := hNil.WithGroup("g")
	if outNil != hNil {
		t.Error("WithGroup with nil Inner should return same handler")
	}
}
