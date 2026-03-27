# Lint-Go: Line Count Gate (2026-03-24)

- [Behaviour](#behaviour)

## Behaviour

`just lint-go` (`.ci_scripts/justfile`) scans repository `.go` files (excludes `.git`, `vendor`, `.venv`, `tmp`, and `_bdd/tmp`).

For each file over 1000 lines it logs an error with the path (repo-relative) and line count, without stopping the scan.

`go vet` and `staticcheck` run for every module listed in `go.work`; failures in those steps do not short-circuit later modules.

The recipe exits with a non-zero status if any file exceeds 1000 lines or if vet or staticcheck failed for any module.
