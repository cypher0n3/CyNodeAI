package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/problem"
	"github.com/cypher0n3/cynodeai/go_shared_libs/httplimits"
)

func decodeResponseJSON(resp *http.Response, out any) error {
	return json.NewDecoder(io.LimitReader(resp.Body, httplimits.DefaultMaxHTTPResponseBytes)).Decode(out)
}

func (c *Client) doRequest(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Response, error) {
	baseStr, tok := c.readURLAndToken()
	base, err := url.Parse(baseStr)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u, err := base.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return c.HTTPClient.Do(req)
}

func (c *Client) doPostJSON(ctx context.Context, path string, reqBody any, wantStatus int, out any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	resp, err := c.doRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != wantStatus {
		return c.parseError(resp)
	}
	if err := decodeResponseJSON(resp, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// doPostJSONNoAuth posts JSON without Authorization header (e.g. refresh).
func (c *Client) doPostJSONNoAuth(ctx context.Context, path string, reqBody any, wantStatus int, out any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	baseStr, _ := c.readURLAndToken()
	base, err := url.Parse(baseStr)
	if err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	u, err := base.Parse(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != wantStatus {
		return c.parseError(resp)
	}
	if err := decodeResponseJSON(resp, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// doGetJSON performs authenticated GET and decodes JSON into out.
func (c *Client) doGetJSON(ctx context.Context, path string, out any) error {
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return c.parseError(resp)
	}
	if err := decodeResponseJSON(resp, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// HTTPError carries the HTTP status for exit-code mapping (401->3, 404->4, etc.).
type HTTPError struct {
	Status int
	Err    error
}

func (e *HTTPError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("HTTP %d", e.Status)
}

func (e *HTTPError) Unwrap() error { return e.Err }

// IsUnauthorized reports whether err is or wraps an *HTTPError with HTTP 401.
// Used by the TUI to open in-session login recovery (REQ-CLIENT-0190).
func IsUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	var he *HTTPError
	return errors.As(err, &he) && he.Status == http.StatusUnauthorized
}

func (c *Client) parseError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var p problem.Details
	if len(body) > 0 {
		_ = json.Unmarshal(body, &p)
	}
	var msg string
	if p.Detail != "" {
		msg = fmt.Sprintf("%s: %s", resp.Status, p.Detail)
	} else {
		msg = resp.Status
	}
	return &HTTPError{Status: resp.StatusCode, Err: fmt.Errorf("%s", msg)}
}
