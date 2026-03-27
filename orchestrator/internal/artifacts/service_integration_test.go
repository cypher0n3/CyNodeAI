package artifacts

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/config"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/s3blob"
)

// errBlobGet fails GetObject (blob read errors after DB metadata exists).
type errBlobGet struct{ inner *s3blob.MemStore }

func (e errBlobGet) PutObject(ctx context.Context, key string, body []byte, ct *string) error {
	return e.inner.PutObject(ctx, key, body, ct)
}

func (e errBlobGet) GetObject(ctx context.Context, key string) ([]byte, error) {
	return nil, errors.New("forced get failure")
}

func (e errBlobGet) DeleteObject(ctx context.Context, key string) error {
	return e.inner.DeleteObject(ctx, key)
}

// errBlobPut fails PutObject (exercises CreateFromBody storage failure).
type errBlobPut struct{ inner *s3blob.MemStore }

func (e errBlobPut) PutObject(ctx context.Context, key string, body []byte, ct *string) error {
	return errors.New("forced put failure")
}

func (e errBlobPut) GetObject(ctx context.Context, key string) ([]byte, error) {
	return e.inner.GetObject(ctx, key)
}

func (e errBlobPut) DeleteObject(ctx context.Context, key string) error {
	return e.inner.DeleteObject(ctx, key)
}

// errBlobDelete fails DeleteObject (exercises Delete storage failure).
type errBlobDelete struct{ inner *s3blob.MemStore }

func (e errBlobDelete) PutObject(ctx context.Context, key string, body []byte, ct *string) error {
	return e.inner.PutObject(ctx, key, body, ct)
}

func (e errBlobDelete) GetObject(ctx context.Context, key string) ([]byte, error) {
	return e.inner.GetObject(ctx, key)
}

func (e errBlobDelete) DeleteObject(ctx context.Context, key string) error {
	return errors.New("forced delete failure")
}

func TestIntegration_NewServiceFromConfig_emptyEndpoint(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc, err := NewServiceFromConfig(ctx, db, &config.OrchestratorConfig{})
	if err != nil || svc != nil {
		t.Fatalf("empty S3 endpoint: svc=%v err=%v", svc, err)
	}
}

func TestIntegration_NewServiceFromConfig_whitespaceEndpoint(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc, err := NewServiceFromConfig(ctx, db, &config.OrchestratorConfig{ArtifactsS3Endpoint: "  \t"})
	if err != nil || svc != nil {
		t.Fatalf("whitespace-only endpoint: svc=%v err=%v", svc, err)
	}
}

func TestIntegration_UpdateBlob_forbiddenOtherUser(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc := NewServiceWithBlob(db, s3blob.NewMemStore(), 1024*1024)
	alice, err := db.CreateUser(ctx, "art-upd-a-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	bob, err := db.CreateUser(ctx, "art-upd-b-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := alice.ID
	ct := testPlainTextCT
	art, err := svc.CreateFromBody(ctx, uid, alice.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "owned-upd.txt", Body: []byte("x"), ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	if _, err := svc.UpdateBlob(ctx, bob.ID, bob.Handle, art.ID, []byte("z"), &ct, nil); !errors.Is(err, ErrForbidden) {
		t.Fatalf("UpdateBlob: %v", err)
	}
}

func TestIntegration_Delete_forbiddenOtherUser(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc := NewServiceWithBlob(db, s3blob.NewMemStore(), 1024*1024)
	alice, err := db.CreateUser(ctx, "art-del-a-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	bob, err := db.CreateUser(ctx, "art-del-b-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := alice.ID
	ct := testPlainTextCT
	art, err := svc.CreateFromBody(ctx, uid, alice.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "owned-del.txt", Body: []byte("x"), ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	if err := svc.Delete(ctx, bob.ID, bob.Handle, art.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Delete: %v", err)
	}
}

func TestIntegration_CreateFromBody_nilContentType(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc := NewServiceWithBlob(db, s3blob.NewMemStore(), 1024*1024)
	user, err := db.CreateUser(ctx, "art-nilct-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	art, err := svc.CreateFromBody(ctx, uid, user.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "nil-ct.txt", Body: []byte("x"),
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	if art.ContentType != nil {
		t.Fatal("expected nil content type")
	}
}

func TestIntegration_ServiceUserScopeRoundTrip(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	blob := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, blob, 1024*1024)

	user, err := db.CreateUser(ctx, "art-svc-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	handle := user.Handle

	body := []byte("integration blob")
	ct := testPlainTextCT
	art, err := svc.CreateFromBody(ctx, uid, handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "int/path.txt", Body: body, ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	if art.ChecksumSHA256 == nil {
		t.Fatal("expected checksum for small blob")
	}

	got, meta, err := svc.GetBlob(ctx, uid, handle, art.ID)
	if err != nil {
		t.Fatalf("GetBlob: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatal("blob mismatch")
	}
	if meta.Path != "int/path.txt" {
		t.Fatalf("path %q", meta.Path)
	}

	ct2 := testPlainTextCT
	if _, err := svc.UpdateBlob(ctx, uid, handle, art.ID, []byte("v2"), &ct2, nil); err != nil {
		t.Fatalf("UpdateBlob: %v", err)
	}

	p := database.ListOrchestratorArtifactsParams{ScopeLevel: "user", OwnerUserID: &uid, Limit: 20}
	rows, err := svc.List(ctx, uid, handle, p)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) < 1 {
		t.Fatal("expected listed artifacts")
	}

	other, err := db.CreateUser(ctx, "art-other-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser other: %v", err)
	}
	if _, _, err := svc.GetBlob(ctx, other.ID, other.Handle, art.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	if err := svc.Delete(ctx, uid, handle, art.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestIntegration_CreateFromBody_putFails(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	mem := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, errBlobPut{inner: mem}, 1024*1024)
	user, err := db.CreateUser(ctx, "art-putfail-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	ct := testPlainTextCT
	_, err = svc.CreateFromBody(ctx, uid, user.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "pf.txt", Body: []byte("x"), ContentType: &ct,
	})
	if err == nil {
		t.Fatal("expected PutObject error")
	}
}

func TestIntegration_GetByScopePath_blobReadFails(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	mem := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, mem, 1024*1024)
	user, err := db.CreateUser(ctx, "art-gspf-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	ct := testPlainTextCT
	_, err = svc.CreateFromBody(ctx, uid, user.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "gspf.txt", Body: []byte("z"), ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	bad := NewServiceWithBlob(db, errBlobGet{inner: mem}, 1024*1024)
	_, _, err = bad.GetByScopePath(ctx, uid, user.Handle, "user", &uid, nil, nil, "gspf.txt")
	if err == nil {
		t.Fatal("expected GetObject error")
	}
}

func TestIntegration_GetBlob_blobReadFails(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	mem := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, mem, 1024*1024)
	user, err := db.CreateUser(ctx, "art-getfail-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	ct := testPlainTextCT
	art, err := svc.CreateFromBody(ctx, uid, user.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "gf.txt", Body: []byte("y"), ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	bad := NewServiceWithBlob(db, errBlobGet{inner: mem}, 1024*1024)
	_, _, err = bad.GetBlob(ctx, uid, user.Handle, art.ID)
	if err == nil {
		t.Fatal("expected GetObject error")
	}
}

func TestIntegration_Delete_blobDeleteFails(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	mem := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, mem, 1024*1024)
	user, err := db.CreateUser(ctx, "art-delfail-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	ct := testPlainTextCT
	art, err := svc.CreateFromBody(ctx, uid, user.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "del-blob.txt", Body: []byte("x"), ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	bad := NewServiceWithBlob(db, errBlobDelete{inner: mem}, 1024*1024)
	if err := bad.Delete(ctx, uid, user.Handle, art.ID); err == nil {
		t.Fatal("expected DeleteObject error")
	}
}

func TestIntegration_UpdateBlob_putFails(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	mem := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, mem, 1024*1024)
	user, err := db.CreateUser(ctx, "art-updfail-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	ct := testPlainTextCT
	art, err := svc.CreateFromBody(ctx, uid, user.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "upd-blob.txt", Body: []byte("x"), ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	bad := NewServiceWithBlob(db, errBlobPut{inner: mem}, 1024*1024)
	if _, err := bad.UpdateBlob(ctx, uid, user.Handle, art.ID, []byte("y"), &ct, nil); err == nil {
		t.Fatal("expected PutObject error")
	}
}

func TestIntegration_ServiceUserScopeReadGrant(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	blob := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, blob, 1024*1024)

	owner, err := db.CreateUser(ctx, "art-grant-a-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	other, err := db.CreateUser(ctx, "art-grant-b-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser other: %v", err)
	}
	uid := owner.ID
	body := []byte("granted-read")
	ct := testPlainTextCT
	art, err := svc.CreateFromBody(ctx, uid, owner.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "grant/read.txt", Body: body, ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	if _, _, err := svc.GetBlob(ctx, other.ID, other.Handle, art.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("before grant: %v", err)
	}
	if err := db.GrantArtifactRead(ctx, art.ID, other.ID); err != nil {
		t.Fatalf("GrantArtifactRead: %v", err)
	}
	got, _, err := svc.GetBlob(ctx, other.ID, other.Handle, art.ID)
	if err != nil {
		t.Fatalf("GetBlob after grant: %v", err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("blob mismatch")
	}
}

func TestIntegration_SystemDeleteArtifact(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	blob := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, blob, 1024)

	user, err := db.CreateUser(ctx, "art-sysdel-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	ct := testPlainTextCT
	art, err := svc.CreateFromBody(ctx, uid, user.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "sysdel.txt", Body: []byte("x"), ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	if err := svc.SystemDeleteArtifact(ctx, art.ID); err != nil {
		t.Fatalf("SystemDeleteArtifact: %v", err)
	}
	if _, err := db.GetOrchestratorArtifactByID(ctx, art.ID); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("after system delete: %v", err)
	}
}

func TestIntegration_BackfillAndPrune(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	blob := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, blob, 1024*1024)

	user, err := db.CreateUser(ctx, "art-bf-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	ct := testPlainTextCT
	_, err = svc.CreateFromBody(ctx, uid, user.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "bf.txt", Body: []byte("x"), ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	if _, err := svc.BackfillMissingHashesOnce(ctx, 5); err != nil {
		t.Fatalf("BackfillMissingHashesOnce: %v", err)
	}

	n, err := svc.PruneStaleByMaxAgeOnce(ctx, 0, 10)
	if err != nil || n != 0 {
		t.Fatalf("Prune with zero max age: n=%d err=%v", n, err)
	}
}

func TestIntegration_MCPGet_crossUserForbidden(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc := NewServiceWithBlob(db, s3blob.NewMemStore(), 1024*1024)
	alice, err := db.CreateUser(ctx, "mcp-xa-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	bob, err := db.CreateUser(ctx, "mcp-xb-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := svc.MCPPut(ctx, alice.ID, alice.Handle, map[string]interface{}{
		"path":                 "priv-mcp.txt",
		"scope":                "user",
		"content_bytes_base64": "Zm9v",
		"user_id":              alice.ID.String(),
	}); err != nil {
		t.Fatalf("MCPPut: %v", err)
	}
	_, err = svc.MCPGet(ctx, bob.ID, bob.Handle, map[string]interface{}{
		"path":    "priv-mcp.txt",
		"scope":   "user",
		"user_id": alice.ID.String(),
	})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("MCPGet: %v", err)
	}
}

func TestIntegration_MCP_argValidation(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	svc := NewServiceWithBlob(db, s3blob.NewMemStore(), 1024*1024)
	u, err := db.CreateUser(ctx, "mcp-arg-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := u.ID
	if _, err := svc.MCPPut(ctx, uid, u.Handle, map[string]interface{}{}); err == nil {
		t.Fatal("MCPPut missing fields")
	}
	if _, err := svc.MCPGet(ctx, uid, u.Handle, map[string]interface{}{"path": "x"}); err == nil {
		t.Fatal("MCPGet missing scope")
	}
	if _, err := svc.MCPGet(ctx, uid, u.Handle, map[string]interface{}{"scope": "user"}); err == nil {
		t.Fatal("MCPGet missing path")
	}
	if _, err := svc.MCPList(ctx, uid, u.Handle, map[string]interface{}{}); err == nil {
		t.Fatal("MCPList missing scope")
	}
	if _, err := svc.MCPPut(ctx, uid, u.Handle, map[string]interface{}{
		"path": "b", "scope": "project", "content_bytes_base64": "Zm9v",
	}); err == nil {
		t.Fatal("MCPPut project scope without project_id")
	}
	if _, err := svc.MCPPut(ctx, uid, u.Handle, map[string]interface{}{
		"path": "bad/../x", "scope": "user", "content_bytes_base64": "Zm9v",
		"user_id": uid.String(),
	}); err == nil {
		t.Fatal("MCPPut invalid path")
	}
	if _, err := svc.MCPPut(ctx, uid, u.Handle, map[string]interface{}{
		"path": "c", "scope": "nope", "content_bytes_base64": "Zm9v",
	}); err == nil {
		t.Fatal("MCPPut invalid scope level")
	}
}

func TestIntegration_MCPPutGetList(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	blob := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, blob, 1024*1024)

	user, err := db.CreateUser(ctx, "art-mcp-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	args := map[string]interface{}{
		"path":                 "mcp/x.txt",
		"scope":                "user",
		"content_bytes_base64": "Zm9v", // "foo"
		"content_type":         testPlainTextCT,
		"user_id":              uid.String(),
	}
	if _, err := svc.MCPPut(ctx, uid, user.Handle, args); err != nil {
		t.Fatalf("MCPPut: %v", err)
	}
	out, err := svc.MCPGet(ctx, uid, user.Handle, map[string]interface{}{
		"path":    "mcp/x.txt",
		"scope":   "user",
		"user_id": uid.String(),
	})
	if err != nil {
		t.Fatalf("MCPGet: %v", err)
	}
	if out["status"] != "success" {
		t.Fatalf("MCPGet status: %v", out)
	}
	listOut, err := svc.MCPList(ctx, uid, user.Handle, map[string]interface{}{
		"scope":   "user",
		"user_id": uid.String(),
		"limit":   float64(50),
	})
	if err != nil {
		t.Fatalf("MCPList: %v", err)
	}
	if listOut["status"] != "success" {
		t.Fatalf("MCPList: %v", listOut)
	}

	// Second put same path exercises MCPPut upsert (UpdateBlob path).
	args["content_bytes_base64"] = "YmFy" // "bar"
	if _, err := svc.MCPPut(ctx, uid, user.Handle, args); err != nil {
		t.Fatalf("MCPPut upsert: %v", err)
	}
}

func TestIntegration_BackfillPopulatesChecksum(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	blob := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, blob, 1024)

	user, err := db.CreateUser(ctx, "art-bf-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	ct := testPlainTextCT
	art, err := svc.CreateFromBody(ctx, uid, user.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "bf-check.txt", Body: []byte("hello"), ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	if err := db.GORM().WithContext(ctx).Model(&database.OrchestratorArtifactRecord{}).
		Where("id = ?", art.ID).
		Update("checksum_sha256", nil).Error; err != nil {
		t.Fatalf("clear checksum: %v", err)
	}
	n, err := svc.BackfillMissingHashesOnce(ctx, 20)
	if err != nil {
		t.Fatalf("BackfillMissingHashesOnce: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected backfill updates, got %d", n)
	}
	got, err := db.GetOrchestratorArtifactByID(ctx, art.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ChecksumSHA256 == nil || *got.ChecksumSHA256 == "" {
		t.Fatal("expected checksum after backfill")
	}
}

func TestIntegration_PruneStale_skipsOnBlobDeleteFailure(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	mem := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, mem, 1024)
	user, err := db.CreateUser(ctx, "art-prune-skip-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	ct := testPlainTextCT
	art, err := svc.CreateFromBody(ctx, uid, user.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "prune-skip.txt", Body: []byte("z"), ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	past := time.Now().UTC().Add(-48 * time.Hour)
	if err := db.GORM().WithContext(ctx).Model(&database.OrchestratorArtifactRecord{}).
		Where("id = ?", art.ID).Update("created_at", past).Error; err != nil {
		t.Fatalf("backdate created_at: %v", err)
	}
	bad := NewServiceWithBlob(db, errBlobDelete{inner: mem}, 1024)
	n, err := bad.PruneStaleByMaxAgeOnce(ctx, time.Hour, 10)
	if err != nil {
		t.Fatalf("PruneStaleByMaxAgeOnce: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected no successful deletes when blob delete fails, got n=%d", n)
	}
}

func TestIntegration_PruneStaleByMaxAge_deletesOldRows(t *testing.T) {
	ctx := context.Background()
	db := tcArtifactsDB(t, ctx)
	blob := s3blob.NewMemStore()
	svc := NewServiceWithBlob(db, blob, 1024)
	user, err := db.CreateUser(ctx, "art-prune-"+uuid.New().String()[:8], nil)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	uid := user.ID
	ct := testPlainTextCT
	art, err := svc.CreateFromBody(ctx, uid, user.Handle, &CreateFromBodyInput{
		Level: "user", OwnerUserID: &uid, ArtifactPath: "old.txt", Body: []byte("z"), ContentType: &ct,
	})
	if err != nil {
		t.Fatalf("CreateFromBody: %v", err)
	}
	past := time.Now().UTC().Add(-48 * time.Hour)
	if err := db.GORM().WithContext(ctx).Model(&database.OrchestratorArtifactRecord{}).
		Where("id = ?", art.ID).Update("created_at", past).Error; err != nil {
		t.Fatalf("backdate created_at: %v", err)
	}
	n, err := svc.PruneStaleByMaxAgeOnce(ctx, time.Hour, 10)
	if err != nil {
		t.Fatalf("PruneStaleByMaxAgeOnce: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected prune, got n=%d", n)
	}
	if _, err := db.GetOrchestratorArtifactByID(ctx, art.ID); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("after prune: %v", err)
	}
}
