package workerapiserver

import (
	"testing"
)

func TestLoadProxyConfigFromEnv_MCPToolsURLPrecedence(t *testing.T) {
	tests := []struct {
		name         string
		toolsURL     string
		gatewayURL   string
		wantToolsURL string
	}{
		{
			name:         "explicit overrides deprecated gateway",
			toolsURL:     "http://explicit:1",
			gatewayURL:   "http://old:2",
			wantToolsURL: "http://explicit:1",
		},
		{
			name:         "deprecated gateway alias when tools empty",
			toolsURL:     "",
			gatewayURL:   "http://legacy-alias:3",
			wantToolsURL: "http://legacy-alias:3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ORCHESTRATOR_URL", "http://localhost:12082")
			t.Setenv("ORCHESTRATOR_MCP_TOOLS_BASE_URL", tt.toolsURL)
			t.Setenv("ORCHESTRATOR_MCP_GATEWAY_BASE_URL", tt.gatewayURL)
			t.Setenv("ORCHESTRATOR_INTERNAL_PROXY_BASE_URL", "")

			cfg, err := loadProxyConfigFromEnv(t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.InternalProxy.MCPToolsBaseURL; got != tt.wantToolsURL {
				t.Fatalf("MCPToolsBaseURL = %q, want %q", got, tt.wantToolsURL)
			}
		})
	}
}

func TestLoadProxyConfigFromEnv_DerivesFromOrchestratorURL_ControlPlane(t *testing.T) {
	t.Setenv("ORCHESTRATOR_URL", "http://cynodeai-control-plane:12082/")
	t.Setenv("ORCHESTRATOR_MCP_TOOLS_BASE_URL", "")
	t.Setenv("ORCHESTRATOR_MCP_GATEWAY_BASE_URL", "")
	t.Setenv("ORCHESTRATOR_INTERNAL_PROXY_BASE_URL", "")

	cfg, err := loadProxyConfigFromEnv(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "http://cynodeai-control-plane:12082"
	if got := cfg.InternalProxy.MCPToolsBaseURL; got != want {
		t.Fatalf("MCPToolsBaseURL = %q, want %q (must track control plane, not a separate :12083 gateway)", got, want)
	}
}
