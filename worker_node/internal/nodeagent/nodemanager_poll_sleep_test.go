package nodeagent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func TestDoAgentTokenRefRequest_OK(t *testing.T) {
	expAt := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"agent_token":            "tok",
			"agent_token_expires_at": expAt,
		})
	}))
	defer srv.Close()
	cfg := &Config{NodeSlug: "n1", HTTPTimeout: 5 * time.Second}
	svc := &nodepayloads.ConfigManagedService{ServiceID: "s1", ServiceType: serviceTypePMA}
	tok, gotExp, err := doAgentTokenRefRequest(t.Context(), cfg, svc, srv.URL)
	if err != nil || tok != "tok" || gotExp != expAt {
		t.Fatalf("got %q %q err %v", tok, gotExp, err)
	}
}

func TestDoAgentTokenRefRequest_Non2XX(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	cfg := &Config{NodeSlug: "n1", HTTPTimeout: 2 * time.Second}
	svc := &nodepayloads.ConfigManagedService{ServiceID: "s1", ServiceType: serviceTypePMA}
	_, _, err := doAgentTokenRefRequest(t.Context(), cfg, svc, srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDoAgentTokenRefRequest_InvalidBody(t *testing.T) {
	doAgentTokenRefExpectError(t, "not-json", "expected decode error")
}

func TestDoAgentTokenRefRequest_EmptyToken(t *testing.T) {
	doAgentTokenRefExpectError(t, `{"agent_token":"  "}`, "expected error")
}

func TestDoAgentTokenRefRequest_InvalidExpiresAt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"agent_token":            "tok",
			"agent_token_expires_at": "not-rfc3339",
		})
	}))
	defer srv.Close()
	cfg := &Config{NodeSlug: "n1", HTTPTimeout: 2 * time.Second}
	svc := &nodepayloads.ConfigManagedService{ServiceID: "s1", ServiceType: serviceTypePMA}
	_, _, err := doAgentTokenRefRequest(t.Context(), cfg, svc, srv.URL)
	if err == nil {
		t.Fatal("expected error")
	}
}
