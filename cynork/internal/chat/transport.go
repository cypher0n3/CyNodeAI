// Package chat provides shared chat-session and transport abstractions for the CLI chat command
// and the future fullscreen TUI, so request shaping, slash handling, and transcript behavior
// are reusable. See docs/tech_specs/cynork_tui.md and docs/dev_docs/2026-03-12_plan_next_round_execution.md Phase 4.
// Streaming contract: CYNAI.USRGWY.OpenAIChatApi.Streaming, REQ-CLIENT-0209.
package chat

import (
	"context"

	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

// streamDeltaBufSize is the channel buffer for streaming delta events.
// Sized to absorb short bursts while keeping the goroutine from blocking on the receiver.
const streamDeltaBufSize = 32

// AssistantTurn is the canonical one-logical-turn result from the gateway (visible text;
// optional response_id for continuation). Used by CLI and TUI for transcript and display.
type AssistantTurn struct {
	VisibleText string
	ResponseID  string
}

// ChatStreamDelta is one incremental event from a streaming assistant turn.
// When Done is true the stream has ended; Err carries any terminal error.
// Amendment is set when the gateway sends cynodeai.amendment (e.g. secret_redaction): replace accumulated visible text in place.
// At most one of the structured fields (Thinking, Tool*, Heartbeat, IterationStart) should be set per event.
type ChatStreamDelta struct {
	Delta      string
	Done       bool
	Err        error
	ResponseID string
	// Amendment is the full redacted content when event type is secret_redaction; replace in-flight text in place.
	Amendment                string
	AmendmentScope           string // "turn" (default) or "iteration"
	AmendmentTargetIteration int
	Thinking                 string
	ToolName                 string
	ToolArgs                 string
	// IterationStart is true when the gateway emits cynodeai.iteration_start; Iteration is the iteration index.
	IterationStart   bool
	Iteration        int
	IsHeartbeat      bool
	HeartbeatElapsed int
	HeartbeatStatus  string
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
	resp, err := t.Client.ChatWithOptions(ctx, message, model, projectID)
	if err != nil {
		return nil, err
	}
	return &AssistantTurn{VisibleText: resp.Response}, nil
}

// StreamMessage implements ChatTransport using the completions endpoint with stream=true.
func (t *CompletionsTransport) StreamMessage(ctx context.Context, message, model, projectID string) (<-chan ChatStreamDelta, error) {
	ch := make(chan ChatStreamDelta, streamDeltaBufSize)
	go t.pumpCompletionsStream(ctx, ch, message, model, projectID)
	return ch, nil
}

func (t *CompletionsTransport) pumpCompletionsStream(ctx context.Context, ch chan<- ChatStreamDelta, message, model, projectID string) {
	defer close(ch)
	extra := streamExtraForDeltas(ch, ctx)
	onDelta := func(delta string) {
		select {
		case ch <- ChatStreamDelta{Delta: delta}:
		case <-ctx.Done():
		}
	}
	onAmendment := func(redacted string) {
		select {
		case ch <- ChatStreamDelta{Amendment: redacted}:
		case <-ctx.Done():
		}
	}
	err := t.Client.ChatStream(ctx, message, model, projectID, onDelta, onAmendment, extra)
	ch <- ChatStreamDelta{Done: true, Err: err}
}

// ResponsesTransport uses POST /v1/responses.
type ResponsesTransport struct {
	Client *gateway.Client
}

// SendMessage implements ChatTransport using the responses endpoint (non-streaming).
func (t *ResponsesTransport) SendMessage(ctx context.Context, message, model, projectID string) (*AssistantTurn, error) {
	resp, err := t.Client.ResponsesWithOptions(ctx, message, model, projectID)
	if err != nil {
		return nil, err
	}
	return &AssistantTurn{VisibleText: resp.VisibleText, ResponseID: resp.ResponseID}, nil
}

// StreamMessage implements ChatTransport using the responses endpoint with stream=true.
func (t *ResponsesTransport) StreamMessage(ctx context.Context, message, model, projectID string) (<-chan ChatStreamDelta, error) {
	ch := make(chan ChatStreamDelta, streamDeltaBufSize)
	go t.pumpResponsesStream(ctx, ch, message, model, projectID)
	return ch, nil
}

func (t *ResponsesTransport) pumpResponsesStream(ctx context.Context, ch chan<- ChatStreamDelta, message, model, projectID string) {
	defer close(ch)
	extra := streamExtraForDeltas(ch, ctx)
	onDelta := func(delta string) {
		select {
		case ch <- ChatStreamDelta{Delta: delta}:
		case <-ctx.Done():
		}
	}
	onAmendment := func(redacted string) {
		select {
		case ch <- ChatStreamDelta{Amendment: redacted}:
		case <-ctx.Done():
		}
	}
	respID, err := t.Client.ResponsesStream(ctx, message, model, projectID, onDelta, onAmendment, extra)
	ch <- ChatStreamDelta{Done: true, Err: err, ResponseID: respID}
}

// streamExtraForDeltas builds gateway SSE extension callbacks that forward into ch.
func streamExtraForDeltas(ch chan<- ChatStreamDelta, ctx context.Context) *gateway.StreamExtra {
	return &gateway.StreamExtra{
		OnThinking: func(content string) {
			select {
			case ch <- ChatStreamDelta{Thinking: content}:
			case <-ctx.Done():
			}
		},
		OnToolCall: func(name, args string) {
			select {
			case ch <- ChatStreamDelta{ToolName: name, ToolArgs: args}:
			case <-ctx.Done():
			}
		},
		OnHeartbeat: func(elapsedSec int, status string) {
			select {
			case ch <- ChatStreamDelta{IsHeartbeat: true, HeartbeatElapsed: elapsedSec, HeartbeatStatus: status}:
			case <-ctx.Done():
			}
		},
		OnIterationStart: func(iter int) {
			select {
			case ch <- ChatStreamDelta{IterationStart: true, Iteration: iter}:
			case <-ctx.Done():
			}
		},
	}
}
