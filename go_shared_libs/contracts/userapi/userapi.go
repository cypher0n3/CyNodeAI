// Package userapi defines request and response types for the User API Gateway.
// Single source of truth for auth, users, tasks, and chat used by orchestrator handlers and cynork (and future Web Console).
// See docs/tech_specs/user_api_gateway.md and REQ-CLIENT-0004 (CLI/Web Console parity).
package userapi

import "encoding/json"

// API-facing task/job status constants (returned in REST responses; used by CLI/Web Console).
// Canonical spelling is American "canceled".
const (
	StatusQueued     = "queued"
	StatusRunning    = "running"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusCanceled   = "canceled"
	StatusSuperseded = "superseded"
)

// --- Auth ---

// LoginRequest is the body for POST /v1/auth/login.
type LoginRequest struct {
	Handle   string `json:"handle"`
	Password string `json:"password"`
}

// LoginResponse is the body returned by POST /v1/auth/login and POST /v1/auth/refresh.
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// RefreshRequest is the body for POST /v1/auth/refresh.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// LogoutRequest is the body for POST /v1/auth/logout.
type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// UserResponse is the body returned by GET /v1/users/me.
type UserResponse struct {
	ID       string  `json:"id"`
	Handle   string  `json:"handle"`
	Email    *string `json:"email,omitempty"`
	IsActive bool    `json:"is_active"`
}

// --- Tasks ---

// CreateTaskRequest is the body for POST /v1/tasks.
type CreateTaskRequest struct {
	Prompt       string  `json:"prompt"`
	ProjectID    *string `json:"project_id,omitempty"`
	UseInference bool    `json:"use_inference,omitempty"`
	InputMode    string  `json:"input_mode,omitempty"`
	// UseSBA when true creates a single job with SBA runner (job_spec_json); prompt is passed as task context (P2-10).
	UseSBA bool `json:"use_sba,omitempty"`
	// TaskName is optional; orchestrator normalizes and ensures uniqueness per user_api_gateway.md Task Naming.
	TaskName *string `json:"task_name,omitempty"`
	// Attachments are optional path strings (CLI) or identifiers for file uploads; acceptance path per REQ-ORCHES-0127.
	Attachments []string `json:"attachments,omitempty"`
}

// TaskResponse is the task in create/get/list responses (CLI spec: task_id, status, optional task_name).
// Attachments lists artifact paths for the task (REQ-ORCHES-0127, REQ-CLIENT-0157).
type TaskResponse struct {
	ID          string   `json:"id"`
	TaskID      string   `json:"task_id"`
	Status      string   `json:"status"`
	TaskName    *string  `json:"task_name,omitempty"`
	Prompt      *string  `json:"prompt,omitempty"`
	Summary     *string  `json:"summary,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	Attachments []string `json:"attachments,omitempty"`
}

// ResolveTaskID returns the task identifier (task_id if set, else id).
func (t *TaskResponse) ResolveTaskID() string {
	if t.TaskID != "" {
		return t.TaskID
	}
	return t.ID
}

// ListTasksResponse is the body of GET /v1/tasks.
type ListTasksResponse struct {
	Tasks      []TaskResponse `json:"tasks"`
	NextOffset *int           `json:"next_offset,omitempty"`
	// next_cursor is always present (empty string when there is no further page).
	NextCursor string `json:"next_cursor"`
}

// CancelTaskResponse is the body of POST /v1/tasks/{id}/cancel.
type CancelTaskResponse struct {
	TaskID   string `json:"task_id"`
	Canceled bool   `json:"canceled"`
}

// TaskResultResponse is the body of GET /v1/tasks/{id}/result.
type TaskResultResponse struct {
	TaskID string        `json:"task_id"`
	Status string        `json:"status"`
	Jobs   []JobResponse `json:"jobs"`
}

// JobResponse is one job in a task result.
type JobResponse struct {
	ID        string  `json:"id"`
	Status    string  `json:"status"`
	Result    *string `json:"result,omitempty"`
	StartedAt *string `json:"started_at,omitempty"`
	EndedAt   *string `json:"ended_at,omitempty"`
}

// TaskLogsResponse is the body of GET /v1/tasks/{id}/logs.
type TaskLogsResponse struct {
	TaskID string `json:"task_id"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// --- Chat (OpenAI-compatible) ---

// ChatMessage is one message in the OpenAI messages array.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionsRequest is the request body for POST /v1/chat/completions (subset we use).
type ChatCompletionsRequest struct {
	Model    string        `json:"model,omitempty"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

// ChatCompletionsChoice is one choice in the chat completions response.
type ChatCompletionsChoice struct {
	Index   int `json:"index"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	FinishReason string `json:"finish_reason"`
}

// ChatCompletionsResponse is the response from POST /v1/chat/completions (subset we use).
type ChatCompletionsResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []ChatCompletionsChoice `json:"choices"`
}

// --- POST /v1/responses (OpenAI Responses API, first-pass) ---

// ResponsesCreateRequest is the request body for POST /v1/responses.
// Input can be a plain string or an ordered message-like array for multi-turn continuation.
type ResponsesCreateRequest struct {
	Model              string          `json:"model,omitempty"`
	Input              json.RawMessage `json:"input"` // string or array of message-like items
	PreviousResponseID string          `json:"previous_response_id,omitempty"`
	Stream             bool            `json:"stream,omitempty"`
}

// ResponsesOutputText is one output item in the responses format (visible text).
type ResponsesOutputText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ResponsesCreateResponse is the response from POST /v1/responses (first-pass subset).
type ResponsesCreateResponse struct {
	ID      string                `json:"id"`
	Object  string                `json:"object"`
	Created int64                 `json:"created"`
	Output  []ResponsesOutputText `json:"output"`
}

// --- SSE streaming types (CYNAI.USRGWY.OpenAIChatApi.Streaming) ---

// ChatCompletionChunkDelta is the delta in a streaming chat completion chunk.
type ChatCompletionChunkDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

// ChatCompletionChunkChoice is one choice in a streaming chunk event.
type ChatCompletionChunkChoice struct {
	Index        int                      `json:"index"`
	Delta        ChatCompletionChunkDelta `json:"delta"`
	FinishReason *string                  `json:"finish_reason"`
}

// ChatCompletionChunk is the SSE event payload for streaming chat completions.
// Object is "chat.completion.chunk".
type ChatCompletionChunk struct {
	ID      string                      `json:"id"`
	Object  string                      `json:"object"`
	Created int64                       `json:"created"`
	Model   string                      `json:"model"`
	Choices []ChatCompletionChunkChoice `json:"choices"`
}

// --- CyNodeAI SSE extension events (StreamingPerEndpointSSEFormat) ---

// SSE event type names for CyNodeAI streaming extensions.
// Chat-completions and responses endpoints emit these as named event: lines.
const (
	SSEEventThinkingDelta  = "cynodeai.thinking_delta"
	SSEEventToolCall       = "cynodeai.tool_call"
	SSEEventToolProgress   = "cynodeai.tool_progress"
	SSEEventIterationStart = "cynodeai.iteration_start"
	SSEEventAmendment      = "cynodeai.amendment"
	SSEEventHeartbeat      = "cynodeai.heartbeat"
)

// SSEThinkingDeltaPayload is the data payload for event: cynodeai.thinking_delta.
type SSEThinkingDeltaPayload struct {
	Content string `json:"content"`
}

// SSEToolCallPayload is the data payload for event: cynodeai.tool_call.
type SSEToolCallPayload struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// SSEToolProgressPayload is the data payload for event: cynodeai.tool_progress.
type SSEToolProgressPayload struct {
	State   string `json:"state"`
	Tool    string `json:"tool"`
	Preview string `json:"preview,omitempty"`
}

// SSEIterationStartPayload is the data payload for event: cynodeai.iteration_start.
type SSEIterationStartPayload struct {
	Iteration int `json:"iteration"`
}

// SSEAmendmentPayload is the data payload for event: cynodeai.amendment.
// Type is "secret_redaction" or "overwrite"; scope/iteration for overwrite.
type SSEAmendmentPayload struct {
	Type           string   `json:"type"`
	Content        string   `json:"content"`
	RedactionKinds []string `json:"redaction_kinds,omitempty"`
	Scope          string   `json:"scope,omitempty"`
	Iteration      *int     `json:"iteration,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

// SSEHeartbeatPayload is the data payload for event: cynodeai.heartbeat.
type SSEHeartbeatPayload struct {
	ElapsedS int    `json:"elapsed_s"`
	Status   string `json:"status"`
}
