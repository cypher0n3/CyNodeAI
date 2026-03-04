package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// ContainerRow represents one container_inventory row for API response.
type ContainerRow struct {
	ContainerID   string            `json:"container_id"`
	ContainerName string            `json:"container_name"`
	Kind          string            `json:"kind"`
	Runtime       string            `json:"runtime"`
	ImageRef      string            `json:"image_ref"`
	CreatedAt     string            `json:"created_at"`
	LastSeenAt    string            `json:"last_seen_at"`
	Status        string            `json:"status"`
	ExitCode      *int              `json:"exit_code,omitempty"`
	TaskID        string            `json:"task_id,omitempty"`
	JobID         string            `json:"job_id,omitempty"`
	Labels        map[string]string `json:"labels"`
}

func listContainersQuery(kind, status, taskID, jobID, pageToken string, limit int) (q string, args []interface{}, offset int) {
	q = "SELECT container_id, container_name, kind, runtime, image_ref, created_at, last_seen_at, status, exit_code, task_id, job_id, labels_json FROM container_inventory WHERE 1=1"
	if kind != "" {
		q += " AND kind = ?"
		args = append(args, kind)
	}
	if status != "" {
		q += " AND status = ?"
		args = append(args, status)
	}
	if taskID != "" {
		q += " AND task_id = ?"
		args = append(args, taskID)
	}
	if jobID != "" {
		q += " AND job_id = ?"
		args = append(args, jobID)
	}
	q += " ORDER BY container_id LIMIT ? OFFSET ?"
	if pageToken != "" {
		var o int
		if _, err := fmt.Sscanf(pageToken, "%d", &o); err == nil && o >= 0 {
			offset = o
		}
	}
	args = append(args, limit+1, offset)
	return q, args, offset
}

func scanContainerRow(exitCode sql.NullInt64, taskIDVal, jobIDVal sql.NullString, labelsJSON string, c *ContainerRow) {
	if exitCode.Valid {
		v := int(exitCode.Int64)
		c.ExitCode = &v
	}
	if taskIDVal.Valid {
		c.TaskID = taskIDVal.String
	}
	if jobIDVal.Valid {
		c.JobID = jobIDVal.String
	}
	c.Labels = make(map[string]string)
	_ = json.Unmarshal([]byte(labelsJSON), &c.Labels)
}

// ListContainers returns containers with optional filters and pagination. Max limit 1000.
func (s *Store) ListContainers(ctx context.Context, kind, status, taskID, jobID, pageToken string, limit int) ([]ContainerRow, string, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	q, args, offset := listContainersQuery(kind, status, taskID, jobID, pageToken, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = rows.Close() }()
	var list []ContainerRow
	for rows.Next() {
		var c ContainerRow
		var exitCode sql.NullInt64
		var taskIDVal, jobIDVal sql.NullString
		var labelsJSON string
		if err := rows.Scan(&c.ContainerID, &c.ContainerName, &c.Kind, &c.Runtime, &c.ImageRef, &c.CreatedAt, &c.LastSeenAt, &c.Status, &exitCode, &taskIDVal, &jobIDVal, &labelsJSON); err != nil {
			return nil, "", err
		}
		scanContainerRow(exitCode, taskIDVal, jobIDVal, labelsJSON, &c)
		list = append(list, c)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	nextToken := ""
	if len(list) > limit {
		list = list[:limit]
		nextToken = fmt.Sprintf("%d", offset+limit)
	}
	return list, nextToken, nil
}

// GetContainer returns one container by ID or nil if not found.
func (s *Store) GetContainer(ctx context.Context, containerID string) (*ContainerRow, error) {
	var c ContainerRow
	var exitCode sql.NullInt64
	var taskIDVal, jobIDVal sql.NullString
	var labelsJSON string
	err := s.db.QueryRowContext(ctx,
		"SELECT container_id, container_name, kind, runtime, image_ref, created_at, last_seen_at, status, exit_code, task_id, job_id, labels_json FROM container_inventory WHERE container_id = ?",
		containerID,
	).Scan(&c.ContainerID, &c.ContainerName, &c.Kind, &c.Runtime, &c.ImageRef, &c.CreatedAt, &c.LastSeenAt, &c.Status, &exitCode, &taskIDVal, &jobIDVal, &labelsJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	scanContainerRow(exitCode, taskIDVal, jobIDVal, labelsJSON, &c)
	return &c, nil
}
