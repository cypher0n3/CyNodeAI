// Package sba provides an MCP gateway client for sandbox-allowlist tools.
package sba

import "github.com/cypher0n3/cynodeai/agents/internal/mcpclient"

// MCPClient calls the orchestrator MCP gateway (POST /v1/mcp/tools/call).
type MCPClient = mcpclient.Client

// NewMCPClient creates a client using SBA_MCP_GATEWAY_URL or MCP_GATEWAY_URL. Empty if unset.
func NewMCPClient() *MCPClient {
	return mcpclient.NewSBAClient()
}
