package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/testutil"
)

func TestDevResetPMASessionStateHandler_Unauthorized(t *testing.T) {
	db := testutil.NewMockDB()
	h := DevResetPMASessionStateHandler(db, "correct-psk", nil)
	req := httptest.NewRequest(http.MethodPost, "/internal/dev/reset-pma-session-state", http.NoBody)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDevResetPMASessionStateHandler_Errors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		psk    string
		method string
		want   int
	}{
		{name: "disabled_when_psk_empty", psk: "", method: http.MethodPost, want: http.StatusNotFound},
		{name: "method_not_allowed", psk: "psk", method: http.MethodGet, want: http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := testutil.NewMockDB()
			h := DevResetPMASessionStateHandler(db, tc.psk, nil)
			req := httptest.NewRequest(tc.method, "/internal/dev/reset-pma-session-state", http.NoBody)
			rec := httptest.NewRecorder()
			h(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("code=%d want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestDevResetPMASessionStateHandler_OK(t *testing.T) {
	db := testutil.NewMockDB()
	h := DevResetPMASessionStateHandler(db, "psk", nil)
	req := httptest.NewRequest(http.MethodPost, "/internal/dev/reset-pma-session-state", strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer psk")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code=%d want %d body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
}

func TestDevResetPMASessionState_ListBindingsError(t *testing.T) {
	db := testutil.NewMockDB()
	db.ForceError = errors.New("list failed")
	defer func() { db.ForceError = nil }()
	if err := DevResetPMASessionState(context.Background(), db, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestDevResetPMASessionStateHandler_InternalError(t *testing.T) {
	db := testutil.NewMockDB()
	db.ForceError = errors.New("list failed")
	defer func() { db.ForceError = nil }()
	h := DevResetPMASessionStateHandler(db, "psk", nil)
	req := httptest.NewRequest(http.MethodPost, "/internal/dev/reset-pma-session-state", http.NoBody)
	req.Header.Set("Authorization", "Bearer psk")
	rec := httptest.NewRecorder()
	h(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code=%d want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestExtractBearerTokenDevReset(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	if extractBearerTokenDevReset(req) != "" {
		t.Fatal("empty auth")
	}
	req.Header.Set("Authorization", "Basic x")
	if extractBearerTokenDevReset(req) != "" {
		t.Fatal("not bearer")
	}
	req.Header.Set("Authorization", "Bear")
	if extractBearerTokenDevReset(req) != "" {
		t.Fatal("short prefix")
	}
	req.Header.Set("Authorization", "Bearer  tok")
	if got := extractBearerTokenDevReset(req); got != "tok" {
		t.Fatalf("got %q", got)
	}
}
