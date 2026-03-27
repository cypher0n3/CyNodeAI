// Package artifacts implements scope-partitioned artifact storage (DB metadata + S3 blobs).
package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/s3blob"
)

// ErrForbidden is returned when RBAC denies access.
var ErrForbidden = errors.New("forbidden")

const (
	scopeLevelUser    = "user"
	scopeLevelGroup   = "group"
	scopeLevelProject = "project"
	scopeLevelGlobal  = "global"
	adminHandle       = "admin"
)

// Service coordinates PostgreSQL metadata and S3 blob storage.
type Service struct {
	DB                 *database.DB
	Blob               s3blob.BlobStore
	HashInlineMaxBytes int64
}

// ScopePartition builds the stable partition key for uniqueness (see orchestrator_artifacts_storage.md).
func ScopePartition(level string, ownerUserID, groupID, projectID *uuid.UUID) (string, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case scopeLevelUser:
		if ownerUserID == nil {
			return "", errors.New("owner_user_id required for user scope")
		}
		return "user:" + ownerUserID.String(), nil
	case scopeLevelGroup:
		if groupID == nil {
			return "", errors.New("group_id required for group scope")
		}
		return "group:" + groupID.String(), nil
	case scopeLevelProject:
		if projectID == nil {
			return "", errors.New("project_id required for project scope")
		}
		return "project:" + projectID.String(), nil
	case scopeLevelGlobal:
		return "global", nil
	default:
		return "", errors.New("invalid scope_level")
	}
}

// SanitizePath rejects traversal and empty paths.
func SanitizePath(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("path required")
	}
	if strings.Contains(p, "..") {
		return "", errors.New("path must not contain parent directory segments")
	}
	p = path.Clean("/" + p)
	if p == "/" || strings.HasPrefix(p, "/..") {
		return "", errors.New("invalid path")
	}
	out := strings.TrimPrefix(p, "/")
	if out == "" {
		return "", errors.New("invalid path")
	}
	return out, nil
}

// CreateFromBody uploads bytes and inserts metadata.
func (s *Service) CreateFromBody(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, in *CreateFromBodyInput) (*models.OrchestratorArtifact, error) {
	if s == nil || s.DB == nil || s.Blob == nil {
		return nil, errors.New("artifacts service not configured")
	}
	if in == nil {
		return nil, errors.New("create input required")
	}
	partition, err := ScopePartition(in.Level, in.OwnerUserID, in.GroupID, in.ProjectID)
	if err != nil {
		return nil, err
	}
	ap, err := SanitizePath(in.ArtifactPath)
	if err != nil {
		return nil, err
	}
	if err := s.canWriteScope(ctx, subjectUserID, subjectHandle, strings.ToLower(in.Level), in.OwnerUserID, in.GroupID, in.ProjectID); err != nil {
		return nil, err
	}
	id := uuid.New()
	storageKey := fmt.Sprintf("artifacts/%s", id.String())
	if err := s.Blob.PutObject(ctx, storageKey, in.Body, in.ContentType); err != nil {
		return nil, err
	}
	var checksum *string
	if s.HashInlineMaxBytes <= 0 || int64(len(in.Body)) <= s.HashInlineMaxBytes {
		sum := sha256.Sum256(in.Body)
		h := hex.EncodeToString(sum[:])
		checksum = &h
	}
	sz := int64(len(in.Body))
	row := &models.OrchestratorArtifact{
		OrchestratorArtifactBase: models.OrchestratorArtifactBase{
			ScopeLevel:          strings.ToLower(in.Level),
			ScopePartition:      partition,
			OwnerUserID:         in.OwnerUserID,
			GroupID:             in.GroupID,
			ProjectID:           in.ProjectID,
			Path:                ap,
			StorageRef:          storageKey,
			SizeBytes:           &sz,
			ContentType:         in.ContentType,
			ChecksumSHA256:      checksum,
			CreatedByJobID:      in.CreatedByJobID,
			LastModifiedByJobID: nil,
			CorrelationTaskID:   in.CorrelationTaskID,
			RunID:               in.RunID,
		},
		ID: id,
	}
	return s.DB.CreateOrchestratorArtifact(ctx, row)
}

// GetBlob resolves RBAC and returns bytes + metadata.
func (s *Service) GetBlob(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID) ([]byte, *models.OrchestratorArtifact, error) {
	if s == nil || s.DB == nil || s.Blob == nil {
		return nil, nil, errors.New("artifacts service not configured")
	}
	art, err := s.DB.GetOrchestratorArtifactByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	if err := s.canReadScope(ctx, subjectUserID, subjectHandle, art); err != nil {
		return nil, nil, err
	}
	data, err := s.Blob.GetObject(ctx, art.StorageRef)
	if err != nil {
		return nil, nil, err
	}
	return data, art, nil
}

// GetByScopePath returns blob for scope + path (MCP artifact.get).
func (s *Service) GetByScopePath(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, level string,
	ownerUserID, groupID, projectID *uuid.UUID, artifactPath string,
) ([]byte, *models.OrchestratorArtifact, error) {
	partition, err := ScopePartition(level, ownerUserID, groupID, projectID)
	if err != nil {
		return nil, nil, err
	}
	ap, err := SanitizePath(artifactPath)
	if err != nil {
		return nil, nil, err
	}
	art, err := s.DB.GetOrchestratorArtifactByScopePartitionAndPath(ctx, partition, ap)
	if err != nil {
		return nil, nil, err
	}
	if err := s.canReadScope(ctx, subjectUserID, subjectHandle, art); err != nil {
		return nil, nil, err
	}
	data, err := s.Blob.GetObject(ctx, art.StorageRef)
	if err != nil {
		return nil, nil, err
	}
	return data, art, nil
}

// UpdateBlob overwrites S3 and metadata.
func (s *Service) UpdateBlob(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID, body []byte, contentType *string, lastModJob *uuid.UUID) (*models.OrchestratorArtifact, error) {
	if s == nil || s.DB == nil || s.Blob == nil {
		return nil, errors.New("artifacts service not configured")
	}
	art, err := s.DB.GetOrchestratorArtifactByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.canWriteScope(ctx, subjectUserID, subjectHandle, art.ScopeLevel, art.OwnerUserID, art.GroupID, art.ProjectID); err != nil {
		return nil, err
	}
	if err := s.Blob.PutObject(ctx, art.StorageRef, body, contentType); err != nil {
		return nil, err
	}
	sz := int64(len(body))
	var checksum *string
	if s.HashInlineMaxBytes <= 0 || int64(len(body)) <= s.HashInlineMaxBytes {
		sum := sha256.Sum256(body)
		h := hex.EncodeToString(sum[:])
		checksum = &h
	}
	if err := s.DB.UpdateOrchestratorArtifactMetadata(ctx, id, &sz, contentType, checksum, lastModJob); err != nil {
		return nil, err
	}
	return s.DB.GetOrchestratorArtifactByID(ctx, id)
}

// Delete removes S3 object, vector rows, and DB row.
func (s *Service) Delete(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID) error {
	if s == nil || s.DB == nil || s.Blob == nil {
		return errors.New("artifacts service not configured")
	}
	art, err := s.DB.GetOrchestratorArtifactByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.canDeleteScope(ctx, subjectUserID, subjectHandle, art); err != nil {
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

// List returns metadata rows visible under RBAC.
func (s *Service) List(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, p database.ListOrchestratorArtifactsParams) ([]*models.OrchestratorArtifact, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("artifacts service not configured")
	}
	if err := s.canListScope(ctx, subjectUserID, subjectHandle, p); err != nil {
		return nil, err
	}
	return s.DB.ListOrchestratorArtifacts(ctx, p)
}

func (s *Service) canReadScope(ctx context.Context, userID uuid.UUID, handle string, art *models.OrchestratorArtifact) error {
	if art == nil {
		return errors.New("nil artifact")
	}
	if strings.EqualFold(art.ScopeLevel, scopeLevelUser) {
		if art.OwnerUserID != nil && *art.OwnerUserID == userID {
			return nil
		}
		ok, err := s.DB.HasArtifactReadGrant(ctx, art.ID, userID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		return ErrForbidden
	}
	return s.checkScope(ctx, userID, handle, art.ScopeLevel, art.OwnerUserID, art.GroupID, art.ProjectID, false)
}

func (s *Service) canWriteScope(ctx context.Context, userID uuid.UUID, handle, level string, owner, group, project *uuid.UUID) error {
	return s.checkScope(ctx, userID, handle, level, owner, group, project, true)
}

func (s *Service) canDeleteScope(ctx context.Context, userID uuid.UUID, handle string, art *models.OrchestratorArtifact) error {
	return s.checkScope(ctx, userID, handle, art.ScopeLevel, art.OwnerUserID, art.GroupID, art.ProjectID, true)
}

func (s *Service) canListScope(ctx context.Context, userID uuid.UUID, handle string, p database.ListOrchestratorArtifactsParams) error {
	_, err := ScopePartition(p.ScopeLevel, p.OwnerUserID, p.GroupID, p.ProjectID)
	if err != nil {
		return err
	}
	switch strings.ToLower(p.ScopeLevel) {
	case scopeLevelUser:
		if p.OwnerUserID != nil && *p.OwnerUserID == userID {
			return nil
		}
		return ErrForbidden
	case scopeLevelProject:
		if p.ProjectID == nil {
			return errors.New("project_id required")
		}
		return s.projectAccess(ctx, userID, handle, *p.ProjectID)
	case scopeLevelGroup:
		if handle == adminHandle {
			return nil
		}
		return ErrForbidden
	case scopeLevelGlobal:
		if handle == adminHandle {
			return nil
		}
		return ErrForbidden
	default:
		return errors.New("invalid scope_level")
	}
}

func (s *Service) checkScope(ctx context.Context, userID uuid.UUID, handle, level string, owner, group, project *uuid.UUID, write bool) error {
	switch strings.ToLower(level) {
	case scopeLevelUser:
		if owner != nil && *owner == userID {
			return nil
		}
		return ErrForbidden
	case scopeLevelProject:
		if project == nil {
			return errors.New("project_id required for project scope")
		}
		return s.projectAccess(ctx, userID, handle, *project)
	case scopeLevelGroup:
		if handle == adminHandle {
			return nil
		}
		return ErrForbidden
	case scopeLevelGlobal:
		if handle == adminHandle {
			return nil
		}
		return ErrForbidden
	default:
		return errors.New("invalid scope_level")
	}
}

func (s *Service) projectAccess(ctx context.Context, userID uuid.UUID, handle string, projectID uuid.UUID) error {
	if handle == adminHandle {
		return nil
	}
	projs, err := s.DB.ListAuthorizedProjectsForUser(ctx, userID, "", 50, 0)
	if err != nil {
		return err
	}
	for _, p := range projs {
		if p.ID == projectID {
			return nil
		}
	}
	return ErrForbidden
}

// MCPPut decodes base64 content and creates or overwrites by scope+path.
func (s *Service) MCPPut(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, args map[string]interface{}) (map[string]interface{}, error) {
	pathStr := strArg(args, "path")
	scope := strArg(args, "scope")
	b64 := strArg(args, "content_bytes_base64")
	if pathStr == "" || scope == "" || b64 == "" {
		return nil, errors.New("path, scope, and content_bytes_base64 required")
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, errors.New("invalid content_bytes_base64")
	}
	var ownerID, groupID, projectID *uuid.UUID
	if u := uuidArg(args, "user_id"); u != nil {
		ownerID = u
	} else {
		ownerID = &subjectUserID
	}
	groupID = uuidArg(args, "group_id")
	projectID = uuidArg(args, "project_id")
	ct := strArg(args, "content_type")
	var ctPtr *string
	if ct != "" {
		ctPtr = &ct
	}
	// Upsert: if exists, update
	partition, err := ScopePartition(scope, ownerID, groupID, projectID)
	if err != nil {
		return nil, err
	}
	ap, err := SanitizePath(pathStr)
	if err != nil {
		return nil, err
	}
	existing, err := s.DB.GetOrchestratorArtifactByScopePartitionAndPath(ctx, partition, ap)
	if err == nil && existing != nil {
		updated, uerr := s.UpdateBlob(ctx, subjectUserID, subjectHandle, existing.ID, raw, ctPtr, uuidArg(args, "job_id"))
		if uerr != nil {
			return nil, uerr
		}
		return map[string]interface{}{
			"status":       "success",
			"artifact_id":  updated.ID.String(),
			"path":         updated.Path,
			"scope_level":  updated.ScopeLevel,
			"size_bytes":   updated.SizeBytes,
			"content_type": updated.ContentType,
		}, nil
	}
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, err
	}
	art, err := s.CreateFromBody(ctx, subjectUserID, subjectHandle, &CreateFromBodyInput{
		Level:             scope,
		OwnerUserID:       ownerID,
		GroupID:           groupID,
		ProjectID:         projectID,
		ArtifactPath:      pathStr,
		Body:              raw,
		ContentType:       ctPtr,
		CreatedByJobID:    uuidArg(args, "job_id"),
		CorrelationTaskID: uuidArg(args, "task_id"),
	})
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":       "success",
		"artifact_id":  art.ID.String(),
		"path":         art.Path,
		"scope_level":  art.ScopeLevel,
		"size_bytes":   art.SizeBytes,
		"content_type": art.ContentType,
	}, nil
}

// MCPGet returns base64 content by scope + path.
func (s *Service) MCPGet(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, args map[string]interface{}) (map[string]interface{}, error) {
	pathStr := strArg(args, "path")
	scope := strArg(args, "scope")
	if pathStr == "" || scope == "" {
		return nil, errors.New("path and scope required")
	}
	ownerID := uuidArg(args, "user_id")
	if ownerID == nil {
		ownerID = &subjectUserID
	}
	groupID := uuidArg(args, "group_id")
	projectID := uuidArg(args, "project_id")
	data, art, err := s.GetByScopePath(ctx, subjectUserID, subjectHandle, scope, ownerID, groupID, projectID, pathStr)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"status":               "success",
		"artifact_id":          art.ID.String(),
		"path":                 art.Path,
		"content_bytes_base64": base64.StdEncoding.EncodeToString(data),
		"content_type":         art.ContentType,
		"size_bytes":           art.SizeBytes,
		"checksum_sha256":      art.ChecksumSHA256,
	}, nil
}

// MCPList lists paths in a scope partition.
func (s *Service) MCPList(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, args map[string]interface{}) (map[string]interface{}, error) {
	scope := strArg(args, "scope")
	if scope == "" {
		return nil, errors.New("scope required")
	}
	ownerID := uuidArg(args, "user_id")
	if ownerID == nil {
		ownerID = &subjectUserID
	}
	groupID := uuidArg(args, "group_id")
	projectID := uuidArg(args, "project_id")
	limit := intArg(args, "limit")
	if limit <= 0 {
		limit = 100
	}
	p := database.ListOrchestratorArtifactsParams{
		ScopeLevel:  scope,
		OwnerUserID: ownerID,
		GroupID:     groupID,
		ProjectID:   projectID,
		Limit:       limit,
	}
	if tid := uuidArg(args, "task_id"); tid != nil {
		p.CorrelationTaskID = tid
	}
	rows, err := s.List(ctx, subjectUserID, subjectHandle, p)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(rows))
	for _, r := range rows {
		items = append(items, map[string]interface{}{
			"artifact_id": r.ID.String(),
			"path":        r.Path,
			"size_bytes":  r.SizeBytes,
			"created_at":  r.CreatedAt,
		})
	}
	return map[string]interface{}{"status": "success", "artifacts": items}, nil
}

func strArg(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	v, _ := args[key].(string)
	return v
}

func intArg(args map[string]interface{}, key string) int {
	if args == nil {
		return 0
	}
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func uuidArg(args map[string]interface{}, key string) *uuid.UUID {
	if args == nil {
		return nil
	}
	s, ok := args[key].(string)
	if !ok || s == "" {
		return nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil
	}
	return &id
}
