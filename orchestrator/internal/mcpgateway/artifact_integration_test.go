package mcpgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/artifacts"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/s3blob"
)

func TestIntegration_ArtifactTools_putGetList(t *testing.T) {
	ctx := context.Background()
	db := tcMCPIntegrationDB(t, ctx)
	t.Cleanup(func() { SetArtifactToolService(nil) })
	SetArtifactToolService(artifacts.NewServiceWithBlob(db, s3blob.NewMemStore(), 1024))

	user, err := db.CreateUser(ctx, "mcp-art-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID.String()

	putBody := fmt.Sprintf(`{"tool_name":"artifact.put","arguments":{"user_id":%q,"path":"gw/mcp.txt","scope":"user","content_bytes_base64":"Zm9v","content_type":"text/plain"}}`, uid)
	code, putResp := callToolHandlerWithStoreAndBody(t, db, putBody)
	if code != http.StatusOK {
		t.Fatalf("artifact.put: status %d body %s", code, putResp)
	}

	getBody := fmt.Sprintf(`{"tool_name":"artifact.get","arguments":{"user_id":%q,"path":"gw/mcp.txt","scope":"user"}}`, uid)
	code, getResp := callToolHandlerWithStoreAndBody(t, db, getBody)
	if code != http.StatusOK {
		t.Fatalf("artifact.get: status %d body %s", code, getResp)
	}
	var getOut map[string]interface{}
	if err := json.Unmarshal(getResp, &getOut); err != nil {
		t.Fatalf("unmarshal get: %v", err)
	}
	if getOut["content_bytes_base64"] != "Zm9v" {
		t.Fatalf("content: %v", getOut["content_bytes_base64"])
	}

	listBody := fmt.Sprintf(`{"tool_name":"artifact.list","arguments":{"user_id":%q,"scope":"user"}}`, uid)
	code, listResp := callToolHandlerWithStoreAndBody(t, db, listBody)
	if code != http.StatusOK {
		t.Fatalf("artifact.list: status %d body %s", code, listResp)
	}
	var listOut map[string]interface{}
	if err := json.Unmarshal(listResp, &listOut); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listOut["status"] != "success" {
		t.Fatalf("list status: %v", listOut)
	}
}

func TestIntegration_ArtifactTools_missingUserIDAndNotFound(t *testing.T) {
	ctx := context.Background()
	db := tcMCPIntegrationDB(t, ctx)
	t.Cleanup(func() { SetArtifactToolService(nil) })
	SetArtifactToolService(artifacts.NewServiceWithBlob(db, s3blob.NewMemStore(), 1024))

	user, err := db.CreateUser(ctx, "mcp-art2-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID.String()

	code, body := callToolHandlerWithStoreAndBody(t, db, `{"tool_name":"artifact.get","arguments":{"path":"x","scope":"user"}}`)
	if code != http.StatusBadRequest {
		t.Fatalf("missing user_id: %d %s", code, body)
	}

	getNF := fmt.Sprintf(`{"tool_name":"artifact.get","arguments":{"user_id":%q,"path":"missing/path.txt","scope":"user"}}`, uid)
	code, body = callToolHandlerWithStoreAndBody(t, db, getNF)
	if code != http.StatusNotFound {
		t.Fatalf("not found: %d %s", code, body)
	}

	unknownUser := uuid.New().String()
	getBadUser := fmt.Sprintf(`{"tool_name":"artifact.get","arguments":{"user_id":%q,"path":"x","scope":"user"}}`, unknownUser)
	code, body = callToolHandlerWithStoreAndBody(t, db, getBadUser)
	if code != http.StatusNotFound {
		t.Fatalf("unknown user: %d %s", code, body)
	}

	putBadUser := fmt.Sprintf(`{"tool_name":"artifact.put","arguments":{"user_id":%q,"path":"x","scope":"user","content_bytes_base64":"Zm9v","content_type":"text/plain"}}`, unknownUser)
	code, body = callToolHandlerWithStoreAndBody(t, db, putBadUser)
	if code != http.StatusNotFound {
		t.Fatalf("put unknown user: %d %s", code, body)
	}

	listBadUser := fmt.Sprintf(`{"tool_name":"artifact.list","arguments":{"user_id":%q,"scope":"user"}}`, unknownUser)
	code, body = callToolHandlerWithStoreAndBody(t, db, listBadUser)
	if code != http.StatusNotFound {
		t.Fatalf("list unknown user: %d %s", code, body)
	}
}
