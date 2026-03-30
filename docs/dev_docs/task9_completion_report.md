# Task 9 Completion Report - Cynork Gateway `Client` Timeout and Thread Safety

## Summary

`cynork/internal/gateway/client.go` and `client_http.go`: unexported `baseURL` and `token` behind `sync.RWMutex`, `NewClient` sets `http.Client{ Timeout: 30s }`, exported `BaseURL()`, `Token()`, `SetBaseURL()`, `SetToken()`.

Call sites in cynork (`tui`, slash commands, chat) use setters instead of field assignment.

`client_timeout_race_test.go` holds `TestClientTimeout` and `TestClientRace` (keeps `client_test.go` under the lint line-count cap).

## Validation

- `go test -v -race -run 'TestClientTimeout|TestClientRace' ./cynork/internal/gateway/...`
- `just lint` / `just test-go-cover`

## Plan

YAML `st-087`-`st-099` and Task 9 markdown checklists updated in `docs/dev_docs/_plan_003_short_term.md`.
