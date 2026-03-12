package sba

import (
	"context"
	"testing"
)

const mockLLMWantHello = "hello"

func TestMockLLM_Call(t *testing.T) {
	m := &MockLLM{Responses: []string{mockLLMWantHello}}
	out, err := m.Call(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if out != mockLLMWantHello {
		t.Errorf("out = %q", out)
	}
	out2, _ := m.Call(context.Background(), "second")
	if out2 != "Final Answer: Done" {
		t.Errorf("out2 = %q", out2)
	}
}

func TestMockLLM_Call_ContextCanceled_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := &MockLLM{}
	_, err := m.Call(ctx, "prompt")
	if err == nil {
		t.Fatal("expected error when ctx canceled")
	}
}
