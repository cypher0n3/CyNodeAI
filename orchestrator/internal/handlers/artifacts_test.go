package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/artifacts"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
	"github.com/google/uuid"
)

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeArtSvc implements artifacts.HandlerAPI for handler tests.
type fakeArtSvc struct {
	createErr error
	createArt *models.OrchestratorArtifact

	getData []byte
	getArt  *models.OrchestratorArtifact
	getErr  error

	updateErr error
	updateArt *models.OrchestratorArtifact

	deleteErr error

	listErr  error
	listArts []*models.OrchestratorArtifact
}

func (f *fakeArtSvc) CreateFromBody(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, level string,
	ownerUserID, groupID, projectID *uuid.UUID,
	artifactPath string, body []byte, contentType *string,
	createdByJobID, correlationTaskID *uuid.UUID, runID *uuid.UUID,
) (*models.OrchestratorArtifact, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.createArt, nil
}

func (f *fakeArtSvc) GetBlob(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID) ([]byte, *models.OrchestratorArtifact, error) {
	if f.getErr != nil {
		return nil, nil, f.getErr
	}
	return f.getData, f.getArt, nil
}

func (f *fakeArtSvc) UpdateBlob(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID, body []byte, contentType *string, lastModJob *uuid.UUID) (*models.OrchestratorArtifact, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return f.updateArt, nil
}

func (f *fakeArtSvc) Delete(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, id uuid.UUID) error {
	return f.deleteErr
}

func (f *fakeArtSvc) List(ctx context.Context, subjectUserID uuid.UUID, subjectHandle string, p database.ListOrchestratorArtifactsParams) ([]*models.OrchestratorArtifact, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listArts, nil
}

var _ artifacts.HandlerAPI = (*fakeArtSvc)(nil)

func reqWithUser(method, rawURL string, body io.Reader, uid uuid.UUID, handle string) *http.Request {
	req := httptest.NewRequest(method, rawURL, body)
	return req.WithContext(SetUserContext(context.Background(), uid, handle))
}

func TestArtifactsHandler_nilService_returns503(t *testing.T) {
	h := NewArtifactsHandler(nil, testLogger())
	uid := uuid.New()
	req := reqWithUser(http.MethodGet, "/v1/artifacts?scope_level=user", http.NoBody, uid, "tester")
	rec := httptest.NewRecorder()
	h.Find(rec, req)
	assertStatusCode(t, rec, http.StatusServiceUnavailable)
}

func TestArtifactsHandler_nilService_allMethods503(t *testing.T) {
	h := NewArtifactsHandler(nil, testLogger())
	uid := uuid.New()
	id := uuid.New()
	cases := []struct {
		name   string
		method string
		path   string
		body   io.Reader
		setID  bool
	}{
		{"Create", http.MethodPost, "/v1/artifacts?scope_level=user&path=p", strings.NewReader("x"), false},
		{"Read", http.MethodGet, "/v1/artifacts/x", http.NoBody, true},
		{"Update", http.MethodPut, "/v1/artifacts/x", strings.NewReader("x"), true},
		{"Delete", http.MethodDelete, "/v1/artifacts/x", http.NoBody, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := reqWithUser(tc.method, tc.path, tc.body, uid, "u1")
			if tc.setID {
				req.SetPathValue("artifact_id", id.String())
			}
			rec := httptest.NewRecorder()
			switch tc.method {
			case http.MethodPost:
				h.Create(rec, req)
			case http.MethodGet:
				h.Read(rec, req)
			case http.MethodPut:
				h.Update(rec, req)
			case http.MethodDelete:
				h.Delete(rec, req)
			default:
				t.Fatalf("bad method %s", tc.method)
			}
			assertStatusCode(t, rec, http.StatusServiceUnavailable)
		})
	}
}

func TestArtifactsHandler_Create(t *testing.T) {
	uid := uuid.New()
	artID := uuid.New()
	created := time.Unix(1700000000, 0).UTC()
	okArt := &models.OrchestratorArtifact{
		OrchestratorArtifactBase: models.OrchestratorArtifactBase{
			ScopeLevel: "user",
			Path:       "a/b.txt",
			StorageRef: "s3://b/k",
		},
		ID:        artID,
		CreatedAt: created,
	}
	sz := int64(3)
	okArt.SizeBytes = &sz
	ct := "text/plain"
	okArt.ContentType = &ct
	sum := "deadbeef"
	okArt.ChecksumSHA256 = &sum

	t.Run("success", func(t *testing.T) {
		f := &fakeArtSvc{createArt: okArt}
		h := NewArtifactsHandler(f, testLogger())
		q := url.Values{}
		q.Set("scope_level", "user")
		q.Set("path", "a/b.txt")
		req := reqWithUser(http.MethodPost, "/v1/artifacts?"+q.Encode(), strings.NewReader("hey"), uid, "u1")
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()
		h.Create(rec, req)
		assertStatusCode(t, rec, http.StatusCreated)
	})

	t.Run("post_query_variants", func(t *testing.T) {
		variants := []struct {
			name string
			raw  string
			want int
		}{
			{"missing_scope_level", "/v1/artifacts?path=x", http.StatusBadRequest},
			{"missing_path", "/v1/artifacts?scope_level=user", http.StatusBadRequest},
			{"filename_query_param", "/v1/artifacts?scope_level=user&filename=viaq.txt", http.StatusCreated},
			{"user_scope_defaults_owner", "/v1/artifacts?scope_level=user&path=p", http.StatusCreated},
		}
		for _, v := range variants {
			t.Run(v.name, func(t *testing.T) {
				f := &fakeArtSvc{createArt: okArt}
				h := NewArtifactsHandler(f, testLogger())
				req := reqWithUser(http.MethodPost, v.raw, strings.NewReader("x"), uid, "u1")
				rec := httptest.NewRecorder()
				h.Create(rec, req)
				assertStatusCode(t, rec, v.want)
			})
		}
	})

	t.Run("path_from_x_filename", func(t *testing.T) {
		f := &fakeArtSvc{createArt: okArt}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodPost, "/v1/artifacts?scope_level=user", strings.NewReader("x"), uid, "u1")
		req.Header.Set("X-Filename", "from-header.txt")
		rec := httptest.NewRecorder()
		h.Create(rec, req)
		assertStatusCode(t, rec, http.StatusCreated)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		f := &fakeArtSvc{createArt: okArt}
		h := NewArtifactsHandler(f, testLogger())
		req := httptest.NewRequest(http.MethodPost, "/v1/artifacts?scope_level=user&path=p", strings.NewReader("x"))
		rec := httptest.NewRecorder()
		h.Create(rec, req)
		assertStatusCode(t, rec, http.StatusUnauthorized)
	})

	t.Run("svc_errors", func(t *testing.T) {
		errCases := []struct {
			name string
			err  error
			want int
		}{
			{"err_not_found", database.ErrNotFound, http.StatusNotFound},
			{"err_exists", database.ErrExists, http.StatusConflict},
			{"err_forbidden", artifacts.ErrForbidden, http.StatusForbidden},
			{"err_forbidden_string", errors.New("Forbidden by policy"), http.StatusForbidden},
			{"err_internal", errors.New("boom"), http.StatusInternalServerError},
		}
		for _, ec := range errCases {
			t.Run(ec.name, func(t *testing.T) {
				f := &fakeArtSvc{createErr: ec.err}
				h := NewArtifactsHandler(f, testLogger())
				req := reqWithUser(http.MethodPost, "/v1/artifacts?scope_level=user&path=p", strings.NewReader("x"), uid, "u1")
				rec := httptest.NewRecorder()
				h.Create(rec, req)
				assertStatusCode(t, rec, ec.want)
			})
		}
	})

	t.Run("body_read_error", func(t *testing.T) {
		f := &fakeArtSvc{createArt: okArt}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodPost, "/v1/artifacts?scope_level=user&path=p", io.NopCloser(errReader{}), uid, "u1")
		rec := httptest.NewRecorder()
		h.Create(rec, req)
		assertStatusCode(t, rec, http.StatusBadRequest)
	})

}

func TestArtifactsHandler_ReadUpdateDelete(t *testing.T) {
	uid := uuid.New()
	id := uuid.New()
	ct := "application/json"
	art := &models.OrchestratorArtifact{
		OrchestratorArtifactBase: models.OrchestratorArtifactBase{
			Path:        "f.json",
			ContentType: &ct,
		},
		ID: id,
	}

	t.Run("read_ok", func(t *testing.T) {
		f := &fakeArtSvc{getData: []byte("{}"), getArt: art}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodGet, "/v1/artifacts/"+id.String(), http.NoBody, uid, "u1")
		req.SetPathValue("artifact_id", id.String())
		rec := httptest.NewRecorder()
		h.Read(rec, req)
		assertStatusCode(t, rec, http.StatusOK)
		if got := rec.Header().Get("Content-Type"); got != ct {
			t.Fatalf("Content-Type: got %q, want %q", got, ct)
		}
	})

	t.Run("read_default_octet_stream", func(t *testing.T) {
		plain := &models.OrchestratorArtifact{OrchestratorArtifactBase: models.OrchestratorArtifactBase{Path: "b"}, ID: id}
		f := &fakeArtSvc{getData: []byte("x"), getArt: plain}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodGet, "/v1/artifacts/"+id.String(), http.NoBody, uid, "u1")
		req.SetPathValue("artifact_id", id.String())
		rec := httptest.NewRecorder()
		h.Read(rec, req)
		assertStatusCode(t, rec, http.StatusOK)
		if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("Content-Type: got %q", got)
		}
	})

	t.Run("read_invalid_id", func(t *testing.T) {
		f := &fakeArtSvc{}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodGet, "/v1/artifacts/not-a-uuid", http.NoBody, uid, "u1")
		req.SetPathValue("artifact_id", "not-a-uuid")
		rec := httptest.NewRecorder()
		h.Read(rec, req)
		assertStatusCode(t, rec, http.StatusBadRequest)
	})

	t.Run("update_ok", func(t *testing.T) {
		up := *art
		f := &fakeArtSvc{updateArt: &up}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodPut, "/v1/artifacts/"+id.String(), strings.NewReader("[]"), uid, "u1")
		req.SetPathValue("artifact_id", id.String())
		rec := httptest.NewRecorder()
		h.Update(rec, req)
		assertStatusCode(t, rec, http.StatusOK)
	})

	t.Run("delete_no_content", func(t *testing.T) {
		f := &fakeArtSvc{}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodDelete, "/v1/artifacts/"+id.String(), http.NoBody, uid, "u1")
		req.SetPathValue("artifact_id", id.String())
		rec := httptest.NewRecorder()
		h.Delete(rec, req)
		assertStatusCode(t, rec, http.StatusNoContent)
	})

	t.Run("path_value_id_fallback", func(t *testing.T) {
		f := &fakeArtSvc{getData: []byte("z"), getArt: art}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodGet, "/v1/artifacts/x", http.NoBody, uid, "u1")
		req.SetPathValue("id", id.String())
		rec := httptest.NewRecorder()
		h.Read(rec, req)
		assertStatusCode(t, rec, http.StatusOK)
	})

	t.Run("read_delete_svc_errors", func(t *testing.T) {
		cases := []struct {
			name   string
			method string
			f      *fakeArtSvc
			run    func(*ArtifactsHandler, *httptest.ResponseRecorder, *http.Request)
		}{
			{"read_getblob_err", http.MethodGet, &fakeArtSvc{getErr: errors.New("s3 down")}, func(h *ArtifactsHandler, rec *httptest.ResponseRecorder, req *http.Request) { h.Read(rec, req) }},
			{"delete_svc_err", http.MethodDelete, &fakeArtSvc{deleteErr: errors.New("fail")}, func(h *ArtifactsHandler, rec *httptest.ResponseRecorder, req *http.Request) { h.Delete(rec, req) }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				h := NewArtifactsHandler(tc.f, testLogger())
				req := reqWithUser(tc.method, "/v1/artifacts/x", http.NoBody, uid, "u1")
				req.SetPathValue("artifact_id", id.String())
				rec := httptest.NewRecorder()
				tc.run(h, rec, req)
				assertStatusCode(t, rec, http.StatusInternalServerError)
			})
		}
	})

	t.Run("read_content_type_empty_string_uses_octet_stream", func(t *testing.T) {
		empty := ""
		plain := &models.OrchestratorArtifact{
			OrchestratorArtifactBase: models.OrchestratorArtifactBase{Path: "b", ContentType: &empty},
			ID:                       id,
		}
		f := &fakeArtSvc{getData: []byte("x"), getArt: plain}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodGet, "/v1/artifacts/x", http.NoBody, uid, "u1")
		req.SetPathValue("artifact_id", id.String())
		rec := httptest.NewRecorder()
		h.Read(rec, req)
		assertStatusCode(t, rec, http.StatusOK)
		if got := rec.Header().Get("Content-Type"); got != "application/octet-stream" {
			t.Fatalf("Content-Type: got %q", got)
		}
	})

	t.Run("update_body_err", func(t *testing.T) {
		f := &fakeArtSvc{updateArt: art}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodPut, "/v1/artifacts/x", io.NopCloser(errReader{}), uid, "u1")
		req.SetPathValue("artifact_id", id.String())
		rec := httptest.NewRecorder()
		h.Update(rec, req)
		assertStatusCode(t, rec, http.StatusBadRequest)
	})

	t.Run("update_svc_err", func(t *testing.T) {
		f := &fakeArtSvc{updateErr: errors.New("fail")}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodPut, "/v1/artifacts/x", strings.NewReader("[]"), uid, "u1")
		req.SetPathValue("artifact_id", id.String())
		rec := httptest.NewRecorder()
		h.Update(rec, req)
		assertStatusCode(t, rec, http.StatusInternalServerError)
	})
}

func TestArtifactsHandler_Find(t *testing.T) {
	uid := uuid.New()
	aid := uuid.New()
	ct := "text/plain"
	row := &models.OrchestratorArtifact{
		OrchestratorArtifactBase: models.OrchestratorArtifactBase{
			ScopeLevel:  "user",
			Path:        "p",
			ContentType: &ct,
		},
		ID:        aid,
		CreatedAt: time.Now().UTC(),
	}
	sz := int64(1)
	row.SizeBytes = &sz

	t.Run("ok_with_pagination", func(t *testing.T) {
		f := &fakeArtSvc{listArts: []*models.OrchestratorArtifact{row}}
		h := NewArtifactsHandler(f, testLogger())
		raw := "/v1/artifacts?scope_level=user&limit=10&offset=5"
		req := reqWithUser(http.MethodGet, raw, http.NoBody, uid, "u1")
		rec := httptest.NewRecorder()
		h.Find(rec, req)
		assertStatusCode(t, rec, http.StatusOK)
	})

	t.Run("list_user_scope_owner_from_user_id_query", func(t *testing.T) {
		f := &fakeArtSvc{listArts: []*models.OrchestratorArtifact{row}}
		h := NewArtifactsHandler(f, testLogger())
		raw := "/v1/artifacts?scope_level=user&user_id=" + uid.String()
		req := reqWithUser(http.MethodGet, raw, http.NoBody, uid, "u1")
		rec := httptest.NewRecorder()
		h.Find(rec, req)
		assertStatusCode(t, rec, http.StatusOK)
	})

	t.Run("missing_scope_level", func(t *testing.T) {
		f := &fakeArtSvc{}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodGet, "/v1/artifacts", http.NoBody, uid, "u1")
		rec := httptest.NewRecorder()
		h.Find(rec, req)
		assertStatusCode(t, rec, http.StatusBadRequest)
	})

	t.Run("list_err", func(t *testing.T) {
		f := &fakeArtSvc{listErr: errors.New("db")}
		h := NewArtifactsHandler(f, testLogger())
		req := reqWithUser(http.MethodGet, "/v1/artifacts?scope_level=user", http.NoBody, uid, "u1")
		rec := httptest.NewRecorder()
		h.Find(rec, req)
		assertStatusCode(t, rec, http.StatusInternalServerError)
	})
}

func TestParseUUIDQuery(t *testing.T) {
	u := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	q := url.Values{}
	q.Set("k", u.String())
	if got := parseUUIDQuery(q, "k"); got == nil || *got != u {
		t.Fatal("expected parsed uuid")
	}
	if parseUUIDQuery(q, "missing") != nil {
		t.Fatal("want nil")
	}
	q.Set("bad", "nope")
	if parseUUIDQuery(q, "bad") != nil {
		t.Fatal("invalid should be nil")
	}
}
