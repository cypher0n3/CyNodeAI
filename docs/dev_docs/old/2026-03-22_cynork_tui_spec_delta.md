# Cynork TUI: Implementation vs Specs (Delta)

- [Purpose](#purpose)
- [Scope of Latest Commit (Cynork)](#scope-of-latest-commit-cynork)
- [Summary of Implementation Behavior](#summary-of-implementation-behavior)
- [Differences From Current Specs](#differences-from-current-specs)
  - [`docs/tech_specs/cynork_tui.md` (Layout and Interaction)](#docstech_specscynork_tuimd-layout-and-interaction)
  - [`docs/requirements/client.md` (REQ-CLIENT-0206)](#docsrequirementsclientmd-req-client-0206)
  - [Slash Commands Spec (`cynork_tui_slash_commands.md`)](#slash-commands-spec-cynork_tui_slash_commandsmd)
  - [Draft Specification Files](#draft-specification-files)
- [Known Limitations](#known-limitations)
- [Recommended Spec Updates](#recommended-spec-updates)
- [Traceability](#traceability)
- [Files Reference (TUI-Focused)](#files-reference-tui-focused)

## Purpose

This note records **cynork** TUI behavior and code changes relative to **docs/tech_specs** and **docs/requirements**.
It suggests spec updates so documents match what ships.

It combines work from the **2026-03-22** implementation thread (composer editing, copy/clipboard UX, footnotes) and the cynork-related tree as of commit `90da858` (*feat: MCP gateway package, PMA tool routing, cynork TUI, worker orchestrator proxy*).
That commit introduced the full TUI package and related wiring, not only the items from the thread.

**Later updates (same dev-doc thread):** composer **Up/Down** vs **Ctrl+Up/Ctrl+Down**, wrap-aware vertical movement, composer **inner width** (border clipping), login overlay UX (centering, column-aligned labels, true-black panel `#000000`, UTF-8 cursors matching composer semantics, word keys disabled on password).
Treat this file as the rolling delta until specs are merged.

## Scope of Latest Commit (Cynork)

The commit touches **27 files** under `cynork/` (see `git show 90da858 --stat -- cynork/`).

Beyond the interaction details below, it includes new TUI core (`model.go`, `view_render.go`, scrollback rendering, viewport, login overlay hooks), slash system (`slash.go`, `slash_menu.go`, catalog, completion), composer helpers (`composer_input.go`), clipboard (`clipboard.go`), status/health (`status_indicator.go`), and chat/config wiring.

This delta does not re-derive every line of `cynork_cli.md` or gateway specs.
It focuses on layout, composer, copy, and slash where the tech spec is explicit.

## Summary of Implementation Behavior

- **Composer cursor:** Byte offset into `Input`.
  **Left/Right** move by rune (including across lines).
  **Ctrl+Left/Right** move by whitespace-separated words.

- **Composer vertical keys:** **Up/Down** move the caret along **wrapped display rows** (width budget matches the bordered composer: inner width `Width - 2` for borders, padding `(0,1)`, soft segments by display width with a **`"> "`** first-row allowance), not only at explicit `\n` boundaries.
  **Ctrl+Up / Ctrl+Down** cycle **sent-message** input history (newest first).
  When the slash menu is open with matches, **Up/Down** still navigate the menu; **Ctrl+Up/Ctrl+Down** do nothing in that case.

- **Cursor rendering:** TUI-drawn **block** over the current rune using **reverse video** on the same background as the composer panel (base style `236` / `252`), not a separate bar glyph.
  This avoids ANSI reset stripping the panel background.

- **Composer box width:** `Style.Width` is the **inner** width before the border; **NormalBorder** adds two columns.
  The code uses **`innerW := m.Width - 2`** so the full bordered composer fits the terminal (avoids clipping the **right** border).

- **Multiline window:** Composer shows up to **5** logical lines.
  If there are more, the window **scrolls** so the line containing the cursor stays visible.
  (Scrolling is still by **logical** lines; a single logical line that **wraps** to many terminal rows can make the visible window feel tight-known UX limitation.)

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

- **Footnote under composer:** `copySelectFootnote` lists **Ctrl+`↑↓` prior messages**, **Alt+Enter newline**, selection, and copy shortcuts (not only status-bar `composerHint`).
  It does **not** repeat plain **`↑↓`** (those move within the composer / slash menu).

- **Status bar hint (REQ-CLIENT-0206):** Still **`/ commands · @ files · ! shell`** in the status line.

- **Login overlay (REQ-CLIENT-0190):** In-TUI login is a **rounded** card, **horizontally and vertically centered** on the main view.
  Labels use a **fixed-width** right-aligned column; values use **true black** background **`#000000`** on text segments, gaps, blank rows, centered padding, and the bordered block so scrollback does not bleed through as grey.
  **Per-field byte cursors** (UTF-8) match composer semantics: **Left/Right** by rune, **Backspace** deletes the previous code point, typing **inserts at the caret**.
  **Ctrl+Left/Ctrl+Right** word motion applies to **gateway and username only**; on the **password** field those chords are **ignored** (left/right by rune still work).
    Caret uses the same **reverse-video** pattern as the composer (no separate bar glyph).
    Landmark **`[CYNRK_AUTH_RECOVERY_READY]`** remains verbatim for PTY/E2E.

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

- `docs/draft_specs/integrated/cynork_tui_spec_proposal.md` may diverge from both merged spec and implementation.
  Treat as **non-canonical** until folded into `cynork_tui.md`.

## Known Limitations

- **Wrap vs movement:** Vertical movement follows a **display-width** model (with a **`"> "`** first-line allowance) that may differ slightly from **lipgloss/cellbuf** word-wrap break positions at the edge case.

- **Composer visible line window:** Still scrolls by **logical** `\n`-separated lines (up to 5); one very long wrapped line does not scroll the composer window by **wrapped** row.

- **Input history:** **Ctrl+Up/Ctrl+Down** recall **sent** messages only; there is no separate ring for an unsent draft.

- **Shift+Enter:** Cannot be honored as newline-only without terminal keyboard protocol support and a stack that exposes distinct chords (future Bubble Tea / terminal features).

## Recommended Spec Updates

1. **`cynork_tui.md` newline (L150):** Document **Alt+Enter** and **Ctrl+J** as the supported newline-without-send keys.
   Replace unconditional **Shift+Enter MUST** with **SHOULD** where reported, or **MAY** with Kitty-style enhancements, plus a **Note** on TTY bytes for Enter vs Shift+Enter.

2. **`cynork_tui.md` cursor:** Optional sentence that the reference build uses **reverse-video** on the active character (or reverse space at end of line).

3. **`cynork_tui_slash_commands.md`:** Add **`/copy`**, **`/copy last`**, **`/copy all`** (transcript rules, system lines, ClipNote, empty cases, relation to **Ctrl+Y**).

4. **Queued drafts section (L160+):** Mark deferred or align **MUST** language with what is implemented.

5. **REQ-CLIENT-0206:** Optional note that discoverability **MAY** use a second line (e.g. copy shortcuts + newline keys).

6. **`cynork_tui.md` composer keys:** Document **Up/Down** (caret / slash menu) vs **Ctrl+Up/Ctrl+Down** (input history), and that **history** is sent messages only.

7. **REQ-CLIENT-0190 / auth recovery:** Optional note on in-TUI login layout (centering, label alignment), **true black** panel, caret behavior, and **Ctrl+word** disabled on password.

## Traceability

- **`CYNAI.CLIENT.CynorkChat.TUILayout`:** Composer cursor, newline keys - `cynork_tui.md`.

- **REQ-CLIENT-0206:** Composer hints - `client.md` + `cynork_tui.md`.

- **REQ-CLIENT-0190:** In-session login overlay - `client.md` (Auth Recovery) + `cynork_tui.md` where applicable.

- **`CYNAI.CLIENT.CynorkTuiSlashCommands`:** `/copy` - `cynork_tui_slash_commands.md`.

## Files Reference (TUI-Focused)

- `cynork/internal/tui/model.go` - Keys, Enter, `/copy` dispatch, `copyClipboardResultMsg`, scrollback cache, View layout, login overlay (`renderLoginOverlay`, login field cursors).

- `cynork/internal/tui/composer_input.go` - Shared UTF-8 string cursor helpers (`moveStringCursorRune`, insert/delete, word motion), composer methods, visible line range.

- `cynork/internal/tui/composer_wrap.go` - Composer **wrap width** and **visual-row** helpers for Up/Down.

- `cynork/internal/tui/view_render.go` - Composer lines, `renderStyledLineWithCursor`, base style + reverse cursor, `copySelectFootnote`, scrollback glamour cache.

- `cynork/internal/tui/slash.go` - `/copy`, `slashClipboardSequence`.

- `cynork/internal/tui/clipboard.go` - OS clipboard.

- `cynork/internal/tui/slash_menu.go` - Completion; cursor sync on replace.

- `cynork/cmd/tui.go` - `tea.NewProgram` options.
