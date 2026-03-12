#!/usr/bin/env python3
"""
Check each Python functional test (e2e_*.py) for proper requirements traces.

- Discovers scripts/test_scripts/e2e_*.py (excludes e2e_state.py, e2e_tags.py).
- Each file must have a '# Traces:' comment in the header (within first MAX_HEADER_LINES).
- The Traces block (that line plus any following consecutive #-comment lines) must
  reference at least one canonical requirement id: REQ-<DOMAIN>-<NNNN>.

Exit code: 0 when all checks pass; 1 when any validation fails.
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

MAX_HEADER_LINES = 40
# REQ-<DOMAIN>-<NNNN> (domain uppercase letters, NNNN 4 digits).
REQ_ID_RE = re.compile(r"REQ-[A-Z]+-\d{4}")


def _repo_root() -> Path:
    return Path(__file__).resolve().parent.parent


def _e2e_script_paths(repo_root: Path) -> list[Path]:
    scripts_dir = repo_root / "scripts" / "test_scripts"
    if not scripts_dir.is_dir():
        return []
    out = []
    for p in sorted(scripts_dir.glob("e2e_*.py")):
        if p.name in ("e2e_state.py", "e2e_tags.py"):
            continue
        out.append(p)
    return out


def _find_traces_block(lines: list[str]) -> tuple[int | None, str]:
    """
    Find '# Traces:' in the first MAX_HEADER_LINES; return (line_1based, full_block_text).

    Block is the Traces line plus any following lines that are #-comments (continuation).
    """
    for i in range(min(len(lines), MAX_HEADER_LINES)):
        line = lines[i]
        stripped = line.strip()
        if not stripped.startswith("# Traces:"):
            continue
        block = [stripped]
        j = i + 1
        while j < min(len(lines), MAX_HEADER_LINES):
            next_line = lines[j].strip()
            if not next_line.startswith("#") or next_line == "#":
                break
            block.append(next_line)
            j += 1
        return (i + 1, " ".join(block))
    return (None, "")


def check_file(path: Path) -> list[tuple[int, str]]:
    """
    Run requirements-trace checks on one file. Return list of (line_no, message).
    """
    issues = []
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as e:
        return [(0, f"read error: {e}")]
    lines = text.splitlines()
    line_no, block = _find_traces_block(lines)
    if line_no is None:
        msg = f"missing '# Traces:' comment in header (within first {MAX_HEADER_LINES} lines)"
        issues.append((1, msg))
        return issues
    if not REQ_ID_RE.search(block):
        issues.append((
            line_no,
            "Traces block must reference at least one requirement id (REQ-<DOMAIN>-<NNNN>)",
        ))
    return issues


def main() -> int:
    repo_root = _repo_root()
    paths = _e2e_script_paths(repo_root)
    all_issues: list[tuple[Path, int, str]] = []
    for path in paths:
        for line_no, msg in check_file(path):
            all_issues.append((path, line_no, msg))

    if not all_issues:
        return 0
    for path, line_no, msg in all_issues:
        rel = path.relative_to(repo_root)
        print(f"{rel}:{line_no}: {msg}", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
