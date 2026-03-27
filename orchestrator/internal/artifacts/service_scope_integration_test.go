package artifacts

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/s3blob"
)

const testPlainTextCT = "text/plain"

type scopeListFixtures struct {
	ctx   context.Context
	db    *database.DB
	blob  *s3blob.MemStore
	svc   *Service
	alice *models.User
	bob   *models.User
	proj  *models.Project
	ct    string
}

func setupScopeListFixtures(t *testing.T) *scopeListFixtures {
	t.Helper()
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	blob := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, blob, 1024*1024)

	alice, err := db.CreateUser(ctx, "scope-alice-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	bob, err := db.CreateUser(ctx, "scope-bob-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	proj, err := db.GetOrCreateDefaultProjectForUser(ctx, alice.ID)
	if err != nil {
		t.Fatalf("project: %v", err)
	}
	return &scopeListFixtures{
		ctx: ctx, db: db, blob: blob, svc: svc, alice: alice, bob: bob, proj: proj, ct: testPlainTextCT,
	}
}

func mustAdminUser(t *testing.T, ctx context.Context, db *database.DB) *models.User {
	t.Helper()
	adm, err := db.GetUserByHandle(ctx, "admin")
	if err != nil {
		if !errors.Is(err, database.ErrNotFound) {
			t.Fatal(err)
		}
		adm, err = db.CreateUser(ctx, "admin", nil)
		if err != nil {
			t.Fatalf("CreateUser admin: %v", err)
		}
	}
	return adm
}

func mustCreateFromBodyOK(t *testing.T, svc *Service, ctx context.Context, subjectID uuid.UUID, subjectHandle string, in *CreateFromBodyInput) {
	t.Helper()
	if _, err := svc.CreateFromBody(ctx, subjectID, subjectHandle, in); err != nil {
		t.Fatal(err)
	}
}

func TestIntegration_ListAndScopes(t *testing.T) {
	f := setupScopeListFixtures(t)
	t.Run("list_group_and_global_non_admin_forbidden", func(t *testing.T) {
		scopeListGroupGlobalNonAdminForbidden(t, f)
	})
	t.Run("list_user_scope_with_offset", func(t *testing.T) {
		scopeListUserWithOffset(t, f)
	})
	t.Run("list_user_scope_wrong_principal", func(t *testing.T) {
		scopeListUserWrongPrincipal(t, f)
	})
	t.Run("list_project_scope_own_project", func(t *testing.T) {
		scopeListProjectOwnProject(t, f)
	})
	t.Run("get_by_scope_path_project_forbidden_other_user", func(t *testing.T) {
		scopeGetByPathProjectForbidden(t, f)
	})
	t.Run("list_validation_errors", func(t *testing.T) {
		scopeListValidationErrors(t, f)
	})
	t.Run("get_blob_global_non_admin_forbidden", func(t *testing.T) {
		scopeGetBlobGlobalNonAdmin(t, f)
	})
	t.Run("get_blob_group_scope_non_admin_forbidden", func(t *testing.T) {
		scopeGetBlobGroupNonAdmin(t, f)
	})
	t.Run("group_and_global_admin_handle", func(t *testing.T) {
		scopeGroupGlobalAdminHandle(t, f)
	})
	t.Run("delete_project_scoped", func(t *testing.T) {
		scopeDeleteProjectScoped(t, f)
	})
	t.Run("update_blob_and_large_hash_defer", func(t *testing.T) {
		scopeUpdateBlobLargeHashDefer(t, f)
	})
	t.Run("MCP_args_and_put_update", func(t *testing.T) {
		scopeMCPArgsAndPutUpdate(t, f)
	})
	t.Run("MCPGet_invalid_b64", func(t *testing.T) {
		scopeMCPInvalidB64(t, f)
	})
	t.Run("nil_service_methods", func(t *testing.T) {
		scopeNilServiceMethods(t, f)
	})
}

func scopeListGroupGlobalNonAdminForbidden(t *testing.T, f *scopeListFixtures) {
	adm := mustAdminUser(t, f.ctx, f.db)
	ct := f.ct
	gid := uuid.New()
	mustCreateFromBodyOK(t, f.svc, f.ctx, adm.ID, adm.Handle, &CreateFromBodyInput{
		Level: "group", GroupID: &gid, ArtifactPath: "g-list.txt", Body: []byte("x"), ContentType: &ct,
	})
	_, err := f.svc.List(f.ctx, f.alice.ID, f.alice.Handle, database.ListOrchestratorArtifactsParams{
		ScopeLevel: "group",
		GroupID:    &gid,
		Limit:      10,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("List group non-admin: %v", err)
	}
	mustCreateFromBodyOK(t, f.svc, f.ctx, adm.ID, adm.Handle, &CreateFromBodyInput{
		Level: "global", ArtifactPath: "glob-list.txt", Body: []byte("x"), ContentType: &ct,
	})
	_, err = f.svc.List(f.ctx, f.alice.ID, f.alice.Handle, database.ListOrchestratorArtifactsParams{
		ScopeLevel: "global",
		Limit:      10,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("List global non-admin: %v", err)
	}
}

func scopeListUserWithOffset(t *testing.T, f *scopeListFixtures) {
	ct := f.ct
	for i := range 3 {
		path := fmt.Sprintf("off-%d.txt", i)
		mustCreateFromBodyOK(t, f.svc, f.ctx, f.alice.ID, f.alice.Handle, &CreateFromBodyInput{
			Level: "user", OwnerUserID: &f.alice.ID, ArtifactPath: path, Body: []byte("x"), ContentType: &ct,
		})
	}
	rows, err := f.svc.List(f.ctx, f.alice.ID, f.alice.Handle, database.ListOrchestratorArtifactsParams{
		ScopeLevel:  "user",
		OwnerUserID: &f.alice.ID,
		Limit:       2,
		Offset:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 1 {
		t.Fatal("expected rows with offset")
	}
}

func scopeListUserWrongPrincipal(t *testing.T, f *scopeListFixtures) {
	ct := f.ct
	mustCreateFromBodyOK(t, f.svc, f.ctx, f.alice.ID, f.alice.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &f.alice.ID, ArtifactPath: "u1.txt", Body: []byte("a"), ContentType: &ct,
	})
	_, err := f.svc.List(f.ctx, f.bob.ID, f.bob.Handle, database.ListOrchestratorArtifactsParams{
		ScopeLevel:  "user",
		OwnerUserID: &f.alice.ID,
		Limit:       10,
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("List: %v", err)
	}
}

func scopeListProjectOwnProject(t *testing.T, f *scopeListFixtures) {
	ct := f.ct
	_, err := f.svc.CreateFromBody(f.ctx, f.alice.ID, f.alice.Handle, &CreateFromBodyInput{
		Level: "project", ProjectID: &f.proj.ID, ArtifactPath: "p1.txt", Body: []byte("p"), ContentType: &ct,
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.List(f.ctx, f.alice.ID, f.alice.Handle, database.ListOrchestratorArtifactsParams{
		ScopeLevel: "project",
		ProjectID:  &f.proj.ID,
		Limit:      10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 1 {
		t.Fatal("expected rows")
	}
}

func scopeGetByPathProjectForbidden(t *testing.T, f *scopeListFixtures) {
	ct := f.ct
	mustCreateFromBodyOK(t, f.svc, f.ctx, f.alice.ID, f.alice.Handle, &CreateFromBodyInput{
		Level: "project", ProjectID: &f.proj.ID, ArtifactPath: "p-bob-deny.txt", Body: []byte("z"), ContentType: &ct,
	})
	_, _, err := f.svc.GetByScopePath(f.ctx, f.bob.ID, f.bob.Handle, "project", nil, nil, &f.proj.ID, "p-bob-deny.txt")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("GetByScopePath: %v", err)
	}
}

func scopeListValidationErrors(t *testing.T, f *scopeListFixtures) {
	cases := []struct {
		name string
		p    database.ListOrchestratorArtifactsParams
	}{
		{"invalid_scope", database.ListOrchestratorArtifactsParams{ScopeLevel: "nope", Limit: 10}},
		{"project_missing_id", database.ListOrchestratorArtifactsParams{ScopeLevel: "project", Limit: 10}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.List(f.ctx, f.alice.ID, f.alice.Handle, tc.p)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func scopeGetBlobGlobalNonAdmin(t *testing.T, f *scopeListFixtures) {
	adm := mustAdminUser(t, f.ctx, f.db)
	ct := f.ct
	art, err := f.svc.CreateFromBody(f.ctx, adm.ID, adm.Handle, &CreateFromBodyInput{
		Level: "global", ArtifactPath: "glob-nonadmin.txt", Body: []byte("x"), ContentType: &ct,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = f.svc.GetBlob(f.ctx, f.alice.ID, f.alice.Handle, art.ID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("GetBlob global non-admin: %v", err)
	}
}

func scopeGetBlobGroupNonAdmin(t *testing.T, f *scopeListFixtures) {
	adm := mustAdminUser(t, f.ctx, f.db)
	ct := f.ct
	gid := uuid.New()
	art, err := f.svc.CreateFromBody(f.ctx, adm.ID, adm.Handle, &CreateFromBodyInput{
		Level: "group", GroupID: &gid, ArtifactPath: "g-read.txt", Body: []byte("gr"), ContentType: &ct,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = f.svc.GetBlob(f.ctx, f.alice.ID, f.alice.Handle, art.ID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("GetBlob group non-admin: %v", err)
	}
}

func scopeGroupGlobalAdminHandle(t *testing.T, f *scopeListFixtures) {
	adm := mustAdminUser(t, f.ctx, f.db)
	ct := f.ct
	gid := uuid.New()
	_, err := f.svc.CreateFromBody(f.ctx, adm.ID, adm.Handle, &CreateFromBodyInput{
		Level: "group", GroupID: &gid, ArtifactPath: "g.txt", Body: []byte("g"), ContentType: &ct,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.CreateFromBody(f.ctx, adm.ID, adm.Handle, &CreateFromBodyInput{
		Level: "global", ArtifactPath: "glob.txt", Body: []byte("gl"), ContentType: &ct,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.List(f.ctx, adm.ID, adm.Handle, database.ListOrchestratorArtifactsParams{ScopeLevel: "group", GroupID: &gid, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.List(f.ctx, adm.ID, adm.Handle, database.ListOrchestratorArtifactsParams{ScopeLevel: "global", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
}

func scopeDeleteProjectScoped(t *testing.T, f *scopeListFixtures) {
	ct := f.ct
	art, err := f.svc.CreateFromBody(f.ctx, f.alice.ID, f.alice.Handle, &CreateFromBodyInput{
		Level: "project", ProjectID: &f.proj.ID, ArtifactPath: "del-me.txt", Body: []byte("d"), ContentType: &ct,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Delete(f.ctx, f.alice.ID, f.alice.Handle, art.ID); err != nil {
		t.Fatal(err)
	}
}

func scopeUpdateBlobLargeHashDefer(t *testing.T, f *scopeListFixtures) {
	ct := f.ct
	small := NewServiceWithBlob(f.db, f.blob, 4)
	art, err := small.CreateFromBody(f.ctx, f.alice.ID, f.alice.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &f.alice.ID, ArtifactPath: "big.bin", Body: []byte("12345678"), ContentType: &ct,
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.ChecksumSHA256 != nil {
		t.Fatal("expected deferred hash for large inline threshold")
	}
	if _, err := small.UpdateBlob(f.ctx, f.alice.ID, f.alice.Handle, art.ID, []byte("x"), &ct, nil); err != nil {
		t.Fatal(err)
	}
}

func scopeMCPArgsAndPutUpdate(t *testing.T, f *scopeListFixtures) {
	_, err := f.svc.MCPPut(f.ctx, f.alice.ID, f.alice.Handle, map[string]interface{}{
		"path":                 "mcp/up.txt",
		"scope":                "user",
		"content_bytes_base64": "Zm9v",
		"user_id":              f.alice.ID.String(),
		"limit":                42,
		"task_id":              uuid.New().String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.MCPPut(f.ctx, f.alice.ID, f.alice.Handle, map[string]interface{}{
		"path":                 "mcp/up.txt",
		"scope":                "user",
		"content_bytes_base64": "YmFy",
		"user_id":              f.alice.ID.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.MCPList(f.ctx, f.alice.ID, f.alice.Handle, map[string]interface{}{
		"scope":   "user",
		"user_id": f.alice.ID.String(),
		"limit":   10,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.MCPList(f.ctx, f.alice.ID, f.alice.Handle, map[string]interface{}{
		"scope":   "user",
		"user_id": f.alice.ID.String(),
		"limit":   10,
		"task_id": uuid.New().String(),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func scopeMCPInvalidB64(t *testing.T, f *scopeListFixtures) {
	_, err := f.svc.MCPPut(f.ctx, f.alice.ID, f.alice.Handle, map[string]interface{}{
		"path":                 "bad",
		"scope":                "user",
		"content_bytes_base64": "@@@",
		"user_id":              f.alice.ID.String(),
	})
	if err == nil {
		t.Fatal("expected invalid base64")
	}
}

func scopeNilServiceMethods(t *testing.T, f *scopeListFixtures) {
	var nilSvc *Service
	if _, err := nilSvc.CreateFromBody(f.ctx, uuid.New(), "x", &CreateFromBodyInput{
		Level: "user", OwnerUserID: &f.alice.ID, ArtifactPath: "p", Body: nil,
	}); err == nil {
		t.Fatal("expected error")
	}
	if _, _, err := nilSvc.GetBlob(f.ctx, f.alice.ID, f.alice.Handle, uuid.New()); err == nil {
		t.Fatal("expected error")
	}
	if err := nilSvc.Delete(f.ctx, f.alice.ID, f.alice.Handle, uuid.New()); err == nil {
		t.Fatal("expected error")
	}
}

func Test_NewServiceWithBlob_edges(t *testing.T) {
	if NewServiceWithBlob(nil, s3blob.NewMemStore(), 100) != nil {
		t.Fatal("expected nil")
	}
	db := &database.DB{}
	if NewServiceWithBlob(db, nil, 100) != nil {
		t.Fatal("expected nil")
	}
	s := NewServiceWithBlob(db, s3blob.NewMemStore(), 0)
	if s == nil || s.HashInlineMaxBytes != 1024*1024 {
		t.Fatalf("hash default: %+v", s)
	}
}

func Test_SystemDeleteArtifact_not_found(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	blob := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, blob, 1024)
	if err := svc.SystemDeleteArtifact(ctx, uuid.New()); err == nil {
		t.Fatal("expected error")
	}
}

func Test_GetByScopePath_errors(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc := NewServiceWithBlob(db, s3blob.NewMemStore(), 1024)
	u, err := db.CreateUser(ctx, "gsp-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.GetByScopePath(ctx, u.ID, u.Handle, "user", &u.ID, nil, nil, "../bad")
	if err == nil {
		t.Fatal("expected sanitize error")
	}
}
