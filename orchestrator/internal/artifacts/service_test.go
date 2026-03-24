package artifacts

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
)

func TestList_unconfiguredService(t *testing.T) {
	u := uuid.New()
	var svc *Service
	_, err := svc.List(context.Background(), u, "x", database.ListOrchestratorArtifactsParams{
		ScopeLevel:  "user",
		OwnerUserID: &u,
		Limit:       10,
	})
	if err == nil {
		t.Fatal("expected error when service/db nil")
	}
}

func TestScopePartition_invalidLevel(t *testing.T) {
	_, err := ScopePartition("invalid", nil, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestScopePartition(t *testing.T) {
	uid := uuid.New()
	gid := uuid.New()
	pid := uuid.New()
	okCases := []struct {
		name  string
		level string
		owner *uuid.UUID
		group *uuid.UUID
		proj  *uuid.UUID
		want  string
	}{
		{"user", "user", &uid, nil, nil, "user:" + uid.String()},
		{"group", "group", nil, &gid, nil, "group:" + gid.String()},
		{"project", "project", nil, nil, &pid, "project:" + pid.String()},
		{"global", "global", nil, nil, nil, "global"},
	}
	for _, tc := range okCases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ScopePartition(tc.level, tc.owner, tc.group, tc.proj)
			if err != nil {
				t.Fatal(err)
			}
			if p != tc.want {
				t.Fatalf("got %q", p)
			}
		})
	}
	t.Run("user_missing_owner", func(t *testing.T) {
		_, err := ScopePartition("user", nil, nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestSanitizePath(t *testing.T) {
	got, err := SanitizePath("docs/report.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "docs/report.md" {
		t.Fatalf("got %q", got)
	}
	if _, err := SanitizePath("../etc/passwd"); err == nil {
		t.Fatal("expected error for traversal")
	}
	if _, err := SanitizePath(""); err == nil {
		t.Fatal("expected error for empty")
	}
	if _, err := SanitizePath("   "); err == nil {
		t.Fatal("expected error for whitespace-only path")
	}
}
