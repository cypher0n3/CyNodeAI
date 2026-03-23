# Cynork TUI: Implementation vs Specs (Delta)

- [Purpose](#purpose)
- [Scope of Latest Commit (Cynork)](#scope-of-latest-commit-cynork)
- [Summary of Implementation Behavior](#summary-of-implementation-behavior)
- [Differences From Current Specs](#differences-from-current-specs)
- [Known Limitations](#known-limitations)
- [Recommended Spec Updates](#recommended-spec-updates)
- [Traceability](#traceability)
- [Files Reference (TUI-Focused)](#files-reference-tui-focused)

## Purpose

This note records **cynork** TUI behavior and code changes relative to **docs/tech_specs** and **docs/requirements**.
It suggests spec updates so documents match what ships.

It combines work from the **2026-03-22** implementation thread (composer editing, copy/clipboard UX, footnotes) and the cynork-related tree as of commit `90da858` (*feat: MCP gateway package, PMA tool routing, cynork TUI, worker orchestrator proxy*).
That commit introduced the full TUI package and related wiring, not only the items from the thread.

## Scope of Latest Commit (Cynork)

The commit touches **27 files** under `cynork/` (see `git show 90da858 --stat -- cynork/`).

Beyond the interaction details below, it includes new TUI core (`model.go`, `view_render.go`, scrollback rendering, viewport, login overlay hooks), slash system (`slash.go`, `slash_menu.go`, catalog, completion), composer helpers (`composer_input.go`), clipboard (`clipboard.go`), status/health (`status_indicator.go`), and chat/config wiring.

This delta does not re-derive every line of `cynork_cli.md` or gateway specs.
It focuses on layout, composer, copy, and slash where the tech spec is explicit.

## Summary of Implementation Behavior

- **Composer cursor:** Byte offset into `Input`.
  **Left/Right** move by rune (including across lines).
  **Ctrl+Left/Right** move by whitespace-separated words.

- **Cursor rendering:** TUI-drawn **block** over the current rune using **reverse video** on the same background as the composer panel (base style `236` / `252`), not a separate bar glyph.
  This avoids ANSI reset stripping the panel background.

- **Multiline window:** Composer shows up to **5** logical lines.
  If there are more, the window **scrolls** so the line containing the cursor stays visible.

- **Newline without send:** **Alt+Enter** and **Ctrl+J** insert `\n`.
  **`shift+enter`** is recognized if the runtime reports it.
  On typical terminals **Shift+Enter is indistinguishable from Enter** (both `KeyEnter`), so it still **sends**.

- **Copy feedback order:** **`tea.Sequence`**: first message applies **scrollback system line + ClipNote** immediately; second step runs **`CopyToClipboard`**.
  This avoids waiting on clipboard helpers before any UI feedback.

- **`/copy` semantics:** **`/copy`**, **`/copy last`**: last assistant plain text.
  **`/copy all`**: `plainTranscript` (chat lines; skips system lines with the dim `·` prefix).
  Success strings include **"All text copied to clipboard."** for **all**; empty-transcript **all** still shows that success line without calling the OS clipboard.
  Empty last: **"No assistant message to copy."**

- **`CopyToClipboard`:** Rejects only **empty** string (no longer trims entire payload), so whitespace-only content is not collapsed away before copy.

- **Scrollback cache:** On copy result, **`scrollbackCacheValid = false`** so glamour rebuild picks up new system lines.

- **Footnote under composer:** `copySelectFootnote` includes **Alt+Enter newline** and copy shortcuts (not only status-bar `composerHint`).

- **Status bar hint (REQ-CLIENT-0206):** Still **`/ commands · @ files · ! shell`** in the status line.

## Differences From Current Specs

The following call out gaps between `docs/tech_specs` and the current cynork TUI build.

### `docs/tech_specs/cynork_tui.md` (Layout and Interaction)

- **L148-149 (visible caret):** Spec asks for a visible caret distinguishable from text.
  Implementation uses a reverse-video block on the active cell.
  **Aligned** (TUI-rendered equivalent).

- **L150 (Shift+Enter newline):** Spec says **`Shift+Enter` MUST** insert a newline.
  Implementation uses **Alt+Enter** and **Ctrl+J**; Shift+Enter usually matches Enter (send) with Bubble Tea v1 and common terminals.
  **Mismatch:** spec assumes Shift+Enter is distinct; implementation reflects terminal and library limits.

- **L151 (search and copy in scrollback):** Spec **SHOULD** support search and copy.
  Partial: **Ctrl+Y**, **`/copy`**, selection footnote; full search not implied by this work.
  **Gap** (pre-existing SHOULD).

- **L160-188 (queued drafts, Ctrl+Q, Ctrl+Enter, streaming Enter):** Narrative **MUST**/SHOULD behavior may not match current code.
  **Likely gap** - confirm in a separate audit.

- **L137-138 (discoverability hint):** Status **`composerHint`** satisfies REQ; extra copy/newline text is in a **footnote** under the composer.
  **Aligned** for REQ-0206; spec could mention an optional second footnote line.

### `docs/requirements/client.md` (REQ-CLIENT-0206)

- Requires hinting **`/`**, **`@`**, **`!`** near the composer.
- Implementation: status bar string + dim footnote for copy/selection/newline.
- **Aligned**; optional clarification that hints **MAY** be split (status vs footnote).

### Slash Commands Spec (`cynork_tui_slash_commands.md`)

- No dedicated **`/copy`** section in the doc (no matches for "copy" in a quick grep at time of writing).
- **`/copy`** exists in code with **last | all**, clipboard integration, and system-line feedback.
  **Gap:** document **`/copy`** behavior and empty cases.

### Draft Specification Files

- `docs/draft_specs/cynork_tui_spec_proposal.md` may diverge from both merged spec and implementation.
  Treat as **non-canonical** until folded into `cynork_tui.md`.

## Known Limitations

- **Wrapped lines:** Cursor position is logical; **lipgloss-wrapped** long lines may not align the block cursor with wrapped screen rows.

- **Shift+Enter:** Cannot be honored as newline-only without terminal keyboard protocol support and a stack that exposes distinct chords (future Bubble Tea / terminal features).

## Recommended Spec Updates

1. **`cynork_tui.md` newline (L150):** Document **Alt+Enter** and **Ctrl+J** as the supported newline-without-send keys.
   Replace unconditional **Shift+Enter MUST** with **SHOULD** where reported, or **MAY** with Kitty-style enhancements, plus a **Note** on TTY bytes for Enter vs Shift+Enter.

2. **`cynork_tui.md` cursor:** Optional sentence that the reference build uses **reverse-video** on the active character (or reverse space at end of line).

3. **`cynork_tui_slash_commands.md`:** Add **`/copy`**, **`/copy last`**, **`/copy all`** (transcript rules, system lines, ClipNote, empty cases, relation to **Ctrl+Y**).

4. **Queued drafts section (L160+):** Mark deferred or align **MUST** language with what is implemented.

5. **REQ-CLIENT-0206:** Optional note that discoverability **MAY** use a second line (e.g. copy shortcuts + newline keys).

## Traceability

- **`CYNAI.CLIENT.CynorkChat.TUILayout`:** Composer cursor, newline keys - `cynork_tui.md`.

- **REQ-CLIENT-0206:** Composer hints - `client.md` + `cynork_tui.md`.

- **`CYNAI.CLIENT.CynorkTuiSlashCommands`:** `/copy` - `cynork_tui_slash_commands.md`.

## Files Reference (TUI-Focused)

- `cynork/internal/tui/model.go` - Keys, Enter, `/copy` dispatch, `copyClipboardResultMsg`, scrollback cache, View layout.

- `cynork/internal/tui/composer_input.go` - Cursor math, word motion, visible line range.

- `cynork/internal/tui/view_render.go` - Composer lines, base style + reverse cursor, scrollback glamour cache.

- `cynork/internal/tui/slash.go` - `/copy`, `slashClipboardSequence`.

- `cynork/internal/tui/clipboard.go` - OS clipboard.

- `cynork/internal/tui/slash_menu.go` - Completion; cursor sync on replace.

- `cynork/cmd/tui.go` - `tea.NewProgram` options.

---

*Generated for planning and spec maintenance; update or remove when official spec edits land.*
