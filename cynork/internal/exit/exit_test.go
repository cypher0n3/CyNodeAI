package exit

import (
	"errors"
	"testing"
)

func TestError_Error(t *testing.T) {
	t.Run("with err", func(t *testing.T) {
		e := New(CodeAuth, errors.New("bad token"))
		if got := e.Error(); got != "bad token" {
			t.Errorf("Error() = %q, want bad token", got)
		}
	})
	t.Run("nil err", func(t *testing.T) {
		e := New(CodeAuth, nil)
		if got := e.Error(); got != "exit 3" {
			t.Errorf("Error() = %q, want exit 3", got)
		}
	})
}

func TestError_Unwrap(t *testing.T) {
	inner := errors.New("inner")
	e := New(CodeUsage, inner)
	if got := e.Unwrap(); got != inner {
		t.Errorf("Unwrap() = %v, want inner", got)
	}
}

func TestNew(t *testing.T) {
	e := New(CodeNotFound, errors.New("missing"))
	if e.Code != CodeNotFound || e.Err == nil {
		t.Errorf("New: code=%d err=%v", e.Code, e.Err)
	}
}

func TestHelpers(t *testing.T) {
	err := errors.New("x")
	tests := []struct {
		name string
		fn   func(error) *Error
		code int
	}{
		{"Usage", Usage, CodeUsage},
		{"Auth", Auth, CodeAuth},
		{"NotFound", NotFound, CodeNotFound},
		{"Conflict", Conflict, CodeConflict},
		{"Validation", Validation, CodeValidation},
		{"Gateway", Gateway, CodeGateway},
		{"Internal", Internal, CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.fn(err)
			if e == nil || e.Code != tt.code || e.Err != err {
				t.Errorf("%s: got %+v", tt.name, e)
			}
		})
	}
}

func TestCodeOf(t *testing.T) {
	if got := CodeOf(nil); got != CodeSuccess {
		t.Errorf("CodeOf(nil) = %d, want %d", got, CodeSuccess)
	}
	if got := CodeOf(errors.New("other")); got != 1 {
		t.Errorf("CodeOf(other) = %d, want 1", got)
	}
	if got := CodeOf(Auth(errors.New("x"))); got != CodeAuth {
		t.Errorf("CodeOf(Auth) = %d, want %d", got, CodeAuth)
	}
}
