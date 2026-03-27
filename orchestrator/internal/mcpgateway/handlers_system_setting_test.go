package mcpgateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestToolCallHandler_SystemSettingGet_NotFound(t *testing.T) {
	callToolHandlerPOST(
		t,
		`{"tool_name":"system_setting.get","arguments":{"key":"no.such.key"}}`,
		http.StatusNotFound,
	)
}

func TestToolCallHandler_SystemSettingList_Empty(t *testing.T) {
	mock := testutil.NewMockDB()
	body := `{"tool_name":"system_setting.list","arguments":{}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d %s", code, respBody)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["entries"] == nil {
		t.Error("expected entries")
	}
}

func TestToolCallHandler_SystemSettingGet_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	_, _ = callToolHandlerWithStoreAndBody(t, mock, `{"tool_name":"system_setting.create","arguments":{"key":"ss.get.ok","value":"v","value_type":"string"}}`)
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, `{"tool_name":"system_setting.get","arguments":{"key":"ss.get.ok"}}`)
	if code != http.StatusOK {
		t.Fatalf("get: %d %s", code, respBody)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatal(err)
	}
	if out["key"] != "ss.get.ok" {
		t.Fatalf("key: %v", out["key"])
	}
}

func TestToolCallHandler_SystemSettingList_keyPrefixAndCursor(t *testing.T) {
	mock := testutil.NewMockDB()
	for _, k := range []string{"pref.a", "pref.b", "other.x"} {
		_, _ = callToolHandlerWithStoreAndBody(t, mock, fmt.Sprintf(
			`{"tool_name":"system_setting.create","arguments":{"key":%q,"value":"1","value_type":"string"}}`, k))
	}
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, `{"tool_name":"system_setting.list","arguments":{"key_prefix":"pref.","limit":1}}`)
	if code != http.StatusOK {
		t.Fatalf("list: %d %s", code, respBody)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatal(err)
	}
	entries, _ := out["entries"].([]interface{})
	if len(entries) != 1 {
		t.Fatalf("entries: %v", out["entries"])
	}
	next, _ := out["next_cursor"].(string)
	if next == "" {
		t.Fatal("expected next_cursor")
	}
	code, respBody = callToolHandlerWithStoreAndBody(t, mock, fmt.Sprintf(
		`{"tool_name":"system_setting.list","arguments":{"key_prefix":"pref.","limit":1,"cursor":%q}}`, next))
	if code != http.StatusOK {
		t.Fatalf("page2: %d %s", code, respBody)
	}
}

func TestToolCallHandler_SystemSettingGet_MissingKey(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"system_setting.get","arguments":{}}`, http.StatusBadRequest)
}

func TestToolCallHandler_SystemSettingCreate_Update_Delete_RoundTrip(t *testing.T) {
	mock := testutil.NewMockDB()
	createBody := `{"tool_name":"system_setting.create","arguments":{"key":"mcp.test.key","value":"one","value_type":"string","reason":"r","updated_by":"u"}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, createBody)
	if code != http.StatusOK {
		t.Fatalf("create: %d %s", code, respBody)
	}
	upBody := `{"tool_name":"system_setting.update","arguments":{"key":"mcp.test.key","value":"two","value_type":"string","expected_version":1}}`
	code, respBody = callToolHandlerWithStoreAndBody(t, mock, upBody)
	if code != http.StatusOK {
		t.Fatalf("update: %d %s", code, respBody)
	}
	delBody := `{"tool_name":"system_setting.delete","arguments":{"key":"mcp.test.key","expected_version":2}}`
	code, respBody = callToolHandlerWithStoreAndBody(t, mock, delBody)
	if code != http.StatusOK {
		t.Fatalf("delete: %d %s", code, respBody)
	}
}

func TestToolCallHandler_SystemSettingCreate_Conflict(t *testing.T) {
	mock := testutil.NewMockDB()
	body := `{"tool_name":"system_setting.create","arguments":{"key":"dup.key","value":"a","value_type":"string"}}`
	code, _ := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("first create: %d", code)
	}
	code, _ = callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusConflict {
		t.Fatalf("second create: got %d want 409", code)
	}
}

func TestToolCallHandler_SystemSettingCreate_InvalidArgs(t *testing.T) {
	mock := testutil.NewMockDB()
	body := `{"tool_name":"system_setting.create","arguments":{"key":"only-key"}}`
	code, _ := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusBadRequest {
		t.Fatalf("got %d", code)
	}
}

func TestToolCallHandler_SystemSettingDelete_MissingKey(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"system_setting.delete","arguments":{}}`, http.StatusBadRequest)
}
func TestHandleSystemSettingList_StoreError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("db")
	rec := &models.McpToolCallAuditLog{}
	code, _, _ := handleSystemSettingList(context.Background(), mock, map[string]interface{}{}, rec)
	if code != http.StatusInternalServerError {
		t.Fatalf("got %d", code)
	}
}

func TestHandleSystemSettingCreateUpdate_StoreError(t *testing.T) {
	args := map[string]interface{}{"key": "k", "value": "v", "value_type": "string"}
	cases := []mcpHandlerToolCase{
		{"create", func(m *testutil.MockDB) { m.ForceError = errors.New("db") }, handleSystemSettingCreate},
		{"update", func(m *testutil.MockDB) { m.UpdateSystemSettingErr = errors.New("db") }, handleSystemSettingUpdate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := testutil.NewMockDB()
			tc.prep(mock)
			rec := &models.McpToolCallAuditLog{}
			code, _, _ := tc.call(context.Background(), mock, args, rec)
			if code != http.StatusInternalServerError {
				t.Fatalf("got %d", code)
			}
		})
	}
}

func TestHandleSystemSettingDelete_StoreError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.DeleteSystemSettingErr = errors.New("db")
	rec := &models.McpToolCallAuditLog{}
	code, _, _ := handleSystemSettingDelete(context.Background(), mock, map[string]interface{}{"key": "k"}, rec)
	if code != http.StatusInternalServerError {
		t.Fatalf("got %d", code)
	}
}
