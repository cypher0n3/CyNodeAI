package mcpgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestToolCallHandler_PreferenceGet_NotFound(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.get","arguments":{"scope_type":"system","key":"missing"}}`, http.StatusNotFound)
}

func TestToolCallHandler_PreferenceGet_BadArgs(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.get"}`, http.StatusBadRequest)
	callToolHandlerPOST(t, `{"tool_name":"preference.get","arguments":{"scope_type":"system"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceGet_ScopeIDRequired(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.get","arguments":{"scope_type":"user","key":"k"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceList_ScopeIDRequired(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.list","arguments":{"scope_type":"user"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceList_LimitZero(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.list","arguments":{"scope_type":"system","limit":0}}`, http.StatusOK)
}

func TestToolCallHandler_PreferenceList_Empty(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.list","arguments":{"scope_type":"system"}}`, http.StatusOK)
}

func TestToolCallHandler_PreferenceList_WithLimitAndCursor(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.list","arguments":{"scope_type":"system","limit":5,"cursor":"0"}}`, http.StatusOK)
}

func TestToolCallHandler_PreferenceEffective_BadArgs(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.effective"}`, http.StatusBadRequest)
}

func TestToolCallHandler_DenyAuditWriteFails(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ForceError = errors.New("audit write failed")
	callToolHandlerWithStore(t, mock, `{"tool_name":"preference.effective"}`, http.StatusInternalServerError)
}

func TestToolCallHandler_PreferenceList_UserScope(t *testing.T) {
	mock := testutil.NewMockDB()
	uid := uuid.New()
	val := testPreferenceVal
	mock.PreferenceEntries = append(mock.PreferenceEntries, &models.PreferenceEntry{
		PreferenceEntryBase: models.PreferenceEntryBase{
			ScopeType: "user",
			ScopeID:   &uid,
			Key:       "k",
			Value:     &val,
			ValueType: "string",
			Version:   1,
		},
		ID:        uuid.New(),
		UpdatedAt: time.Now().UTC(),
	})
	handler := ToolCallHandler(mock, slog.Default(), nil)
	body := `{"tool_name":"preference.list","arguments":{"scope_type":"user","scope_id":"` + uid.String() + `"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/mcp/tools/call", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", rec.Code)
	}
}

func TestToolCallHandler_PreferenceGet_Found(t *testing.T) {
	mock := testutil.NewMockDB()
	v := `"val"`
	mock.PreferenceEntries = append(mock.PreferenceEntries, &models.PreferenceEntry{
		PreferenceEntryBase: models.PreferenceEntryBase{
			ScopeType: "system",
			ScopeID:   nil,
			Key:       "a.key",
			Value:     &v,
			ValueType: "string",
			Version:   1,
		},
		ID:        uuid.New(),
		UpdatedAt: time.Now().UTC(),
	})
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, `{"tool_name":"preference.get","arguments":{"scope_type":"system","key":"a.key"}}`)
	if code != http.StatusOK {
		t.Fatalf("got status %d", code)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["key"] != "a.key" || out["value_type"] != "string" {
		t.Errorf("expected key a.key and value_type string, got %v", out)
	}
}

func TestToolCallHandler_PreferenceList_ScopeTypeRequired(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.list","arguments":{}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceEffective_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(context.Background(), nil, "p", nil)
	val := testPreferenceVal
	mock.PreferenceEntries = append(mock.PreferenceEntries, &models.PreferenceEntry{
		PreferenceEntryBase: models.PreferenceEntryBase{
			ScopeType: "system",
			Key:       "x",
			Value:     &val,
			ValueType: "string",
			Version:   1,
		},
		ID:        uuid.New(),
		UpdatedAt: time.Now().UTC(),
	})
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, `{"tool_name":"preference.effective","arguments":{"task_id":"`+task.ID.String()+`"}}`)
	if code != http.StatusOK {
		t.Fatalf("got status %d", code)
	}
	var out struct {
		Effective map[string]interface{} `json:"effective"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Effective["x"] == nil {
		t.Errorf("effective should contain x, got %v", out.Effective)
	}
}

func TestToolCallHandler_PreferenceGet_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.GetPreferenceErr = errors.New("db error")
	callToolHandlerWithStore(t, mock, `{"tool_name":"preference.get","arguments":{"scope_type":"system","key":"k"}}`, http.StatusInternalServerError)
}

func TestToolCallHandler_PreferenceList_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.ListPreferencesErr = errors.New("db error")
	callToolHandlerWithStore(t, mock, `{"tool_name":"preference.list","arguments":{"scope_type":"system"}}`, http.StatusInternalServerError)
}

func TestToolCallHandler_PreferenceEffective_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(context.Background(), nil, "p", nil)
	mock.GetEffectivePreferencesForTaskErr = errors.New("db error")
	body := `{"tool_name":"preference.effective","arguments":{"task_id":"` + task.ID.String() + `"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusInternalServerError)
}

func TestToolCallHandler_PreferenceCreate_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	body := `{"tool_name":"preference.create","arguments":{"scope_type":"system","key":"new.key","value":"\"v\"","value_type":"string"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusOK)
	if len(mock.PreferenceEntries) != 1 || mock.PreferenceEntries[0].Key != "new.key" {
		t.Errorf("expected one preference new.key, got %d entries", len(mock.PreferenceEntries))
	}
}

func TestToolCallHandler_PreferenceCreate_Conflict(t *testing.T) {
	mock := mockWithSystemPreference(t, "exists")
	body := `{"tool_name":"preference.create","arguments":{"scope_type":"system","key":"exists","value":"\"x\"","value_type":"string"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusConflict)
}

func TestToolCallHandler_PreferenceCreate_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.CreatePreferenceErr = errors.New("db error")
	body := `{"tool_name":"preference.create","arguments":{"scope_type":"system","key":"k","value":"\"v\"","value_type":"string"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusInternalServerError)
}

func TestToolCallHandler_PreferenceList_WithEntriesAndKeyPrefix(t *testing.T) {
	mock := mockWithSystemPreference(t, "pref.a.key")
	code, body := callToolHandlerWithStoreAndBody(t, mock, `{"tool_name":"preference.list","arguments":{"scope_type":"system","key_prefix":"pref.","limit":5}}`)
	if code != http.StatusOK {
		t.Fatalf("got status %d", code)
	}
	var out struct {
		Entries    []map[string]interface{} `json:"entries"`
		NextCursor string                   `json:"next_cursor"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Entries) != 1 || out.Entries[0]["key"] != "pref.a.key" {
		t.Errorf("expected one entry pref.a.key, got %+v", out.Entries)
	}
}

func TestToolCallHandler_PreferenceCreate_BadArgs(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.create","arguments":{"scope_type":"system","key":"k"}}`, http.StatusBadRequest)
	callToolHandlerPOST(t, `{"tool_name":"preference.create","arguments":{"scope_type":"user","key":"k","value":"\"v\"","value_type":"string"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceUpdate_Success(t *testing.T) {
	mock := mockWithSystemPreference(t, "up.key")
	body := `{"tool_name":"preference.update","arguments":{"scope_type":"system","key":"up.key","value":"\"new\"","value_type":"string"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusOK)
	if len(mock.PreferenceEntries) != 1 || *mock.PreferenceEntries[0].Value != `"new"` || mock.PreferenceEntries[0].Version != 2 {
		t.Errorf("expected updated value and version 2, got %+v", mock.PreferenceEntries[0])
	}
}

func TestToolCallHandler_PreferenceUpdate_SuccessWithExpectedVersion(t *testing.T) {
	mock := mockWithSystemPreference(t, "ver.key")
	body := `{"tool_name":"preference.update","arguments":{"scope_type":"system","key":"ver.key","value":"\"v2\"","value_type":"string","expected_version":1}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d", code)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["version"] != float64(2) {
		t.Errorf("expected version 2, got %v", out["version"])
	}
}

func TestToolCallHandler_PreferenceDelete_WithExpectedVersion(t *testing.T) {
	mock := mockWithSystemPreference(t, "delver.key")
	mock.PreferenceEntries[0].Version = 2
	body := `{"tool_name":"preference.delete","arguments":{"scope_type":"system","key":"delver.key","expected_version":2}}`
	callToolHandlerWithStore(t, mock, body, http.StatusOK)
	if len(mock.PreferenceEntries) != 0 {
		t.Errorf("expected preference deleted, got %d entries", len(mock.PreferenceEntries))
	}
}

func TestToolCallHandler_PreferenceUpdate_WithReasonAndUpdatedBy(t *testing.T) {
	mock := mockWithSystemPreference(t, "reason.key")
	body := `{"tool_name":"preference.update","arguments":{"scope_type":"system","key":"reason.key","value":"\"updated\"","value_type":"string","reason":"test","updated_by":"bdd"}}`
	code, _ := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Errorf("got status %d, want 200", code)
	}
}

func TestToolCallHandler_PreferenceUpdate_Conflict(t *testing.T) {
	mock := mockWithSystemPreference(t, "ver.key")
	body := `{"tool_name":"preference.update","arguments":{"scope_type":"system","key":"ver.key","value":"\"x\"","value_type":"string","expected_version":2}}`
	callToolHandlerWithStore(t, mock, body, http.StatusConflict)
}

// TestHandlePreferenceUpdate_ExpectedVersionInt hits the int branch of expected_version (JSON unmarshals numbers as float64).
func TestHandlePreferenceUpdate_ExpectedVersionInt(t *testing.T) {
	mock := mockWithSystemPreference(t, "intver.key")
	ctx := context.Background()
	args := map[string]interface{}{
		"scope_type": "system", "key": "intver.key", "value": `"v2"`, "value_type": "string",
		"expected_version": 1, // int type
	}
	rec := &models.McpToolCallAuditLog{}
	code, body, _ := handlePreferenceUpdate(ctx, mock, args, rec)
	if code != http.StatusOK {
		t.Fatalf("got status %d %s", code, body)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["version"] != float64(2) {
		t.Errorf("version = %v", out["version"])
	}
}

// TestHandlePreferenceDelete_ExpectedVersionInt hits the int branch of expected_version.
func TestHandlePreferenceDelete_ExpectedVersionInt(t *testing.T) {
	mock := mockWithSystemPreference(t, "intdel.key")
	ctx := context.Background()
	args := map[string]interface{}{"scope_type": "system", "key": "intdel.key", "expected_version": 1}
	rec := &models.McpToolCallAuditLog{}
	code, _, _ := handlePreferenceDelete(ctx, mock, args, rec)
	if code != http.StatusOK {
		t.Fatalf("got status %d", code)
	}
	if len(mock.PreferenceEntries) != 0 {
		t.Error("expected preference deleted")
	}
}

func TestToolCallHandler_PreferenceDelete_Success(t *testing.T) {
	mock := mockWithSystemPreference(t, "del.key")
	body := `{"tool_name":"preference.delete","arguments":{"scope_type":"system","key":"del.key"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusOK)
	if len(mock.PreferenceEntries) != 0 {
		t.Errorf("expected preference deleted, got %d entries", len(mock.PreferenceEntries))
	}
}

func TestToolCallHandler_PreferenceDelete_WithReason(t *testing.T) {
	mock := mockWithSystemPreference(t, "reason.del")
	body := `{"tool_name":"preference.delete","arguments":{"scope_type":"system","key":"reason.del","reason":"cleanup"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusOK)
	if len(mock.PreferenceEntries) != 0 {
		t.Errorf("expected preference deleted, got %d entries", len(mock.PreferenceEntries))
	}
}

func TestToolCallHandler_PreferenceDelete_NotFound(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.delete","arguments":{"scope_type":"system","key":"nonexistent"}}`, http.StatusNotFound)
}
func TestToolCallHandler_PreferenceEffective_EmptyEffective(t *testing.T) {
	mock := testutil.NewMockDB()
	task, _ := mock.CreateTask(context.Background(), nil, "p", nil)
	body := `{"tool_name":"preference.effective","arguments":{"task_id":"` + task.ID.String() + `"}}`
	code, respBody := callToolHandlerWithStoreAndBody(t, mock, body)
	if code != http.StatusOK {
		t.Fatalf("got status %d %s", code, respBody)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(respBody, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["effective"] == nil {
		t.Error("expected effective key")
	}
}

func TestToolCallHandler_PreferenceUpdate_UserScopeNoScopeID(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.update","arguments":{"scope_type":"user","key":"k","value":"\"v\"","value_type":"string"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceUpdate_BadArgs(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.update","arguments":{"scope_type":"system","key":"k"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceDelete_ScopeTypeKeyRequired(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.delete","arguments":{}}`, http.StatusBadRequest)
	callToolHandlerPOST(t, `{"tool_name":"preference.delete","arguments":{"scope_type":"system"}}`, http.StatusBadRequest)
	callToolHandlerPOST(t, `{"tool_name":"preference.delete","arguments":{"key":"k"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceDelete_UserScopeNoScopeID(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.delete","arguments":{"scope_type":"user","key":"k"}}`, http.StatusBadRequest)
}

func TestToolCallHandler_PreferenceDelete_ExpectedVersionFloat(t *testing.T) {
	mock := mockWithSystemPreference(t, "verfloat")
	body := `{"tool_name":"preference.delete","arguments":{"scope_type":"system","key":"verfloat","expected_version":1.0}}`
	callToolHandlerWithStore(t, mock, body, http.StatusOK)
	if len(mock.PreferenceEntries) != 0 {
		t.Errorf("expected deleted, got %d entries", len(mock.PreferenceEntries))
	}
}
func TestToolCallHandler_PreferenceUpdate_NotFound(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"preference.update","arguments":{"scope_type":"system","key":"nonexistent","value":"\"v\"","value_type":"string"}}`, http.StatusNotFound)
}

func TestToolCallHandler_PreferenceUpdate_InternalError(t *testing.T) {
	mock := testutil.NewMockDB()
	mock.UpdatePreferenceErr = errors.New("db error")
	body := `{"tool_name":"preference.update","arguments":{"scope_type":"system","key":"k","value":"\"v\"","value_type":"string"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusInternalServerError)
}

func TestToolCallHandler_PreferenceDelete_Conflict(t *testing.T) {
	mock := mockWithSystemPreference(t, "ver.del")
	body := `{"tool_name":"preference.delete","arguments":{"scope_type":"system","key":"ver.del","expected_version":2}}`
	callToolHandlerWithStore(t, mock, body, http.StatusConflict)
}

func TestToolCallHandler_PreferenceDelete_InternalError(t *testing.T) {
	mock := mockWithSystemPreference(t, "del.err")
	mock.DeletePreferenceErr = errors.New("db error")
	body := `{"tool_name":"preference.delete","arguments":{"scope_type":"system","key":"del.err"}}`
	callToolHandlerWithStore(t, mock, body, http.StatusInternalServerError)
}
func TestHandlePreferenceEffective_StoreError(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	mock.GetEffectivePreferencesForTaskErr = errors.New("injected store error")
	rec := &models.McpToolCallAuditLog{}
	tid := uuid.New()
	code, body, _ := handlePreferenceEffective(ctx, mock, map[string]interface{}{"task_id": tid.String()}, rec)
	if code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", code, body)
	}
}

func TestHandlePreferenceEffective_DirectMissingTaskID(t *testing.T) {
	ctx := context.Background()
	mock := testutil.NewMockDB()
	rec := &models.McpToolCallAuditLog{}
	code, body, _ := handlePreferenceEffective(ctx, mock, map[string]interface{}{}, rec)
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", code, body)
	}
	if !bytes.Contains(body, []byte("task_id")) {
		t.Errorf("body: %s", body)
	}
}
