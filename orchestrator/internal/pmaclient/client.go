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
	"unicode/utf8"
)

// defaultPMAHTTPTimeout caps PMA HTTP client calls (stream and non-stream). Kept in sync with
// gateway chatCompletionTimeout so the gateway context cancels before the client hard-timeout.
const defaultPMAHTTPTimeout = 300 * time.Second

// maxHTTPErrorBodyBytes limits how much of a non-OK response body is attached to errors (debugging).
const maxHTTPErrorBodyBytes = 512

// httpErrWithBody returns an error including status and a one-line, truncated body snippet when present.
func httpErrWithBody(prefix, status string, body io.Reader) error {
	snip := readHTTPErrorSnippet(body)
	if snip == "" {
		return fmt.Errorf("%s returned %s", prefix, status)
	}
	return fmt.Errorf("%s returned %s: %s", prefix, status, snip)
}

func readHTTPErrorSnippet(r io.Reader) string {
	if r == nil {
		return ""
	}
	b, err := io.ReadAll(io.LimitReader(r, maxHTTPErrorBodyBytes))
	if err != nil || len(b) == 0 {
		return ""
	}
	if !utf8.Valid(b) {
		return ""
	}
	s := strings.TrimSpace(string(b))
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t':
			return ' '
		}
		if r < 32 {
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	const maxRunes = 400
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	rs := []rune(s)
	return string(rs[:maxRunes]) + "…"
}

// streamHTTPClient returns the client for PMA NDJSON streaming. Uses a 300s wall timeout so
// long agent runs are allowed while still bounding hung connections; caller context may cancel earlier.
func streamHTTPClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: defaultPMAHTTPTimeout}
}

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
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"`
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
		client = &http.Client{Timeout: defaultPMAHTTPTimeout}
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
		return "", httpErrWithBody("PMA chat completion", resp.Status, resp.Body)
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
	OnThinking       func(string) error
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
	client = streamHTTPClient(client)
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
		return httpErrWithBody("PMA chat completion", resp.Status, resp.Body)
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

func ndjsonIntField(raw map[string]json.RawMessage, key string, fn func(int) error) error {
	n, ok := raw[key]
	if !ok || fn == nil {
		return nil
	}
	var v int
	if json.Unmarshal(n, &v) != nil {
		return nil
	}
	return fn(v)
}

func ndjsonStringField(raw map[string]json.RawMessage, key string, fn func(string) error) error {
	d, ok := raw[key]
	if !ok || fn == nil {
		return nil
	}
	var s string
	if json.Unmarshal(d, &s) != nil || s == "" {
		return nil
	}
	return fn(s)
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
	if err := ndjsonIntField(raw, "iteration_start", func(iter int) error {
		if cb.OnIterationStart == nil {
			return nil
		}
		return cb.OnIterationStart(iter)
	}); err != nil {
		return err
	}
	if err := ndjsonStringField(raw, "delta", func(s string) error {
		if cb.OnDelta == nil {
			return nil
		}
		return cb.OnDelta(s)
	}); err != nil {
		return err
	}
	return ndjsonStringField(raw, "thinking", func(s string) error {
		if cb.OnThinking == nil {
			return nil
		}
		return cb.OnThinking(s)
	})
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
		return httpErrWithBody("PMA proxy stream", resp.Status, resp.Body)
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
		return "", httpErrWithBody("PMA proxy call", resp.Status, resp.Body)
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
