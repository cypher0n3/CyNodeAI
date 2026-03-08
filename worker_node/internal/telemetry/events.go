// Package telemetry: container_event and log_event writes per worker_telemetry_api.md.

package telemetry

import (
	"context"
	"encoding/json"
	"time"
)

// InsertContainerEvent inserts one container_event row (lifecycle transition).
func (s *Store) InsertContainerEvent(ctx context.Context, eventID, occurredAt, containerID, action, status string, exitCode *int, taskID, jobID string, details map[string]interface{}) error {
	detailsJSON := "{}"
	if details != nil {
		b, _ := json.Marshal(details)
		detailsJSON = string(b)
	}
	return s.db.WithContext(ctx).Create(&ContainerEvent{
		EventID:     eventID,
		OccurredAt:  occurredAt,
		ContainerID: containerID,
		Action:      action,
		Status:      status,
		ExitCode:    exitCode,
		TaskID:      taskID,
		JobID:       jobID,
		DetailsJSON: detailsJSON,
	}).Error
}

// Stream name constants for log events (schema CHECK allows only these).
const (
	StreamStdout = "stdout"
	StreamStderr = "stderr"
)

// LogEventInput is used to insert a log_event row per worker_telemetry_api.md.
type LogEventInput struct {
	LogID       string
	OccurredAt  string            // RFC 3339 UTC; empty => time.Now().UTC()
	SourceKind  string            // "service" or "container"
	SourceName  string            // e.g. "worker_api", "node_manager", or container name
	ContainerID string            // optional when source_kind=container
	Stream      string            // StreamStdout or StreamStderr for container
	Level       string            // optional
	Message     string
	Fields      map[string]string // optional structured fields
}

// InsertLogEvent inserts one log_event row.
// For source_kind=service, stream must still be a valid CHECK value; we use "stdout" as sentinel.
func (s *Store) InsertLogEvent(ctx context.Context, in *LogEventInput) error {
	if in == nil {
		return nil
	}
	if in.OccurredAt == "" {
		in.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	}
	stream := in.Stream
	if stream != StreamStdout && stream != StreamStderr {
		stream = StreamStdout // sentinel for service-origin events (schema CHECK allows only stdout/stderr)
	}
	fieldsJSON := "{}"
	if in.Fields != nil {
		b, _ := json.Marshal(in.Fields)
		fieldsJSON = string(b)
	}
	return s.db.WithContext(ctx).Create(&LogEvent{
		LogID:       in.LogID,
		OccurredAt:  in.OccurredAt,
		SourceKind:  in.SourceKind,
		SourceName:  in.SourceName,
		ContainerID: in.ContainerID,
		Stream:      stream,
		Level:       in.Level,
		Message:     in.Message,
		FieldsJSON:  fieldsJSON,
	}).Error
}
