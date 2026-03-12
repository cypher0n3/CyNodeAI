// Package sba provides langchaingo tools that call the MCP gateway (sandbox allowlist).
package sba

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tmc/langchaingo/tools"
)

// MCPTool lets the agent call the orchestrator MCP gateway (artifact., memory., skills., web., api., help.).
type MCPTool struct {
	client *MCPClient
}

// NewMCPTool returns a langchaingo tool that forwards to the MCP gateway.
// When client.BaseURL is empty, Call returns a message that MCP is not configured.
func NewMCPTool(client *MCPClient) tools.Tool {
	return &MCPTool{client: client}
}

// Name returns the tool name the agent uses.
func (m *MCPTool) Name() string { return "mcp_call" }

// Description describes allowed tools for the agent.
func (m *MCPTool) Description() string {
	return `Call the orchestrator MCP gateway (sandbox allowlist). Input JSON: {"tool_name": "NAME", "arguments": {...}}.
Allowed tool_name: artifact.put, artifact.get, artifact.list, memory.add, memory.list, memory.retrieve, memory.delete, skills.list, skills.get, web.fetch, api.call, help.* (when implemented).`
}

// Call runs the tool.
func (m *MCPTool) Call(ctx context.Context, input string) (string, error) {
	if m.client == nil || m.client.BaseURL == "" {
		return "MCP gateway not configured (set SBA_MCP_GATEWAY_URL or MCP_GATEWAY_URL)", nil
	}
	var req struct {
		ToolName  string                 `json:"tool_name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(input)), &req); err != nil {
		return "", fmt.Errorf("invalid mcp_call input JSON: %w", err)
	}
	if req.ToolName == "" {
		return "", fmt.Errorf("tool_name required")
	}
	body, code, err := m.client.Call(ctx, req.ToolName, req.Arguments)
	if err != nil {
		return "", err
	}
	if code == http.StatusForbidden || strings.Contains(string(body), "ext_net_required") || strings.Contains(string(body), "network denied") {
		return "", ErrExtNetRequired
	}
	if code != http.StatusOK {
		return string(body), nil
	}
	return string(body), nil
}

// ErrExtNetRequired is returned when the gateway or tool signals network was needed but denied.
var ErrExtNetRequired = &extNetRequiredError{}

type extNetRequiredError struct{}

func (e *extNetRequiredError) Error() string { return "network required but denied" }

// IsExtNetRequired reports whether err is ext_net_required.
func IsExtNetRequired(err error) bool {
	if err == nil {
		return false
	}
	var target *extNetRequiredError
	return errors.As(err, &target)
}

var _ tools.Tool = (*MCPTool)(nil)
