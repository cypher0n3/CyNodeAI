# MCP SDK Installation and Usage

- [Document Overview](#document-overview)
- [Goals](#goals)
- [Go SDK (Recommended for REST Services)](#go-sdk-recommended-for-rest-services)
- [Python SDK (When a Python Workflow Runtime is Used)](#python-sdk-when-a-python-workflow-runtime-is-used)
- [Reference Servers for Local Development](#reference-servers-for-local-development)

## Document Overview

This document defines practical conventions for installing and using MCP SDKs in CyNodeAI code.
It focuses on making MCP integration easy to adopt while keeping versions pinned and reproducible.

This document does not define CyNodeAI tool policy.
Tool policy is defined in [`docs/tech_specs/mcp_tooling.md`](mcp_tooling.md) and [`docs/tech_specs/mcp_gateway_enforcement.md`](mcp_gateway_enforcement.md).

## Goals

- Make MCP SDK installation repeatable and version-pinned.
- Prefer Go MCP SDK usage for CyNodeAI REST APIs.
- Support optional Python MCP SDK usage for a Python workflow runtime.

## Go SDK (Recommended for REST Services)

All REST APIs in this system MUST be implemented in Go.
MCP integration code that lives inside REST services SHOULD use the official Go MCP SDK.

SDK

- Go SDK repository: `github.com/modelcontextprotocol/go-sdk`
- Primary package: `github.com/modelcontextprotocol/go-sdk/mcp`

Version pinning

- MCP SDK versions MUST be pinned in each service `go.mod`.
- Avoid unpinned `go get` in documentation and scripts.

Recommended repo layout

- Each Go service SHOULD have its own `go.mod` at the service root.
- If a multi-module layout is used, a `go.work` MAY be used for local development.

Install and import (example)

```go
// Install:
//   go get "github.com/modelcontextprotocol/go-sdk/mcp@<PINNED_VERSION>" && go mod tidy
//
// Import:
import "github.com/modelcontextprotocol/go-sdk/mcp"
```

Toolchain

- Follow [`docs/tech_specs/go_rest_api_standards.md`](go_rest_api_standards.md) for toolchain pinning guidance.

## Python SDK (When a Python Workflow Runtime is Used)

CyNodeAI MAY implement the workflow engine in a separate process (for example a Python LangGraph runtime).
If the workflow runtime needs MCP protocol support directly, it SHOULD use the official Python MCP SDK.

Guidance

- Pin Python dependencies and keep them isolated to the workflow runtime directory.
- The workflow runtime MUST NOT connect directly to PostgreSQL.
  It MUST use MCP database tools (or an internal service that enforces the same policy).

## Reference Servers for Local Development

Reference MCP servers exist for learning and local experimentation.
They are not production-ready.

Source

- Reference servers: [`modelcontextprotocol/servers`](https://github.com/modelcontextprotocol/servers)

Recommended use

- Use reference servers only in local development environments.
- Prefer running them with minimal permissions and explicit allowlists.
