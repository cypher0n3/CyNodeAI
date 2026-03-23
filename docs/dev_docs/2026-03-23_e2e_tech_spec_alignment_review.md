# E2E vs Recent Tech Spec Revisions

- [Review Scope](#review-scope)
- [Commits Summarized (Newest First)](#commits-summarized-newest-first)
- [E2E: Already Aligned](#e2e-already-aligned)
- [Gaps / Follow-Ups](#gaps--follow-ups)
- [Tests to Remove](#tests-to-remove)

## Review Scope

Last six commits on `mvp/phase-2`, `docs/tech_specs/` changes, and
`scripts/test_scripts/` E2E alignment.

## Commits Summarized (Newest First)

1. **47def83** - MCP gateway on control-plane, PMA streaming, worker internal MCP
   proxy, cynork TUI tests; `ports_and_endpoints.md` tweak; `e2e_0660_worker_pma_proxy.py`
   assertion update.
2. **e99a761** - `cynork_tui.md`, `cynork_tui_slash_commands.md`, `client.md`: composer
   keys (Alt+Enter, Ctrl+J, wrapped-line Up/Down, Ctrl+Up/Ctrl+Down history), `/copy`
   family, queued drafts explicitly not implemented.
3. **a5a0daf** - PTY harness key map, `e2e_0765`, `fetch_gateway_access_token`,
   `e2e_0610` comment wrap only.
4. **9109882** - Wrap-aware composer (code); dev_docs spec delta (not normative tech_specs).
5. **274ab76** - MCP control-plane routing; deprecate standalone gateway `:12083`;
   `ports_and_endpoints.md`, PMA/MCP specs, `meta.md`.
6. **3854b0b** - dev_docs only (plan TOC, TUI spec delta).

## E2E: Already Aligned

- **MCP on control-plane (12082), not standalone 12083:** `config.CONTROL_PLANE_API`
  targets 12082; no Python E2E references `:12083`.
  `e2e_0660_worker_pma_proxy.py` documents removal of standalone worker-api-only proxy tests and asserts MCP tool path `/v1/mcp/tools/call` (control-plane semantics).
- **User-gateway streaming:** `e2e_0610`, `e2e_0630`, `e2e_0640` use `config.USER_API`
  (12080); unchanged by MCP routing move.
- **TUI composer / history (e99a761 + 9109882):** `e2e_0765` covers footnote
  (`prior messages`, `Alt+Enter`), multiline (`ctrl+j`, `alt+enter`), narrow terminal,
  `/auth login` overlay, `Ctrl+Up` recall with `CYNORK_TOKEN` - matches updated specs.
- **Token handling:** `helpers.fetch_gateway_access_token` plus `CYNORK_TOKEN` matches
  documented behavior (config may not persist bearer token for PTY tests).

## Gaps / Follow-Ups

No mandatory removals found.

- **`/copy`, `/copy last`, `/copy all`:** Specified in `cynork_tui_slash_commands.md`
  (e99a761); no E2E in `scripts/test_scripts` exercises these yet.
  Consider adding PTY tests in `e2e_0760` or a small dedicated module when implementation is stable.
- **`Ctrl+Down`:** Spec requires paired history navigation; `e2e_0765` only asserts
  `Ctrl+Up` recall.
  Optional second test for symmetry.
- **BDD (`features/cynork/`):** Not exhaustively scanned.
  If scenarios assert old Shift+Enter-as-newline or pre-wrap-only Up/Down, update to match
  `docs/tech_specs/cynork_tui.md` (wrapped-line movement, Ctrl+Up/Ctrl+Down for sent
  history).

## Tests to Remove

None identified from spec drift in this pass.
Deprecated standalone MCP gateway is not asserted by current Python E2E.
