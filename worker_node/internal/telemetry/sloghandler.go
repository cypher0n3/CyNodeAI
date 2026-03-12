// Package telemetry: slog handler that writes log events to the telemetry store per worker_telemetry_api.md.

package telemetry

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// LogHandler wraps a slog.Handler and also writes each log record to the telemetry store as a log_event (service, source_name=worker_api).
// Insert failures are ignored to avoid feedback loops. Safe for concurrent use.
type LogHandler struct {
	Inner  slog.Handler
	Store  *Store
	Source string // source_name, e.g. "worker_api"
}

// Enabled reports whether the inner handler would log at level.
func (h *LogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Inner != nil && h.Inner.Enabled(ctx, level)
}

// Handle implements slog.Handler. It forwards to Inner and, if Store is set, inserts a log_event asynchronously.
//
//nolint:gocritic // hugeParam: slog.Handler.Handle requires value by interface.
func (h *LogHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.Inner != nil {
		_ = h.Inner.Handle(ctx, r)
	}
	if h.Store == nil || h.Source == "" {
		return nil
	}
	occurredAt := r.Time.UTC().Format(time.RFC3339)
	if r.Time.IsZero() {
		occurredAt = time.Now().UTC().Format(time.RFC3339)
	}
	fields := make(map[string]string)
	r.Attrs(func(a slog.Attr) bool {
		fields[a.Key] = a.Value.String()
		return true
	})
	in := LogEventInput{
		LogID:      uuid.New().String(),
		OccurredAt: occurredAt,
		SourceKind: "service",
		SourceName: h.Source,
		Level:      r.Level.String(),
		Message:    r.Message,
		Fields:     fields,
	}
	go func() {
		bg := context.WithoutCancel(ctx)
		_ = h.Store.InsertLogEvent(bg, &in)
	}()
	return nil
}

// WithAttrs returns a handler that includes attrs. Forwards to Inner.WithAttrs and keeps Store/Source.
func (h *LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if h.Inner == nil {
		return h
	}
	return &LogHandler{Inner: h.Inner.WithAttrs(attrs), Store: h.Store, Source: h.Source}
}

// WithGroup returns a handler for the group. Forwards to Inner.WithGroup and keeps Store/Source.
func (h *LogHandler) WithGroup(name string) slog.Handler {
	if h.Inner == nil {
		return h
	}
	return &LogHandler{Inner: h.Inner.WithGroup(name), Store: h.Store, Source: h.Source}
}
