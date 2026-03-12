package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestInsertNodeBoot(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	row := NodeBootRow{
		BootID:        "boot-1",
		NodeSlug:      "node-01",
		BuildVersion:  "1.0.0",
		PlatformOS:    "linux",
		PlatformArch:  "amd64",
		KernelVersion: "5.10",
	}
	if err := s.InsertNodeBoot(ctx, &row); err != nil {
		t.Fatalf("InsertNodeBoot: %v", err)
	}
	// GetLatestNodeBoot should return it
	got, err := s.GetLatestNodeBoot(ctx)
	if err != nil {
		t.Fatalf("GetLatestNodeBoot: %v", err)
	}
	if got == nil || got.BootID != "boot-1" || got.NodeSlug != "node-01" || got.PlatformOS != "linux" {
		t.Errorf("GetLatestNodeBoot: %+v", got)
	}
	if got.BootedAt == "" {
		t.Error("BootedAt should be set by default")
	}
}

func TestInsertNodeBoot_setsBootedAtWhenEmpty(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	row := NodeBootRow{BootID: "b2", NodeSlug: "n", BuildVersion: "v", PlatformOS: "linux", PlatformArch: "amd64", KernelVersion: ""}
	if err := s.InsertNodeBoot(ctx, &row); err != nil {
		t.Fatalf("InsertNodeBoot: %v", err)
	}
	got, _ := s.GetLatestNodeBoot(ctx)
	if got == nil || got.BootedAt == "" {
		t.Errorf("BootedAt not set: %+v", got)
	}
	if _, err := time.Parse(time.RFC3339, got.BootedAt); err != nil {
		t.Errorf("BootedAt not RFC3339: %q", got.BootedAt)
	}
}

func TestGetLatestNodeBoot_empty(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	got, err := s.GetLatestNodeBoot(ctx)
	if err != nil {
		t.Fatalf("GetLatestNodeBoot: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil when empty, got %+v", got)
	}
}

func TestGetLatestNodeBoot_returnsLatest(t *testing.T) {
	ctx := context.Background()
	s := openStore(t, ctx)
	defer func() { _ = s.Close() }()
	t1 := "2020-01-01T00:00:00Z"
	t2 := "2020-01-02T00:00:00Z"
	if err := s.InsertNodeBoot(ctx, &NodeBootRow{BootID: "old", BootedAt: t1, NodeSlug: "n", BuildVersion: "v", PlatformOS: "linux", PlatformArch: "amd64", KernelVersion: ""}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertNodeBoot(ctx, &NodeBootRow{BootID: "new", BootedAt: t2, NodeSlug: "n", BuildVersion: "v", PlatformOS: "linux", PlatformArch: "amd64", KernelVersion: ""}); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetLatestNodeBoot(ctx)
	if err != nil || got == nil || got.BootID != "new" {
		t.Errorf("GetLatestNodeBoot: err=%v got=%+v", err, got)
	}
}
