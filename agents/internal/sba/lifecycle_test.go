package sba

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/sbajob"
)

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
