// Package exit provides exit codes per CLI spec (usage, auth, not found, etc.).
// See docs/tech_specs/cli_management_app.md (Exit Codes).
package exit

import "fmt"

// Codes per spec: 0 success, 2 usage, 3 auth, 4 not found, 5 conflict, 6 validation, 7 gateway, 8 internal.
const (
	CodeSuccess    = 0
	CodeUsage      = 2
	CodeAuth       = 3
	CodeNotFound   = 4
	CodeConflict   = 5
	CodeValidation = 6
	CodeGateway    = 7
	CodeInternal   = 8
)

// Error carries an exit code and underlying error.
type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit %d", e.Code)
}

func (e *Error) Unwrap() error { return e.Err }

// New returns an exit error with the given code and optional err.
func New(code int, err error) *Error {
	return &Error{Code: code, Err: err}
}

// Usage returns exit code 2 (usage error).
func Usage(err error) *Error { return New(CodeUsage, err) }

// Auth returns exit code 3 (auth/authz error).
func Auth(err error) *Error { return New(CodeAuth, err) }

// NotFound returns exit code 4 (404).
func NotFound(err error) *Error { return New(CodeNotFound, err) }

// Conflict returns exit code 5 (409).
func Conflict(err error) *Error { return New(CodeConflict, err) }

// Validation returns exit code 6 (400/422).
func Validation(err error) *Error { return New(CodeValidation, err) }

// Gateway returns exit code 7 (5xx / network).
func Gateway(err error) *Error { return New(CodeGateway, err) }

// Internal returns exit code 8 (unexpected CLI failure).
func Internal(err error) *Error { return New(CodeInternal, err) }

// CodeOf returns the exit code for err: if err is *Error use its code, else 1.
func CodeOf(err error) int {
	if err == nil {
		return CodeSuccess
	}
	if ex, ok := err.(*Error); ok {
		return ex.Code
	}
	return 1
}
