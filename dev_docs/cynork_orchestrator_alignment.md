# Cynork vs Orchestrator Alignment

- [Orchestrator User-Gateway (Implemented)](#orchestrator-user-gateway-implemented)
- [What was un-stubbed](#what-was-un-stubbed)
- [What remains stubbed in cynork](#what-remains-stubbed-in-cynork)
- [Summary](#summary)

## Orchestrator User-Gateway (Implemented)

From `orchestrator/cmd/user-gateway/main.go` and `orchestrator/internal/handlers/tasks.go`:

- `GET /healthz` - GET - Cynork: `status` (real)
- `POST /v1/auth/login` - POST - Cynork: auth (real)
- `POST /v1/auth/refresh` - POST - Cynork: (real)
- `POST /v1/auth/logout` - POST - Cynork: auth (real)
- `GET /v1/users/me` - GET - Cynork: whoami (real)
- `POST /v1/tasks` - POST - Cynork: task create (real)
- `GET /v1/tasks` - GET - Cynork: task list (real)
- `GET /v1/tasks/{id}` - GET - Cynork: task get (real)
- `GET /v1/tasks/{id}/result` - GET - Cynork: task result (real)
- `POST /v1/tasks/{id}/cancel` - POST - Cynork: task cancel (real)
- `GET /v1/tasks/{id}/logs` - GET - Cynork: task logs (real)
- `POST /v1/chat` - POST - Cynork: chat (real, after un-stub)

## What Was Un-Stubbed

- **Chat:** Cynork previously implemented chat as "create task + poll `GET /v1/tasks/{id}/result`" in the client.
  The orchestrator now provides `POST /v1/chat` (request `{"message":"..."}`, response `{"response":"..."}`).
  Cynork was updated to call `gateway.Chat(message)` so it uses the real chat endpoint; the client-side create+poll loop was removed.

## What Remains Stubbed in Cynork

The user-gateway has **no** routes for:

- `/v1/creds`
- `/v1/prefs`, `/v1/prefs/effective`
- `/v1/settings`
- `/v1/nodes`
- `/v1/skills/load`
- `/v1/audit`

Cynork commands for creds, prefs, settings, nodes, skills, and audit still call these paths; against a real orchestrator they would get 404 until the orchestrator adds these APIs.
BDD uses a mock that stubs these endpoints so scenarios pass.

## Summary

- **Task list, get, cancel, result, logs:** Already using real orchestrator endpoints; no change.
- **Chat:** Switched from client-side create+poll to real `POST /v1/chat`.
- **Creds, prefs, settings, nodes, skills, audit:** Remain stubbed in cynork until the orchestrator implements the corresponding routes.

Last updated: 2026-02-21
