package mcpgateway

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/httplimits"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestMaxBytes_OversizeMCPBodyRejected(t *testing.T) {
	h := ToolCallHandler(testutil.NewMockDB(), slog.Default(), nil)
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	pad := strings.Repeat("x", int(httplimits.DefaultMaxAPIRequestBodyBytes)+1024)
	body := `{"tool_name":"help.list","arguments":{"p":"` + pad + `"}}`
	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
}
