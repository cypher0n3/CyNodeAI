package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/google/uuid"
)

// SystemDeleteArtifact removes blob, vector rows, and DB metadata without RBAC checks.
// Intended for background jobs only (stale cleanup), not for HTTP handlers.
func (s *Service) SystemDeleteArtifact(ctx context.Context, id uuid.UUID) error {
	if s == nil || s.DB == nil || s.Blob == nil {
		return errors.New("artifacts service not configured")
	}
	art, err := s.DB.GetOrchestratorArtifactByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.Blob.DeleteObject(ctx, art.StorageRef); err != nil {
		return err
	}
	if err := s.DB.DeleteVectorItemsForArtifact(ctx, id); err != nil {
		return err
	}
	return s.DB.DeleteOrchestratorArtifactByID(ctx, id)
}

// BackfillMissingHashesOnce computes checksum_sha256 for up to batch rows that lack it (reads blob from storage).
func (s *Service) BackfillMissingHashesOnce(ctx context.Context, batch int) (int, error) {
	if s == nil || s.DB == nil || s.Blob == nil {
		return 0, errors.New("artifacts service not configured")
	}
	if batch <= 0 {
		batch = 50
	}
	ids, err := s.DB.ListOrchestratorArtifactIDsMissingChecksum(ctx, batch)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, id := range ids {
		art, err := s.DB.GetOrchestratorArtifactByID(ctx, id)
		if err != nil {
			continue
		}
		data, err := s.Blob.GetObject(ctx, art.StorageRef)
		if err != nil {
			continue
		}
		sum := sha256.Sum256(data)
		h := hex.EncodeToString(sum[:])
		sz := int64(len(data))
		szPtr := &sz
		if err := s.DB.UpdateOrchestratorArtifactMetadata(ctx, id, szPtr, art.ContentType, &h, nil); err != nil {
			continue
		}
		updated++
	}
	return updated, nil
}

// PruneStaleByMaxAgeOnce deletes up to batch artifacts whose created_at is older than now-maxAge (system job).
func (s *Service) PruneStaleByMaxAgeOnce(ctx context.Context, maxAge time.Duration, batch int) (int, error) {
	if s == nil {
		return 0, errors.New("artifacts service not configured")
	}
	if maxAge <= 0 {
		return 0, nil
	}
	if batch <= 0 {
		batch = 50
	}
	cutoff := time.Now().UTC().Add(-maxAge)
	ids, err := s.DB.ListOrchestratorArtifactIDsCreatedBefore(ctx, cutoff, batch)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, id := range ids {
		if err := s.SystemDeleteArtifact(ctx, id); err != nil {
			continue
		}
		n++
	}
	return n, nil
}
