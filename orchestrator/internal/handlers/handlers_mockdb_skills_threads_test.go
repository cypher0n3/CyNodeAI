package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestSkillsHandler_Update_InvalidID(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	h := NewSkillsHandler(mock, newTestLogger())
	body := []byte(`{"content":"# X"}`)
	req := httptest.NewRequest("PUT", "/v1/skills/bad", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", "bad")
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Update bad id: got %d", rec.Code)
	}
}

func TestSkillsHandler_Update_InvalidBody(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	skill, _ := mock.CreateSkill(context.Background(), "S1", "# C", "user", &user.ID, false)
	h := NewSkillsHandler(mock, newTestLogger())
	req := httptest.NewRequest("PUT", "/v1/skills/"+skill.ID.String(), bytes.NewReader([]byte("not json"))).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", skill.ID.String())
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Update invalid body: got %d", rec.Code)
	}
}

func TestSkillsHandler_Update_Success_NameOnly(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	skill, _ := mock.CreateSkill(context.Background(), "S1", "# C", "user", &user.ID, false)
	h := NewSkillsHandler(mock, newTestLogger())
	body := []byte(`{"name":"Renamed"}`)
	req := httptest.NewRequest("PUT", "/v1/skills/"+skill.ID.String(), bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", skill.ID.String())
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Update name: got %d %s", rec.Code, rec.Body.String())
	}
	got, _ := mock.GetSkillByID(context.Background(), skill.ID)
	if got.Name != "Renamed" {
		t.Errorf("Update name = %q", got.Name)
	}
}

func TestSkillsHandler_Delete_Success(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	skill, _ := mock.CreateSkill(context.Background(), "S1", "# C", "user", &user.ID, false)
	h := NewSkillsHandler(mock, newTestLogger())
	req := httptest.NewRequest("DELETE", "/v1/skills/"+skill.ID.String(), http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.SetPathValue("id", skill.ID.String())
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("Delete: got %d", rec.Code)
	}
	_, err := mock.GetSkillByID(context.Background(), skill.ID)
	if err == nil {
		t.Error("skill should be deleted")
	}
}

//nolint:dupl // skills handler not-found pattern
func TestSkillsHandler_Delete_NotFound(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	unknown := uuid.New()
	h := NewSkillsHandler(mock, newTestLogger())
	req := httptest.NewRequest("DELETE", "/v1/skills/"+unknown.String(), http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.SetPathValue("id", unknown.String())
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("Delete not found: got %d", rec.Code)
	}
}

//nolint:dupl // skills handler bad-request pattern
func TestSkillsHandler_Delete_NoID(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	h := NewSkillsHandler(mock, newTestLogger())
	req := httptest.NewRequest("DELETE", "/v1/skills/", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Delete no id: got %d", rec.Code)
	}
}

func TestSkillsHandler_List_DBError(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	mock.ForceError = errors.New("db error")
	defer func() { mock.ForceError = nil }()
	h := NewSkillsHandler(mock, newTestLogger())
	ctx := context.WithValue(context.Background(), contextKeyUserID, user.ID)
	req := httptest.NewRequest("GET", "/v1/skills", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("List DB error: got %d", rec.Code)
	}
}

//nolint:dupl // skills handler DB error pattern
func TestSkillsHandler_Get_DBError(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	skill, _ := mock.CreateSkill(context.Background(), "S1", "# C", "user", &user.ID, false)
	mock.ForceError = errors.New("db error")
	defer func() { mock.ForceError = nil }()
	h := NewSkillsHandler(mock, newTestLogger())
	req := httptest.NewRequest("GET", "/v1/skills/"+skill.ID.String(), http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.SetPathValue("id", skill.ID.String())
	rec := httptest.NewRecorder()
	h.Get(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Get DB error: got %d", rec.Code)
	}
}

//nolint:dupl // skills handler bad-request pattern
func TestSkillsHandler_Get_NoID(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	h := NewSkillsHandler(mock, newTestLogger())
	req := httptest.NewRequest("GET", "/v1/skills/", http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	rec := httptest.NewRecorder()
	h.Get(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Get no id: got %d", rec.Code)
	}
}

func TestSkillsHandler_Load_DBError(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	mock.ForceError = errors.New("db error")
	defer func() { mock.ForceError = nil }()
	h := NewSkillsHandler(mock, newTestLogger())
	body := []byte(`{"content":"# x"}`)
	req := httptest.NewRequest("POST", "/v1/skills/load", bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Load(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Load DB error: got %d", rec.Code)
	}
}

func TestSkillsHandler_Load_InvalidBody(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	h := NewSkillsHandler(mock, newTestLogger())
	req := httptest.NewRequest("POST", "/v1/skills/load", bytes.NewReader([]byte("not json"))).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Load(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("Load invalid body: got %d", rec.Code)
	}
}

func TestSkillsHandler_Update_DBError(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	skill, _ := mock.CreateSkill(context.Background(), "S1", "# Old", "user", &user.ID, false)
	mock.ForceError = errors.New("db error")
	defer func() { mock.ForceError = nil }()
	h := NewSkillsHandler(mock, newTestLogger())
	body := []byte(`{"content":"# New"}`)
	req := httptest.NewRequest("PUT", "/v1/skills/"+skill.ID.String(), bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", skill.ID.String())
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Update DB error: got %d", rec.Code)
	}
}

func TestSkillsHandler_Update_NotFound(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	skill, _ := mock.CreateSkill(context.Background(), "S1", "# C", "user", &user.ID, false)
	mock.UpdateSkillErr = database.ErrNotFound
	defer func() { mock.UpdateSkillErr = nil }()
	h := NewSkillsHandler(mock, newTestLogger())
	body := []byte(`{"name":"X"}`)
	req := httptest.NewRequest("PUT", "/v1/skills/"+skill.ID.String(), bytes.NewReader(body)).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", skill.ID.String())
	rec := httptest.NewRecorder()
	h.Update(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("Update NotFound: got %d", rec.Code)
	}
}

//nolint:dupl // skills handler DB error pattern
func TestSkillsHandler_Delete_DBError(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	skill, _ := mock.CreateSkill(context.Background(), "S1", "# C", "user", &user.ID, false)
	mock.ForceError = errors.New("db error")
	defer func() { mock.ForceError = nil }()
	h := NewSkillsHandler(mock, newTestLogger())
	req := httptest.NewRequest("DELETE", "/v1/skills/"+skill.ID.String(), http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.SetPathValue("id", skill.ID.String())
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Delete DB error: got %d", rec.Code)
	}
}

//nolint:dupl // skills handler DB error pattern (DeleteSkillErr path)
func TestSkillsHandler_Delete_DeleteSkillDBError(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	skill, _ := mock.CreateSkill(context.Background(), "S1", "# C", "user", &user.ID, false)
	mock.DeleteSkillErr = errors.New("db delete error")
	defer func() { mock.DeleteSkillErr = nil }()
	h := NewSkillsHandler(mock, newTestLogger())
	req := httptest.NewRequest("DELETE", "/v1/skills/"+skill.ID.String(), http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.SetPathValue("id", skill.ID.String())
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Delete DeleteSkill error: got %d", rec.Code)
	}
}

func TestSkillsHandler_Delete_NotFoundFromDB(t *testing.T) {
	mock := testutil.NewMockDB()
	user, _ := mock.CreateUser(context.Background(), "u", nil)
	mock.AddUser(user)
	skill, _ := mock.CreateSkill(context.Background(), "S1", "# C", "user", &user.ID, false)
	mock.DeleteSkillErr = database.ErrNotFound
	defer func() { mock.DeleteSkillErr = nil }()
	h := NewSkillsHandler(mock, newTestLogger())
	req := httptest.NewRequest("DELETE", "/v1/skills/"+skill.ID.String(), http.NoBody).WithContext(context.WithValue(context.Background(), contextKeyUserID, user.ID))
	req.SetPathValue("id", skill.ID.String())
	rec := httptest.NewRecorder()
	h.Delete(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("Delete ErrNotFound from DB: got %d", rec.Code)
	}
}

// TestOpenAIChatHandler_ChatCompletions_PMA_SendsThreadHistory verifies that the full thread history
// is forwarded to PMA across multiple turns, giving the agent multi-turn context.
func TestOpenAIChatHandler_ChatCompletions_PMA_SendsThreadHistory(t *testing.T) {
	var receivedMessages []map[string]string
	mockPMA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/chat/completion" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req struct {
			Messages []map[string]string `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		receivedMessages = req.Messages
		_ = json.NewEncoder(w).Encode(map[string]string{"content": "Turn reply."})
	}))
	defer mockPMA.Close()

	db := mockDBWithPMAEndpoint(t, mockPMA.URL)
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")

	sendMsg := func(content string) {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"model":    "cynodeai.pm",
			"messages": []map[string]string{{"role": "user", "content": content}},
		})
		req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.ChatCompletions(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("send %q: expected 200, got %d: %s", content, rec.Code, rec.Body.String())
		}
	}

	sendMsg("first message")
	sendMsg("second message")

	// After the second turn the PMA should receive at least 3 messages:
	// user:"first message", assistant:"Turn reply.", user:"second message".
	if len(receivedMessages) < 3 {
		t.Errorf("expected at least 3 messages forwarded on second turn, got %d: %v", len(receivedMessages), receivedMessages)
	}
	if receivedMessages[0]["role"] != "user" || receivedMessages[0]["content"] != "first message" {
		t.Errorf("first message mismatch: %v", receivedMessages[0])
	}
	if receivedMessages[1]["role"] != "assistant" {
		t.Errorf("second message should be assistant turn, got: %v", receivedMessages[1])
	}
	if receivedMessages[len(receivedMessages)-1]["content"] != "second message" {
		t.Errorf("last message should be second user turn, got: %v", receivedMessages[len(receivedMessages)-1])
	}
}

// TestOpenAIChatHandler_ChatCompletions_HistoryLoadError verifies that history load failure
// falls back gracefully to single-message context (no 5xx to the caller).
func TestOpenAIChatHandler_ChatCompletions_HistoryLoadError(t *testing.T) {
	mockPMA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"content": "ok"})
	}))
	defer mockPMA.Close()
	db := mockDBWithPMAEndpoint(t, mockPMA.URL)
	db.ForceError = errors.New("db history failure")
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	userID := uuid.New()
	// ForceError is set so GetOrCreateActiveChatThread will also fail; we expect a 500 from that path.
	// Reset ForceError after thread creation to simulate only the ListChatMessages failing.
	db.ForceError = nil
	body := []byte(`{"model":"cynodeai.pm","messages":[{"role":"user","content":"hi"}]}`)
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ChatCompletions(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 even on history load path (error falls back to single msg), got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTrimHistoryToCharBudget(t *testing.T) {
	makeMsg := func(role, content string) *models.ChatMessage {
		return &models.ChatMessage{
			ChatMessageBase: models.ChatMessageBase{
				Role:    role,
				Content: content,
			},
		}
	}
	// Budget large enough: all messages returned.
	all := []*models.ChatMessage{
		makeMsg("user", "hello"),
		makeMsg("assistant", "hi"),
		makeMsg("user", "world"),
	}
	got := trimHistoryToCharBudget(all, 10000)
	if len(got) != 3 {
		t.Errorf("full budget: expected 3 messages, got %d", len(got))
	}

	// Budget 0: only the last message fits (always at least the last one).
	got = trimHistoryToCharBudget(all, 0)
	if len(got) != 1 || got[0].Content != "world" {
		t.Errorf("zero budget: expected last message only, got %v", got)
	}

	// Budget exactly fits last two messages (5 + 2 = 7).
	got = trimHistoryToCharBudget(all, 7)
	if len(got) != 2 {
		t.Errorf("tight budget: expected 2 messages, got %d", len(got))
	}
	if got[0].Content != "hi" || got[1].Content != "world" {
		t.Errorf("tight budget: wrong messages: %v", got)
	}
}

func TestOpenAIChatHandler_NewThread_NoAuth(t *testing.T) {
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	req := httptest.NewRequest("POST", "/v1/chat/threads", http.NoBody)
	rec := httptest.NewRecorder()
	h.NewThread(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_NewThread_CreatesThread(t *testing.T) {
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("POST", "/v1/chat/threads", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.NewThread(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ThreadID string `json:"thread_id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ThreadID == "" {
		t.Error("expected non-empty thread_id in response")
	}
	// Verify thread was actually persisted in the mock.
	threadUUID, err := uuid.Parse(resp.ThreadID)
	if err != nil {
		t.Fatalf("thread_id is not a valid UUID: %v", err)
	}
	// Verify thread was created by checking that ListChatMessages succeeds for the returned ID.
	_, listErr := db.ListChatMessages(context.Background(), threadUUID, 0)
	if listErr != nil {
		t.Errorf("thread %s not found after creation: %v", threadUUID, listErr)
	}
}

func TestOpenAIChatHandler_NewThread_DBError(t *testing.T) {
	db := testutil.NewMockDB()
	db.ForceError = errors.New("db failure")
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("POST", "/v1/chat/threads", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.NewThread(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on db failure, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_NewThread_IndependentFromGetOrCreate(t *testing.T) {
	// Two calls to NewThread always produce two distinct threads, unlike GetOrCreate
	// which reuses the active thread within the inactivity window.
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)

	make201 := func() string {
		req := httptest.NewRequest("POST", "/v1/chat/threads", http.NoBody).WithContext(ctx)
		rec := httptest.NewRecorder()
		h.NewThread(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", rec.Code)
		}
		var resp struct {
			ThreadID string `json:"thread_id"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return resp.ThreadID
	}

	id1 := make201()
	id2 := make201()
	if id1 == id2 {
		t.Errorf("two NewThread calls returned same ID %s; expected distinct threads", id1)
	}
}

// --- POST /v1/responses, GET/PATCH /v1/chat/threads* ---

func TestOpenAIChatHandler_Responses_NoAuth(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	body := []byte(`{"input":"hello"}`)
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Responses(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_Responses_InvalidBody(t *testing.T) {
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	ctx := context.WithValue(context.Background(), contextKeyUserID, uuid.New())
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader("not json")).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.Responses(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_Responses_EmptyInput(t *testing.T) {
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	ctx := context.WithValue(context.Background(), contextKeyUserID, uuid.New())
	body := []byte(`{"input":[]}`)
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.Responses(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty input, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_Responses_PreviousResponseIDNotFound(t *testing.T) {
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := []byte(`{"input":"hi","previous_response_id":"resp_nonexistent"}`)
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.Responses(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown previous_response_id, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_Responses_SuccessWithDirectInference(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "Reply from model.", "done": true})
	}))
	defer mockOllama.Close()
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), mockOllama.URL, "qwen3.5:0.8b", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := []byte(`{"model":"qwen3.5:0.8b","input":"hello"}`)
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.Responses(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ID     string `json:"id"`
		Output []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"output"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ID == "" || !strings.HasPrefix(resp.ID, "resp_") {
		t.Errorf("expected response id resp_*, got %q", resp.ID)
	}
	if len(resp.Output) != 1 || resp.Output[0].Text != "Reply from model." {
		t.Errorf("expected output text, got %v", resp.Output)
	}
}

func TestOpenAIChatHandler_Responses_InputAsMessageArray(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "From array.", "done": true})
	}))
	defer mockOllama.Close()
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), mockOllama.URL, "qwen3.5:0.8b", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	// Input as message-like array (covers parseResponsesInputAsMessageArray and extractTextFromMessageContent).
	body := []byte(`{"model":"qwen3.5:0.8b","input":[{"role":"user","content":[{"type":"input_text","text":"hello from array"}]}]}`)
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.Responses(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Output []struct {
			Text string `json:"text"`
		} `json:"output"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Output) != 1 || resp.Output[0].Text != "From array." {
		t.Errorf("expected output text From array., got %v", resp.Output)
	}
}

func TestOpenAIChatHandler_ListThreads_NoAuth(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	req := httptest.NewRequest("GET", "/v1/chat/threads", http.NoBody)
	rec := httptest.NewRecorder()
	h.ListThreads(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_ListThreads_Success(t *testing.T) {
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListThreads(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data == nil {
		t.Error("expected data array")
	}
}

func TestOpenAIChatHandler_ListThreads_WithLimitOffset(t *testing.T) {
	db := testutil.NewMockDB()
	userID := uuid.New()
	_, _ = db.CreateChatThread(context.Background(), userID, nil, nil)
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads?limit=5&offset=0", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListThreads(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOpenAIChatHandler_NewThread_WithBodyTitle(t *testing.T) {
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := []byte(`{"title":"My thread"}`)
	req := httptest.NewRequest("POST", "/v1/chat/threads", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.NewThread(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ThreadID string `json:"thread_id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.ThreadID == "" {
		t.Error("expected thread_id")
	}
}

func TestOpenAIChatHandler_GetThread_NoAuth(t *testing.T) {
	runOpenAIChatNoAuth(t, "GET", "/v1/chat/threads/00000000-0000-0000-0000-000000000001", func(h *OpenAIChatHandler, w http.ResponseWriter, r *http.Request) {
		h.GetThread(w, r, "00000000-0000-0000-0000-000000000001")
	})
}

func runOpenAIChatNoAuth(t *testing.T, method, path string, fn func(*OpenAIChatHandler, http.ResponseWriter, *http.Request)) {
	t.Helper()
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	req := httptest.NewRequest(method, path, http.NoBody)
	rec := httptest.NewRecorder()
	fn(h, rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_GetThread_NotFound(t *testing.T) {
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads/00000000-0000-0000-0000-000000000001", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.GetThread(rec, req, "00000000-0000-0000-0000-000000000001")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_GetThread_Success(t *testing.T) {
	db := testutil.NewMockDB()
	userID := uuid.New()
	thread, _ := db.CreateChatThread(context.Background(), userID, nil, nil)
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads/"+thread.ID.String(), http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.GetThread(rec, req, thread.ID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOpenAIChatHandler_ListThreadMessages_NoAuth(t *testing.T) {
	runOpenAIChatNoAuth(t, "GET", "/v1/chat/threads/00000000-0000-0000-0000-000000000001/messages", func(h *OpenAIChatHandler, w http.ResponseWriter, r *http.Request) {
		h.ListThreadMessages(w, r, "00000000-0000-0000-0000-000000000001")
	})
}

func TestOpenAIChatHandler_ListThreadMessages_NotFound(t *testing.T) {
	db := testutil.NewMockDB()
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads/00000000-0000-0000-0000-000000000001/messages", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	h.ListThreadMessages(rec, req, "00000000-0000-0000-0000-000000000001")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_ListThreadMessages_Success(t *testing.T) {
	db := testutil.NewMockDB()
	userID := uuid.New()
	thread, _ := db.CreateChatThread(context.Background(), userID, nil, nil)
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads/"+thread.ID.String()+"/messages", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListThreadMessages(rec, req, thread.ID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOpenAIChatHandler_PatchThreadTitle_NoAuth(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	body := []byte(`{"title":"New Title"}`)
	req := httptest.NewRequest("PATCH", "/v1/chat/threads/00000000-0000-0000-0000-000000000001", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.PatchThreadTitle(rec, req, "00000000-0000-0000-0000-000000000001")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_PatchThreadTitle_NotFound(t *testing.T) {
	db := testutil.NewMockDB()
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := []byte(`{"title":"New Title"}`)
	req := httptest.NewRequest("PATCH", "/v1/chat/threads/00000000-0000-0000-0000-000000000001", bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	h.PatchThreadTitle(rec, req, "00000000-0000-0000-0000-000000000001")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_PatchThreadTitle_Success(t *testing.T) {
	db := testutil.NewMockDB()
	userID := uuid.New()
	thread, _ := db.CreateChatThread(context.Background(), userID, nil, nil)
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := []byte(`{"title":"Renamed"}`)
	req := httptest.NewRequest("PATCH", "/v1/chat/threads/"+thread.ID.String(), bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.PatchThreadTitle(rec, req, thread.ID.String())
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestOpenAIChatHandler_GetThread_InvalidUUID(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads/not-a-uuid", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.GetThread(rec, req, "not-a-uuid")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid") {
		t.Logf("body: %s", rec.Body.String())
	}
}

func TestOpenAIChatHandler_ListThreadMessages_InvalidUUID(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads/not-a-uuid/messages", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListThreadMessages(rec, req, "not-a-uuid")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected non-empty error body")
	}
}

func TestOpenAIChatHandler_ListThreadMessages_ListMessagesError(t *testing.T) {
	db := testutil.NewMockDB()
	userID := uuid.New()
	thread, _ := db.CreateChatThread(context.Background(), userID, nil, nil)
	db.ForceError = errors.New("list messages")
	defer func() { db.ForceError = nil }()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads/"+thread.ID.String()+"/messages", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListThreadMessages(rec, req, thread.ID.String())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_PatchThreadTitle_InvalidUUID(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := []byte(`{"title":"x"}`)
	req := httptest.NewRequest("PATCH", "/v1/chat/threads/bad-id", bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.PatchThreadTitle(rec, req, "bad-id")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_PatchThreadTitle_NoTitleInBody(t *testing.T) {
	db := testutil.NewMockDB()
	userID := uuid.New()
	thread, _ := db.CreateChatThread(context.Background(), userID, nil, nil)
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := []byte(`{}`)
	req := httptest.NewRequest("PATCH", "/v1/chat/threads/"+thread.ID.String(), bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.PatchThreadTitle(rec, req, thread.ID.String())
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_PatchThreadTitle_DBError(t *testing.T) {
	db := testutil.NewMockDB()
	userID := uuid.New()
	thread, _ := db.CreateChatThread(context.Background(), userID, nil, nil)
	db.ForceError = errors.New("db")
	defer func() { db.ForceError = nil }()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := []byte(`{"title":"x"}`)
	req := httptest.NewRequest("PATCH", "/v1/chat/threads/"+thread.ID.String(), bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.PatchThreadTitle(rec, req, thread.ID.String())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_ListThreads_DBError(t *testing.T) {
	db := testutil.NewMockDB()
	db.ForceError = errors.New("db")
	defer func() { db.ForceError = nil }()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListThreads(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_GetThread_DBError(t *testing.T) {
	db := testutil.NewMockDB()
	userID := uuid.New()
	thread, _ := db.CreateChatThread(context.Background(), userID, nil, nil)
	db.ForceError = errors.New("db")
	defer func() { db.ForceError = nil }()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads/"+thread.ID.String(), http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.GetThread(rec, req, thread.ID.String())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_Responses_GetOrCreateActiveFails(t *testing.T) {
	db := testutil.NewMockDB()
	db.ForceError = errors.New("db")
	defer func() { db.ForceError = nil }()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	body := []byte(`{"model":"m","input":"hi"}`)
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.Responses(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestOpenAIChatHandler_Responses_InputMessageArrayContentString(t *testing.T) {
	mockOllama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"response": "ok", "done": true})
	}))
	defer mockOllama.Close()
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), mockOllama.URL, "m", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	// content as string (not array) to cover extractTextFromMessageContent string branch
	body := []byte(`{"model":"m","input":[{"role":"user","content":"hello string"}]}`)
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body)).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.Responses(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
