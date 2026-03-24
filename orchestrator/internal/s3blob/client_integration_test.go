package s3blob

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupRootlessPodmanHostS3() {
	if os.Getenv("DOCKER_HOST") != "" {
		return
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		return
	}
	sock := filepath.Join(runtimeDir, "podman", "podman.sock")
	if _, err := os.Stat(sock); err != nil {
		return
	}
	_ = os.Setenv("DOCKER_HOST", "unix://"+sock)
}

func endpointForceIPv4(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := u.Hostname()
	if host == "localhost" || host == "::1" {
		port := u.Port()
		if port == "" {
			port = "9000"
		}
		u.Host = "127.0.0.1:" + port
		return u.String()
	}
	return raw
}

func minioRoundTripObjectOps(t *testing.T, ctx context.Context, client *Client, key string, body []byte, ct string) {
	t.Helper()
	if err := client.PutObject(ctx, key, body, &ct); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	got, err := client.GetObject(ctx, key)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("GetObject: %v, %q", err, got)
	}
	if err := client.DeleteObject(ctx, key); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	if _, err := client.GetObject(ctx, key); err == nil {
		t.Fatal("GetObject after delete: expected error")
	}
	if err := client.PutObject(ctx, key+"-nct", body, nil); err != nil {
		t.Fatalf("PutObject nil content type: %v", err)
	}
	got2, err := client.GetObject(ctx, key + "-nct")
	if err != nil || !bytes.Equal(got2, body) {
		t.Fatalf("GetObject: %v", err)
	}
	if client.Bucket() != "artifact-test" {
		t.Fatalf("Bucket: %q", client.Bucket())
	}
}

func TestIntegration_MinIOClientRoundTrip(t *testing.T) {
	if os.Getenv("SKIP_TESTCONTAINERS") != "" {
		t.Skip("SKIP_TESTCONTAINERS set")
	}
	setupRootlessPodmanHostS3()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:RELEASE.2024-10-13T13-34-11Z",
		ExposedPorts: []string{"9000/tcp"},
		Cmd:          []string{"server", "/data"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		WaitingFor: wait.ForListeningPort("9000/tcp"),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		ProviderType:     testcontainers.ProviderPodman,
	})
	if err != nil {
		t.Fatalf("minio container: %v", err)
	}
	defer func() {
		termCtx, termCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer termCancel()
		_ = c.Terminate(termCtx)
	}()

	host, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("Host: %v", err)
	}
	mp, err := c.MappedPort(ctx, "9000")
	if err != nil {
		t.Fatalf("MappedPort: %v", err)
	}
	raw := fmt.Sprintf("http://%s:%s", host, mp.Port())
	ep := endpointForceIPv4(raw)

	ct := "text/plain"
	client, err := New(ctx, &Config{
		Endpoint:  ep,
		Region:    "us-east-1",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "artifact-test",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	body := []byte("hello-minio")
	minioRoundTripObjectOps(t, ctx, client, "k1/obj.bin", body, ct)

	emptyCT := ""
	if err := client.PutObject(ctx, "k1/empty-ct.txt", body, &emptyCT); err != nil {
		t.Fatalf("PutObject empty string content type: %v", err)
	}

	// Third client with empty Region exercises default region branch in New.
	client3, err := New(ctx, &Config{
		Endpoint:  ep,
		Region:    "",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "artifact-test",
	})
	if err != nil {
		t.Fatalf("New with default region: %v", err)
	}
	if client3.Bucket() != "artifact-test" {
		t.Fatalf("client3 Bucket: %q", client3.Bucket())
	}

	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if err := client.PutObject(cancelCtx, "k1/nope", body, nil); err == nil {
		t.Fatal("PutObject with canceled context: expected error")
	}
	if err := client.PutObject(ctx, "k1/cancel-get", body, nil); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	cancelCtx2, cancel2 := context.WithCancel(ctx)
	cancel2()
	if _, err := client.GetObject(cancelCtx2, "k1/cancel-get"); err == nil {
		t.Fatal("GetObject with canceled context: expected error")
	}
	if err := client.PutObject(ctx, "k1/cancel-del", body, nil); err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	cancelCtx3, cancel3 := context.WithCancel(ctx)
	cancel3()
	if err := client.DeleteObject(cancelCtx3, "k1/cancel-del"); err == nil {
		t.Fatal("DeleteObject with canceled context: expected error")
	}

	// Second client with same bucket exercises HeadBucket success path in ensureBucket.
	client2, err := New(ctx, &Config{
		Endpoint:  ep,
		Region:    "us-east-1",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "artifact-test",
	})
	if err != nil {
		t.Fatalf("New second client: %v", err)
	}
	if client2.Bucket() != "artifact-test" {
		t.Fatalf("second Bucket: %q", client2.Bucket())
	}
}
