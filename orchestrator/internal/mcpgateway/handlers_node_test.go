package mcpgateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestToolCallHandler_NodeList_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	_, _ = mock.CreateNode(context.Background(), "worker-alpha")
	body := `{"tool_name":"node.list","arguments":{}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d %s", code, respBody)
	}
	var out struct {
		Nodes []map[string]interface{} `json:"nodes"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Nodes) < 1 {
		t.Errorf("expected at least one node")
	}
}

func TestToolCallHandler_NodeGet_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	node, _ := mock.CreateNode(context.Background(), "worker-beta")
	body := `{"tool_name":"node.get","arguments":{"node_slug":"worker-beta"}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d %s", code, respBody)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["node_slug"] != node.NodeSlug {
		t.Errorf("node_slug = %v", parsed["node_slug"])
	}
}

func TestToolCallHandler_NodeGet_NotFound(t *testing.T) {
	mock := testutil.NewMockDB()
	body := `{"tool_name":"node.get","arguments":{"node_slug":"missing-node"}}`
	code, _ := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusNotFound {
		t.Fatalf("got status %d want 404", code)
	}
}

func TestToolCallHandler_NodeGet_BadArgs(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"node.get","arguments":{}}`, http.StatusBadRequest)
}

func TestToolCallHandler_NodeList_InvalidCursor(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"node.list","arguments":{"cursor":"not-a-number"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_NodeList_StoreError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("db down")
	code, body := callToolHandlerWithStoreAndBody(t, mock, `{"tool_name":"node.list","arguments":{}}`)
	if code != http.StatusInternalServerError {
		t.Fatalf("got %d %s", code, body)
	}
}

func TestToolCallHandler_NodeList_NextCursor(t *testing.T) {
	mock := testutil.NewMockDB()
	for i := range 3 {
		_, _ = mock.CreateNode(context.Background(), fmt.Sprintf("node-cursor-%d", i))
	}
	code, body := callToolHandlerWithStoreAndBody(t, mock, `{"tool_name":"node.list","arguments":{"limit":2}}`)
	if code != http.StatusOK {
		t.Fatalf("got %d %s", code, body)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out["next_cursor"] == nil {
		t.Fatal("expected next_cursor when more nodes than limit")
	}
}
