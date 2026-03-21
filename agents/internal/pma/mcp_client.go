// Package pma provides MCP gateway client for cynode-pma (orchestrator-side).
// See docs/tech_specs/cynode_pma.md (MCP Tool Access) and mcp_gateway_enforcement.md.
package pma

import "github.com/cypher0n3/cynodeai/agents/internal/mcpclient"

// MCPClient calls the orchestrator MCP gateway (POST /v1/mcp/tools/call).
type MCPClient = mcpclient.Client

// NewMCPClient creates a client from PMA_MCP_GATEWAY_URL or MCP_GATEWAY_URL. Empty BaseURL if unset.
func NewMCPClient() *MCPClient {
	return mcpclient.NewPMClient()
}
