package nodeagent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func TestRefreshNodeConfig_FetchErrorKeepsCurrent(t *testing.T) {
	cur := &nodepayloads.NodeConfigurationPayload{Version: 1, ConfigVersion: "cur"}
	cfg := &Config{HTTPTimeout: 50 * time.Millisecond}
	bootstrap := &BootstrapData{
		NodeJWT:       "jwt",
		NodeConfigURL: "http://127.0.0.1:19998/no-listener",
	}
	got := refreshNodeConfig(t.Context(), nil, cfg, bootstrap, cur)
	if got != cur {
		t.Fatal("expected current on fetch error")
	}
}

func TestRefreshNodeConfig_NilCurrentReturnsFetched(t *testing.T) {
	srv := newTestServerNodeConfigPayload(t, "new")
	cfg := &Config{HTTPTimeout: 5 * time.Second}
	bootstrap := &BootstrapData{NodeJWT: "t", NodeConfigURL: srv.URL}
	got := refreshNodeConfig(t.Context(), nil, cfg, bootstrap, nil)
	if got == nil || got.ConfigVersion != "new" {
		t.Fatalf("got %+v", got)
	}
}

func TestRefreshNodeConfig_VersionBump(t *testing.T) {
	srv := newTestServerNodeConfigPayload(t, "v2")
	cur := &nodepayloads.NodeConfigurationPayload{Version: 1, ConfigVersion: "v1"}
	cfg := &Config{HTTPTimeout: 5 * time.Second}
	bootstrap := &BootstrapData{NodeJWT: "t", NodeConfigURL: srv.URL}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	got := refreshNodeConfig(t.Context(), log, cfg, bootstrap, cur)
	if got == nil || got.ConfigVersion != "v2" {
		t.Fatalf("got %+v", got)
	}
}

func TestRefreshNodeConfig_ManagedServicesChange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(nodepayloads.NodeConfigurationPayload{
			Version:       1,
			ConfigVersion: "same",
			ManagedServices: &nodepayloads.ConfigManagedServices{
				Services: []nodepayloads.ConfigManagedService{{ServiceID: "b"}},
			},
		})
	}))
	defer srv.Close()
	cur := &nodepayloads.NodeConfigurationPayload{
		Version:       1,
		ConfigVersion: "same",
		ManagedServices: &nodepayloads.ConfigManagedServices{
			Services: []nodepayloads.ConfigManagedService{{ServiceID: "a"}},
		},
	}
	cfg := &Config{HTTPTimeout: 5 * time.Second}
	bootstrap := &BootstrapData{NodeJWT: "t", NodeConfigURL: srv.URL}
	got := refreshNodeConfig(t.Context(), nil, cfg, bootstrap, cur)
	if got == nil || len(got.ManagedServices.Services) != 1 || got.ManagedServices.Services[0].ServiceID != "b" {
		t.Fatalf("got %+v", got)
	}
}

func TestManagedServicesConfigChanged_NilCases(t *testing.T) {
	if !managedServicesConfigChanged(nil, &nodepayloads.NodeConfigurationPayload{}) {
		t.Fatal("nil vs non-nil")
	}
	if managedServicesConfigChanged(&nodepayloads.NodeConfigurationPayload{}, &nodepayloads.NodeConfigurationPayload{}) {
		t.Fatal("both empty")
	}
}

func TestInferenceBackendPullSpecChanged_ModelDiffers(t *testing.T) {
	o := &nodepayloads.NodeConfigurationPayload{InferenceBackend: &nodepayloads.ConfigInferenceBackend{SelectedModel: "a"}}
	n := &nodepayloads.NodeConfigurationPayload{InferenceBackend: &nodepayloads.ConfigInferenceBackend{SelectedModel: "b"}}
	if !inferenceBackendPullSpecChanged(o, n) {
		t.Fatal()
	}
}

func TestReconcileManagedServices_NoOp(t *testing.T) {
	reconcileManagedServices(context.Background(), nil, nil, nil)
	reconcileManagedServices(context.Background(), nil, &nodepayloads.NodeConfigurationPayload{}, &RunOptions{})
}

func TestWaitForOrchestratorReadiness_ReadyzOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	t.Setenv("NODE_MANAGER_READINESS_TIMEOUT", "500ms")
	cfg := &Config{OrchestratorURL: srv.URL}
	if err := waitForOrchestratorReadiness(t.Context(), nil, cfg); err != nil {
		t.Fatal(err)
	}
}

func TestSendConfigAck_Success(t *testing.T) {
	var saw bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			saw = true
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	cfg := &Config{HTTPTimeout: 5 * time.Second}
	bootstrap := &BootstrapData{NodeJWT: "jwt", NodeConfigURL: srv.URL}
	nc := &nodepayloads.NodeConfigurationPayload{
		Version:       1,
		ConfigVersion: "v1",
		NodeSlug:      "n1",
	}
	if err := SendConfigAck(t.Context(), cfg, bootstrap, nc, "applied"); err != nil {
		t.Fatal(err)
	}
	if !saw {
		t.Fatal("expected POST")
	}
}

func TestSendConfigAck_NilNodeConfigRejected(t *testing.T) {
	if err := SendConfigAck(t.Context(), &Config{}, &BootstrapData{}, nil, "applied"); err == nil {
		t.Fatal("expected error")
	}
}

func TestReconcileManagedServices_EmptySlice(t *testing.T) {
	var called bool
	opts := &RunOptions{
		StartManagedServices: func(_ context.Context, svcs []nodepayloads.ConfigManagedService) error {
			called = true
			if len(svcs) != 0 {
				t.Fatalf("want empty slice, got %d", len(svcs))
			}
			return nil
		},
	}
	cfg := &nodepayloads.NodeConfigurationPayload{
		ManagedServices: &nodepayloads.ConfigManagedServices{Services: []nodepayloads.ConfigManagedService{}},
	}
	reconcileManagedServices(context.Background(), slog.Default(), cfg, opts)
	if !called {
		t.Fatal("expected StartManagedServices")
	}
}
