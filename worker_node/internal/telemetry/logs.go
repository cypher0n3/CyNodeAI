package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// LogEventRow is one log_event for API response.
type LogEventRow struct {
	OccurredAt  string            `json:"occurred_at"`
	SourceKind  string            `json:"source_kind"`
	SourceName  string            `json:"source_name"`
	ContainerID string            `json:"container_id,omitempty"`
	Stream      string            `json:"stream,omitempty"`
	Level       string            `json:"level,omitempty"`
	Message     string            `json:"message"`
	Fields      map[string]string `json:"fields"`
}

// TruncatedMetadata describes log response truncation per spec.
type TruncatedMetadata struct {
	LimitedBy string `json:"limited_by"` // count | bytes | none
	MaxBytes  int    `json:"max_bytes"`
}

const limitedByBytes = "bytes"
const limitedByCount = "count"
const limitedByNone = "none"

func buildLogsQuery(sourceKind, sourceName, containerID, stream, since, until, pageToken string, limit int) (q string, args []interface{}, offset int, err error) {
	if sourceKind == "" && containerID == "" {
		return "", nil, 0, fmt.Errorf("at least one of source_kind+source_name or source_kind=container+container_id required")
	}
	q = "SELECT occurred_at, source_kind, source_name, container_id, stream, level, message, fields_json FROM log_event WHERE 1=1"
	if sourceKind != "" {
		q += " AND source_kind = ?"
		args = append(args, sourceKind)
	}
	if sourceName != "" {
		q += " AND source_name = ?"
		args = append(args, sourceName)
	}
	if containerID != "" {
		q += " AND container_id = ?"
		args = append(args, containerID)
	}
	if stream != "" {
		q += " AND stream = ?"
		args = append(args, stream)
	}
	if since != "" {
		q += " AND occurred_at >= ?"
		args = append(args, since)
	}
	if until != "" {
		q += " AND occurred_at < ?"
		args = append(args, until)
	}
	q += " ORDER BY occurred_at, log_id LIMIT ? OFFSET ?"
	if pageToken != "" {
		var o int
		if _, scanErr := fmt.Sscanf(pageToken, "%d", &o); scanErr == nil && o >= 0 {
			offset = o
		}
	}
	args = append(args, limit+1, offset)
	return q, args, offset, nil
}

func scanLogEventRow(containerIDVal, streamVal, levelVal sql.NullString, fieldsJSON string, e *LogEventRow) {
	if containerIDVal.Valid {
		e.ContainerID = containerIDVal.String
	}
	if streamVal.Valid {
		e.Stream = streamVal.String
	}
	if levelVal.Valid {
		e.Level = levelVal.String
	}
	e.Fields = make(map[string]string)
	_ = json.Unmarshal([]byte(fieldsJSON), &e.Fields)
}

// QueryLogs returns events with optional filters and pagination. Max response 1 MiB.
func (s *Store) QueryLogs(ctx context.Context, sourceKind, sourceName, containerID, stream, since, until, pageToken string, limit int) (events []LogEventRow, truncated TruncatedMetadata, nextToken string, err error) {
	truncated.MaxBytes = maxLogRespBytes
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	q, args, offset, err := buildLogsQuery(sourceKind, sourceName, containerID, stream, since, until, pageToken, limit)
	if err != nil {
		return nil, truncated, "", err
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, truncated, "", err
	}
	defer func() { _ = rows.Close() }()
	var list []LogEventRow
	approxBytes := 0
	for rows.Next() {
		var e LogEventRow
		var containerIDVal sql.NullString
		var streamVal, levelVal sql.NullString
		var fieldsJSON string
		if err := rows.Scan(&e.OccurredAt, &e.SourceKind, &e.SourceName, &containerIDVal, &streamVal, &levelVal, &e.Message, &fieldsJSON); err != nil {
			return nil, truncated, "", err
		}
		scanLogEventRow(containerIDVal, streamVal, levelVal, fieldsJSON, &e)
		approxBytes += len(e.Message) + len(e.OccurredAt) + len(fieldsJSON) + 128
		if approxBytes > maxLogRespBytes {
			truncated.LimitedBy = limitedByBytes
			break
		}
		list = append(list, e)
	}
	if err := rows.Err(); err != nil {
		return nil, truncated, "", err
	}
	if len(list) > limit {
		list = list[:limit]
		if truncated.LimitedBy == "" {
			truncated.LimitedBy = limitedByCount
		}
		nextToken = fmt.Sprintf("%d", offset+limit)
	}
	if truncated.LimitedBy == "" {
		truncated.LimitedBy = limitedByNone
	}
	return list, truncated, nextToken, nil
}
