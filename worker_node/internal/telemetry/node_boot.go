// Package telemetry: node_boot table per worker_telemetry_api.md.

package telemetry

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// NodeBootRow is one node_boot row for insert or read (API type).
type NodeBootRow struct {
	BootID        string
	BootedAt      string // RFC 3339 UTC
	NodeSlug      string
	BuildVersion  string
	PlatformOS    string
	PlatformArch  string
	KernelVersion string
}

// InsertNodeBoot inserts one node_boot row.
// Idempotent per boot_id: call once per process boot (e.g. worker-api startup).
func (s *Store) InsertNodeBoot(ctx context.Context, row *NodeBootRow) error {
	if row == nil {
		return nil
	}
	bootedAt := row.BootedAt
	if bootedAt == "" {
		bootedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return s.db.WithContext(ctx).Create(&NodeBoot{
		BootID:        row.BootID,
		BootedAt:      bootedAt,
		NodeSlug:      row.NodeSlug,
		BuildVersion:  row.BuildVersion,
		PlatformOS:    row.PlatformOS,
		PlatformArch:  row.PlatformArch,
		KernelVersion: row.KernelVersion,
	}).Error
}

// GetLatestNodeBoot returns the most recent node_boot row by booted_at, or nil if none.
func (s *Store) GetLatestNodeBoot(ctx context.Context) (*NodeBootRow, error) {
	var m NodeBoot
	err := s.db.WithContext(ctx).Order("booted_at DESC").First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &NodeBootRow{
		BootID:        m.BootID,
		BootedAt:      m.BootedAt,
		NodeSlug:      m.NodeSlug,
		BuildVersion:  m.BuildVersion,
		PlatformOS:    m.PlatformOS,
		PlatformArch:  m.PlatformArch,
		KernelVersion: m.KernelVersion,
	}, nil
}
