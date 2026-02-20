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
type CreateTaskRequest struct {
	Prompt string `json:"prompt"`
}

// TaskResponse is the task in create/get responses.
type TaskResponse struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	Prompt    *string `json:"prompt,omitempty"`
	Summary   *string `json:"summary,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
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
	TaskID string         `json:"task_id"`
	Status string         `json:"status"`
	Jobs   []JobResponse  `json:"jobs"`
}

// GetTaskResult calls GET /v1/tasks/{id}/result (requires auth).
func (c *Client) GetTaskResult(taskID string) (*TaskResultResponse, error) {
	path := "/v1/tasks/" + url.PathEscape(taskID) + "/result"
	resp, err := c.doRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseError(resp)
	}
	var out TaskResultResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode task result response: %w", err)
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

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var problem ProblemDetails
	if len(body) > 0 {
		_ = json.Unmarshal(body, &problem)
	}
	if problem.Detail != "" {
		return fmt.Errorf("%s: %s", resp.Status, problem.Detail)
	}
	return fmt.Errorf("%s", resp.Status)
}
