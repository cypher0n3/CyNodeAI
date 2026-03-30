package workerapiserver

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/workerapi"
	"github.com/cypher0n3/cynodeai/go_shared_libs/httplimits"
)

func TestMaxBytes_OversizeJobsRunBodyRejected(t *testing.T) {
	h := embedJobsRunHandler(stubEmbedRunner{}, "/tmp", "")
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	// Minimal JSON with padding to exceed default API body limit (embed uses same 10 MiB cap as httplimits default).
	pad := bytes.Repeat([]byte("x"), int(httplimits.DefaultMaxAPIRequestBodyBytes)+1024)
	body := append([]byte(`{"version":1,"task_id":"t","job_id":"j","sandbox":{"job_spec_json":"`), pad...)
	body = append(body, []byte(`"}}`)...)
	req, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(body))
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

type stubEmbedRunner struct{}

func (stubEmbedRunner) RunJob(context.Context, *workerapi.RunJobRequest, string) (*workerapi.RunJobResponse, error) {
	return &workerapi.RunJobResponse{Version: 1}, nil
}

func (stubEmbedRunner) Ready(context.Context) (bool, string) {
	return true, ""
}
