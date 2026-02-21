# CI Scripts

- [Overview](#overview)
- [Scripts](#scripts)
  - [Doc links validator](#doc-links-validator)
  - [Feature files validator](#feature-files-validator)
  - [Tech spec duplication checker](#tech-spec-duplication-checker)

## Overview

This directory holds Python scripts used for continuous integration and documentation validation.
Scripts are run from the repository root (e.g. via the [justfile](../justfile)).
Reports are written to [dev_docs/](../dev_docs/); temporary files, if any, go in [tmp/](../tmp/).

**See also:** [docs/README.md](../docs/README.md) (documentation index), [README.md](../README.md) (project overview).

## Scripts

The following scripts are available.
Each can be run from the repo root; some are wired to justfile targets.

### Doc Links Validator

[validate_doc_links.py](validate_doc_links.py) validates internal file links in [docs/](../docs/) markdown.

- Checks that link targets exist and that fragment identifiers exist in the target file.
- Exit 0 if all links valid, 1 if any broken.
- **Run:** `just validate-doc-links`
- **Options:** `--no-fragments` (skip fragment checks), `--report PATH` (write report to path).

### Feature Files Validator

[validate_feature_files.py](validate_feature_files.py) validates Gherkin feature file conventions under [features/](../features/).

- Enforces conventions in [features/README.md](../features/README.md) and
  [docs/docs_standards/spec_authoring_writing_and_validation.md](../docs/docs_standards/spec_authoring_writing_and_validation.md).
- Exit 0 when all feature files are valid, 1 when any validation errors are found.
- **Run:** `just validate-feature-files`

### Tech Spec Duplication Checker

[check_tech_spec_duplication.py](check_tech_spec_duplication.py) checks [docs/tech_specs/](../docs/tech_specs/) for duplicated or conflicting spec text in a CPD-style way.

The script fingerprints contiguous blocks of normalized lines across tech_spec markdown files.
Any block that appears in more than one file (or in multiple places in the same file) is reported so it can be deduplicated or consolidated.

#### Checker Behavior

- Scans all `.md` files under the tech_specs directory (default: [docs/tech_specs/](../docs/tech_specs/)).
- Uses a sliding window of contiguous lines (default 4 lines per block).
- Normalizes lines (collapse whitespace, strip) and skips boilerplate (TOC links, `---`, empty lines, short lines).
- Hashes each block; blocks with the same fingerprint in two or more locations are reported as duplicates.
- Writes a report to the given path when `--report` is used (e.g. `tmp/tech_spec_duplication_report.txt` to avoid committing).
- Exit 0 if no duplicates, 1 if duplicates found (unless `--no-fail` is used).

#### Running the Checker

From repo root: `just check-tech-spec-duplication` (output to stdout only; exits 0).
To write a report file, pass script args: `just check-tech-spec-duplication --report tmp/tech_spec_duplication_report.txt`.

Or run the script directly:

```bash
python3 .ci_scripts/check_tech_spec_duplication.py
```

Optional report path:

```bash
python3 .ci_scripts/check_tech_spec_duplication.py --report tmp/tech_spec_duplication_report.txt
```

#### Checker Options

- `--specs-dir DIR` - Tech specs directory to scan (default: `docs/tech_specs`).
- `--min-lines N` - Minimum consecutive lines per block (default: 4).
- `--min-line-length N` - Minimum normalized line length to count toward a block (default: 15).
- `--report PATH` - Write report to this path (default: stdout only).
- `--no-fail` - Exit 0 even when duplicates are found.

#### Report Format

The report lists each duplicate block with:

- Block fingerprint (first 16 chars of hash).
- File and line range for each occurrence (e.g. `access_control.md:95-98`).
- A multi-line preview of the block content (up to 6 lines, 100 chars per line).

Use the report to decide whether to consolidate text into a single source of truth (e.g. [docs/tech_specs/postgres_schema.md](../docs/tech_specs/postgres_schema.md)) and link from other specs, or to accept intentional duplication (e.g. admin/CLI parity).

#### Linting Scripts

All scripts in this directory are included in `just lint-python` (paths `scripts,.ci_scripts`).
Ensure changes pass `just lint-python` before committing.
