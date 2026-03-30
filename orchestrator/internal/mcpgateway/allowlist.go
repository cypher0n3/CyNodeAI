package mcpgateway

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
)

// ToolCallAuth configures optional bearer-token identity for POST /v1/mcp/tools/call.
// When at least one token is non-empty, callers must send Authorization: Bearer <token>
// matching a configured identity; allowlists apply per role (REQ-MCPGAT-0114, mcp_gateway_enforcement.md).
//
// PM token is typically WORKER_INTERNAL_AGENT_TOKEN (PMA / managed services). Sandbox token is
// MCP_SANDBOX_AGENT_BEARER_TOKEN. PA token is MCP_PA_AGENT_BEARER_TOKEN (Project Analyst).
type ToolCallAuth struct {
	PMToken      string
	SandboxToken string
	PAToken      string
}

// AgentRole is a coarse agent identity derived from a bearer token.
type AgentRole int

const (
	AgentRoleNone AgentRole = iota
	AgentRolePM
	AgentRoleSandbox
	AgentRolePA
)

func (a *ToolCallAuth) anyTokenConfigured() bool {
	if a == nil {
		return false
	}
	return strings.TrimSpace(a.PMToken) != "" ||
		strings.TrimSpace(a.SandboxToken) != "" ||
		strings.TrimSpace(a.PAToken) != ""
}

// resolveRole returns the agent role for a bearer token. ok is false when no configured token matches.
// Order: PM, sandbox, PA (use distinct secret values for each).
func (a *ToolCallAuth) resolveRole(bearer string) (AgentRole, bool) {
	if a == nil {
		return AgentRoleNone, false
	}
	bearer = strings.TrimSpace(bearer)
	if bearer == "" {
		return AgentRoleNone, false
	}
	pm := strings.TrimSpace(a.PMToken)
	if pm != "" && tokensEqualConstantTime(bearer, pm) {
		return AgentRolePM, true
	}
	sand := strings.TrimSpace(a.SandboxToken)
	if sand != "" && tokensEqualConstantTime(bearer, sand) {
		return AgentRoleSandbox, true
	}
	pa := strings.TrimSpace(a.PAToken)
	if pa != "" && tokensEqualConstantTime(bearer, pa) {
		return AgentRolePA, true
	}
	return AgentRoleNone, false
}

func tokensEqualConstantTime(a, b string) bool {
	aa := []byte(a)
	bb := []byte(b)
	if len(aa) != len(bb) {
		return false
	}
	return subtle.ConstantTimeCompare(aa, bb) == 1
}

func parseBearerToken(h string) string {
	h = strings.TrimSpace(h)
	const prefix = "Bearer "
	if len(h) < len(prefix) {
		return ""
	}
	if !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// sandboxAllowedTools is the worker/sandbox allowlist (exact tool names).
// See docs/tech_specs/mcp_tools/access_allowlists_and_scope.md Worker Agent Allowlist.
var sandboxAllowedTools = map[string]struct{}{
	"help.list":            {},
	"help.get":             {},
	"preference.get":       {},
	"preference.list":      {},
	"preference.effective": {},
	"artifact.get":         {},
	"skills.list":          {},
	"skills.get":           {},
}

func sandboxAllowsTool(toolName string) bool {
	_, ok := sandboxAllowedTools[toolName]
	return ok
}

// pmToolPrefixes covers the Project Manager agent surface (prefix match).
// See docs/tech_specs/mcp_tools/access_allowlists_and_scope.md Project Manager Agent Allowlist.
var pmToolPrefixes = []string{
	"task.", "project.", "preference.", "job.", "system_setting.", "specification.",
	"node.", "sandbox.", "artifact.", "artifacts.", "model.", "connector.",
	"web.", "api.", "git.", "help.", "persona.", "skills.",
}

// paToolPrefixes covers the Project Analyst agent surface (prefix match; narrower than PM).
var paToolPrefixes = []string{
	"task.", "project.", "preference.", "job.", "artifact.", "artifacts.", "help.", "persona.", "skills.",
}

var paBlockedPrefixes = []string{
	"node.", "sandbox.", "system_setting.", "connector.", "model.",
}

func hasAllowedToolPrefix(toolName string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(toolName, p) {
			return true
		}
	}
	return false
}

func pmAllowsTool(toolName string) bool {
	return hasAllowedToolPrefix(toolName, pmToolPrefixes)
}

func paAllowsTool(toolName string) bool {
	for _, p := range paBlockedPrefixes {
		if strings.HasPrefix(toolName, p) {
			return false
		}
	}
	if toolName == "task.cancel" {
		return false
	}
	return hasAllowedToolPrefix(toolName, paToolPrefixes)
}

func roleAllowsTool(role AgentRole, toolName string) bool {
	switch role {
	case AgentRolePM:
		return pmAllowsTool(toolName)
	case AgentRoleSandbox:
		return sandboxAllowsTool(toolName)
	case AgentRolePA:
		return paAllowsTool(toolName)
	default:
		return false
	}
}

func writeAgentAuthDeny(ctx context.Context, w http.ResponseWriter, store database.Store, logger *slog.Logger, toolName string, args map[string]interface{}, start time.Time, httpStatus int, errorType, jsonBody string) {
	rec := newDenyAuditRecord(toolName, errorType, args)
	ms := int(time.Since(start).Milliseconds())
	rec.DurationMs = &ms
	if err := store.CreateMcpToolCallAuditLog(ctx, rec); err != nil {
		logger.Error("create mcp tool call audit log", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_, _ = w.Write([]byte(jsonBody))
}

// tryAgentAllowlist returns false if the handler already wrote a response (401/403).
func tryAgentAllowlist(
	ctx context.Context,
	w http.ResponseWriter,
	r *http.Request,
	store database.Store,
	logger *slog.Logger,
	toolName string,
	args map[string]interface{},
	auth *ToolCallAuth,
	start time.Time,
) bool {
	if auth == nil || !auth.anyTokenConfigured() {
		return true
	}
	tok := parseBearerToken(r.Header.Get("Authorization"))
	if tok == "" {
		writeAgentAuthDeny(ctx, w, store, logger, toolName, args, start, http.StatusUnauthorized,
			"missing_bearer", `{"error":"authentication required"}`)
		return false
	}
	role, ok := auth.resolveRole(tok)
	if !ok {
		writeAgentAuthDeny(ctx, w, store, logger, toolName, args, start, http.StatusUnauthorized, "invalid_agent_token", `{"error":"invalid or unrecognized agent token"}`)
		return false
	}
	if !roleAllowsTool(role, toolName) {
		writeAgentAuthDeny(ctx, w, store, logger, toolName, args, start, http.StatusForbidden, "agent_allowlist_denied", `{"error":"tool not allowed for this agent role"}`)
		return false
	}
	return true
}
