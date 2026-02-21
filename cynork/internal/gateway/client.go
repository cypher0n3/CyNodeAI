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
)

// Client calls the User API Gateway (auth, tasks, health).
type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// NewClient returns a client for the given base URL (e.g. http://localhost:8080).
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

// LoginRequest is the body for POST /v1/auth/login.
type LoginRequest struct {
	Handle   string `json:"handle"`
	Password string `json:"password"`
}

// LoginResponse is the body returned by POST /v1/auth/login.
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// Login calls POST /v1/auth/login and returns the token response.
func (c *Client) Login(req LoginRequest) (*LoginResponse, error) {
	var out LoginResponse
	if err := c.doPostJSON("/v1/auth/login", req, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UserResponse is the body returned by GET /v1/users/me.
type UserResponse struct {
	ID       string  `json:"id"`
	Handle   string  `json:"handle"`
	Email    *string `json:"email,omitempty"`
	IsActive bool    `json:"is_active"`
}

// GetMe calls GET /v1/users/me (requires auth).
func (c *Client) GetMe() (*UserResponse, error) {
	resp, err := c.doRequest(http.MethodGet, "/v1/users/me", nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}
	var out UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode user response: %w", err)
	}
	return &out, nil
}

// HealthResponse is the body returned by GET /healthz (plain "ok").
type HealthResponse struct{}

// Health calls GET /healthz and returns nil if status 200.
func (c *Client) Health() error {
	resp, err := c.doRequest(http.MethodGet, "/healthz", nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz: %s", resp.Status)
	}
	return nil
}

// CreateTaskRequest is the body for POST /v1/tasks.
// InputMode: "prompt" (default) = natural language, inference; "script" or "commands" = literal shell.
type CreateTaskRequest struct {
	Prompt       string `json:"prompt"`
	UseInference bool   `json:"use_inference,omitempty"`
	InputMode    string `json:"input_mode,omitempty"`
}

// TaskResponse is the task in create/get responses.
// Gateway may return "id" or "task_id"; CLI spec uses task_id.
type TaskResponse struct {
	ID        string  `json:"id"`
	TaskID    string  `json:"task_id"` // alias for list/get responses
	Status    string  `json:"status"`
	TaskName  *string `json:"task_name,omitempty"`
	Prompt    *string `json:"prompt,omitempty"`
	Summary   *string `json:"summary,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// ResolveTaskID returns the task identifier (task_id if set, else id).
func (t *TaskResponse) ResolveTaskID() string {
	if t.TaskID != "" {
		return t.TaskID
	}
	return t.ID
}

// ListTasksRequest holds query params for GET /v1/tasks.
type ListTasksRequest struct {
	Limit  int    // default 50, max 200
	Offset int    // for pagination
	Status string // optional filter: queued, running, completed, failed, cancelled/canceled
}

// ListTasksResponse is the body of GET /v1/tasks.
type ListTasksResponse struct {
	Tasks      []TaskResponse `json:"tasks"`
	NextOffset *int           `json:"next_offset,omitempty"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// ListTasks calls GET /v1/tasks (requires auth).
func (c *Client) ListTasks(req ListTasksRequest) (*ListTasksResponse, error) {
	q := url.Values{}
	if req.Limit > 0 {
		q.Set("limit", fmt.Sprint(req.Limit))
	}
	if req.Offset > 0 {
		q.Set("offset", fmt.Sprint(req.Offset))
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
	var out ListTasksResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode list tasks response: %w", err)
	}
	for i := range out.Tasks {
		normalizeTaskResponse(&out.Tasks[i])
	}
	return &out, nil
}

func normalizeTaskResponse(t *TaskResponse) {
	if t.TaskID == "" && t.ID != "" {
		t.TaskID = t.ID
	}
	if t.ID == "" && t.TaskID != "" {
		t.ID = t.TaskID
	}
}

// GetTask calls GET /v1/tasks/{id} (requires auth).
func (c *Client) GetTask(taskID string) (*TaskResponse, error) {
	path := "/v1/tasks/" + url.PathEscape(taskID)
	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}
	var out TaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode task response: %w", err)
	}
	normalizeTaskResponse(&out)
	return &out, nil
}

// CancelTaskResponse is the body of POST /v1/tasks/{id}/cancel.
type CancelTaskResponse struct {
	TaskID   string `json:"task_id"`
	Canceled bool   `json:"canceled"`
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
func (c *Client) CancelTask(taskID string) (*CancelTaskResponse, error) {
	var out CancelTaskResponse
	if err := c.doTaskPath(http.MethodPost, "/v1/tasks/"+url.PathEscape(taskID)+"/cancel", &out, "decode cancel task response"); err != nil {
		return nil, err
	}
	return &out, nil
}

// TaskLogsResponse is the body of GET /v1/tasks/{id}/logs.
type TaskLogsResponse struct {
	TaskID string `json:"task_id"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// GetTaskLogs calls GET /v1/tasks/{id}/logs (requires auth).
// Stream query param: stdout | stderr | all (default).
func (c *Client) GetTaskLogs(taskID, stream string) (*TaskLogsResponse, error) {
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
	var out TaskLogsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode task logs response: %w", err)
	}
	return &out, nil
}

// ChatRequest is the request body for POST /v1/chat (orchestrator).
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse is the response body for POST /v1/chat.
type ChatResponse struct {
	Response string `json:"response"`
}

// Chat calls POST /v1/chat (requires auth). Uses the real orchestrator chat endpoint; server creates task and returns response when terminal.
func (c *Client) Chat(message string) (*ChatResponse, error) {
	var out ChatResponse
	if err := c.doPostJSON("/v1/chat", ChatRequest{Message: message}, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateTask calls POST /v1/tasks (requires auth).
func (c *Client) CreateTask(req CreateTaskRequest) (*TaskResponse, error) {
	var out TaskResponse
	if err := c.doPostJSON("/v1/tasks", req, http.StatusCreated, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// JobResponse is a single job in a task result.
type JobResponse struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	Result    *string `json:"result,omitempty"`
	StartedAt *string `json:"started_at,omitempty"`
	EndedAt   *string `json:"ended_at,omitempty"`
}

// TaskResultResponse is the body returned by GET /v1/tasks/{id}/result.
type TaskResultResponse struct {
	TaskID string        `json:"task_id"`
	Status string        `json:"status"`
	Jobs   []JobResponse `json:"jobs"`
}

// GetTaskResult calls GET /v1/tasks/{id}/result (requires auth).
//
//nolint:dupl // same pattern as CancelTask by design
func (c *Client) GetTaskResult(taskID string) (*TaskResultResponse, error) {
	var out TaskResultResponse
	if err := c.doTaskPath(http.MethodGet, "/v1/tasks/"+url.PathEscape(taskID)+"/result", &out, "decode task result response"); err != nil {
		return nil, err
	}
	return &out, nil
}

// ProblemDetails is RFC 9457 problem details from the API.
type ProblemDetails struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
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
	var problem ProblemDetails
	if len(body) > 0 {
		_ = json.Unmarshal(body, &problem)
	}
	var msg string
	if problem.Detail != "" {
		msg = fmt.Sprintf("%s: %s", resp.Status, problem.Detail)
	} else {
		msg = resp.Status
	}
	return &HTTPError{Status: resp.StatusCode, Err: fmt.Errorf("%s", msg)}
}
