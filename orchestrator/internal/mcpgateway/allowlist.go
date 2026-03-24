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
// When at least one token is non-empty, a request that includes Authorization: Bearer <token>
// is matched to an agent role and gateway allowlists apply (REQ-MCPGAT-0114, mcp_gateway_enforcement.md).
// Requests without a Bearer header remain unrestricted (legacy control-plane access for tests and ops).
//
// PM token is typically WORKER_INTERNAL_AGENT_TOKEN (PMA / managed services). Sandbox token is a
// separate credential used for worker / sandbox agent tool surfaces (MCP_SANDBOX_AGENT_BEARER_TOKEN).
type ToolCallAuth struct {
	PMToken      string
	SandboxToken string
}

// AgentRole is a coarse agent identity derived from a bearer token.
type AgentRole int

const (
	AgentRoleNone AgentRole = iota
	AgentRolePM
	AgentRoleSandbox
)

func (a *ToolCallAuth) anyTokenConfigured() bool {
	if a == nil {
		return false
	}
	return strings.TrimSpace(a.PMToken) != "" || strings.TrimSpace(a.SandboxToken) != ""
}

// resolveRole returns the agent role for a bearer token. ok is false when no configured token matches.
// PM token is checked before sandbox; configure distinct values for WORKER_INTERNAL_AGENT_TOKEN and
// MCP_SANDBOX_AGENT_BEARER_TOKEN when both are used.
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

// sandboxAllowedTools is the worker/sandbox allowlist for tools implemented on the control-plane
// gateway (see docs/tech_specs/mcp_tools/access_allowlists_and_scope.md Worker Agent Allowlist).
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
		return true
	}
	role, ok := auth.resolveRole(tok)
	if !ok {
		writeAgentAuthDeny(ctx, w, store, logger, toolName, args, start, http.StatusUnauthorized, "invalid_agent_token", `{"error":"invalid or unrecognized agent token"}`)
		return false
	}
	if role == AgentRoleSandbox && !sandboxAllowsTool(toolName) {
		writeAgentAuthDeny(ctx, w, store, logger, toolName, args, start, http.StatusForbidden, "agent_allowlist_denied", `{"error":"tool not allowed for this agent role"}`)
		return false
	}
	return true
}
