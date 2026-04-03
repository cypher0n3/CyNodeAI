package nodeagent

import (
	"os"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func TestApplyWorkerProxyConfigEnv_SetsEnv(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Unsetenv("WORKER_NODE_CONFIG_JSON")
		_ = os.Unsetenv("ORCHESTRATOR_INTERNAL_PROXY_BASE_URL")
		_ = os.Unsetenv("WORKER_MANAGED_SERVICE_TARGETS_JSON")
	})
	cfg := &nodepayloads.NodeConfigurationPayload{
		Version: 1,
		Orchestrator: nodepayloads.ConfigOrchestrator{
			BaseURL: "http://cp.example",
		},
		NodeSlug: "n1",
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{
				{
					ServiceID:   "pma-main",
					ServiceType: serviceTypePMA,
					Orchestrator: &nodepayloads.ConfigManagedServiceOrchestrator{
						AgentToken: "secret-token",
					},
				},
			},
		},
	}
	applyWorkerProxyConfigEnv(cfg)
	if os.Getenv("ORCHESTRATOR_INTERNAL_PROXY_BASE_URL") != "http://cp.example" {
		t.Fatalf("ORCHESTRATOR_INTERNAL_PROXY_BASE_URL not set")
	}
	if v := os.Getenv("WORKER_NODE_CONFIG_JSON"); v == "" || strings.Contains(v, "secret-token") {
		t.Fatalf("expected sanitized JSON without raw agent token")
	}
}
