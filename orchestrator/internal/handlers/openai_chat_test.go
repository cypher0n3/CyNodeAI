package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
	"github.com/google/uuid"
)

// mockTimeoutError implements net.Error for testing isTransientInferenceError.
type mockTimeoutError struct{ msg string }

func (e *mockTimeoutError) Error() string   { return e.msg }
func (e *mockTimeoutError) Timeout() bool   { return true }
func (e *mockTimeoutError) Temporary() bool { return true }

var _ net.Error = (*mockTimeoutError)(nil)

func TestIsTransientInferenceError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"deadline exceeded", context.DeadlineExceeded, false},
		{"context canceled", context.Canceled, false},
		{"net timeout", &mockTimeoutError{"timeout"}, true},
		{"connection refused", fmt.Errorf("dial tcp: connection refused"), true},
		{"returned 5xx", fmt.Errorf("PMA returned 503"), true},
		{"other error", fmt.Errorf("some other error"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isTransientInferenceError(tc.err)
			if got != tc.want {
				t.Errorf("isTransientInferenceError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestExtractUserContentFromResponsesInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain string", `"hello world"`, "hello world"},
		{"empty string", `""`, ""},
		{
			"message array user role with text field",
			`[{"role":"user","text":"hi there"}]`,
			"hi there",
		},
		{
			"message array user role with content array",
			`[{"role":"user","content":[{"text":"from content"}]}]`,
			"from content",
		},
		{
			"message array skips non-user roles",
			`[{"role":"assistant","text":"skip me"},{"role":"user","text":"pick me"}]`,
			"pick me",
		},
		{
			"message array invalid JSON",
			`not-json`,
			"",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var input json.RawMessage
			if tc.input != "" {
				input = json.RawMessage(tc.input)
			}
			got := extractUserContentFromResponsesInput(input)
			if got != tc.want {
				t.Errorf("extractUserContentFromResponsesInput(%s) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestResponsesContextMessages_EmptyHistory(t *testing.T) {
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	threadID := uuid.New()
	redacted := buildTestChatMessages("user content")
	result := h.buildChatContextMessages(context.Background(), threadID, redacted)
	// Empty history → returns redacted unchanged.
	if len(result) != len(redacted) {
		t.Errorf("len = %d, want %d", len(result), len(redacted))
	}
}

func TestResponsesContextMessages_WithHistory(t *testing.T) {
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	threadID := uuid.New()
	// Seed thread messages in the mock DB.
	for _, msg := range []struct{ role, content string }{
		{"user", "first"},
		{"assistant", "reply"},
	} {
		_, err := db.AppendChatMessage(context.Background(), threadID, msg.role, msg.content, nil)
		if err != nil {
			t.Fatalf("AppendChatMessage: %v", err)
		}
	}
	redacted := buildTestChatMessages("new question")
	result := h.buildChatContextMessages(context.Background(), threadID, redacted)
	// With history, trimHistoryToCharBudget is applied.
	if len(result) == 0 {
		t.Error("expected non-empty context messages")
	}
}

func TestParseResponsesInputAsMessageArray_ContentArray(t *testing.T) {
	input := json.RawMessage(`[{"role":"user","content":[{"text":"from array"}]}]`)
	got := parseResponsesInputAsMessageArray(input)
	if got != "from array" {
		t.Errorf("got %q, want from array", got)
	}
}

func TestParseResponsesInputAsMessageArray_EmptyArray(t *testing.T) {
	got := parseResponsesInputAsMessageArray(json.RawMessage(`[]`))
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// buildTestChatMessages builds a minimal redacted message slice for testing.
func buildTestChatMessages(content string) []userapi.ChatMessage {
	return []userapi.ChatMessage{{Role: "user", Content: content}}
}

// --- mock DB types for resolveThreadForResponses ---

type responseIDNotFoundDB struct{ *testutil.MockDB }

func (m *responseIDNotFoundDB) GetThreadByResponseID(_ context.Context, _ string, _ uuid.UUID) (*models.ChatThread, error) {
	return nil, database.ErrNotFound
}

type responseIDErrDB struct{ *testutil.MockDB }

func (m *responseIDErrDB) GetThreadByResponseID(_ context.Context, _ string, _ uuid.UUID) (*models.ChatThread, error) {
	return nil, errors.New("db error")
}

type getOrCreateErrDB struct{ *testutil.MockDB }

func (m *getOrCreateErrDB) GetOrCreateActiveChatThread(_ context.Context, _ uuid.UUID, _ *uuid.UUID) (*models.ChatThread, error) {
	return nil, errors.New("db error")
}

func TestResolveThreadForResponses_ErrorPaths(t *testing.T) {
	type rCase struct {
		name       string
		db         database.Store
		prevRespID string
		wantStatus int
	}
	cases := []rCase{
		{
			name:       "prev_resp_id_not_found",
			db:         &responseIDNotFoundDB{testutil.NewMockDB()},
			prevRespID: "some-id",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "prev_resp_id_db_error",
			db:         &responseIDErrDB{testutil.NewMockDB()},
			prevRespID: "some-id",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "get_or_create_error",
			db:         &getOrCreateErrDB{testutil.NewMockDB()},
			prevRespID: "",
			wantStatus: http.StatusInternalServerError,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewOpenAIChatHandler(tc.db, newTestLogger(), "", "", "")
			_, status, _ := h.resolveThreadForResponses(context.Background(), uuid.New(), nil, tc.prevRespID)
			if status != tc.wantStatus {
				t.Errorf("expected %d, got %d", tc.wantStatus, status)
			}
		})
	}
}

// --- mock DB for responsesContextMessages error ---

type listMsgErrDB struct{ *testutil.MockDB }

func (m *listMsgErrDB) ListChatMessages(_ context.Context, _ uuid.UUID, _ int) ([]*models.ChatMessage, error) {
	return nil, errors.New("list error")
}

func TestResponsesContextMessages_ListError(t *testing.T) {
	h := NewOpenAIChatHandler(&listMsgErrDB{testutil.NewMockDB()}, newTestLogger(), "", "", "")
	redacted := buildTestChatMessages("hi")
	result := h.buildChatContextMessages(context.Background(), uuid.New(), redacted)
	if len(result) != len(redacted) {
		t.Errorf("expected redacted unchanged (len=%d), got len=%d", len(redacted), len(result))
	}
}

// --- decodeAndValidateResponsesRequest ---

func TestDecodeAndValidateResponsesRequest_EmptyInput(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	body := []byte(`{"model":"test"}`)
	req := httptest.NewRequest("POST", "/v1/responses", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	_, status, msg := h.decodeAndValidateResponsesRequest(req)
	if status != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", status)
	}
	if msg != "input is required" {
		t.Errorf("expected 'input is required', got %q", msg)
	}
}

// --- ListThreadMessages error paths ---

type getChatThreadByIDErrDB struct{ *testutil.MockDB }

func (m *getChatThreadByIDErrDB) GetChatThreadByID(_ context.Context, _, _ uuid.UUID) (*models.ChatThread, error) {
	return nil, errors.New("db error")
}

func TestListThreadMessages_NoAuth(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	req := httptest.NewRequest("GET", "/v1/chat/threads/"+uuid.New().String()+"/messages", http.NoBody)
	rec := httptest.NewRecorder()
	h.ListThreadMessages(rec, req, uuid.New().String())
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestListThreadMessages_InvalidID(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads/bad-id/messages", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListThreadMessages(rec, req, "bad-id")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestListThreadMessages_ThreadNotFound(t *testing.T) {
	h := NewOpenAIChatHandler(testutil.NewMockDB(), newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	threadID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/chat/threads/"+threadID.String()+"/messages", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListThreadMessages(rec, req, threadID.String())
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestListThreadMessages_GetThreadError(t *testing.T) {
	h := NewOpenAIChatHandler(&getChatThreadByIDErrDB{testutil.NewMockDB()}, newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	threadID := uuid.New()
	req := httptest.NewRequest("GET", "/v1/chat/threads/"+threadID.String()+"/messages", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListThreadMessages(rec, req, threadID.String())
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestListThreadMessages_WithLimitQueryParam(t *testing.T) {
	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	userID := uuid.New()
	// create a thread owned by userID
	thread, err := db.GetOrCreateActiveChatThread(context.Background(), userID, nil)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("GET", "/v1/chat/threads/"+thread.ID.String()+"/messages?limit=10", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ListThreadMessages(rec, req, thread.ID.String())
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- NewThread DB error path ---

type createChatThreadErrDB struct{ *testutil.MockDB }

func (m *createChatThreadErrDB) CreateChatThread(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _ *string) (*models.ChatThread, error) {
	return nil, errors.New("create thread error")
}

func TestNewThread_DBError(t *testing.T) {
	h := NewOpenAIChatHandler(&createChatThreadErrDB{testutil.NewMockDB()}, newTestLogger(), "", "", "")
	userID := uuid.New()
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	req := httptest.NewRequest("POST", "/v1/chat/threads", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.NewThread(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// --- completeViaPMAStream ---

func TestCompleteViaPMAStream_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-ndjson")
		// PMA NDJSON: iteration_start then deltas (pmaclient processNDJSONLine expects top-level keys).
		_, _ = fmt.Fprintln(w, `{"iteration_start":1}`)
		_, _ = fmt.Fprintln(w, `{"delta":"hello "}`)
		_, _ = fmt.Fprintln(w, `{"delta":"world"}`)
	}))
	defer srv.Close()

	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	cand := pmaEndpointCandidate{endpoint: srv.URL}
	threadID := uuid.New()
	userID := uuid.New()
	rec := httptest.NewRecorder()

	err := h.completeViaPMAStream(
		context.Background(), rec, cand,
		[]userapi.ChatMessage{{Role: "user", Content: "test"}},
		threadID, &userID, nil, time.Now(), "test-model", "cid", nil,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "data:") {
		t.Errorf("expected SSE data in response: %s", body)
	}
	if !strings.Contains(body, "event: cynodeai.iteration_start") {
		t.Errorf("expected cynodeai.iteration_start event in response: %s", body)
	}
}

func TestCompleteViaPMAStream_Error(t *testing.T) {
	// Server immediately closes to simulate connection error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	srv.Close()

	db := testutil.NewMockDB()
	h := NewOpenAIChatHandler(db, newTestLogger(), "", "", "")
	cand := pmaEndpointCandidate{endpoint: srv.URL}
	threadID := uuid.New()
	userID := uuid.New()
	rec := httptest.NewRecorder()

	err := h.completeViaPMAStream(
		context.Background(), rec, cand,
		[]userapi.ChatMessage{{Role: "user", Content: "test"}},
		threadID, &userID, nil, time.Now(), "test-model", "cid", nil,
	)
	if err == nil {
		t.Error("expected an error from closed server")
	}
}
