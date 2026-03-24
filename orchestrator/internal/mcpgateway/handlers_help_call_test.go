package mcpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestToolCallHandler_HelpList_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	body := jsonMCPBodyHelpList
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d body %s", code, respBody)
	}
	var out struct {
		Topics []map[string]interface{} `json:"topics"`
		Hint   string                   `json:"hint"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Topics) < 1 {
		t.Errorf("expected topics, got %+v", out)
	}
}

func TestToolCallHandler_HelpGet_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	body := `{"tool_name":"help.get","arguments":{}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d", code)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["content"] == nil || out["content"] == "" {
		t.Errorf("expected non-empty content, got %v", out)
	}
	if _, has := out["task_id"]; has {
		t.Errorf("unexpected task_id in response: %v", out)
	}
}

func TestToolCallHandler_HelpGet_NoTaskID_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	body := `{"tool_name":"help.get","arguments":{"topic":"tools"}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d body %s", code, respBody)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["content"] == nil {
		t.Errorf("expected content, got %v", out)
	}
	if _, has := out["task_id"]; has {
		t.Errorf("unexpected task_id in response: %v", out)
	}
}
func TestTruncateHelp_TruncatesLongString(t *testing.T) {
	s := strings.Repeat("z", helpMaxBytes+100)
	out := truncateHelp(s)
	if len(out) != helpMaxBytes {
		t.Fatalf("len(out)=%d want %d", len(out), helpMaxBytes)
	}
}

func TestHelpGetMarkdown_TopicAndPathBranches(t *testing.T) {
	if !strings.Contains(helpGetMarkdown("tools", ""), "MCP") {
		t.Errorf("tools snippet: %s", helpGetMarkdown("tools", ""))
	}
	if !strings.Contains(helpGetMarkdown("", "/docs/path"), "informational") {
		t.Errorf("path branch: %s", helpGetMarkdown("", "/docs/path"))
	}
	if !strings.Contains(helpGetMarkdown("unknown-topic", ""), "CyNodeAI MCP") {
		t.Errorf("unknown topic falls back to overview: %s", helpGetMarkdown("unknown-topic", ""))
	}
}
func TestHandleHelpList_Direct(t *testing.T) {
	ctx := context.Background()
	code, body, _ := handleHelpList(ctx, testutil.NewMockDB(), nil, &models.McpToolCallAuditLog{})
	if code != http.StatusOK {
		t.Fatalf("handleHelpList: %d", code)
	}
	if !bytes.Contains(body, []byte(`"topics"`)) {
		t.Errorf("expected topics in body: %s", body)
	}
}

func TestHandleHelpGet_DirectPaths(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	rec := &models.McpToolCallAuditLog{}
	code, body, _ := handleHelpGet(ctx, mock, map[string]interface{}{"topic": "tools"}, rec)
	if code != http.StatusOK {
		t.Fatalf("help.get no task_id: %d", code)
	}
	var noTask map[string]interface{}
	if err := json.Unmarshal(body, &noTask); err != nil {
		t.Fatal(err)
	}
	if _, has := noTask["task_id"]; has {
		t.Error("did not expect task_id when omitted")
	}
	code, body, _ = handleHelpGet(ctx, mock, map[string]interface{}{"topic": "tools"}, rec)
	if code != http.StatusOK {
		t.Fatalf("help.get topic: %d", code)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out["content"] == nil {
		t.Error("expected content")
	}
	code, body, _ = handleHelpGet(ctx, mock, map[string]interface{}{"path": "/docs"}, rec)
	if code != http.StatusOK {
		t.Fatalf("help.get path: %d", code)
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out["content"] == nil {
		t.Error("expected content for path")
	}
	code, body, _ = handleHelpGet(ctx, mock, map[string]interface{}{"topic": "not-a-known-topic"}, rec)
	if code != http.StatusOK {
		t.Fatalf("help.get unknown topic: %d", code)
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatal(err)
	}
	if out["content"] == nil {
		t.Error("expected default overview for unknown topic")
	}
}
func TestHelpGetMarkdown_TruncationAndUnknownTopic(t *testing.T) {
	if got := helpGetMarkdown("no-such-topic", ""); got == "" {
		t.Error("expected default overview")
	}
	if got := helpGetMarkdown("", "/docs"); got == "" {
		t.Error("expected path branch")
	}
	long := strings.Repeat("a", helpMaxBytes+10)
	if len(truncateHelp(long)) != helpMaxBytes {
		t.Errorf("truncate: %d", len(truncateHelp(long)))
	}
}
