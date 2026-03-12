package telemetry

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
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

// QueryLogs returns events with optional filters and pagination. Max response 1 MiB.
func (s *Store) QueryLogs(ctx context.Context, sourceKind, sourceName, containerID, stream, since, until, pageToken string, limit int) (events []LogEventRow, truncated TruncatedMetadata, nextToken string, err error) {
	truncated.MaxBytes = maxLogRespBytes
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	if sourceKind == "" && containerID == "" {
		return nil, truncated, "", fmt.Errorf("at least one of source_kind+source_name or source_kind=container+container_id required")
	}
	offset := parseLogPageToken(pageToken)
	q := s.buildLogQuery(ctx, sourceKind, sourceName, containerID, stream, since, until)
	var rows []LogEvent
	if err := q.Order("occurred_at, log_id").Offset(offset).Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, truncated, "", err
	}
	list, truncated, nextToken := applyLogTruncation(rows, limit, offset, truncated)
	return list, truncated, nextToken, nil
}

func parseLogPageToken(pageToken string) int {
	var offset int
	if n, _ := fmt.Sscanf(pageToken, "%d", &offset); n == 1 && offset >= 0 {
		return offset
	}
	return 0
}

func (s *Store) buildLogQuery(ctx context.Context, sourceKind, sourceName, containerID, stream, since, until string) *gorm.DB {
	q := s.db.WithContext(ctx).Model(&LogEvent{})
	if sourceKind != "" {
		q = q.Where("source_kind = ?", sourceKind)
	}
	if sourceName != "" {
		q = q.Where("source_name = ?", sourceName)
	}
	if containerID != "" {
		q = q.Where("container_id = ?", containerID)
	}
	if stream != "" {
		q = q.Where("stream = ?", stream)
	}
	if since != "" {
		q = q.Where("occurred_at >= ?", since)
	}
	if until != "" {
		q = q.Where("occurred_at < ?", until)
	}
	return q
}

func applyLogTruncation(rows []LogEvent, limit, offset int, truncated TruncatedMetadata) ([]LogEventRow, TruncatedMetadata, string) {
	list := make([]LogEventRow, 0, len(rows))
	approxBytes := 0
	for i := range rows {
		e := logEventToRow(&rows[i])
		approxBytes += len(e.Message) + len(e.OccurredAt) + 128
		if approxBytes > maxLogRespBytes {
			truncated.LimitedBy = limitedByBytes
			break
		}
		list = append(list, e)
	}
	var nextToken string
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
	return list, truncated, nextToken
}

func logEventToRow(m *LogEvent) LogEventRow {
	e := LogEventRow{
		OccurredAt:  m.OccurredAt,
		SourceKind:  m.SourceKind,
		SourceName:  m.SourceName,
		ContainerID: m.ContainerID,
		Stream:      m.Stream,
		Level:       m.Level,
		Message:     m.Message,
		Fields:      make(map[string]string),
	}
	_ = json.Unmarshal([]byte(m.FieldsJSON), &e.Fields)
	return e
}
