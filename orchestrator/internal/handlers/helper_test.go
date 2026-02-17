package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
)

// recordedRequest returns a new request and response recorder for the given method, path, and body.
func recordedRequest(method, path string, body []byte) (*http.Request, *httptest.ResponseRecorder) {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	} else {
		r = http.NoBody
	}
	req := httptest.NewRequest(method, path, r)
	rec := httptest.NewRecorder()
	return req, rec
}

// recordedRequestJSON marshals v to JSON and returns a request and recorder.
func recordedRequestJSON(method, path string, v interface{}) (*http.Request, *httptest.ResponseRecorder) {
	body, _ := json.Marshal(v)
	return recordedRequest(method, path, body)
}

// assertStatusCode fails the test if rec.Code != want.
func assertStatusCode(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Errorf("status: got %d, want %d", rec.Code, want)
	}
}

// runHandlerTest runs a request through hf and asserts the response status. Use for simple status-only tests.
func runHandlerTest(t *testing.T, method, path string, body []byte, hf func(http.ResponseWriter, *http.Request), wantStatus int) {
	t.Helper()
	req, rec := recordedRequest(method, path, body)
	hf(rec, req)
	assertStatusCode(t, rec, wantStatus)
}

// roundTripJSON marshals v and unmarshals into dest; use for JSON roundtrip tests.
func roundTripJSON(t *testing.T, v, dest interface{}) {
	t.Helper()
	jsonData, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := json.Unmarshal(jsonData, dest); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
}

// requestWithUserContext returns a request and recorder with user context set; for tests that need authenticated user.
func requestWithUserContext(method, path string, body []byte, userID uuid.UUID) (*http.Request, *httptest.ResponseRecorder) {
	req, rec := recordedRequest(method, path, body)
	ctx := context.WithValue(context.Background(), contextKeyUserID, userID)
	return req.WithContext(ctx), rec
}

// testNodeCapabilityReport builds a NodeCapabilityReport for tests; avoids duplicating the struct literal.
func testNodeCapabilityReport(nodeSlug, name string, cpuCores, ramMB int) NodeCapabilityReport {
	return NodeCapabilityReport{
		Version:    1,
		ReportedAt: time.Now().UTC().Format(time.RFC3339),
		Node:       NodeCapabilityNode{NodeSlug: nodeSlug, Name: name, Labels: []string{"test"}},
		Platform:   NodeCapabilityPlatform{OS: "linux", Arch: "amd64"},
		Compute:    NodeCapabilityCompute{CPUCores: cpuCores, RAMMB: ramMB},
	}
}
