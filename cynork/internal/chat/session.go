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

// SetToken updates the underlying client token (e.g. after auth refresh).
func (s *Session) SetToken(token string) {
	if s.Client != nil {
		s.Client.SetToken(token)
	}
}

// SendMessage sends one user message and returns the assistant turn.
func (s *Session) SendMessage(ctx context.Context, message string) (*AssistantTurn, error) {
	return s.Transport.SendMessage(ctx, message, s.Model, s.ProjectID)
}

// NewThread creates a new chat thread via the gateway; uses session project context.
func (s *Session) NewThread() (threadID string, err error) {
	return s.Client.NewChatThread(s.ProjectID)
}
