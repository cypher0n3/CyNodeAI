package artifacts

import (
	"context"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// HandlerAPI is the HTTP handler contract implemented by *Service (mockable in tests).
type HandlerAPI interface {
	CreateFromBody(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, level string, ownerUserID, groupID, projectID *uuid.UUID, artifactPath string, body []byte, contentType *string, createdByJobID, correlationTaskID *uuid.UUID, runID *uuid.UUID) (*models.OrchestratorArtifact, error)
	GetBlob(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID) ([]byte, *models.OrchestratorArtifact, error)
	UpdateBlob(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID, body []byte, contentType *string, lastModJob *uuid.UUID) (*models.OrchestratorArtifact, error)
	Delete(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID) error
	List(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, p database.ListOrchestratorArtifactsParams) ([]*models.OrchestratorArtifact, error)
}

var _ HandlerAPI = (*Service)(nil)
