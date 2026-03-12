// Package readiness provides shared orchestrator readiness checks.
// See docs/tech_specs/orchestrator.md (CYNAI.ORCHES.Rule.HealthEndpoints).
package readiness

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
)

// InferencePathAvailable returns true when at least one inference path exists:
// a dispatchable node or an active external API credential.
func InferencePathAvailable(ctx context.Context, store database.Store) (bool, error) {
	nodes, err := store.ListDispatchableNodes(ctx)
	if err != nil {
		return false, err
	}
	if len(nodes) > 0 {
		return true, nil
	}
	hasCred, err := store.HasAnyActiveApiCredential(ctx)
	if err != nil {
		return false, err
	}
	return hasCred, nil
}

// HasWorkerReportedPMAReady returns true when at least one dispatchable node has
// reported a managed PMA service in the "ready" state via its capability snapshot.
func HasWorkerReportedPMAReady(ctx context.Context, store database.Store) bool {
	nodes, err := store.ListDispatchableNodes(ctx)
	if err != nil || len(nodes) == 0 {
		return false
	}
	for _, n := range nodes {
		snap, err := store.GetLatestNodeCapabilitySnapshot(ctx, n.ID)
		if err != nil || snap == "" {
			continue
		}
		var report nodepayloads.CapabilityReport
		if json.Unmarshal([]byte(snap), &report) != nil || report.ManagedServicesStatus == nil {
			continue
		}
		for i := range report.ManagedServicesStatus.Services {
			svc := &report.ManagedServicesStatus.Services[i]
			if strings.EqualFold(strings.TrimSpace(svc.ServiceType), "pma") &&
				strings.TrimSpace(svc.State) == "ready" &&
				len(svc.Endpoints) > 0 {
				return true
			}
		}
	}
	return false
}

// PMASubprocessReady checks whether a local PMA subprocess is reachable at listenAddr.
func PMASubprocessReady(ctx context.Context, listenAddr string) bool {
	idx := strings.LastIndex(listenAddr, ":")
	if idx < 0 {
		return false
	}
	port := listenAddr[idx+1:]
	if port == "" {
		return false
	}
	url := "http://127.0.0.1:" + port + "/healthz"
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
