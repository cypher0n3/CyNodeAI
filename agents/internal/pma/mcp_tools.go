// Package pma provides langchaingo tools that call the orchestrator MCP gateway (PM allowlist).
// See docs/tech_specs/mcp_tools/ and access_allowlists_and_scope.md (PM agent allowlist).
package pma

import (
	"github.com/cypher0n3/cynodeai/agents/internal/mcpclient"
	"github.com/tmc/langchaingo/tools"
)

// MsgMCPGatewayNotConfigured is returned when MCP gateway URL is not set.
const MsgMCPGatewayNotConfigured = "MCP gateway not configured (set PMA_MCP_GATEWAY_URL, MCP_GATEWAY_URL, MCP_GATEWAY_PROXY_URL, or ORCHESTRATOR_MCP_TOOLS_BASE_URL / ORCHESTRATOR_URL)"

// NewMCPTool returns a langchaingo tool that forwards to the MCP gateway.
// When client.BaseURL is empty, Call returns a message that MCP is not configured.
func NewMCPTool(client *MCPClient) tools.Tool {
	desc := `Call the orchestrator MCP gateway (PM allowlist). Input JSON: {"tool_name": "NAME", "arguments": {...}}.
Allowed tool_name values include task.get, task.list, task.result, task.cancel, task.logs, project.get, project.list, help.list, help.get, preference.*, job.get, artifact.get, skills.*, and others per docs/tech_specs/mcp_tools/.`
	return mcpclient.NewLangchainTool(client, desc, MsgMCPGatewayNotConfigured)
}
