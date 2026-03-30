// Package pma provides streaming LLM wrapper for standard path (capable model + MCP).
// See docs/tech_specs/cynode_pma.md: StreamingLLMWrapper, PMAStreamingNDJSONFormat.
package pma

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/tmc/langchaingo/llms"
)

// streamingLLM wraps an llms.Model to emit NDJSON (iteration_start, delta, done) to the response writer.
// Used when req.Stream is true on the capable-model + MCP path.
type streamingLLM struct {
	inner     llms.Model
	w         http.ResponseWriter
	enc       *json.Encoder
	flusher   http.Flusher
	iteration *int
	mu        sync.Mutex
}

// newStreamingLLM returns an llms.Model that writes iteration_start before each GenerateContent
// and streams token deltas as NDJSON lines. iteration is incremented per GenerateContent call.
func newStreamingLLM(inner llms.Model, w http.ResponseWriter, iteration *int) llms.Model {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	var flusher http.Flusher
	if f, ok := w.(http.Flusher); ok {
		flusher = f
	}
	return &streamingLLM{
		inner:     inner,
		w:         w,
		enc:       enc,
		flusher:   flusher,
		iteration: iteration,
	}
}

// GenerateContent implements llms.Model. Emits iteration_start then delegates to inner with StreamingFunc.
func (s *streamingLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	s.mu.Lock()
	*s.iteration++
	iter := *s.iteration
	s.mu.Unlock()

	if err := s.enc.Encode(map[string]int{"iteration_start": iter}); err != nil {
		return nil, err
	}
	if s.flusher != nil {
		s.flusher.Flush()
	}

	clf := newStreamingClassifier()
	var raw []streamEmitted
	streamFn := func(ctx context.Context, chunk []byte) error {
		for _, em := range clf.Feed(string(chunk)) {
			raw = append(raw, em)
			if err := s.encodeStreamEmitted(em); err != nil {
				return err
			}
		}
		return nil
	}
	opts := append([]llms.CallOption{}, options...)
	opts = append(opts, llms.WithStreamingFunc(streamFn))
	resp, err := s.inner.GenerateContent(ctx, messages, opts...)
	if err != nil {
		return resp, err
	}
	for _, em := range clf.Flush() {
		raw = append(raw, em)
		if err := s.encodeStreamEmitted(em); err != nil {
			return resp, err
		}
	}
	combined := joinEmittedVisible(raw) + joinEmittedThinking(raw) + joinEmittedToolCalls(raw)
	if detectSecrets(combined) {
		rv, _, kinds := redactKnownSecrets(joinEmittedVisible(raw))
		if err := encodeOverwriteNDJSON(s.enc, s.w, iter, rv, "iteration", "secret_redaction", kinds); err != nil {
			return resp, err
		}
	}
	return resp, nil
}

func (s *streamingLLM) encodeStreamEmitted(em streamEmitted) error {
	rem := redactStreamEmitted(em)
	var err error
	switch rem.Kind {
	case streamEmitDelta:
		err = s.enc.Encode(map[string]string{"delta": rem.Text})
	case streamEmitThinking:
		err = s.enc.Encode(map[string]string{"thinking": rem.Text})
	case streamEmitToolCall:
		err = s.enc.Encode(map[string]any{
			"tool_call": map[string]string{"name": "stream", "arguments": rem.Text},
		})
	default:
		return nil
	}
	if err != nil {
		return err
	}
	if s.flusher != nil {
		s.flusher.Flush()
	}
	return nil
}

// Call implements llms.Model via single-prompt generation.
func (s *streamingLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	return llms.GenerateFromSinglePrompt(ctx, s, prompt, options...)
}

var _ llms.Model = (*streamingLLM)(nil)
