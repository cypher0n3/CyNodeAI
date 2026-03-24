package artifacts

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/s3blob"
)

func TestIntegration_NewServiceFromConfig_minio(t *testing.T) {
	if os.Getenv("SKIP_TESTCONTAINERS") != "" {
		t.Skip("SKIP_TESTCONTAINERS set")
	}
	setupRootlessPodmanHostForTests()

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
	ep := urlForceIPv4Localhost(raw, "9000")

	db := tcArtifactsDB(t, context.Background())
	svc, err := NewServiceFromConfig(context.Background(), db, &config.OrchestratorConfig{
		ArtifactsS3Endpoint:        ep,
		ArtifactsS3Bucket:          "artifact-wire-test",
		ArtifactsS3Region:          "us-east-1",
		ArtifactsS3AccessKey:       "minioadmin",
		ArtifactsS3SecretKey:       "minioadmin",
		ArtifactHashInlineMaxBytes: -1,
	})
	if err != nil {
		t.Fatalf("NewServiceFromConfig: %v", err)
	}
	if svc == nil || svc.Blob == nil {
		t.Fatal("expected non-nil service with blob store")
	}
	if svc.HashInlineMaxBytes != 1024*1024 {
		t.Fatalf("hash inline default: %d", svc.HashInlineMaxBytes)
	}

	svc2, err := NewServiceFromConfig(context.Background(), db, &config.OrchestratorConfig{
		ArtifactsS3Endpoint:  ep,
		ArtifactsS3AccessKey: "minioadmin",
		ArtifactsS3SecretKey: "minioadmin",
	})
	if err != nil {
		t.Fatalf("NewServiceFromConfig defaults: %v", err)
	}
	cl, ok := svc2.Blob.(*s3blob.Client)
	if !ok || cl.Bucket() != "cynodeai-artifacts" {
		t.Fatalf("default bucket: ok=%v bucket=%q", ok, cl.Bucket())
	}
}
