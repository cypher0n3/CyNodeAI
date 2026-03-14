package chat

import (
	"context"

	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

// Session holds chat session state (model, project, transport, client for thread/auth)
// on an instance so the fullscreen TUI and PTY tests can run without package-level globals.
type Session struct {
	Client    *gateway.Client
	Transport ChatTransport

	Model     string
	ProjectID string
	// CurrentThreadID is set when the user creates or switches to a thread (for display and rename).
	CurrentThreadID string

	Plain   bool
	NoColor bool
}

// NewSession returns a session using the completions transport by default.
func NewSession(client *gateway.Client) *Session {
	return &Session{
		Client:    client,
		Transport: &CompletionsTransport{Client: client},
	}
}

// NewSessionWithResponses uses POST /v1/responses as the chat transport.
func NewSessionWithResponses(client *gateway.Client) *Session {
	return &Session{
		Client:    client,
		Transport: &ResponsesTransport{Client: client},
	}
}

// SetModel sets the in-session model for the next send.
func (s *Session) SetModel(model string) {
	s.Model = model
}

// SetProjectID sets the in-session project (OpenAI-Project) for the next send.
func (s *Session) SetProjectID(id string) {
	s.ProjectID = id
}

// SetCurrentThreadID sets the active thread (for display and rename).
func (s *Session) SetCurrentThreadID(id string) {
	s.CurrentThreadID = id
}

// SetToken updates the underlying client token (e.g. after auth refresh).
// Both Session.Client and the Transport's client are the same at creation, so
// updating one updates the other. Use SetClient when replacing the client entirely.
func (s *Session) SetToken(token string) {
	if s.Client != nil {
		s.Client.SetToken(token)
	}
}

// SetClient replaces the session's gateway client and updates the transport to use it.
// Call this after /auth login or any client replacement so chat requests use the new client/token.
func (s *Session) SetClient(client *gateway.Client) {
	s.Client = client
	switch t := s.Transport.(type) {
	case *CompletionsTransport:
		t.Client = client
	case *ResponsesTransport:
		t.Client = client
	}
}

// SendMessage sends one user message and returns the assistant turn (non-streaming).
func (s *Session) SendMessage(ctx context.Context, message string) (*AssistantTurn, error) {
	return s.Transport.SendMessage(ctx, message, s.Model, s.ProjectID)
}

// StreamMessage sends one user message and returns a channel of incremental deltas.
// The caller must drain the channel or cancel ctx. Per REQ-CLIENT-0209.
func (s *Session) StreamMessage(ctx context.Context, message string) (<-chan ChatStreamDelta, error) {
	return s.Transport.StreamMessage(ctx, message, s.Model, s.ProjectID)
}

// NewThread creates a new chat thread via the gateway; uses session project context.
// On success sets CurrentThreadID to the new thread id.
func (s *Session) NewThread() (threadID string, err error) {
	threadID, err = s.Client.NewChatThread(s.ProjectID)
	if err != nil {
		return "", err
	}
	s.CurrentThreadID = threadID
	return threadID, nil
}

// ListThreads returns threads for the current user and project (recent-first, paginated).
func (s *Session) ListThreads(limit, offset int) ([]gateway.ChatThreadItem, error) {
	return s.Client.ListChatThreads(s.ProjectID, limit, offset)
}

// PatchThreadTitle renames the thread; requires CurrentThreadID or pass threadID.
func (s *Session) PatchThreadTitle(threadID, title string) error {
	if threadID == "" {
		threadID = s.CurrentThreadID
	}
	return s.Client.PatchThreadTitle(threadID, title)
}
