// Package mcpclient provides a shared HTTP client for calling the orchestrator MCP gateway
// (POST /v1/mcp/tools/call). Used by PMA and SBA.
package mcpclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/httplimits"
)

const (
	envPmaMcpGateway   = "PMA_MCP_GATEWAY_URL"
	envSbaMcpGateway   = "SBA_MCP_GATEWAY_URL"
	envMcpGateway      = "MCP_GATEWAY_URL"
	envMcpGatewayProxy = "MCP_GATEWAY_PROXY_URL"
	// ORCHESTRATOR_MCP_TOOLS_BASE_URL is the preferred explicit base for the control-plane MCP tool endpoint.
	envOrchestratorMCPToolsBaseURL = "ORCHESTRATOR_MCP_TOOLS_BASE_URL"
	// ORCHESTRATOR_MCP_GATEWAY_BASE_URL is a deprecated alias for ORCHESTRATOR_MCP_TOOLS_BASE_URL.
	envOrchestratorMCPGatewayBaseURL = "ORCHESTRATOR_MCP_GATEWAY_BASE_URL"
	envOrchestratorURL               = "ORCHESTRATOR_URL"
	defaultMCPTimeout                = 30 * time.Second
	// ToolsCallPath is POST /v1/mcp/tools/call on the MCP gateway.
	ToolsCallPath = "/v1/mcp/tools/call"
	// ProxyCallPath is the worker internal path that wraps MCP calls (managed service proxy).
	ProxyCallPath = "/v1/worker/internal/orchestrator/mcp:call"
)

// Client calls the orchestrator MCP gateway.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewPMClient resolves the MCP tools base URL with this precedence:
// PMA_MCP_GATEWAY_URL, MCP_GATEWAY_URL, MCP_GATEWAY_PROXY_URL (worker-injected internal proxy),
// ORCHESTRATOR_MCP_TOOLS_BASE_URL, deprecated ORCHESTRATOR_MCP_GATEWAY_BASE_URL, ORCHESTRATOR_URL.
func NewPMClient() *Client {
	baseURL := strings.TrimSpace(os.Getenv(envPmaMcpGateway))
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv(envMcpGateway))
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv(envMcpGatewayProxy))
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv(envOrchestratorMCPToolsBaseURL))
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv(envOrchestratorMCPGatewayBaseURL))
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv(envOrchestratorURL))
	}
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: defaultMCPTimeout,
		},
	}
}

// NewSBAClient resolves the MCP tools base URL with this precedence:
// SBA_MCP_GATEWAY_URL, MCP_GATEWAY_URL, ORCHESTRATOR_MCP_TOOLS_BASE_URL,
// deprecated ORCHESTRATOR_MCP_GATEWAY_BASE_URL, ORCHESTRATOR_URL.
func NewSBAClient() *Client {
	u := strings.TrimSpace(os.Getenv(envSbaMcpGateway))
	if u == "" {
		u = strings.TrimSpace(os.Getenv(envMcpGateway))
	}
	if u == "" {
		u = strings.TrimSpace(os.Getenv(envOrchestratorMCPToolsBaseURL))
	}
	if u == "" {
		u = strings.TrimSpace(os.Getenv(envOrchestratorMCPGatewayBaseURL))
	}
	if u == "" {
		u = strings.TrimSpace(os.Getenv(envOrchestratorURL))
	}
	return &Client{
		BaseURL: u,
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
func (c *Client) Call(ctx context.Context, toolName string, arguments map[string]interface{}) (body []byte, statusCode int, err error) {
	if c.BaseURL == "" {
		return nil, 0, fmt.Errorf("MCP tools base URL not set (PMA: %s / %s / %s / %s / %s / %s; SBA: %s / %s / %s / %s / %s)",
			envPmaMcpGateway, envMcpGateway, envMcpGatewayProxy, envOrchestratorMCPToolsBaseURL, envOrchestratorMCPGatewayBaseURL, envOrchestratorURL,
			envSbaMcpGateway, envMcpGateway, envOrchestratorMCPToolsBaseURL, envOrchestratorMCPGatewayBaseURL, envOrchestratorURL)
	}
	reqBody := CallRequest{ToolName: toolName, Arguments: arguments}
	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, 0, err
	}
	if isInternalProxyMCPURL(c.BaseURL) {
		return c.callViaWorkerInternalProxy(ctx, raw)
	}
	reqURL := strings.TrimRight(c.BaseURL, "/") + ToolsCallPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	var buf bytes.Buffer
	_, err = buf.ReadFrom(io.LimitReader(resp.Body, httplimits.DefaultMaxHTTPResponseBytes))
	if err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), resp.StatusCode, nil
}

// ManagedServiceProxyRequest is the JSON envelope for worker-mediated MCP proxy calls.
type ManagedServiceProxyRequest struct {
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
	return strings.Contains(strings.TrimSpace(baseURL), ProxyCallPath)
}

func (c *Client) callViaWorkerInternalProxy(ctx context.Context, mcpBody []byte) (body []byte, statusCode int, err error) {
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	requestURL := strings.TrimSpace(c.BaseURL)
	if strings.HasPrefix(requestURL, "http+unix://") {
		unixSocketPath, endpointPath, ok := ParseHTTPUnixEndpoint(requestURL)
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
	reqPayload := ManagedServiceProxyRequest{
		Version: 1,
		Method:  http.MethodPost,
		Path:    ToolsCallPath,
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
	_, err = buf.ReadFrom(io.LimitReader(resp.Body, httplimits.DefaultMaxHTTPResponseBytes))
	if err != nil {
		return nil, 0, err
	}
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

// ParseHTTPUnixEndpoint parses http+unix://escaped-socket/path for worker proxy URLs.
func ParseHTTPUnixEndpoint(rawURL string) (socketPath, endpointPath string, ok bool) {
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
