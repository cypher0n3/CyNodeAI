package nodeagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func newTestServerNodeConfigPayload(t *testing.T, configVersion string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(nodepayloads.NodeConfigurationPayload{
			Version:       1,
			ConfigVersion: configVersion,
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestServerJSONFixedBody(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func doAgentTokenRefExpectError(t *testing.T, responseBody, failMsg string) {
	t.Helper()
	srv := newTestServerJSONFixedBody(t, responseBody)
	cfg := &Config{NodeSlug: "n1", HTTPTimeout: 2 * time.Second}
	svc := &nodepayloads.ConfigManagedService{ServiceID: "s1", ServiceType: serviceTypePMA}
	_, _, err := doAgentTokenRefRequest(context.Background(), cfg, svc, srv.URL)
	if err == nil {
		t.Fatal(failMsg)
	}
}
