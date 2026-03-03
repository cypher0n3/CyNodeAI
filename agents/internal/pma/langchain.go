// Package pma provides langchaingo-based chat completion with MCP tool support.
// See docs/tech_specs/project_manager_agent.md (LLM and Tool Execution) and cynode_pma.md.
package pma

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/tools"
)

const (
	pmaDefaultOllamaURL = "http://localhost:11434"
	pmaDefaultModel     = "tinyllama"
	pmaMaxIterations    = 10
)

// testLLMForCompletion is set by tests to avoid calling real Ollama. Production always leaves it nil.
var testLLMForCompletion llms.Model

// runCompletionWithLangchain runs one agentic completion using langchaingo (Ollama + MCP tool).
// fullPrompt is system context + formatted messages. When mcpClient.BaseURL is empty, returns error so caller can fall back to direct inference.
func runCompletionWithLangchain(ctx context.Context, fullPrompt string, mcpClient *MCPClient, logger *slog.Logger) (string, error) {
	if mcpClient == nil || mcpClient.BaseURL == "" {
		return "", fmt.Errorf("MCP gateway URL not set")
	}
	var llm llms.Model
	if testLLMForCompletion != nil {
		llm = testLLMForCompletion
	} else {
		baseURL := os.Getenv("OLLAMA_BASE_URL")
		if baseURL == "" {
			baseURL = os.Getenv("INFERENCE_URL")
		}
		if baseURL == "" {
			baseURL = pmaDefaultOllamaURL
		}
		model := os.Getenv("INFERENCE_MODEL")
		if model == "" {
			model = pmaDefaultModel
		}
		var err error
		llm, err = ollama.New(
			ollama.WithServerURL(baseURL),
			ollama.WithModel(model),
		)
		if err != nil {
			return "", fmt.Errorf("create ollama llm: %w", err)
		}
	}
	toolsList := []tools.Tool{NewMCPTool(mcpClient)}
	agent := agents.NewOneShotAgent(llm, toolsList,
		agents.WithMaxIterations(pmaMaxIterations),
	)
	exec := agents.NewExecutor(agent,
		agents.WithReturnIntermediateSteps(),
		agents.WithMaxIterations(pmaMaxIterations),
	)
	outputs, err := exec.Call(ctx, map[string]any{"input": fullPrompt})
	if err != nil {
		return "", err
	}
	return extractOutput(outputs), nil
}

func extractOutput(outputs map[string]any) string {
	if v, ok := outputs["output"]; ok && v != nil {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

func runCompletionWithLangchainWithTimeout(ctx context.Context, fullPrompt string, mcpClient *MCPClient, logger *slog.Logger, timeout time.Duration) (string, error) {
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return runCompletionWithLangchain(runCtx, fullPrompt, mcpClient, logger)
}
