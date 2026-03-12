// Package gateway provides a typed HTTP client for the User API Gateway.
// See docs/tech_specs/user_api_gateway.md and docs/tech_specs/cli_management_app.md.
package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

// Client calls the User API Gateway (auth, tasks, health).
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient returns a client for the given base URL (e.g. http://localhost:12080).
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: http.DefaultClient,
	}
}

// SetToken sets the Bearer token for subsequent requests.
func (c *Client) SetToken(token string) {
	c.Token = token
}

// Login calls POST /v1/auth/login and returns the token response.
func (c *Client) Login(req userapi.LoginRequest) (*userapi.LoginResponse, error) {
	var out userapi.LoginResponse
	if err := c.doPostJSON("/v1/auth/login", req, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Refresh calls POST /v1/auth/refresh and returns new tokens (no auth header required).
func (c *Client) Refresh(refreshToken string) (*userapi.LoginResponse, error) {
	req := userapi.RefreshRequest{RefreshToken: refreshToken}
	var out userapi.LoginResponse
	if err := c.doPostJSONNoAuth("/v1/auth/refresh", req, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetMe calls GET /v1/users/me (requires auth).
func (c *Client) GetMe() (*userapi.UserResponse, error) {
	resp, err := c.doRequest(http.MethodGet, "/v1/users/me", nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}
	var out userapi.UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode user response: %w", err)
	}
	return &out, nil
}

// HealthResponse is the body returned by GET /healthz (plain "ok").
// Per cli_management_app_commands_core.md the CLI MUST treat HTTP 200 with body containing "ok" as healthy.
type HealthResponse struct{}

// Health calls GET /healthz and returns nil if status 200 and body contains "ok".
func (c *Client) Health() error {
	resp, err := c.doRequest(http.MethodGet, "/healthz", nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("healthz: read body: %w", err)
	}
	if !strings.Contains(string(body), "ok") {
		return fmt.Errorf("healthz: body must contain %q, got %q", "ok", string(body))
	}
	return nil
}

// ListTasksRequest holds query params for GET /v1/tasks.
type ListTasksRequest struct {
	Limit  int    // default 50, max 200
	Offset int    // for pagination
	Cursor string // cursor-based pagination (opaque string from next_cursor)
	Status string // optional filter: queued, running, completed, failed, canceled
}

// ListTasks calls GET /v1/tasks (requires auth).
func (c *Client) ListTasks(req ListTasksRequest) (*userapi.ListTasksResponse, error) {
	q := url.Values{}
	if req.Limit > 0 {
		q.Set("limit", fmt.Sprint(req.Limit))
	}
	if req.Offset > 0 {
		q.Set("offset", fmt.Sprint(req.Offset))
	}
	if req.Cursor != "" {
		q.Set("cursor", req.Cursor)
	}
	if req.Status != "" {
		q.Set("status", req.Status)
	}
	resp, err := c.doRequest(http.MethodGet, "/v1/tasks", q, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}
	var out userapi.ListTasksResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode list tasks response: %w", err)
	}
	for i := range out.Tasks {
		normalizeTaskResponse(&out.Tasks[i])
	}
	return &out, nil
}

func normalizeTaskResponse(t *userapi.TaskResponse) {
	if t.TaskID == "" && t.ID != "" {
		t.TaskID = t.ID
	}
	if t.ID == "" && t.TaskID != "" {
		t.ID = t.TaskID
	}
}

// GetTask calls GET /v1/tasks/{id} (requires auth).
func (c *Client) GetTask(taskID string) (*userapi.TaskResponse, error) {
	path := "/v1/tasks/" + url.PathEscape(taskID)
	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}
	var out userapi.TaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode task response: %w", err)
	}
	normalizeTaskResponse(&out)
	return &out, nil
}

// doTaskPath performs a request to a task subpath and decodes the JSON response into out.
func (c *Client) doTaskPath(method, path string, out interface{}, decodeErrPrefix string) error {
	resp, err := c.doRequest(method, path, nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s: %w", decodeErrPrefix, err)
	}
	return nil
}

// CancelTask calls POST /v1/tasks/{id}/cancel (requires auth).
//
//nolint:dupl // same pattern as GetTaskResult by design
func (c *Client) CancelTask(taskID string) (*userapi.CancelTaskResponse, error) {
	var out userapi.CancelTaskResponse
	if err := c.doTaskPath(http.MethodPost, "/v1/tasks/"+url.PathEscape(taskID)+"/cancel", &out, "decode cancel task response"); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTaskLogs calls GET /v1/tasks/{id}/logs (requires auth).
// Stream query param: stdout | stderr | all (default).
func (c *Client) GetTaskLogs(taskID, stream string) (*userapi.TaskLogsResponse, error) {
	path := "/v1/tasks/" + url.PathEscape(taskID) + "/logs"
	if stream != "" {
		path += "?stream=" + url.QueryEscape(stream)
	}
	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}
	var out userapi.TaskLogsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode task logs response: %w", err)
	}
	return &out, nil
}

// ChatResponse is the parsed chat result for callers (content from choices[0].message.content).
type ChatResponse struct {
	Response string
}

// ListModelsResponse is the OpenAI-format response from GET /v1/models.
type ListModelsResponse struct {
	Object string           `json:"object"`
	Data   []ListModelEntry `json:"data"`
}

// ListModelEntry is one model in the list.
type ListModelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
}

// ListModels calls GET /v1/models (requires auth).
func (c *Client) ListModels() (*ListModelsResponse, error) {
	var out ListModelsResponse
	if err := c.doGetJSON("/v1/models", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Chat calls POST /v1/chat/completions (requires auth). Sends one user message; returns assistant content per openai_compatible_chat_api.md.
func (c *Client) Chat(message string) (*ChatResponse, error) {
	return c.ChatWithOptions(message, "", "")
}

// ChatWithOptions is like Chat but allows session model and OpenAI-Project header.
// If model is non-empty it is sent in the request body; if projectID is non-empty it is sent as OpenAI-Project header.
func (c *Client) ChatWithOptions(message, model, projectID string) (*ChatResponse, error) {
	req := userapi.ChatCompletionsRequest{
		Model:    model,
		Messages: []userapi.ChatMessage{{Role: "user", Content: message}},
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u, err := base.Parse("/v1/chat/completions")
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if projectID != "" {
		httpReq.Header.Set("OpenAI-Project", projectID)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}
	var out userapi.ChatCompletionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}
	content := ""
	if len(out.Choices) > 0 {
		content = out.Choices[0].Message.Content
	}
	return &ChatResponse{Response: content}, nil
}

// ResponsesResponse is the parsed result from POST /v1/responses (canonical visible text from output items).
type ResponsesResponse struct {
	VisibleText string // concatenated text from output items with type "text"
	ResponseID  string // id from response for continuation
}

// ResponsesWithOptions calls POST /v1/responses with input as a single user message string.
// If model or projectID is non-empty they are sent in the body or as OpenAI-Project header respectively.
func (c *Client) ResponsesWithOptions(message, model, projectID string) (*ResponsesResponse, error) {
	input, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("marshal input: %w", err)
	}
	req := userapi.ResponsesCreateRequest{
		Model: model,
		Input: input,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal responses request: %w", err)
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u, err := base.Parse("/v1/responses")
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if projectID != "" {
		httpReq.Header.Set("OpenAI-Project", projectID)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}
	var out userapi.ResponsesCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode responses: %w", err)
	}
	visible := ""
	for _, item := range out.Output {
		if item.Type == "text" {
			visible += item.Text
		}
	}
	return &ResponsesResponse{VisibleText: visible, ResponseID: out.ID}, nil
}

// NewChatThread calls POST /v1/chat/threads and returns the new thread ID.
// Use this when the user wants to start a fresh conversation context.
func (c *Client) NewChatThread(projectID string) (string, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}
	u, err := base.Parse("/v1/chat/threads")
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, u.String(), http.NoBody)
	if err != nil {
		return "", err
	}
	if c.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if projectID != "" {
		httpReq.Header.Set("OpenAI-Project", projectID)
	}
	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		return "", c.parseError(resp)
	}
	var out struct {
		ThreadID string `json:"thread_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode thread response: %w", err)
	}
	return out.ThreadID, nil
}

// ChatThreadItem is one thread in a list from GET /v1/chat/threads.
type ChatThreadItem struct {
	ID        string  `json:"id"`
	Title     *string `json:"title,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// ListChatThreads calls GET /v1/chat/threads with optional project and pagination.
func (c *Client) ListChatThreads(projectID string, limit, offset int) ([]ChatThreadItem, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u, err := base.Parse("/v1/chat/threads")
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	q := u.Query()
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		q.Set("offset", strconv.Itoa(offset))
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if projectID != "" {
		req.Header.Set("OpenAI-Project", projectID)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}
	var out struct {
		Data []ChatThreadItem `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode list threads response: %w", err)
	}
	return out.Data, nil
}

// PatchThreadTitle calls PATCH /v1/chat/threads/{id} to set the thread title.
func (c *Client) PatchThreadTitle(threadID, title string) error {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	u, err := base.Parse("/v1/chat/threads/" + url.PathEscape(threadID))
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	body := map[string]string{"title": title}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPatch, u.String(), bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	return nil
}

// CreateTask calls POST /v1/tasks (requires auth).
func (c *Client) CreateTask(req *userapi.CreateTaskRequest) (*userapi.TaskResponse, error) {
	var out userapi.TaskResponse
	if err := c.doPostJSON("/v1/tasks", req, http.StatusCreated, &out); err != nil {
		return nil, err
	}
	normalizeTaskResponse(&out)
	return &out, nil
}

// GetTaskResult calls GET /v1/tasks/{id}/result (requires auth).
//
//nolint:dupl // same pattern as CancelTask by design
func (c *Client) GetTaskResult(taskID string) (*userapi.TaskResultResponse, error) {
	var out userapi.TaskResultResponse
	if err := c.doTaskPath(http.MethodGet, "/v1/tasks/"+url.PathEscape(taskID)+"/result", &out, "decode task result response"); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBytes performs an authenticated GET and returns the body (for stub endpoints like /v1/creds).
func (c *Client) GetBytes(path string) ([]byte, error) {
	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}
	return io.ReadAll(resp.Body)
}

// PostBytes performs an authenticated POST with optional body and returns the response body.
func (c *Client) PostBytes(path string, body []byte) ([]byte, error) {
	var r io.Reader
	if len(body) > 0 {
		r = bytes.NewReader(body)
	}
	resp, err := c.doRequest(http.MethodPost, path, nil, r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return nil, c.parseError(resp)
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	return io.ReadAll(resp.Body)
}

// PutBytes performs an authenticated PUT with body and returns the response body.
func (c *Client) PutBytes(path string, body []byte) ([]byte, error) {
	var r io.Reader
	if len(body) > 0 {
		r = bytes.NewReader(body)
	}
	resp, err := c.doRequest(http.MethodPut, path, nil, r)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, c.parseError(resp)
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	return io.ReadAll(resp.Body)
}

// DeleteBytes performs an authenticated DELETE and returns the response body.
func (c *Client) DeleteBytes(path string) ([]byte, error) {
	resp, err := c.doRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, c.parseError(resp)
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) doRequest(method, path string, query url.Values, body io.Reader) (*http.Response, error) {
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u, err := base.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	return c.HTTPClient.Do(req)
}

func (c *Client) doPostJSON(path string, reqBody any, wantStatus int, out any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	resp, err := c.doRequest(http.MethodPost, path, nil, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != wantStatus {
		return c.parseError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// doPostJSONNoAuth posts JSON without Authorization header (e.g. refresh).
func (c *Client) doPostJSONNoAuth(path string, reqBody any, wantStatus int, out any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	base, err := url.Parse(c.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	u, err := base.Parse(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != wantStatus {
		return c.parseError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// doGetJSON performs authenticated GET and decodes JSON into out.
func (c *Client) doGetJSON(path string, out any) error {
	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// HTTPError carries the HTTP status for exit-code mapping (401->3, 404->4, etc.).
type HTTPError struct {
	Status int
	Err    error
}

func (e *HTTPError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("HTTP %d", e.Status)
}

func (e *HTTPError) Unwrap() error { return e.Err }

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var p problem.Details
	if len(body) > 0 {
		_ = json.Unmarshal(body, &p)
	}
	var msg string
	if p.Detail != "" {
		msg = fmt.Sprintf("%s: %s", resp.Status, p.Detail)
	} else {
		msg = resp.Status
	}
	return &HTTPError{Status: resp.StatusCode, Err: fmt.Errorf("%s", msg)}
}
