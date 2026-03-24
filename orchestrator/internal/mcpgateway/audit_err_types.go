package mcpgateway

// Common MCP tool audit error_type values as package-level strings so assignments can use *string without a pointer helper.
var (
	auditErrConflict           = "conflict"
	auditErrInternalError      = "internal_error"
	auditErrNotImplemented     = "not_implemented"
	auditErrNotFound           = "not_found"
	auditErrInvalidArguments   = "invalid_arguments"
	auditErrServiceUnavailable = "service_unavailable"
	auditErrForbidden          = "forbidden"
	auditErrPolicyViolation    = "policy_violation"
)
