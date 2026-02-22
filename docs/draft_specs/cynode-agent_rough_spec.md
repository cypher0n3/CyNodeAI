# CyNode Agent - Sandbox Runner Technical Specification

## 1. Purpose

The `cynode-agent` is a minimal, deterministic runner process that executes structured job specifications inside containerized sandboxes managed by CyNodeAI.

It is not an LLM.
It does not perform planning.
It executes validated instructions issued by the host-side Worker API.

Primary goals:

- Deterministic execution
- Strict schema adherence
- Zero secret handling
- Strong filesystem and command constraints
- Structured, machine-parseable outputs
- Reproducibility and auditability

---

## 2. Design Principles

- Small attack surface
- No network access by default
- No embedded credentials
- Explicit allowlists
- Fail-closed behavior
- Structured JSON I/O only
- Versioned protocol contract
- Reproducible execution environment

---

## 3. Execution Model

### 3.1 Process Model

The agent runs as PID 1 (or main process) inside the sandbox container.

Invocation modes:

1. File-based mode (recommended MVP)

   - Reads `/job/job.json`
   - Writes `/job/result.json`
   - Writes artifacts under `/job/artifacts/`

2. Stdin mode (optional)

   - Reads job JSON from stdin
   - Writes result JSON to stdout

File-based mode simplifies artifact collection and debugging.

---

## 4. Job Specification (`job.json`)

### 4.1 Top-Level Schema

```json
{
  "protocol_version": "1.0",
  "job_id": "uuid",
  "task_id": "uuid",
  "skill_id": "string",
  "constraints": {
    "max_runtime_seconds": 300,
    "allowed_paths": ["/workspace"],
    "allowed_commands": ["git", "go", "golangci-lint"],
    "network_allowed": false,
    "max_output_bytes": 1048576
  },
  "steps": [
    {
      "id": "step-1",
      "type": "run_command",
      "arguments": {}
    }
  ]
}
```

---

## 5. Supported Step Types

The agent must implement only deterministic primitives.

### 5.1 Run_command_command

Executes a command.

Arguments:

```json
{
  "command": "go",
  "args": ["test", "./..."],
  "working_dir": "/workspace",
  "env": {}
}
```

Constraints:

- Command must be in allowed_commands
- working_dir must be within allowed_paths
- No shell interpretation (no `sh -c`)
- Hard timeout enforced
- Output truncated at max_output_bytes

Result:

```json
{
  "exit_code": 0,
  "stdout": "...",
  "stderr": "...",
  "duration_ms": 1200
}
```

---

### 5.2 Write_file_file

Arguments:

```json
{
  "path": "/workspace/main.go",
  "content": "file contents",
  "mode": "0644"
}
```

Constraints:

- Path must be within allowed_paths
- No symlink traversal
- Overwrite allowed only if explicitly enabled in constraints

---

### 5.3 Read_file_file

Arguments:

```json
{
  "path": "/workspace/main.go",
  "max_bytes": 65536
}
```

Constraints:

- Path must be within allowed_paths
- Hard size limit

---

### 5.4 Apply_unified_diff_unified_diff

Arguments:

```json
{
  "diff": "unified diff string"
}
```

Behavior:

- Apply patch relative to workspace root
- Reject patches that escape allowed_paths
- Reject binary patches
- Return list of files modified

---

### 5.5 List_tree_tree

Arguments:

```json
{
  "path": "/workspace",
  "max_depth": 4
}
```

Returns:

- Structured directory tree (not raw shell output)

---

## 6. Result Specification (`result.json`)

### 6.1 Top-Level Schema

```json
{
  "protocol_version": "1.0",
  "job_id": "uuid",
  "status": "success | failure | timeout",
  "started_at": "timestamp",
  "finished_at": "timestamp",
  "steps": [
    {
      "id": "step-1",
      "status": "success | failure",
      "result": {}
    }
  ],
  "artifacts": [
    {
      "path": "/workspace/report.json",
      "sha256": "hash",
      "size_bytes": 1024
    }
  ],
  "resource_usage": {
    "cpu_time_ms": 0,
    "max_rss_bytes": 0
  }
}
```

---

## 7. Security Constraints

The agent must enforce:

- Path canonicalization
- No relative path traversal (`..`)
- No symlink escape
- No execution outside allowlisted commands
- No shell interpolation
- Hard runtime limits
- Hard output limits
- Fail immediately on schema violation

The container runtime enforces:

- CPU and memory limits
- Read-only root filesystem (optional but recommended)
- No network (unless explicitly allowed)
- No mounted secrets
- User namespace isolation

---

## 8. Policy Boundary

The agent does not decide policy.

Policy is enforced by:

- Worker API (host side)
- Container runtime configuration
- Job constraints embedded in job.json

If a step violates constraints, the agent returns:

```json
{
  "status": "failure",
  "error": {
    "type": "policy_violation",
    "message": "command not allowed"
  }
}
```

---

## 9. Observability

The agent must emit:

- Structured logs to stdout (JSON lines)
- Deterministic step boundaries
- Duration per step
- Truncated output notices
- Resource usage metrics

No free-form log text.

---

## 10. Versioning

- `protocol_version` must be checked at startup
- Agent must refuse unknown major versions
- Minor versions may allow backward-compatible fields

---

## 11. Failure Modes

The agent must distinguish:

- schema_error
- policy_violation
- execution_failure
- timeout
- resource_limit_exceeded
- internal_error

Failures must never leave partial JSON.

---

## 12. Non-Goals

The agent will not:

- Host an LLM
- Perform planning
- Interpret natural language
- Fetch external data
- Handle secrets
- Manage credentials
- Retry autonomously

---

## 13. Implementation Notes

Recommended implementation language: Go.

Rationale:

- Static binary
- Small runtime footprint
- Strong JSON handling
- Good process control primitives
- Easy cross-compilation

Binary should be:

- Statically linked
- <20MB preferred
- No external runtime dependencies

---

## 14. MVP Scope

Minimum viable feature set:

- run_command
- write_file
- read_file
- apply_unified_diff
- list_tree
- result.json emission
- strict schema validation
- timeout enforcement

Everything else is optional in v1.

---

## 15. Architectural Outcome

This design ensures:

- LLM generates structured intent
- Host validates and gates
- cynode-agent executes deterministically
- Results are auditable and replayable
- Skills become reproducible job specs

This preserves separation between:

- Intelligence (LLM)
- Policy (host)
- Execution (sandbox agent)

That separation is critical for long-term maintainability and security.
