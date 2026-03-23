// Package pma provides a mock LLM for tests (no real Ollama required).
package pma

import (
	"context"
	"sync"

	"github.com/tmc/langchaingo/llms"
)

// mockLLM implements llms.Model for tests. Returns fixed response.
type mockLLM struct {
	mu        sync.Mutex
	callNum   int
	responses []string
	// errs, when set, makes the i-th GenerateContent call return errs[i] instead of responses[i].
	errs []error
}

func (m *mockLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	i := m.callNum
	if m.errs != nil && i < len(m.errs) && m.errs[i] != nil {
		err := m.errs[i]
		m.callNum++
		m.mu.Unlock()
		return nil, err
	}
	m.callNum++
	m.mu.Unlock()
	var text string
	if i < len(m.responses) {
		text = m.responses[i]
	} else {
		text = "Done"
	}
	var opts llms.CallOptions
	for _, o := range options {
		o(&opts)
	}
	if opts.StreamingFunc != nil && text != "" {
		_ = opts.StreamingFunc(ctx, []byte(text))
	}
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: text}},
	}, nil
}

func (m *mockLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	resp, err := m.GenerateContent(ctx, []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, prompt)}, options...)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Content, nil
}

var _ llms.Model = (*mockLLM)(nil)
