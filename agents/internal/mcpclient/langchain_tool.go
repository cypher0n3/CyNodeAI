package mcpclient

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/tools"
)

// LangchainTool is a langchaingo tool that forwards JSON to POST /v1/mcp/tools/call.
type LangchainTool struct {
	client           *Client
	description      string
	notConfiguredMsg string
}

// NewLangchainTool returns a tools.Tool that forwards {"tool_name","arguments"} to the gateway.
// When notConfiguredMsg is empty, a generic message is used when BaseURL is unset.
func NewLangchainTool(client *Client, description, notConfiguredMsg string) tools.Tool {
	if notConfiguredMsg == "" {
		notConfiguredMsg = "MCP gateway not configured"
	}
	return &LangchainTool{
		client:           client,
		description:      description,
		notConfiguredMsg: notConfiguredMsg,
	}
}

// Name implements tools.Tool.
func (m *LangchainTool) Name() string { return "mcp_call" }

// Description implements tools.Tool.
func (m *LangchainTool) Description() string { return m.description }

// Call implements tools.Tool.
func (m *LangchainTool) Call(ctx context.Context, input string) (string, error) {
	if m.client == nil || m.client.BaseURL == "" {
		return m.notConfiguredMsg, nil
	}
	toolName, args, err := DecodeMCPCallInput(input)
	if err != nil {
		return "", fmt.Errorf("invalid mcp_call input JSON: %w", err)
	}
	if toolName == "" {
		return "", fmt.Errorf("tool_name required")
	}
	body, code, err := m.client.Call(ctx, toolName, args)
	if err != nil {
		return "", err
	}
	if code != 200 {
		return string(body), nil
	}
	return string(body), nil
}

var _ tools.Tool = (*LangchainTool)(nil)
