package nodeagent

import (
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

func TestPodmanHealthcheckArgs(t *testing.T) {
	t.Parallel()
	svcNoHC := &nodepayloads.ConfigManagedService{}
	if podmanHealthcheckArgs(svcNoHC, serviceTypePMA, "podman") != nil {
		t.Fatal("no healthcheck")
	}
	svc := &nodepayloads.ConfigManagedService{Healthcheck: &nodepayloads.ConfigManagedServiceHealthcheck{Path: "/z"}}
	args := podmanHealthcheckArgs(svc, serviceTypePMA, "podman")
	if len(args) == 0 || !strings.Contains(strings.Join(args, " "), "unix-socket") {
		t.Fatalf("pma uds health: %v", args)
	}
	svc2 := &nodepayloads.ConfigManagedService{Healthcheck: &nodepayloads.ConfigManagedServiceHealthcheck{Path: "/h"}}
	if podmanHealthcheckArgs(svc2, "pma", "docker") != nil {
		t.Fatal("non-podman runtime")
	}
	if podmanHealthcheckArgs(svc2, "unknown", "podman") != nil {
		t.Fatal("unknown service type should have no port mapping")
	}
}

func TestHTTPUnixProxyURL(t *testing.T) {
	t.Parallel()
	if httpUnixProxyURL("", "/p") != "" || httpUnixProxyURL("/sock", "") != "" {
		t.Fatal("empty parts")
	}
	got := httpUnixProxyURL("/run/sock", "/v1/x")
	if !strings.HasPrefix(got, "http+unix://") || !strings.Contains(got, "/v1/x") {
		t.Fatalf("got %q", got)
	}
}
