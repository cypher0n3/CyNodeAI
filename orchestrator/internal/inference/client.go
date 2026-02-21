// Package inference provides a client for calling the PM model (Ollama /api/generate).
// Used by the orchestrator so prompt-mode tasks get the prompt to the model and MUST work (MVP Phase 1).
package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const defaultTimeout = 120 * time.Second

// GenerateRequest is the request body for Ollama /api/generate.
type GenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// GenerateChunk is one chunk of the response (single JSON object or NDJSON line).
type GenerateChunk struct {
	Response string `json:"response"`
	Error    string `json:"error"`
	Done     bool   `json:"done"`
}

// CallGenerate calls baseURL/api/generate with the given model and prompt.
// Handles both single-JSON and NDJSON (streamed) responses. Returns the concatenated response or an error message.
func CallGenerate(ctx context.Context, client *http.Client, baseURL, model, prompt string) (string, error) {
	if baseURL == "" {
		return "", fmt.Errorf("inference base URL is required")
	}
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	url := strings.TrimSuffix(baseURL, "/") + "/api/generate"
	body := GenerateRequest{Model: model, Prompt: prompt, Stream: false}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
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
		return "", fmt.Errorf("inference API returned %s", resp.Status)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return "", err
	}
	return parseGenerateResponse(buf.String())
}

func parseGenerateResponse(raw string) (string, error) {
	var chunk GenerateChunk
	if err := json.Unmarshal([]byte(raw), &chunk); err == nil {
		if chunk.Error != "" {
			return "", fmt.Errorf("inference error: %s", chunk.Error)
		}
		return chunk.Response, nil
	}
	return parseGenerateResponseNDJSON(raw)
}

func parseGenerateResponseNDJSON(raw string) (string, error) {
	var out, errMsg strings.Builder
	var chunk GenerateChunk
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if chunk.Error != "" {
			errMsg.WriteString(chunk.Error)
		}
		out.WriteString(chunk.Response)
	}
	if errMsg.Len() > 0 {
		return "", fmt.Errorf("inference error: %s", errMsg.String())
	}
	return out.String(), nil
}
