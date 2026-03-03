// Package pma provides MCP gateway client for cynode-pma (orchestrator-side).
// See docs/tech_specs/cynode_pma.md (MCP Tool Access) and mcp_gateway_enforcement.md.
package pma

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

const (
	envPmaMcpGateway  = "PMA_MCP_GATEWAY_URL"
	envMcpGateway     = "MCP_GATEWAY_URL"
	defaultMCPTimeout  = 30 * time.Second
	mcpToolsCallPath   = "/v1/mcp/tools/call"
)

// MCPClient calls the orchestrator MCP gateway (POST /v1/mcp/tools/call).
// Used when PMA uses langchaingo with MCP tool wrappers.
type MCPClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewMCPClient creates a client from PMA_MCP_GATEWAY_URL or MCP_GATEWAY_URL. Empty BaseURL if unset.
func NewMCPClient() *MCPClient {
	url := os.Getenv(envPmaMcpGateway)
	if url == "" {
		url = os.Getenv(envMcpGateway)
	}
	return &MCPClient{
		BaseURL: url,
		HTTPClient: &http.Client{
			Timeout: defaultMCPTimeout,
		},
	}
}

// MCPCallRequest is the JSON body for POST /v1/mcp/tools/call (per mcp_tool_catalog.md).
type MCPCallRequest struct {
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// Call sends a tool call to the gateway and returns the response body, status code, and error.
func (c *MCPClient) Call(ctx context.Context, toolName string, arguments map[string]interface{}) (body []byte, statusCode int, err error) {
	if c.BaseURL == "" {
		return nil, 0, fmt.Errorf("MCP gateway URL not set (%s or %s)", envPmaMcpGateway, envMcpGateway)
	}
	reqBody := MCPCallRequest{ToolName: toolName, Arguments: arguments}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, err
	}
	url := c.BaseURL + mcpToolsCallPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	return buf.Bytes(), resp.StatusCode, nil
}
