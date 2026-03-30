package httplimits

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWrapRequestBody_LimitsRead(t *testing.T) {
	w := httptest.NewRecorder()
	payload := strings.Repeat("a", int(DefaultMaxAPIRequestBodyBytes)+100)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	WrapRequestBody(w, r, DefaultMaxAPIRequestBodyBytes)
	var buf strings.Builder
	_, err := io.Copy(&buf, r.Body)
	if err != nil && !strings.Contains(err.Error(), "request body too large") {
		t.Fatalf("unexpected err: %v", err)
	}
	if int64(buf.Len()) > DefaultMaxAPIRequestBodyBytes {
		t.Fatalf("read %d bytes, want at most %d", buf.Len(), DefaultMaxAPIRequestBodyBytes)
	}
}

func TestLimitBody_InvokesNext(t *testing.T) {
	called := false
	h := LimitBody(100, func(w http.ResponseWriter, r *http.Request) {
		called = true
		b, _ := io.ReadAll(r.Body)
		if string(b) != "ok" {
			t.Errorf("body %q", b)
		}
	})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("ok"))
	h(w, r)
	if !called {
		t.Fatal("next not called")
	}
}

func TestLimitResponseReader(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("hello world")),
	}
	r := LimitResponseReader(resp, 4)
	var out strings.Builder
	_, _ = io.Copy(&out, r)
	if out.String() != "hell" {
		t.Fatalf("got %q", out.String())
	}
}
