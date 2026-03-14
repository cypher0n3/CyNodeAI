// Package chat provides shared chat-session and transport abstractions for the CLI chat command
// and the future fullscreen TUI, so request shaping, slash handling, and transcript behavior
// are reusable. See docs/tech_specs/cynork_tui.md and docs/dev_docs/2026-03-12_plan_next_round_execution.md Phase 4.
// Streaming contract: CYNAI.USRGWY.OpenAIChatApi.Streaming, REQ-CLIENT-0209.
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

// ChatStreamDelta is one incremental event from a streaming assistant turn.
// When Done is true the stream has ended; Err carries any terminal error.
// Per CYNAI.USRGWY.OpenAIChatApi.Streaming: events are ordered visible-text deltas
// followed by a terminal completion or error event.
type ChatStreamDelta struct {
	// Delta is the incremental visible-text content for this event.
	Delta string
	// Done is true for the terminal event (no more deltas follow).
	Done bool
	// Err is set on the terminal event when the stream ended with an error.
	Err error
	// ResponseID is populated on the final Done event for responses-surface continuation.
	ResponseID string
}

// ChatTransport sends a user message and returns the assistant turn.
// Implementations may call POST /v1/chat/completions or POST /v1/responses.
// StreamMessage provides the streaming path per REQ-CLIENT-0209; SendMessage is the
// buffered fallback used when streaming is not needed (e.g. non-interactive CLI).
type ChatTransport interface {
	// SendMessage sends a message and waits for the complete assistant turn.
	SendMessage(ctx context.Context, message, model, projectID string) (*AssistantTurn, error)
	// StreamMessage sends a message and streams incremental deltas to the returned channel.
	// The caller MUST drain the channel to completion or cancel ctx to release resources.
	// The final event has Done=true; if an error occurred, Err is set on that event.
	StreamMessage(ctx context.Context, message, model, projectID string) (<-chan ChatStreamDelta, error)
}

// CompletionsTransport uses POST /v1/chat/completions.
type CompletionsTransport struct {
	Client *gateway.Client
}

// SendMessage implements ChatTransport using the completions endpoint (non-streaming).
func (t *CompletionsTransport) SendMessage(ctx context.Context, message, model, projectID string) (*AssistantTurn, error) {
	resp, err := t.Client.ChatWithOptions(message, model, projectID)
	if err != nil {
		return nil, err
	}
	return &AssistantTurn{VisibleText: resp.Response}, nil
}

// StreamMessage implements ChatTransport using the completions endpoint with stream=true.
// Returns a channel of incremental deltas. The goroutine exits when the stream ends or
// ctx is canceled, sending a terminal Done event.
func (t *CompletionsTransport) StreamMessage(ctx context.Context, message, model, projectID string) (<-chan ChatStreamDelta, error) {
	ch := make(chan ChatStreamDelta, 32) //nolint:mnd // buffer avoids blocking the SSE reader goroutine
	go func() {
		defer close(ch)
		err := t.Client.ChatStream(ctx, message, model, projectID, func(delta string) {
			select {
			case ch <- ChatStreamDelta{Delta: delta}:
			case <-ctx.Done():
			}
		})
		ch <- ChatStreamDelta{Done: true, Err: err}
	}()
	return ch, nil
}

// ResponsesTransport uses POST /v1/responses.
type ResponsesTransport struct {
	Client *gateway.Client
}

// SendMessage implements ChatTransport using the responses endpoint (non-streaming).
func (t *ResponsesTransport) SendMessage(ctx context.Context, message, model, projectID string) (*AssistantTurn, error) {
	resp, err := t.Client.ResponsesWithOptions(message, model, projectID)
	if err != nil {
		return nil, err
	}
	return &AssistantTurn{VisibleText: resp.VisibleText, ResponseID: resp.ResponseID}, nil
}

// StreamMessage implements ChatTransport using the responses endpoint with stream=true.
func (t *ResponsesTransport) StreamMessage(ctx context.Context, message, model, projectID string) (<-chan ChatStreamDelta, error) {
	ch := make(chan ChatStreamDelta, 32) //nolint:mnd // buffer avoids blocking the SSE reader goroutine
	go func() {
		defer close(ch)
		respID, err := t.Client.ResponsesStream(ctx, message, model, projectID, func(delta string) {
			select {
			case ch <- ChatStreamDelta{Delta: delta}:
			case <-ctx.Done():
			}
		})
		ch <- ChatStreamDelta{Done: true, Err: err, ResponseID: respID}
	}()
	return ch, nil
}
