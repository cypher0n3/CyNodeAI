# Cynork TUI Session Cache (Disk Layout)

## Document Overview

This specification defines **on-disk** layout for the cynork fullscreen TUI **session cache**: one JSON file per TUI session, named with a `session_id`, retention of up to **10** recent session files, and the JSON contract for each file.

It is **not** the cynork YAML config file (`config.yaml`); tokens and credentials are out of scope.

Session cache supports resuming lightweight state (for example last active chat thread id) after gateway or process restart without storing secrets.

The JSON document records **when** the session started (`session_started_at`), **when** the file was last updated (`last_activity_at`), **identity context** (`gateway_url`, `user_id`, `project_id`), and **chat state** (`current_thread_id` for the gateway thread id).

### Cynork TUI Session Cache Traces To

- [REQ-CLIENT-0188](../requirements/client.md#req-client-0188)

### Related Documents

- [Cynork TUI](cynork_tui.md) (Local Cache, [Auth Recovery](cynork_tui.md#spec-cynai-client-cynorkchat-authrecovery))
- [Cynork CLI](cynork_cli.md) (config paths; cache is separate)

## Session Cache Root Directory

- Spec ID: `CYNAI.CLIENT.CynorkTui.SessionCache.RootDirectory` <a id="spec-cynai-client-cynorktui-sessioncache-rootdirectory"></a>

**Resolution.**

- The base cache directory is resolved the same way as other cynork TUI cache data: `XDG_CACHE_HOME/cynork` when `XDG_CACHE_HOME` is set; otherwise `~/.cache/cynork` on Unix.
- If the environment variable `CYNORK_CACHE_DIR` is set, implementations treat its value as the **base** cache directory (overriding the default `XDG_CACHE_HOME` / `~/.cache` resolution for that subtree).

**Session cache directory path.**

- `base_cache_dir/tui_sessions/` where `base_cache_dir` is the resolved base cache directory.

**Behavior.**

- The implementation creates `tui_sessions` under the base cache directory when it first writes a session file.
- The implementation uses directory mode `0700` when creating `tui_sessions` (or any parent under the user's cache home that it creates for this purpose).
- Session JSON files live **directly** under `tui_sessions/` (no nested subdirectories per session).
- Filenames are `<session_id>.json` only; there is no separate global aggregate JSON file in this layout for last-thread data (per-session files hold all cache keys for that session).

### Session Cache Root Directory Traces To

- [REQ-CLIENT-0188](../requirements/client.md#req-client-0188)

## Session Id and Filename

- Spec ID: `CYNAI.CLIENT.CynorkTui.SessionCache.SessionID` <a id="spec-cynai-client-cynorktui-sessioncache-sessionid"></a>

**Session id type.**

- A **UUID version 4** in the canonical lowercase string form (8-4-4-4-12 hex digits with hyphens).

**Filename.**

- On-disk name: `<session_id>.json` (for example `f47ac10b-58cc-4372-a567-0e02b2c3d479.json`).
- The filename MUST equal the `session_id` field in the JSON document so the stem is always the UUID.

**Lifecycle.**

- Each TUI **process** (or each logical interactive session that chooses to persist cache) **SHOULD** generate a new `session_id` at startup when it intends to use session cache.
- If a file is renamed or copied, the `session_id` inside the JSON MUST still match the filename stem; otherwise the file is invalid for reads.

### Session Id and Filename Traces To

- [REQ-CLIENT-0188](../requirements/client.md#req-client-0188)

## Session Cache JSON Document

- Spec ID: `CYNAI.CLIENT.CynorkTui.SessionCache.FileDocument` <a id="spec-cynai-client-cynorktui-sessioncache-filedocument"></a>

**Top-level fields.**

- **Field:** `schema_version`
  - type: integer
  - required: yes
  - description: Must be `1` for this specification revision.
- **Field:** `session_id`
  - type: string
  - required: yes
  - description: Same UUID v4 as the filename stem.
- **Field:** `session_started_at`
  - type: string
  - required: yes
  - description: UTC timestamp in RFC3339 format.
    Wall-clock time when this TUI session was first created (same instant as `session_id` assignment in the process).
    Set once on first persistence; MUST NOT be moved backward on later writes.
- **Field:** `last_activity_at`
  - type: string
  - required: yes
  - description: UTC timestamp in RFC3339 format.
    Updated on every successful write to this file (including thread-only updates).
    Used for eviction ordering (see [Retention of Recent Sessions](#retention-of-recent-sessions)).
- **Field:** `gateway_url`
  - type: string
  - required: no
  - description: Normalized gateway base URL (trimmed, no trailing slash) when known.
    Used with `user_id` and `project_id` when matching a prior session for resume.
- **Field:** `user_id`
  - type: string
  - required: no
  - description: Authenticated user id from `GET /v1/users/me` when known.
- **Field:** `project_id`
  - type: string
  - required: no
  - description: Current session project id when known (empty string means default project).
- **Field:** `current_thread_id`
  - type: string
  - required: no
  - description: Gateway chat thread id for the active conversation (same id returned by `POST /v1/chat/threads` or listed in `GET /v1/chat/threads`).
    Empty or omitted when no thread is selected yet.
    Used to resume after re-auth or restart without requiring `--resume-thread`.
- **Field:** `data`
  - type: object
  - required: yes
  - description: Namespaced cache payload.
    Use `{}` when no extra keys are stored.

**`session_started_at` vs `last_activity_at`.**

- **`session_started_at`** is the **start** of the TUI interactive session (cache lifetime anchor); it is stable for the life of the file.
- **`last_activity_at`** is the **most recent** mutation of the cache file; it is always greater than or equal to `session_started_at` in wall-clock time.

**`current_thread_id` semantics.**

- The value MUST be the **server-issued** thread identifier string, not a user display ordinal or title.
- When the user runs `/thread new` or `/thread switch`, the implementation MUST update `current_thread_id` and `last_activity_at`.

**Keys under `data` (non-exhaustive).**

- **`model_id`** (string, optional): Last selected model id for chat in this session (for display or completion cache).
- **`thread_list_etag`** or **`thread_list_fetched_at`** (string, optional): Opaque cache metadata for thread list (if the implementation caches list responses; format is implementation-defined).
- Any additional keys MUST remain non-secret JSON values per [Forbidden Content](#forbidden-content).

**Extensibility.**

- The implementation MAY add more keys under `data` for completion metadata, thread-list snapshots, or other non-secret caches described in [Cynork TUI - Local Cache](cynork_tui.md#spec-cynai-client-cynorkchat-localcache).

**Encoding and durability.**

- Files MUST be UTF-8 JSON.
- The implementation SHOULD write JSON with stable key ordering for readability (`schema_version`, `session_id`, `session_started_at`, `last_activity_at`, identity fields, `current_thread_id`, then `data`).
- The implementation MUST write via a temporary file in the same directory followed by `rename` (or equivalent) to `<session_id>.json`, and MUST set file mode `0600` on the final file.

**Read compatibility.**

- Readers MUST ignore unknown top-level fields.
- If `session_started_at` is required by this spec but missing in an older file, implementations MAY treat the file as invalid or use the file modification time as a fallback for `session_started_at` only for display or eviction tie-breaks.

### Session Cache JSON Document Traces To

- [REQ-CLIENT-0188](../requirements/client.md#req-client-0188)

## Retention of Recent Sessions

- Spec ID: `CYNAI.CLIENT.CynorkTui.SessionCache.Retention` <a id="spec-cynai-client-cynorktui-sessioncache-retention"></a>

**Limit.**

- The implementation MUST NOT keep more than **`TuiSessionCache.MaxRecentSessions`** session files in `tui_sessions/` at steady state after any write that touches session cache.

**Counting.**

- Only files matching `*.json` under `tui_sessions/` whose names parse as UUID v4 are counted toward the limit.

**Active session.**

- The file for the **active** `session_id` MUST NOT be deleted during eviction while that TUI process still holds that session open.

### `TuiSessionCacheEviction` Algorithm

<a id="algo-cynai-client-cynorktui-sessioncache-eviction"></a>

1. After a successful write to any `<session_id>.json`, list all valid session JSON files in `tui_sessions/`.
2. If the count is less than or equal to `TuiSessionCache.MaxRecentSessions`, stop.
3. Sort candidates for eviction by **least recently active** first: use `last_activity_at` parsed from each file when present and valid; if a file is missing `last_activity_at` or is unreadable, use the file's modification time as a fallback.
4. Exclude the active session file from deletion if the TUI process is still running with that `session_id`.
5. Delete files in order until the count is **at most** `TuiSessionCache.MaxRecentSessions`, never deleting the active session file.

### Retention of Recent Sessions Traces To

- [REQ-CLIENT-0188](../requirements/client.md#req-client-0188)

## Read and Write Operations

- Spec ID: `CYNAI.CLIENT.CynorkTui.SessionCache.Operations` <a id="spec-cynai-client-cynorktui-sessioncache-operations"></a>

**Open.**

- On TUI startup, **generate** a new `session_id` (UUID v4) unless the implementation explicitly supports **resuming** a session from a file (optional future behavior).
- Load the new empty in-memory structure; **create** the corresponding `<session_id>.json` on first cache write.
- On first write, set **`session_started_at`** and **`last_activity_at`** to the same UTC timestamp (RFC3339), and set **`session_id`** to match the filename stem.

**Read for resume.**

- When matching a prior session for resume, compare `gateway_url`, `user_id`, and `project_id` (normalization rules MUST match write rules).
- If the match succeeds, apply **`current_thread_id`** to the in-memory chat session when the client does not already have a current thread (unless `--resume-thread` overrides the CLI).

**Write.**

- On every write, set **`last_activity_at`** to the current UTC time (RFC3339).
- Update **`current_thread_id`** whenever the active gateway thread changes (ensure-thread, `/thread new`, `/thread switch`).
- Update **`gateway_url`**, **`user_id`**, and **`project_id`** when they become known or change.
- Merge updates into **`data`** when using namespaced keys.
- Run eviction per [`TuiSessionCacheEviction` Algorithm](#tuisessioncacheeviction-algorithm) after writes.

### Read and Write Operations Traces To

- [REQ-CLIENT-0188](../requirements/client.md#req-client-0188)

## Forbidden Content

- Spec ID: `CYNAI.CLIENT.CynorkTui.SessionCache.ForbiddenContent` <a id="spec-cynai-client-cynorktui-sessioncache-forbiddencontent"></a>

Session cache files MUST NOT contain access tokens, refresh tokens, passwords, or chat transcript text.

Session cache remains aligned with [Cynork TUI - Local Cache](cynork_tui.md#spec-cynai-client-cynorkchat-localcache) (metadata only).

### Forbidden Content Traces To

- [REQ-CLIENT-0188](../requirements/client.md#req-client-0188)

## Constants

- Spec ID: `CYNAI.CLIENT.CynorkTui.SessionCache.Constants` <a id="spec-cynai-client-cynorktui-sessioncache-constants"></a>

**`TuiSessionCache.MaxRecentSessions`:** `10`.

### Constants Traces To

- [REQ-CLIENT-0188](../requirements/client.md#req-client-0188)
