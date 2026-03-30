package sba

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
)

type countCloseBody struct {
	io.ReadCloser
	n *atomic.Int32
}

func (c *countCloseBody) Close() error {
	c.n.Add(1)
	return c.ReadCloser.Close()
}

type closeCountingTransport struct {
	base http.RoundTripper
	n    *atomic.Int32
}

func (t *closeCountingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.Body != nil {
		resp.Body = &countCloseBody{ReadCloser: resp.Body, n: t.n}
	}
	return resp, nil
}

// TestLifecycleBodyClose asserts lifecycle HTTP responses have their bodies closed (connection hygiene).
func TestLifecycleBodyClose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	var closes atomic.Int32
	lc := &LifecycleClient{
		BaseURL: srv.URL,
		HTTPClient: &http.Client{
			Transport: &closeCountingTransport{base: http.DefaultTransport, n: &closes},
		},
	}
	ctx := context.Background()
	lc.NotifyInProgress(ctx)
	lc.NotifyCompletion(ctx, &sbajob.Result{JobID: "j1", Status: "success"})
	if got := closes.Load(); got != 2 {
		t.Fatalf("response body Close calls = %d, want 2", got)
	}
}

func TestLifecycleClient_NoURL_NoOp(t *testing.T) {
	lc := &LifecycleClient{BaseURL: ""}
	lc.NotifyInProgress(context.Background())
	lc.NotifyCompletion(context.Background(), &sbajob.Result{Status: "success"})
}

func TestLifecycleClient_NotifyCompletion_NilResult_NoOp(t *testing.T) {
	lc := &LifecycleClient{BaseURL: "http://localhost"}
	lc.NotifyCompletion(context.Background(), nil)
}

func TestLifecycleClient_CallbackURLFallback(t *testing.T) {
	t.Setenv("SBA_JOB_STATUS_URL", "")
	t.Setenv("SBA_CALLBACK_URL", "http://callback.example")
	lc := NewLifecycleClient()
	if lc.BaseURL != "http://callback.example" {
		t.Errorf("BaseURL = %q (expected from SBA_CALLBACK_URL)", lc.BaseURL)
	}
}

func TestLifecycleClient_WithURL_PostsInProgressAndCompletion(t *testing.T) {
	var statuses []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Status string `json:"status"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Status != "" {
			statuses = append(statuses, body.Status)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("SBA_JOB_STATUS_URL", srv.URL)
	lc := NewLifecycleClient()
	if lc.BaseURL != srv.URL {
		t.Fatalf("BaseURL = %q", lc.BaseURL)
	}
	ctx := context.Background()
	lc.NotifyInProgress(ctx)
	lc.NotifyCompletion(ctx, &sbajob.Result{JobID: "j1", Status: "success"})
	if len(statuses) != 2 || statuses[0] != "in_progress" || statuses[1] != "completed" {
		t.Errorf("statuses = %v", statuses)
	}
}
