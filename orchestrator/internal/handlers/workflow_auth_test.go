package handlers_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/handlers"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/middleware"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

// TestWorkflowAuth verifies POST workflow handlers wrapped with RequireWorkflowRunnerAuth return 401 without a valid bearer token (Task 5 / REQ-ORCHES-0144).
func TestWorkflowAuth(t *testing.T) {
	db := testutil.NewMockDB()
	h := handlers.NewWorkflowHandler(db, slog.Default())
	auth := middleware.RequireWorkflowRunnerAuth("workflow-runner-token")

	cases := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
		body string
	}{
		{"start", h.Start, `{"task_id":"00000000-0000-0000-0000-000000000001","holder_id":"h"}`},
		{"resume", h.Resume, `{"task_id":"00000000-0000-0000-0000-000000000001"}`},
		{"checkpoint", h.SaveCheckpoint, `{"task_id":"00000000-0000-0000-0000-000000000001"}`},
		{"release", h.Release, `{"task_id":"00000000-0000-0000-0000-000000000001","lease_id":"00000000-0000-0000-0000-000000000002"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := auth(http.HandlerFunc(tc.fn))
			req := httptest.NewRequest(http.MethodPost, "/v1/workflow/"+tc.name, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			wrapped.ServeHTTP(rr, req)
			if rr.Code != http.StatusUnauthorized {
				t.Fatalf("without bearer: status=%d body=%q", rr.Code, rr.Body.String())
			}
		})
	}
}
