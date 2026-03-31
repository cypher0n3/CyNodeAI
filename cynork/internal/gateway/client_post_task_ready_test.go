package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

func TestClient_PostTaskReady(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks/t-1/ready" || r.Method != http.MethodPost {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(userapi.TaskResponse{
			ID:            "t-1",
			Status:        "queued",
			PlanningState: userapi.PlanningStateReady,
		})
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	out, err := client.PostTaskReady(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("PostTaskReady: %v", err)
	}
	if out.ResolveTaskID() != "t-1" || out.PlanningState != userapi.PlanningStateReady {
		t.Fatalf("unexpected response: %+v", out)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestClient_PostTaskReady_UsesCopiedTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tasks/x/ready" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x","status":"queued","planning_state":"ready"}`))
	}))
	defer server.Close()
	var sawCustom bool
	client := NewClient(server.URL)
	client.SetToken("tok")
	client.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			sawCustom = true
			return http.DefaultTransport.RoundTrip(req)
		}),
	}
	_, err := client.PostTaskReady(context.Background(), "x")
	if err != nil {
		t.Fatalf("PostTaskReady: %v", err)
	}
	if !sawCustom {
		t.Fatal("expected PostTaskReady to use HTTPClient.Transport via httpClientForTaskReady")
	}
}

func TestClient_PostTaskReady_NonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.PostTaskReady(context.Background(), "t")
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

func TestClient_PostTaskReady_InvalidJSONBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()
	client := NewClient(server.URL)
	client.SetToken("tok")
	_, err := client.PostTaskReady(context.Background(), "t")
	if err == nil {
		t.Fatal("expected decode error")
	}
}
