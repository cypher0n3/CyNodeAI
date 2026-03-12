// Package sba provides an MCP gateway client for sandbox-allowlist tools.
package sba

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
	envMCPGatewayURL  = "SBA_MCP_GATEWAY_URL"
	envMCPGatewayURL2 = "MCP_GATEWAY_URL"
	defaultMCPTimeout = 30 * time.Second
)

// MCPClient calls the orchestrator MCP gateway (POST /v1/mcp/tools/call).
type MCPClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewMCPClient creates a client using SBA_MCP_GATEWAY_URL or MCP_GATEWAY_URL. Empty if unset.
func NewMCPClient() *MCPClient {
	url := os.Getenv(envMCPGatewayURL)
	if url == "" {
		url = os.Getenv(envMCPGatewayURL2)
	}
	return &MCPClient{
		BaseURL: url,
		HTTPClient: &http.Client{
			Timeout: defaultMCPTimeout,
		},
	}
}

// CallRequest is the JSON body for POST /v1/mcp/tools/call.
type CallRequest struct {
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// Call sends a tool call to the gateway and returns the response body, status code, and error.
func (c *MCPClient) Call(ctx context.Context, toolName string, arguments map[string]interface{}) (body []byte, statusCode int, err error) {
	if c.BaseURL == "" {
		return nil, 0, fmt.Errorf("MCP gateway URL not set (%s or %s)", envMCPGatewayURL, envMCPGatewayURL2)
	}
	reqBody := CallRequest{ToolName: toolName, Arguments: arguments}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, err
	}
	url := c.BaseURL + "/v1/mcp/tools/call"
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
