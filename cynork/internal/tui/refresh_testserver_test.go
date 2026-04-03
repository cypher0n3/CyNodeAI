package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testGatewayAuthRefreshPath = "/v1/auth/refresh"

// testStubRefreshedAccessToken is the access_token string returned by newStubRefreshHTTPServer.
const testStubRefreshedAccessToken = "new"

// newStubRefreshHTTPServer serves POST testGatewayAuthRefreshPath with a fixed JSON token payload (dupl-safe helper).
func newStubRefreshHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testGatewayAuthRefreshPath && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"` + testStubRefreshedAccessToken + `","refresh_token":"nr","expires_in":3600}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	return srv
}
