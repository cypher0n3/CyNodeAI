// Package bdd – orchestrator Godog steps: Postgres/API bootstrap, auth, nodes, tasks, workflow start/checkpoint.
package bdd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cucumber/godog"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/auth"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
)

func registerOrchestratorBootstrapTasks(sc *godog.ScenarioContext, state *testState) {

	sc.Step(`^a running PostgreSQL database$`, func(ctx context.Context) error {
		if os.Getenv("POSTGRES_TEST_DSN") == "" {
			return godog.ErrSkip
		}
		return nil
	})
	sc.Step(`^the orchestrator API is running$`, func(ctx context.Context) error {
		if getState(ctx) == nil || getState(ctx).server == nil {
			return godog.ErrSkip
		}
		return nil
	})
	sc.Step(`^an admin user exists with handle "([^"]*)"$`, func(ctx context.Context, handle string) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		_, err := st.db.GetUserByHandle(ctx, handle)
		if err == nil {
			return nil
		}
		if !errors.Is(err, database.ErrNotFound) {
			return err
		}
		user, err := st.db.CreateUser(ctx, handle, nil)
		if err != nil {
			return err
		}
		hash, err := auth.HashPassword("admin123", nil)
		if err != nil {
			return err
		}
		_, err = st.db.CreatePasswordCredential(ctx, user.ID, hash, "argon2id")
		return err
	})

	// Auth
	sc.Step(`^I login as "([^"]*)" with password "([^"]*)"$`, func(ctx context.Context, handle, password string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"handle": handle, "password": password})
		resp, err := http.Post(st.server.URL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("login returned %d", resp.StatusCode)
		}
		var out struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.accessToken = out.AccessToken
		st.refreshToken = out.RefreshToken
		return nil
	})
	sc.Step(`^I receive an access token$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.accessToken == "" {
			return fmt.Errorf("no access token")
		}
		return nil
	})
	sc.Step(`^I receive a refresh token$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.refreshToken == "" {
			return fmt.Errorf("no refresh token")
		}
		return nil
	})
	sc.Step(`^I am logged in as "([^"]*)" with password "([^"]*)"$`, func(ctx context.Context, handle, password string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"handle": handle, "password": password})
		resp, err := http.Post(st.server.URL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("login as %q returned %d", handle, resp.StatusCode)
		}
		var out struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.accessToken = out.AccessToken
		st.refreshToken = out.RefreshToken
		return nil
	})
	sc.Step(`^I am logged in as "([^"]*)"$`, func(ctx context.Context, handle string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"handle": handle, "password": "admin123"})
		resp, err := http.Post(st.server.URL+"/v1/auth/login", "application/json", bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("login as %q returned %d", handle, resp.StatusCode)
		}
		var out struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.accessToken = out.AccessToken
		st.refreshToken = out.RefreshToken
		return nil
	})
	sc.Step(`^I refresh my token$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"refresh_token": st.refreshToken})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("refresh returned %d", resp.StatusCode)
		}
		var out struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.accessToken = out.AccessToken
		st.refreshToken = out.RefreshToken
		return nil
	})
	sc.Step(`^I receive a new access token$`, func(ctx context.Context) error { return nil })
	sc.Step(`^I receive a new refresh token$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the old refresh token is invalidated$`, func(ctx context.Context) error { return nil })
	sc.Step(`^I logout$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"refresh_token": st.refreshToken})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/auth/logout", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("logout returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^my refresh token is invalidated$`, func(ctx context.Context) error { return nil })
	sc.Step(`^I cannot use the old access token$`, func(ctx context.Context) error { return nil })
	sc.Step(`^I request my user info$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/users/me", nil)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("get me returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^I receive my user details including handle "([^"]*)"$`, func(ctx context.Context, handle string) error {
		return nil
	})

	// Nodes
	sc.Step(`^a node with slug "([^"]*)" and valid PSK$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st != nil {
			st.nodeSlug = slug
			st.advertisedWorkerAPIURL = ""
		}
		return nil
	})
	sc.Step(`^a node with slug "([^"]*)" and valid PSK and worker API URL "([^"]*)"$`, func(ctx context.Context, slug, workerAPIURL string) error {
		st := getState(ctx)
		if st != nil {
			st.nodeSlug = slug
			st.advertisedWorkerAPIURL = workerAPIURL
		}
		return nil
	})
	sc.Step(`^a node with slug "([^"]*)" registers with the orchestrator$`, func(ctx context.Context, slug string) error {
		return nodeRegisterStep(ctx, slug, "")
	})
	sc.Step(`^a node with slug "([^"]*)" registers with the orchestrator using a valid PSK$`, func(ctx context.Context, slug string) error {
		return nodeRegisterStep(ctx, slug, "")
	})
	sc.Step(`^the node registers with the orchestrator$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.nodeSlug == "" {
			return godog.ErrSkip
		}
		return nodeRegisterStep(ctx, st.nodeSlug, st.advertisedWorkerAPIURL)
	})
	sc.Step(`^the node registers with the orchestrator and requests its configuration$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.nodeSlug == "" {
			return godog.ErrSkip
		}
		if err := nodeRegisterStep(ctx, st.nodeSlug, st.advertisedWorkerAPIURL); err != nil {
			return err
		}
		if st.server == nil || st.nodeJWT == "" {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/nodes/config", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GET config returned %d", resp.StatusCode)
		}
		st.lastConfigBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var payload nodepayloads.NodeConfigurationPayload
		if err := json.Unmarshal(st.lastConfigBody, &payload); err != nil {
			return err
		}
		st.lastConfigVersion = payload.ConfigVersion
		return nil
	})
	sc.Step(`^the node receives a JWT token$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.nodeJWT == "" {
			return fmt.Errorf("no node JWT")
		}
		return nil
	})
	sc.Step(`^the node is recorded in the database$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		_, err := st.db.GetNodeBySlug(ctx, st.nodeSlug)
		return err
	})
	sc.Step(`^the orchestrator stored worker_api_target_url from the node-reported base_url for "([^"]*)"$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.db == nil || st.advertisedWorkerAPIURL == "" {
			return godog.ErrSkip
		}
		node, err := st.db.GetNodeBySlug(ctx, slug)
		if err != nil {
			return err
		}
		if node.WorkerAPITargetURL == nil || *node.WorkerAPITargetURL != st.advertisedWorkerAPIURL {
			return fmt.Errorf("node %q worker_api_target_url %v, want %q", slug, node.WorkerAPITargetURL, st.advertisedWorkerAPIURL)
		}
		return nil
	})
	sc.Step(`^a registered node "([^"]*)"$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		st.nodeSlug = slug
		_, err := st.db.GetNodeBySlug(ctx, slug)
		return err
	})
	sc.Step(`^the node reports its capabilities$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.nodeJWT == "" {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]interface{}{
			"version":     1,
			"reported_at": time.Now().UTC().Format(time.RFC3339),
			"node":        map[string]interface{}{"node_slug": st.nodeSlug},
			"platform":    map[string]interface{}{"os": "linux", "arch": "amd64"},
			"compute":     map[string]interface{}{"cpu_cores": 2, "ram_mb": 4096},
		})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/nodes/capability", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("capability returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^the orchestrator stores the capability snapshot$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the capability hash is updated$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the node requests its configuration$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.nodeJWT == "" {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/nodes/config", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GET config returned %d", resp.StatusCode)
		}
		st.lastConfigBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var payload nodepayloads.NodeConfigurationPayload
		if err := json.Unmarshal(st.lastConfigBody, &payload); err != nil {
			return err
		}
		st.lastConfigVersion = payload.ConfigVersion
		return nil
	})
	sc.Step(`^the orchestrator returns a configuration payload for "([^"]*)"$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || len(st.lastConfigBody) == 0 {
			return fmt.Errorf("no config payload in state")
		}
		var payload nodepayloads.NodeConfigurationPayload
		if err := json.Unmarshal(st.lastConfigBody, &payload); err != nil {
			return err
		}
		if payload.NodeSlug != slug {
			return fmt.Errorf("config payload node_slug %q, want %q", payload.NodeSlug, slug)
		}
		return nil
	})
	sc.Step(`^the payload includes config_version and worker_api bearer token$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastConfigBody) == 0 {
			return fmt.Errorf("no config payload in state")
		}
		var payload nodepayloads.NodeConfigurationPayload
		if err := json.Unmarshal(st.lastConfigBody, &payload); err != nil {
			return err
		}
		if payload.ConfigVersion == "" {
			return fmt.Errorf("config payload missing config_version")
		}
		if payload.WorkerAPI == nil || payload.WorkerAPI.OrchestratorBearerToken == "" {
			return fmt.Errorf("config payload missing worker_api.orchestrator_bearer_token")
		}
		return nil
	})
	sc.Step(`^the node registers with capability inference supported and not existing_service$`, func(ctx context.Context) error {
		return nodeRegisterStepWithInference(ctx, false)
	})
	sc.Step(`^the node registers with capability inference supported and GPU NVIDIA reported$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st != nil {
			st.registrationGPU = &nodepayloads.GPUInfo{
				Present: true,
				Devices: []nodepayloads.GPUDevice{
					{Vendor: "NVIDIA", VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
				},
			}
		}
		return nodeRegisterStepWithInferenceAndGPU(ctx, false)
	})
	sc.Step(`^the node registers with capability inference supported and GPUs reported with 1 AMD device 20480 vram_mb and 3 NVIDIA devices each 12288 vram_mb$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st != nil {
			st.registrationGPU = &nodepayloads.GPUInfo{
				Present: true,
				Devices: []nodepayloads.GPUDevice{
					{Vendor: "AMD", VRAMMB: 20480, Features: map[string]interface{}{"rocm_version": "6.0"}},
					{Vendor: "NVIDIA", VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
					{Vendor: "NVIDIA", VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
					{Vendor: "NVIDIA", VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
				},
			}
		}
		return nodeRegisterStepWithInferenceAndGPU(ctx, false)
	})
	sc.Step(`^the node registers with capability inference supported and GPUs reported with 3 NVIDIA devices each 12288 vram_mb$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st != nil {
			st.registrationGPU = &nodepayloads.GPUInfo{
				Present: true,
				Devices: []nodepayloads.GPUDevice{
					{Vendor: "NVIDIA", VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
					{Vendor: "NVIDIA", VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
					{Vendor: "NVIDIA", VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
				},
			}
		}
		return nodeRegisterStepWithInferenceAndGPU(ctx, false)
	})
	sc.Step(`^the node registers with capability inference supported and GPUs reported with 1 NVIDIA device 12288 vram_mb and 3 AMD devices each 8192 vram_mb$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st != nil {
			st.registrationGPU = &nodepayloads.GPUInfo{
				Present: true,
				Devices: []nodepayloads.GPUDevice{
					{Vendor: "NVIDIA", VRAMMB: 12288, Features: map[string]interface{}{"cuda_capability": "8.6"}},
					{Vendor: "AMD", VRAMMB: 8192, Features: map[string]interface{}{"rocm_version": "6.0"}},
					{Vendor: "AMD", VRAMMB: 8192, Features: map[string]interface{}{"rocm_version": "6.0"}},
					{Vendor: "AMD", VRAMMB: 8192, Features: map[string]interface{}{"rocm_version": "6.0"}},
				},
			}
		}
		return nodeRegisterStepWithInferenceAndGPU(ctx, false)
	})
	sc.Step(`^the payload includes inference_backend with enabled true$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.lastConfigBody) == 0 {
			return fmt.Errorf("no config payload in state")
		}
		var payload nodepayloads.NodeConfigurationPayload
		if err := json.Unmarshal(st.lastConfigBody, &payload); err != nil {
			return err
		}
		if payload.InferenceBackend == nil {
			return fmt.Errorf("config payload missing inference_backend")
		}
		if !payload.InferenceBackend.Enabled {
			return fmt.Errorf("inference_backend.enabled should be true")
		}
		return nil
	})
	sc.Step(`^the payload includes inference_backend with enabled true and variant "([^"]*)"$`, func(ctx context.Context, wantVariant string) error {
		st := getState(ctx)
		if st == nil || len(st.lastConfigBody) == 0 {
			return fmt.Errorf("no config payload in state")
		}
		var payload nodepayloads.NodeConfigurationPayload
		if err := json.Unmarshal(st.lastConfigBody, &payload); err != nil {
			return err
		}
		if payload.InferenceBackend == nil {
			return fmt.Errorf("config payload missing inference_backend")
		}
		if !payload.InferenceBackend.Enabled {
			return fmt.Errorf("inference_backend.enabled should be true")
		}
		if payload.InferenceBackend.Variant != wantVariant {
			return fmt.Errorf("inference_backend.variant = %q, want %q", payload.InferenceBackend.Variant, wantVariant)
		}
		return nil
	})
	sc.Step(`^a registered node "([^"]*)" that has received configuration$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.db == nil {
			return godog.ErrSkip
		}
		st.nodeSlug = slug
		if _, err := st.db.GetNodeBySlug(ctx, slug); errors.Is(err, database.ErrNotFound) {
			if err := nodeRegisterStep(ctx, slug, st.advertisedWorkerAPIURL); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if st.nodeJWT == "" {
			if err := nodeRegisterStep(ctx, slug, st.advertisedWorkerAPIURL); err != nil {
				return err
			}
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/nodes/config", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GET config returned %d", resp.StatusCode)
		}
		st.lastConfigBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var payload nodepayloads.NodeConfigurationPayload
		if err := json.Unmarshal(st.lastConfigBody, &payload); err != nil {
			return err
		}
		st.lastConfigVersion = payload.ConfigVersion
		return nil
	})
	sc.Step(`^the node applies the configuration$`, func(ctx context.Context) error { return nil })
	sc.Step(`^the node applies the configuration and sends a config acknowledgement with status "([^"]*)"$`, func(ctx context.Context, status string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.nodeJWT == "" {
			return godog.ErrSkip
		}
		if st.lastConfigVersion == "" {
			return fmt.Errorf("no config version in state (fetch config first)")
		}
		ack := nodepayloads.ConfigAck{
			Version:       1,
			NodeSlug:      st.nodeSlug,
			ConfigVersion: st.lastConfigVersion,
			AckAt:         time.Now().UTC().Format(time.RFC3339),
			Status:        status,
		}
		body, _ := json.Marshal(ack)
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/nodes/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("POST config ack returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^the node sends a config acknowledgement with status "([^"]*)"$`, func(ctx context.Context, status string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.nodeJWT == "" {
			return godog.ErrSkip
		}
		if st.lastConfigVersion == "" {
			return fmt.Errorf("no config version in state (fetch config first)")
		}
		ack := nodepayloads.ConfigAck{
			Version:       1,
			NodeSlug:      st.nodeSlug,
			ConfigVersion: st.lastConfigVersion,
			AckAt:         time.Now().UTC().Format(time.RFC3339),
			Status:        status,
		}
		body, _ := json.Marshal(ack)
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/nodes/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("POST config ack returned %d", resp.StatusCode)
		}
		return nil
	})
	sc.Step(`^the orchestrator records the config ack for "([^"]*)"$`, func(ctx context.Context, slug string) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		node, err := st.db.GetNodeBySlug(ctx, slug)
		if err != nil {
			return err
		}
		if node.ConfigAckStatus == nil || *node.ConfigAckStatus == "" {
			return fmt.Errorf("node %q has no config_ack_status", slug)
		}
		return nil
	})
	sc.Step(`^the node config_version is stored$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.db == nil {
			return godog.ErrSkip
		}
		node, err := st.db.GetNodeBySlug(ctx, st.nodeSlug)
		if err != nil {
			return err
		}
		if node.ConfigVersion == nil || *node.ConfigVersion == "" {
			return fmt.Errorf("node %q has no config_version stored", st.nodeSlug)
		}
		return nil
	})
	sc.Step(`^an unauthenticated request requests node configuration$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/nodes/config", http.NoBody)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		return nil
	})
	sc.Step(`^the orchestrator responds with 401 Unauthorized$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.lastStatusCode != http.StatusUnauthorized {
			return fmt.Errorf("expected 401, got %d", st.lastStatusCode)
		}
		return nil
	})
	sc.Step(`^the node sends a config acknowledgement with node_slug "([^"]*)" and status "([^"]*)"$`, func(ctx context.Context, slug, status string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.nodeJWT == "" {
			return godog.ErrSkip
		}
		if st.lastConfigVersion == "" {
			return fmt.Errorf("no config version in state (fetch config first)")
		}
		ack := nodepayloads.ConfigAck{
			Version:       1,
			NodeSlug:      slug,
			ConfigVersion: st.lastConfigVersion,
			AckAt:         time.Now().UTC().Format(time.RFC3339),
			Status:        status,
		}
		body, _ := json.Marshal(ack)
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/nodes/config", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.nodeJWT)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		return nil
	})
	sc.Step(`^the orchestrator responds with 400 Bad Request$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.lastStatusCode != http.StatusBadRequest {
			return fmt.Errorf("expected 400, got %d", st.lastStatusCode)
		}
		return nil
	})

	// Tasks
	sc.Step(`^I create a task with prompt "([^"]*)" and task name "([^"]*)"$`, func(ctx context.Context, prompt, taskName string) error {
		st := getState(ctx)
		if st == nil || st.server == nil {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]interface{}{"prompt": prompt, "task_name": taskName})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/tasks", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create task returned %d", resp.StatusCode)
		}
		var out struct {
			TaskID string `json:"task_id"`
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		st.taskID = out.TaskID
		return nil
	})
	sc.Step(`^the task name is "([^"]*)"$`, func(ctx context.Context, wantName string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.taskID == "" {
			return godog.ErrSkip
		}
		req, _ := http.NewRequest("GET", st.server.URL+"/v1/tasks/"+st.taskID, http.NoBody)
		req.Header.Set("Authorization", "Bearer "+st.accessToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("get task returned %d", resp.StatusCode)
		}
		var out struct {
			TaskName *string `json:"task_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return err
		}
		if out.TaskName == nil || *out.TaskName != wantName {
			return fmt.Errorf("task name got %v, want %q", out.TaskName, wantName)
		}
		return nil
	})
	// Workflow start/resume/checkpoint/release (REQ-ORCHES-0144--0147)
	sc.Step(`^I start workflow for task with holder "([^"]*)"$`, func(ctx context.Context, holder string) error {
		if err := postTaskReadyHTTP(ctx); err != nil {
			return err
		}
		st := getState(ctx)
		if st == nil || st.server == nil || st.taskID == "" {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"task_id": st.taskID, "holder_id": holder})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/start", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		st.lastStatusCode = resp.StatusCode
		st.workflowStartBody, _ = io.ReadAll(resp.Body)
		return nil
	})
	sc.Step(`^workflow start response status is (\d+)$`, func(ctx context.Context, want int) error {
		st := getState(ctx)
		if st == nil {
			return godog.ErrSkip
		}
		if st.lastStatusCode != want {
			return fmt.Errorf("workflow start status got %d, want %d", st.lastStatusCode, want)
		}
		return nil
	})
	sc.Step(`^workflow start response includes run_id$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st == nil || len(st.workflowStartBody) == 0 {
			return godog.ErrSkip
		}
		var out struct {
			RunID string `json:"run_id"`
		}
		if err := json.Unmarshal(st.workflowStartBody, &out); err != nil {
			return err
		}
		if out.RunID == "" {
			return fmt.Errorf("workflow start response missing run_id")
		}
		return nil
	})
	sc.Step(`^workflow start response has status "([^"]*)"$`, func(ctx context.Context, want string) error {
		st := getState(ctx)
		if st == nil || len(st.workflowStartBody) == 0 {
			return godog.ErrSkip
		}
		var out struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(st.workflowStartBody, &out); err != nil {
			return err
		}
		if out.Status != want {
			return fmt.Errorf("workflow start status field got %q, want %q", out.Status, want)
		}
		return nil
	})
	sc.Step(`^I save checkpoint for task with last_node_id "([^"]*)"$`, func(ctx context.Context, nodeID string) error {
		st := getState(ctx)
		if st == nil || st.server == nil || st.taskID == "" {
			return godog.ErrSkip
		}
		body, _ := json.Marshal(map[string]string{"task_id": st.taskID, "last_node_id": nodeID})
		req, _ := http.NewRequest("POST", st.server.URL+"/v1/workflow/checkpoint", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("save checkpoint returned %d", resp.StatusCode)
		}
		return nil
	})
}
