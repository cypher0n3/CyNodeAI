// Package pma provides MCP gateway client for cynode-pma (orchestrator-side).
// See docs/tech_specs/cynode_pma.md (MCP Tool Access) and mcp_gateway_enforcement.md.
package pma

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	envPmaMcpGateway   = "PMA_MCP_GATEWAY_URL"
	envMcpGateway      = "MCP_GATEWAY_URL"
	envMcpGatewayProxy = "MCP_GATEWAY_PROXY_URL"
	defaultMCPTimeout  = 30 * time.Second
	mcpToolsCallPath   = "/v1/mcp/tools/call"
	mcpProxyCallPath   = "/v1/worker/internal/orchestrator/mcp:call"
)

// MCPClient calls the orchestrator MCP gateway (POST /v1/mcp/tools/call).
// Used when PMA uses langchaingo with MCP tool wrappers.
type MCPClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewMCPClient creates a client from PMA_MCP_GATEWAY_URL or MCP_GATEWAY_URL. Empty BaseURL if unset.
func NewMCPClient() *MCPClient {
	baseURL := os.Getenv(envPmaMcpGateway)
	if baseURL == "" {
		baseURL = os.Getenv(envMcpGateway)
	}
	if baseURL == "" {
		baseURL = os.Getenv(envMcpGatewayProxy)
	}
	return &MCPClient{
		BaseURL: baseURL,
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
		return nil, 0, fmt.Errorf("MCP gateway URL not set (%s, %s, or %s)", envPmaMcpGateway, envMcpGateway, envMcpGatewayProxy)
	}
	reqBody := MCPCallRequest{ToolName: toolName, Arguments: arguments}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, err
	}
	if isInternalProxyMCPURL(c.BaseURL) {
		return c.callViaWorkerInternalProxy(ctx, raw)
	}
	reqURL := strings.TrimRight(c.BaseURL, "/") + mcpToolsCallPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(raw))
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

type managedServiceProxyRequest struct {
	Version int                 `json:"version"`
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers,omitempty"`
	BodyB64 string              `json:"body_b64,omitempty"`
}

type managedServiceProxyResponse struct {
	Version int    `json:"version"`
	Status  int    `json:"status"`
	BodyB64 string `json:"body_b64,omitempty"`
}

func isInternalProxyMCPURL(baseURL string) bool {
	return strings.Contains(strings.TrimSpace(baseURL), mcpProxyCallPath)
}

func (c *MCPClient) callViaWorkerInternalProxy(ctx context.Context, mcpBody []byte) (body []byte, statusCode int, err error) {
	httpClient := c.HTTPClient
	requestURL := strings.TrimSpace(c.BaseURL)
	if strings.HasPrefix(requestURL, "http+unix://") {
		unixSocketPath, endpointPath, ok := parseHTTPUnixEndpoint(requestURL)
		if !ok {
			return nil, 0, fmt.Errorf("invalid http+unix proxy URL")
		}
		httpClient = &http.Client{
			Timeout: defaultMCPTimeout,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{}).DialContext(ctx, "unix", unixSocketPath)
				},
			},
		}
		requestURL = "http://unix" + endpointPath
	}
	reqPayload := managedServiceProxyRequest{
		Version: 1,
		Method:  http.MethodPost,
		Path:    mcpToolsCallPath,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		BodyB64: base64.StdEncoding.EncodeToString(mcpBody),
	}
	raw, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return buf.Bytes(), resp.StatusCode, nil
	}
	var proxyResp managedServiceProxyResponse
	if err := json.Unmarshal(buf.Bytes(), &proxyResp); err != nil {
		return nil, 0, err
	}
	decoded, err := base64.StdEncoding.DecodeString(proxyResp.BodyB64)
	if err != nil {
		return nil, 0, err
	}
	return decoded, proxyResp.Status, nil
}

func parseHTTPUnixEndpoint(rawURL string) (socketPath, endpointPath string, ok bool) {
	trimmed := strings.TrimSpace(rawURL)
	if !strings.HasPrefix(trimmed, "http+unix://") {
		return "", "", false
	}
	withoutScheme := strings.TrimPrefix(trimmed, "http+unix://")
	idx := strings.Index(withoutScheme, "/")
	if idx <= 0 {
		return "", "", false
	}
	escapedSocket := withoutScheme[:idx]
	endpointPath = withoutScheme[idx:]
	socketPath, err := url.PathUnescape(escapedSocket)
	if err != nil || strings.TrimSpace(socketPath) == "" || strings.TrimSpace(endpointPath) == "" {
		return "", "", false
	}
	return socketPath, endpointPath, true
}
