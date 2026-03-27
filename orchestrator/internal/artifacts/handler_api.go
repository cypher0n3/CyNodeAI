package artifacts

import (
	"context"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

// CreateFromBodyInput carries scope, path, payload, and optional job/task correlation for CreateFromBody.
type CreateFromBodyInput struct {
	Level             string
	OwnerUserID       *uuid.UUID
	GroupID           *uuid.UUID
	ProjectID         *uuid.UUID
	ArtifactPath      string
	Body              []byte
	ContentType       *string
	CreatedByJobID    *uuid.UUID
	CorrelationTaskID *uuid.UUID
	RunID             *uuid.UUID
}

// HandlerAPI is the HTTP handler contract implemented by *Service (mockable in tests).
type HandlerAPI interface {
	CreateFromBody(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, in *CreateFromBodyInput) (*models.OrchestratorArtifact, error)
	GetBlob(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID) ([]byte, *models.OrchestratorArtifact, error)
	UpdateBlob(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID, body []byte, contentType *string, lastModJob *uuid.UUID) (*models.OrchestratorArtifact, error)
	Delete(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID) error
	List(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, p database.ListOrchestratorArtifactsParams) ([]*models.OrchestratorArtifact, error)
}

// Compile-time assertion: *Service must implement HandlerAPI. If methods or
// signatures drift, the build fails here instead of at unrelated call sites.
// (*Service)(nil) is a typed nil; no Service instance is allocated.
var _ HandlerAPI = (*Service)(nil)
