# Cynork CLI: Reliable Session Credential Storage

## Summary

Login session credentials are now stored so that multiple consecutive CLI invocations can reuse the same token without re-authenticating.
Changes align with `docs/tech_specs/cli_management_app.md` and improve reliability.
Date: 2026-02-20.

## Changes

The following updates were made so that session storage is reliable and consistent across runs.

### Config Path (Xdg)

Default config path now honors `XDG_CONFIG_HOME`: when set, config is under `$XDG_CONFIG_HOME/cynork/config.yaml`; otherwise `~/.config/cynork/config.yaml`.
Ensures a stable, spec-compliant path across runs and environments.

### Atomic Save

`config.Save` writes to a temp file in the same directory, then renames it to the final path.
A crash or interrupt does not leave a half-written file; the next run sees either the previous config or a complete new one.

### Error Handling

When `--config` is not set, login and logout resolve the default path via `getDefaultConfigPath()` (wrapping `config.ConfigPath()`).
If that fails (e.g. no home dir), the CLI returns a clear error instead of continuing with an empty path.

### Test Coverage

Added/updated tests for XDG path, atomic save failure (non-writable dir), and config-path resolution failure in auth login/logout.
Coverage remains at or above 90% for affected packages.

## Verification

- `just ci` (lint, tests, coverage, BDD) passes.
- Login writes token to the resolved path; subsequent commands load it from the same path.
