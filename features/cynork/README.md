# Cynork Gherkin and Godog Features

<!-- Short-lived overview; body starts at first H2 per markdownlint no-h1-content. -->

## Overview

These files drive `go test ./cynork/_bdd` (see `just test-bdd`).

## Non-TTY Contract Tests

The suite runs in CI without a controlling terminal.

Steps that say “TUI” in the title usually exercise the **same slash-command and session behavior** through `cynork chat` on stdin (see step comments in individual features).

That matches how the product documents the chat surface: slash commands and shell escape work in both `cynork chat` and the full-screen TUI.

## Full-Screen TUI (PTY) Coverage

Rendering, alt-screen layout, mouse, and true `cynork tui` startup are covered by Python functional tests that allocate a PTY, for example:

- `scripts/test_scripts/e2e_0750_tui_pty.py`
- `scripts/test_scripts/e2e_0760_tui_slash_commands.py`
- `scripts/test_scripts/e2e_0765_tui_composer_editor.py`
- `scripts/test_scripts/e2e_0650_tui_streaming_behavior.py`

Use `@wip` on a scenario to skip it in Godog (`Tags: ~@wip` in `cynork/_bdd/suite_test.go`).
