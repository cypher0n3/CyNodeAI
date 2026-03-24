package mcpgateway

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/artifacts"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestArtifactToolErr(t *testing.T) {
	t.Parallel()
	rec := &models.McpToolCallAuditLog{}
	code, body, _ := artifactToolErr(rec, database.ErrNotFound)
	if code != http.StatusNotFound || string(body) != `{"error":"not found"}` {
		t.Fatalf("not found: %d %s", code, body)
	}
	code, body, _ = artifactToolErr(rec, artifacts.ErrForbidden)
	if code != http.StatusForbidden || string(body) != `{"error":"forbidden"}` {
		t.Fatalf("forbidden: %d %s", code, body)
	}
	code, body, _ = artifactToolErr(rec, errors.New("bad arg"))
	if code != http.StatusBadRequest {
		t.Fatalf("generic: %d %s", code, body)
	}
	code, body, _ = artifactToolErr(rec, nil)
	if code != http.StatusBadRequest || string(body) != `{"error":"bad request"}` {
		t.Fatalf("nil err: %d %s", code, body)
	}
}

func TestHandleArtifactGet_Put_List_nilService(t *testing.T) {
	t.Cleanup(func() { SetArtifactToolService(nil) })
	SetArtifactToolService(nil)
	rec := &models.McpToolCallAuditLog{}
	store := testutil.NewMockDB()
	ctx := context.Background()
	args := map[string]interface{}{}
	for _, h := range []mcpToolHandler{
		artifactToolHandler((*artifacts.Service).MCPGet),
		artifactToolHandler((*artifacts.Service).MCPPut),
		artifactToolHandler((*artifacts.Service).MCPList),
	} {
		code, body, _ := h(ctx, store, args, rec)
		if code != http.StatusServiceUnavailable {
			t.Fatalf("want 503, got %d %s", code, body)
		}
	}
}
