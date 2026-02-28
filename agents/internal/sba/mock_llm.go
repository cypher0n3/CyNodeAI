// Package sba provides a mock LLM for tests (no real Ollama required).
package sba

import (
	"context"
	"sync"

	"github.com/tmc/langchaingo/llms"
)

// MockLLM implements llms.Model for tests. It returns fixed responses per call index.
type MockLLM struct {
	mu        sync.Mutex
	CallNum   int
	Responses []string
}

// GenerateContent returns the next response from Responses, or "Final Answer: Done" if none left.
// If ctx is already cancelled, returns ctx.Err() so timeout tests can pass.
func (m *MockLLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	i := m.CallNum
	m.CallNum++
	m.mu.Unlock()
	var text string
	if i < len(m.Responses) {
		text = m.Responses[i]
	} else {
		text = "Final Answer: Done"
	}
	return &llms.ContentResponse{
		Choices: []*llms.ContentChoice{{Content: text}},
	}, nil
}

// Call is the deprecated simplified interface; used by some code paths.
func (m *MockLLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	resp, err := m.GenerateContent(ctx, []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeHuman, prompt)}, options...)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Content, nil
}

var _ llms.Model = (*MockLLM)(nil)
