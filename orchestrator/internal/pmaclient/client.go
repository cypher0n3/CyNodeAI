// Package pmaclient provides a client for the orchestrator to hand off chat completion requests to cynode-pma.
// See docs/tech_specs/openai_compatible_chat_api.md (routing path) and cynode_pma.md.
package pmaclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	Stream   bool          `json:"stream,omitempty"`
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
// baseURL is the PMA base URL or the worker proxy URL (e.g. .../proxy:http).
// workerBearerToken is required when baseURL is a managed proxy URL so the worker-api accepts the request.
func CallChatCompletion(ctx context.Context, client *http.Client, baseURL string, messages []ChatMessage, workerBearerToken string) (string, error) {
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
		return callViaManagedProxy(ctx, client, baseURL, b, workerBearerToken)
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

// PMAStreamCallbacks are invoked for each NDJSON line from PMA stream. OnDelta is required; others are optional.
type PMAStreamCallbacks struct {
	OnDelta          func(string) error
	OnIterationStart func(iteration int) error
}

// CallChatCompletionStream streams completion from PMA; onDelta is called for each token.
// Supports both direct PMA URLs and managed proxy URLs (worker streams upstream response when request has stream: true).
func CallChatCompletionStream(ctx context.Context, client *http.Client, baseURL string, messages []ChatMessage, workerBearerToken string, onDelta func(string) error) error {
	cb := PMAStreamCallbacks{OnDelta: onDelta}
	return CallChatCompletionStreamWithCallbacks(ctx, client, baseURL, messages, workerBearerToken, cb)
}

// CallChatCompletionStreamWithCallbacks streams completion from PMA and invokes callbacks for each NDJSON event (delta, iteration_start, etc.).
func CallChatCompletionStreamWithCallbacks(ctx context.Context, client *http.Client, baseURL string, messages []ChatMessage, workerBearerToken string, cb PMAStreamCallbacks) error {
	if baseURL == "" {
		return fmt.Errorf("PMA base URL is required")
	}
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	baseURL = strings.TrimSpace(baseURL)
	body := CompletionRequest{Messages: messages, Stream: true}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	if looksLikeManagedProxyEndpoint(baseURL) {
		return callViaManagedProxyStreamWithCallbacks(ctx, client, baseURL, b, workerBearerToken, cb)
	}
	url := strings.TrimSuffix(baseURL, "/") + "/internal/chat/completion"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("PMA chat completion returned %s", resp.Status)
	}
	return readNDJSONStreamWithCallbacks(ctx, resp.Body, resp.Header.Get("Content-Type"), cb)
}

func readNDJSONStream(ctx context.Context, body io.Reader, contentType string, onDelta func(string) error) error {
	return readNDJSONStreamWithCallbacks(ctx, body, contentType, PMAStreamCallbacks{OnDelta: onDelta})
}

func readNDJSONStreamWithCallbacks(ctx context.Context, body io.Reader, contentType string, cb PMAStreamCallbacks) error {
	if contentType != "" && !strings.Contains(contentType, "application/x-ndjson") && !strings.Contains(contentType, "application/json") {
		bodyBytes, _ := io.ReadAll(body)
		var single CompletionResponse
		if json.Unmarshal(bodyBytes, &single) == nil && single.Content != "" && cb.OnDelta != nil {
			return cb.OnDelta(single.Content)
		}
		return fmt.Errorf("unexpected PMA response content-type: %s", contentType)
	}
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := processNDJSONLine(scanner.Bytes(), cb); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func processNDJSONLine(line []byte, cb PMAStreamCallbacks) error {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return nil
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(line, &raw) != nil {
		return nil
	}
	if n, ok := raw["iteration_start"]; ok && cb.OnIterationStart != nil {
		var iter int
		if json.Unmarshal(n, &iter) == nil {
			if err := cb.OnIterationStart(iter); err != nil {
				return err
			}
		}
	}
	if d, ok := raw["delta"]; ok && cb.OnDelta != nil {
		var s string
		if json.Unmarshal(d, &s) == nil && s != "" {
			return cb.OnDelta(s)
		}
	}
	// "done" and other keys are ignored; stream continues until body closes
	return nil
}

func callViaManagedProxyStreamWithCallbacks(ctx context.Context, client *http.Client, proxyURL string, handoffBody []byte, workerBearerToken string, cb PMAStreamCallbacks) error {
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
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, proxyURL, bytes.NewReader(rawReq))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if workerBearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+workerBearerToken)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("PMA proxy stream returned %s", resp.Status)
	}
	return readNDJSONStreamWithCallbacks(ctx, resp.Body, resp.Header.Get("Content-Type"), cb)
}

func looksLikeManagedProxyEndpoint(baseURL string) bool {
	return LooksLikeManagedProxyEndpoint(baseURL)
}

// LooksLikeManagedProxyEndpoint returns true when baseURL is a worker managed proxy URL.
func LooksLikeManagedProxyEndpoint(baseURL string) bool {
	trimmed := strings.TrimSuffix(baseURL, "/")
	return strings.Contains(trimmed, "/v1/worker/managed-services/") &&
		strings.HasSuffix(trimmed, "/proxy:http")
}

func callViaManagedProxy(ctx context.Context, client *http.Client, proxyURL string, handoffBody []byte, workerBearerToken string) (string, error) {
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
	if workerBearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+workerBearerToken)
	}
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
