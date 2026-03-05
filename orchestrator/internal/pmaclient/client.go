// Package pmaclient provides a client for the orchestrator to hand off chat completion requests to cynode-pma.
// See docs/tech_specs/openai_compatible_chat_api.md (routing path) and cynode_pma.md.
package pmaclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 120 * time.Second

// ChatMessage is a single message in the handoff request.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionRequest is the body sent to cynode-pma internal chat completion endpoint.
type CompletionRequest struct {
	Messages []ChatMessage `json:"messages"`
}

// CompletionResponse is the response from cynode-pma.
type CompletionResponse struct {
	Content string `json:"content"`
}

type managedProxyRequest struct {
	Version int                 `json:"version"`
	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Headers map[string][]string `json:"headers,omitempty"`
	BodyB64 string              `json:"body_b64,omitempty"`
}

type managedProxyResponse struct {
	Version int                 `json:"version"`
	Status  int                 `json:"status"`
	Headers map[string][]string `json:"headers,omitempty"`
	BodyB64 string              `json:"body_b64,omitempty"`
}

// CallChatCompletion sends the sanitized messages to cynode-pma and returns the completion content.
// baseURL is the PMA base URL (e.g. http://localhost:8090). Returns (content, nil) or ("", error).
func CallChatCompletion(ctx context.Context, client *http.Client, baseURL string, messages []ChatMessage) (string, error) {
	if baseURL == "" {
		return "", fmt.Errorf("PMA base URL is required")
	}
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	baseURL = strings.TrimSpace(baseURL)
	body := CompletionRequest{Messages: messages}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	if looksLikeManagedProxyEndpoint(baseURL) {
		return callViaManagedProxy(ctx, client, baseURL, b)
	}
	url := strings.TrimSuffix(baseURL, "/") + "/internal/chat/completion"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PMA chat completion returned %s", resp.Status)
	}
	var out CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Content, nil
}

func looksLikeManagedProxyEndpoint(baseURL string) bool {
	trimmed := strings.TrimSuffix(baseURL, "/")
	return strings.Contains(trimmed, "/v1/worker/managed-services/") &&
		strings.HasSuffix(trimmed, "/proxy:http")
}

func callViaManagedProxy(ctx context.Context, client *http.Client, proxyURL string, handoffBody []byte) (string, error) {
	reqBody := managedProxyRequest{
		Version: 1,
		Method:  http.MethodPost,
		Path:    "/internal/chat/completion",
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
		BodyB64: base64.StdEncoding.EncodeToString(handoffBody),
	}
	rawReq, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, proxyURL, bytes.NewReader(rawReq))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("PMA proxy call returned %s", resp.Status)
	}
	var proxyResp managedProxyResponse
	if err := json.NewDecoder(resp.Body).Decode(&proxyResp); err != nil {
		return "", err
	}
	if proxyResp.Status != http.StatusOK {
		return "", fmt.Errorf("PMA proxy upstream returned %d", proxyResp.Status)
	}
	rawCompletion, err := base64.StdEncoding.DecodeString(proxyResp.BodyB64)
	if err != nil {
		return "", err
	}
	var out CompletionResponse
	if err := json.Unmarshal(rawCompletion, &out); err != nil {
		return "", err
	}
	return out.Content, nil
}
