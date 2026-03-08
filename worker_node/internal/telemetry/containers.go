package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ContainerRow represents one container_inventory row for API response and for upsert input.
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

// ListContainers returns containers with optional filters and pagination. Max limit 1000.
func (s *Store) ListContainers(ctx context.Context, kind, status, taskID, jobID, pageToken string, limit int) ([]ContainerRow, string, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	offset := 0
	if pageToken != "" {
		if _, err := fmt.Sscanf(pageToken, "%d", &offset); err != nil {
			offset = 0
		}
	}
	q := s.db.WithContext(ctx).Model(&ContainerInventory{})
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if taskID != "" {
		q = q.Where("task_id = ?", taskID)
	}
	if jobID != "" {
		q = q.Where("job_id = ?", jobID)
	}
	var rows []ContainerInventory
	if err := q.Order("container_id").Offset(offset).Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", err
	}
	list := make([]ContainerRow, 0, len(rows))
	for i := range rows {
		c := containerInventoryToRow(&rows[i])
		list = append(list, c)
	}
	nextToken := ""
	if len(list) > limit {
		list = list[:limit]
		nextToken = fmt.Sprintf("%d", offset+limit)
	}
	return list, nextToken, nil
}

func containerInventoryToRow(m *ContainerInventory) ContainerRow {
	c := ContainerRow{
		ContainerID:   m.ContainerID,
		ContainerName: m.ContainerName,
		Kind:          m.Kind,
		Runtime:       m.Runtime,
		ImageRef:      m.ImageRef,
		CreatedAt:     m.CreatedAt,
		LastSeenAt:    m.LastSeenAt,
		Status:        m.Status,
		ExitCode:      m.ExitCode,
		TaskID:        m.TaskID,
		JobID:         m.JobID,
		Labels:        make(map[string]string),
	}
	_ = json.Unmarshal([]byte(m.LabelsJSON), &c.Labels)
	return c
}

// UpsertContainerInventory inserts or replaces one container_inventory row (by container_id).
func (s *Store) UpsertContainerInventory(ctx context.Context, in *ContainerRow) error {
	if in == nil {
		return nil
	}
	labelsJSON := "{}"
	if in.Labels != nil {
		b, _ := json.Marshal(in.Labels)
		labelsJSON = string(b)
	}
	m := ContainerInventory{
		ContainerID:   in.ContainerID,
		ContainerName: in.ContainerName,
		Kind:          in.Kind,
		Runtime:       in.Runtime,
		ImageRef:      in.ImageRef,
		CreatedAt:     in.CreatedAt,
		LastSeenAt:    in.LastSeenAt,
		Status:        in.Status,
		ExitCode:      in.ExitCode,
		TaskID:        in.TaskID,
		JobID:         in.JobID,
		LabelsJSON:    labelsJSON,
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "container_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"container_name", "kind", "runtime", "image_ref", "last_seen_at", "status", "exit_code", "task_id", "job_id", "labels_json"}),
	}).Create(&m).Error
}

// GetContainer returns one container by ID or nil if not found.
func (s *Store) GetContainer(ctx context.Context, containerID string) (*ContainerRow, error) {
	var m ContainerInventory
	err := s.db.WithContext(ctx).Where("container_id = ?", containerID).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	c := containerInventoryToRow(&m)
	return &c, nil
}