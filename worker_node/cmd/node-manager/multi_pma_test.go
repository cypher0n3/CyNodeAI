// Package main: REQ-WORKER-0176 multi-instance PMA reconciliation tests.
package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/nodepayloads"
)

// TestMultiPMA_StartManagedServices_DistinctContainersPerServiceID asserts multiple
// managed_services entries each get their own container name derived from service_id.
func TestMultiPMA_StartManagedServices_DistinctContainersPerServiceID(t *testing.T) {
	var containerNames []string
	fr := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		if len(args) >= 4 && args[0] == "run" && args[2] == "--name" {
			containerNames = append(containerNames, args[3])
		}
		return []byte(""), nil
	})
	withRunner(t, fr)
	services := []nodepayloads.ConfigManagedService{
		{ServiceID: "pma-binding-a", ServiceType: "worker", Image: "img:latest"},
		{ServiceID: "pma-binding-b", ServiceType: "worker", Image: "img:latest"},
	}
	if err := startManagedServices(context.Background(), services); err != nil {
		t.Fatalf("startManagedServices: %v", err)
	}
	if len(containerNames) != 2 {
		t.Fatalf("want 2 podman run --name invocations, got %d (%v)", len(containerNames), containerNames)
	}
	if containerNames[0] == containerNames[1] {
		t.Fatalf("distinct service_id must yield distinct container names: %v", containerNames)
	}
}

// TestStartManagedServices_StopsUndesiredContainers asserts containers matching cynodeai-managed-*
// but absent from desired config get stop/rm before desired services are reconciled.
func TestStartManagedServices_StopsUndesiredContainers(t *testing.T) {
	var stopped []string
	fr := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ps" {
			return []byte("cynodeai-managed-pma-main\ncynodeai-managed-pma-sb-orphan\n"), nil
		}
		if len(args) > 0 && args[0] == "stop" {
			stopped = append(stopped, args[len(args)-1])
		}
		return []byte(""), nil
	})
	withRunner(t, fr)
	services := []nodepayloads.ConfigManagedService{
		{ServiceID: "pma-main", ServiceType: "worker", Image: "img:latest"},
	}
	if err := startManagedServices(context.Background(), services); err != nil {
		t.Fatalf("startManagedServices: %v", err)
	}
	if !slices.Contains(stopped, "cynodeai-managed-pma-sb-orphan") {
		t.Fatalf("expected stop of undesired pma-sb container, stopped=%v", stopped)
	}
	if slices.Contains(stopped, "cynodeai-managed-pma-main") {
		t.Fatalf("must not stop desired pma-main, stopped=%v", stopped)
	}
}

// TestStartManagedServices_SkipOnlyRowsDoesNotStopManagedContainers when every row is invalid,
// desired set is empty and we must not invoke stop on unrelated managed names from ps.
func TestStartManagedServices_SkipOnlyRowsDoesNotStopManagedContainers(t *testing.T) {
	var stopped []string
	fr := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ps" {
			return []byte("cynodeai-managed-pma-main\n"), nil
		}
		if len(args) > 0 && args[0] == "stop" {
			stopped = append(stopped, strings.Join(args, " "))
		}
		return []byte(""), nil
	})
	withRunner(t, fr)
	services := []nodepayloads.ConfigManagedService{
		{ServiceID: "", ServiceType: "pma", Image: "img"},
	}
	if err := startManagedServices(context.Background(), services); err != nil {
		t.Fatalf("startManagedServices: %v", err)
	}
	if len(stopped) != 0 {
		t.Fatalf("expected no stop when desired is empty, got %v", stopped)
	}
}

// TestStartManagedServices_EmptySliceStopsAllManaged asserts len(services)==0 stops every
// cynodeai-managed-* container (admin revoke / orchestrator cleared desired list).
func TestStartManagedServices_EmptySliceStopsAllManaged(t *testing.T) {
	var stopped []string
	fr := fakeRunnerFunc(func(name string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "ps" {
			return []byte("cynodeai-managed-pma-main\ncynodeai-managed-pma-sb-orphan\n"), nil
		}
		if len(args) > 0 && args[0] == "stop" {
			stopped = append(stopped, args[len(args)-1])
		}
		return []byte(""), nil
	})
	withRunner(t, fr)
	if err := startManagedServices(context.Background(), nil); err != nil {
		t.Fatalf("startManagedServices: %v", err)
	}
	if len(stopped) != 2 {
		t.Fatalf("expected stop+rm for both managed containers, stopped count=%d %v", len(stopped), stopped)
	}
}
