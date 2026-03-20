// Package pma provides langchaingo tools that call the orchestrator MCP gateway (PM allowlist).
// See docs/tech_specs/mcp_tools/ and access_allowlists_and_scope.md (PM agent allowlist).
package pma

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/tools"
)

// MCPTool is a langchaingo tool that forwards to the orchestrator MCP gateway (PM scope).
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

// Description describes the PM allowlist tools (db., node., sandbox., artifact., etc.).
func (m *MCPTool) Description() string {
	return `Call the orchestrator MCP gateway (PM allowlist). Input JSON: {"tool_name": "NAME", "arguments": {...}}.
Allowed tool_name: preference.get, preference.list, preference.effective, preference.create, preference.update, preference.delete, task.get, job.get, artifact.get, and others per mcp_tools/.`
}

// MsgMCPGatewayNotConfigured is returned when MCP gateway URL is not set.
const MsgMCPGatewayNotConfigured = "MCP gateway not configured (set PMA_MCP_GATEWAY_URL or MCP_GATEWAY_URL)"

// Call runs the tool.
func (m *MCPTool) Call(ctx context.Context, input string) (string, error) {
	if m.client == nil || m.client.BaseURL == "" {
		return MsgMCPGatewayNotConfigured, nil
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
	if code != 200 {
		return string(body), nil
	}
	return string(body), nil
}

var _ tools.Tool = (*MCPTool)(nil)
