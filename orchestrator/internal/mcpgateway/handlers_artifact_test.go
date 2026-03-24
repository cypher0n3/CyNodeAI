package mcpgateway

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestToolCallHandler_ArtifactGet_ServiceUnavailableOrNotFound(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"service_unavailable", "out/file.txt"},
		{"not_found", "missing/path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := testutil.NewMockDB()
			user, _ := mock.CreateUser(context.Background(), "artuser-"+tc.name, nil)
			mock.AddUser(user)
			body := `{"tool_name":"artifact.get","arguments":{"user_id":"` + user.ID.String() + `","scope":"user","path":"` + tc.path + `"}}`
			callToolHandlerWithStore(t, mock, body, http.StatusServiceUnavailable)
		})
	}
}

func TestToolCallHandler_ArtifactGet_BadArgs(t *testing.T) {
	callToolHandlerPOST(t, `{"tool_name":"artifact.get","arguments":{"task_id":"`+uuid.New().String()+`"}}`, http.StatusBadRequest)
	callToolHandlerPOST(t, `{"tool_name":"artifact.get","arguments":{"path":"x"}}`, http.StatusBadRequest)
}
