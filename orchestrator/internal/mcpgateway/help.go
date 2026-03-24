package mcpgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/cypher0n3/cynodeai/orchestrator/internal/database"
	"github.com/cypher0n3/cynodeai/orchestrator/internal/models"
)

const helpMaxBytes = 32 * 1024

var helpTopicSnippets = map[string]string{
	"tools":    "Use MCP tools via POST /v1/mcp/tools/call with tool_name and arguments. Task tools: task.get, task.list, task.result, task.cancel, task.logs. Project tools: project.get, project.list. Help: help.list (topic index), help.get (markdown).",
	"gateway":  "The MCP gateway validates scoped ids, checks allowlists, writes an audit record per call, and routes to orchestrator store handlers.",
	"projects": "Projects scope preferences and chat. The PM agent typically uses the default project per user unless a specific project_id is supplied in task or chat context.",
}

// helpOverviewDefault is embedded documentation when topic/path are omitted (MVP: no filesystem reads).
const helpOverviewDefault = `# CyNodeAI MCP Gateway (help)

Call POST /v1/mcp/tools/call with JSON: {"tool_name":"...","arguments":{...}}.
Pass user_id, job_id, or other scoped ids as required by each tool (see mcp_tools/). Help tools do not use task_id.

Task tools mirror the User API: list tasks by user, fetch task details, task result (jobs), cancel, and aggregated logs from job stdout/stderr.

Project tools return only projects authorized for the caller (MVP: the per-user default project).

This overview is served from embedded strings in the gateway binary (no external files at runtime).
`

func helpGetMarkdown(topic, path string) string {
	topic = strings.TrimSpace(strings.ToLower(topic))
	path = strings.TrimSpace(path)
	if topic != "" {
		if s, ok := helpTopicSnippets[topic]; ok {
			return truncateHelp(s)
		}
	}
	if path != "" {
		return truncateHelp(helpOverviewDefault + "\n\nRequested path is informational only in MVP; use topic keys such as tools, gateway, projects.")
	}
	return truncateHelp(helpOverviewDefault)
}

func truncateHelp(s string) string {
	if len(s) <= helpMaxBytes {
		return s
	}
	return s[:helpMaxBytes]
}

// handleHelpList returns embedded help topic keys and short summaries. No scoped ids required.
func handleHelpList(_ context.Context, _ database.Store, _ map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	keys := make([]string, 0, len(helpTopicSnippets))
	for k := range helpTopicSnippets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	topics := make([]map[string]interface{}, 0, len(keys))
	for _, k := range keys {
		topics = append(topics, map[string]interface{}{
			"topic":   k,
			"summary": helpTopicSnippets[k],
		})
	}
	out := map[string]interface{}{
		"topics": topics,
		"hint":   "Call help.get with optional topic (tools, gateway, projects) or path for full markdown.",
	}
	b, err := json.Marshal(out)
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		rec.Decision = auditDecisionDeny
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}

func handleHelpGet(_ context.Context, _ database.Store, args map[string]interface{}, rec *models.McpToolCallAuditLog) (code int, body []byte, auditRec *models.McpToolCallAuditLog) {
	auditRec = rec
	topic := strArg(args, "topic")
	path := strArg(args, "path")
	content := helpGetMarkdown(topic, path)
	rec.Decision = auditDecisionAllow
	rec.Status = auditStatusSuccess
	rec.ErrorType = nil
	out := map[string]interface{}{
		"content":   content,
		"truncated": len(content) >= helpMaxBytes,
	}
	b, err := json.Marshal(out)
	if err != nil {
		rec.Status = auditStatusError
		rec.ErrorType = &auditErrInternalError
		return http.StatusInternalServerError, []byte(`{"error":"internal error"}`), auditRec
	}
	return http.StatusOK, b, auditRec
}
