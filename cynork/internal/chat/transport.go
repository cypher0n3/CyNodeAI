// Package chat provides shared chat-session and transport abstractions for the CLI chat command
// and the future fullscreen TUI, so request shaping, slash handling, and transcript behavior
// are reusable. See docs/tech_specs/cynork_tui.md and docs/dev_docs/2026-03-12_plan_next_round_execution.md Phase 4.
package chat

import (
	"context"

	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

// AssistantTurn is the canonical one-logical-turn result from the gateway (visible text;
// optional response_id for continuation). Used by CLI and TUI for transcript and display.
type AssistantTurn struct {
	VisibleText string
	ResponseID  string
}

// ChatTransport sends a user message and returns the assistant turn.
// Implementations may call POST /v1/chat/completions or POST /v1/responses.
type ChatTransport interface {
	SendMessage(ctx context.Context, message, model, projectID string) (*AssistantTurn, error)
}

// CompletionsTransport uses POST /v1/chat/completions.
type CompletionsTransport struct {
	Client *gateway.Client
}

// SendMessage implements ChatTransport using the completions endpoint.
func (t *CompletionsTransport) SendMessage(ctx context.Context, message, model, projectID string) (*AssistantTurn, error) {
	resp, err := t.Client.ChatWithOptions(message, model, projectID)
	if err != nil {
		return nil, err
	}
	return &AssistantTurn{VisibleText: resp.Response}, nil
}

// ResponsesTransport uses POST /v1/responses.
type ResponsesTransport struct {
	Client *gateway.Client
}

// SendMessage implements ChatTransport using the responses endpoint.
func (t *ResponsesTransport) SendMessage(ctx context.Context, message, model, projectID string) (*AssistantTurn, error) {
	resp, err := t.Client.ResponsesWithOptions(message, model, projectID)
	if err != nil {
		return nil, err
	}
	return &AssistantTurn{VisibleText: resp.VisibleText, ResponseID: resp.ResponseID}, nil
}
