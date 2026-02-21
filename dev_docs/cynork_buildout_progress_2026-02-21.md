# Cynork Buildout Progress Report

- [Summary](#summary)
- [Completed (Cynork)](#completed-cynork)
- [Not done](#not-done-per-plan-deferred-or-orchestrator-first)
- [How to run](#how-to-run)
- [Notes](#notes)

## Summary

**Date:** 2026-02-21. **Reference:** [cynork_buildout_plan.md](cynork_buildout_plan.md)

Cynork CLI buildout per section 4 of the plan has been implemented.
Orchestrator work is still ongoing; cynork is ready to call new endpoints once they are available.
BDD runs against a mock gateway that implements the planned API shape.

## Completed (Cynork)

Cynork CLI and gateway client work completed as below.

### Gateway Client (`internal/gateway`)

- `ListTasks`, `GetTask`, `CancelTask`, `GetTaskLogs` with request/response types.
- `GetBytes`, `PostBytes` for stub endpoints (creds, prefs, settings, nodes, audit, skills).
- `HTTPError` with status code for exit-code mapping; `ResolveTaskID()` on `TaskResponse`.
- `ListTasks` uses query params (limit, offset, status) correctly.

### Exit Codes (`internal/exit`)

- Package with codes 0, 2 (usage), 3 (auth), 4 (not found), 5 (conflict), 6 (validation), 7 (gateway), 8 (internal).
- `Execute()` returns `exit.CodeOf(err)`; handlers return `exit.Auth()`, `exitFromGatewayErr(err)` etc.

### CLI Commands

- **Global:** `-o/--output` (table | json); validation in PreRunE.
- **Task:** `task list` (--limit, --offset, --status), `task get`, `task cancel` (-y), `task result` (--wait), `task logs`; create output `task_id=<id>` / JSON.
- **Chat:** `cynork chat`; no token exits 3; loop read line, `/exit`/`/quit`/EOF exit 0; else create task + poll result, print job results.
- **Shell:** `cynork shell` (REPL), `shell -c "command"`; parseArgs for quoted segments.
- **Stubs:** `creds list`, `prefs set`/`get`, `settings set`/`get`, `nodes list`, `skills load`, `audit list` call gateway GET/POST; BDD mock returns 200 with empty JSON.

### Status and Auth

- `status` supports `-o json` (`{"gateway":"ok"}`); health failure returns exit 7.
- Auth whoami/login use `exitFromGatewayErr`; whoami no token returns exit 3.

### BDD (`_bdd/steps.go`)

- Mock gateway: `GET /v1/tasks`, `GET /v1/tasks/{id}`, `POST /v1/tasks/{id}/cancel`, `GET /v1/tasks/{id}/logs`; stubs for `/v1/creds`, `/v1/nodes`, `/v1/audit`, `POST/GET /v1/prefs`, `/v1/settings`, `POST /v1/skills/load`.
- Steps: task list, get, cancel, logs; status with `-o json`; chat (with stdin `/exit`); store task id (parse `task_id=` or JSON); task file, script, commands, attachments; prefs/settings set/get; creds, nodes, skills load, audit list; shell interactive; session persistence.
- Config file reset in Before so "Chat without token" does not inherit token from previous scenario.
- Rebuild cynork binary in Before on every scenario so new code is exercised.

### Tests and CI

- Unit tests for gateway (ListTasks, GetTask, CancelTask, GetTaskLogs, GetBytes, PostBytes, HTTPError); cmd (task list/get/cancel/logs, exit codes, stub commands, chat no-token, shell -c, parseArgs, exitFromGatewayErr branches).
- Feature file: whoami without token expects exit 3.
- `just test-go-cover` passes (all packages >= 90%); `just test-bdd` passes (cynork suite).

## Not Done (Per Plan, Deferred or Orchestrator-First)

- Task create input modes: `-f/--task-file`, `-s/--script`, `--command`/`--commands-file`, `-a/--attach` (plan 4.2.3); BDD steps use `-p` with file content or `--input-mode` where available.
- Tab completion in shell (plan 4.2.5).
- Dedicated chat endpoint (cynork uses create task + poll until orchestrator adds `POST /v1/chat`).

## How to Run

- Build: `just build-cynork`
- Tests: `just test-go-cover` (from repo root)
- BDD: `just test-bdd` or `go test ./cynork/_bdd -count=1`
- Full CI: `just ci`

## Notes

- Orchestrator list/cancel/logs/chat endpoints may not be deployed yet; cynork is ready to call them when available.
- Stub commands (creds, prefs, settings, nodes, skills, audit) hit the mock in BDD; replace with real gateway calls when orchestrator implements those APIs.
