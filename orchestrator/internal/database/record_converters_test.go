package database

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/go_shared_libs/gormmodel"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

func TestSessionRecord_ToSession(t *testing.T) {
	id := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	ts := time.Unix(10, 0).UTC()
	r := &SessionRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        id,
			CreatedAt: ts,
			UpdatedAt: ts,
		},
		SessionBase: models.SessionBase{UserID: id},
	}
	out := r.ToSession()
	if out == nil || out.ID != id || out.UserID != id {
		t.Fatalf("ToSession: %+v", out)
	}
}

func TestApiCredentialRecord_ToApiCredential(t *testing.T) {
	id := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	ts := time.Unix(11, 0).UTC()
	owner := uuid.MustParse("00000000-0000-4000-8000-000000000003")
	r := &ApiCredentialRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        id,
			CreatedAt: ts,
			UpdatedAt: ts,
		},
		ApiCredentialBase: models.ApiCredentialBase{
			OwnerType:      "user",
			OwnerID:        owner,
			Provider:       "openai",
			CredentialType: "api_key",
			CredentialName: "default",
			IsActive:       true,
		},
	}
	out := r.ToApiCredential()
	if out == nil || out.ID != id || out.Provider != "openai" {
		t.Fatalf("ToApiCredential: %+v", out)
	}
}

func TestNodeCapabilityRecord_ToNodeCapability(t *testing.T) {
	id := uuid.MustParse("00000000-0000-4000-8000-000000000004")
	nodeID := uuid.MustParse("00000000-0000-4000-8000-000000000005")
	ts := time.Unix(12, 0).UTC()
	r := &NodeCapabilityRecord{
		GormModelUUID: gormmodel.GormModelUUID{
			ID:        id,
			CreatedAt: ts,
			UpdatedAt: ts,
		},
		NodeCapabilityBase: models.NodeCapabilityBase{
			NodeID:             nodeID,
			ReportedAt:         ts,
			CapabilitySnapshot: `{}`,
		},
	}
	out := r.ToNodeCapability()
	if out == nil || out.ID != id || out.NodeID != nodeID {
		t.Fatalf("ToNodeCapability: %+v", out)
	}
}

func TestSandboxRecord_ToMethods(t *testing.T) {
	id := uuid.MustParse("00000000-0000-4000-8000-000000000006")
	ts := time.Unix(13, 0).UTC()
	img := (&SandboxImageRecord{
		GormModelUUID: gormmodel.GormModelUUID{ID: id, CreatedAt: ts, UpdatedAt: ts},
		SandboxImageBase: models.SandboxImageBase{
			Name: "python-tools",
		},
	}).ToSandboxImage()
	if img == nil || img.Name != "python-tools" {
		t.Fatalf("ToSandboxImage: %+v", img)
	}

	sid := uuid.MustParse("00000000-0000-4000-8000-000000000007")
	ver := (&SandboxImageVersionRecord{
		GormModelUUID: gormmodel.GormModelUUID{ID: id, CreatedAt: ts, UpdatedAt: ts},
		SandboxImageVersionBase: models.SandboxImageVersionBase{
			SandboxImageID: sid,
			Version:        "1.0",
			ImageRef:       "oci/ref",
		},
	}).ToSandboxImageVersion()
	if ver == nil || ver.Version != "1.0" {
		t.Fatalf("ToSandboxImageVersion: %+v", ver)
	}

	nid := uuid.MustParse("00000000-0000-4000-8000-000000000008")
	avail := (&NodeSandboxImageAvailabilityRecord{
		GormModelUUID: gormmodel.GormModelUUID{ID: id, CreatedAt: ts, UpdatedAt: ts},
		NodeSandboxImageAvailabilityBase: models.NodeSandboxImageAvailabilityBase{
			NodeID:                nid,
			SandboxImageVersionID: sid,
			Status:                "ready",
		},
		LastCheckedAt: ts,
	}).ToNodeSandboxImageAvailability()
	if avail == nil || avail.Status != "ready" {
		t.Fatalf("ToNodeSandboxImageAvailability: %+v", avail)
	}
}

func TestTaskDependencyRecord_ToTaskDependency(t *testing.T) {
	id := uuid.MustParse("00000000-0000-4000-8000-000000000009")
	tid := uuid.MustParse("00000000-0000-4000-8000-00000000000a")
	dep := uuid.MustParse("00000000-0000-4000-8000-00000000000b")
	ts := time.Unix(14, 0).UTC()
	r := &TaskDependencyRecord{
		GormModelUUID: gormmodel.GormModelUUID{ID: id, CreatedAt: ts, UpdatedAt: ts},
		TaskDependencyBase: models.TaskDependencyBase{
			TaskID:          tid,
			DependsOnTaskID: dep,
		},
	}
	out := r.ToTaskDependency()
	if out == nil || out.TaskID != tid || out.DependsOnTaskID != dep {
		t.Fatalf("ToTaskDependency: %+v", out)
	}
}

func TestFromUser(t *testing.T) {
	id := uuid.MustParse("00000000-0000-4000-8000-00000000000c")
	ts := time.Unix(15, 0).UTC()
	u := &models.User{
		UserBase: models.UserBase{
			Handle:   "alice",
			IsActive: true,
		},
		ID:        id,
		CreatedAt: ts,
		UpdatedAt: ts,
	}
	r := FromUser(u)
	if r == nil || r.Handle != "alice" || r.ID != id {
		t.Fatalf("FromUser: %+v", r)
	}
}

func TestAuthAuditLogRecord_ToAuthAuditLog(t *testing.T) {
	id := uuid.MustParse("00000000-0000-4000-8000-00000000000d")
	ts := time.Unix(16, 0).UTC()
	r := &AuthAuditLogRecord{
		GormModelUUID: gormmodel.GormModelUUID{ID: id, CreatedAt: ts, UpdatedAt: ts},
		AuthAuditLogBase: models.AuthAuditLogBase{
			EventType: "login",
			Success:   true,
		},
	}
	out := r.ToAuthAuditLog()
	if out == nil || out.ID != id || out.EventType != "login" || !out.Success {
		t.Fatalf("ToAuthAuditLog: %+v", out)
	}
}
