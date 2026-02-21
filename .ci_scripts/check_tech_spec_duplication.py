#!/usr/bin/env python3
"""
Check docs/tech_specs for duplicated or conflicting spec text (CPD-style).

Fingerprints contiguous blocks of normalized lines across tech_spec .md files.
Reports blocks that appear in more than one file or in multiple places in the
same file, so they can be deduplicated or consolidated.

Exit code: 0 if no duplicates above threshold, 1 if duplicates found.
Output to stdout only unless --report PATH is given; temp files (if any) to tmp/.
"""

from __future__ import annotations

import argparse
import hashlib
import re
import sys
from pathlib import Path


def _find_repo_root() -> Path:
    """Assume script lives in repo/.ci_scripts."""
    script_dir = Path(__file__).resolve().parent
    return script_dir.parent


def _collect_spec_files(specs_dir: Path) -> list[Path]:
    """Return sorted list of .md files under specs_dir."""
    if not specs_dir.is_dir():
        return []
    return sorted(specs_dir.rglob("*.md"))


def _normalize_line(line: str) -> str:
    """Collapse whitespace to single space, strip. Preserve case for semantics."""
    s = " ".join(line.split()).strip()
    return s


def _is_boilerplate(line: str) -> bool:
    """True if line is TOC, link-only, or structural noise."""
    s = line.strip()
    if not s or len(s) < 3:
        return True
    if s == "---" or s.startswith("<!--"):
        return True
    if re.match(r"^#+\s*$", s):
        return True
    if re.match(r"^-\s*\[.+\]\([^)]+\)\s*$", s) and "#" in s:
        return True
    return False


def _block_fingerprint(lines: list[str]) -> str:
    """Stable hash of normalized block for deduplication."""
    normalized = "\n".join(_normalize_line(ln) for ln in lines)
    return hashlib.sha256(normalized.encode("utf-8")).hexdigest()


def _extract_blocks(
    path: Path,
    min_lines: int,
    min_line_len: int,
) -> list[tuple[str, int, int, list[str]]]:
    """
    Return (fingerprint, start_line_1based, end_line_1based, raw_lines) for each block.

    Sliding window: every contiguous run of min_lines lines that are not
    boilerplate and have enough substantial lines (>= min_line_len) is fingerprinted.
    """
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return []

    all_lines = text.splitlines()
    blocks: list[tuple[str, int, int, list[str]]] = []

    for i in range(len(all_lines) - min_lines + 1):
        window = all_lines[i:i + min_lines]
        if any(_is_boilerplate(ln) for ln in window):
            continue
        substantial = [ln for ln in window if len(_normalize_line(ln)) >= min_line_len]
        if len(substantial) < min_lines:
            continue
        fp = _block_fingerprint(window)
        start_1 = i + 1
        end_1 = i + min_lines
        blocks.append((fp, start_1, end_1, window))

    return blocks


def _find_duplicates(
    specs_dir: Path,
    min_lines: int = 4,
    min_line_len: int = 15,
) -> list[tuple[str, list[tuple[Path, int, int, list[str]]]]]:
    """
    Return list of (fingerprint, [(path, start_line, end_line, lines), ...])
    for each block that appears more than once (across or within files).
    """
    files = _collect_spec_files(specs_dir)
    fingerprint_to_occurrences: dict[
        str,
        list[tuple[Path, int, int, list[str]]],
    ] = {}

    for path in files:
        for fp, start, end, raw in _extract_blocks(path, min_lines, min_line_len):
            fingerprint_to_occurrences.setdefault(fp, []).append((path, start, end, raw))

    duplicates = [
        (fp, occs)
        for fp, occs in fingerprint_to_occurrences.items()
        if len(occs) > 1
    ]
    return duplicates


def _format_report(
    duplicates: list[tuple[str, list[tuple[Path, int, int, list[str]]]]],
    specs_dir: Path,
) -> list[str]:
    """Produce human-readable report lines."""
    lines = [
        "Tech spec duplication report",
        "===========================",
        f"Specs dir: {specs_dir}",
        f"Duplicate blocks: {len(duplicates)}",
        "",
    ]
    for fp, occs in duplicates:
        lines.append(f"Block fingerprint: {fp[:16]}...")
        for path, start, end, _ in occs:
            try:
                rel = path.relative_to(specs_dir)
            except ValueError:
                rel = path
            lines.append(f"  {rel}:{start}-{end}")
        first_raw = occs[0][3]
        max_preview_lines = 6
        max_line_len = 100
        lines.append("  Preview:")
        for ln in first_raw[:max_preview_lines]:
            trimmed = ln[:max_line_len] + ("..." if len(ln) > max_line_len else "")
            lines.append(f"    {trimmed}")
        if len(first_raw) > max_preview_lines:
            lines.append(f"    ... ({len(first_raw) - max_preview_lines} more lines)")
        lines.append("")
    return lines


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Check tech_specs for duplicated/conflicting spec text (CPD-style).",
    )
    root = _find_repo_root()
    default_specs = root / "docs" / "tech_specs"
    parser.add_argument(
        "--specs-dir",
        type=Path,
        default=default_specs,
        metavar="DIR",
        help=f"Tech specs directory (default: {default_specs})",
    )
    parser.add_argument(
        "--min-lines",
        type=int,
        default=4,
        metavar="N",
        help="Minimum consecutive lines to consider a block (default: 4)",
    )
    parser.add_argument(
        "--min-line-length",
        type=int,
        default=15,
        metavar="N",
        help="Min normalized line length to count toward block (default: 15)",
    )
    parser.add_argument(
        "--report",
        type=Path,
        default=None,
        metavar="PATH",
        help="Write report to path (default: stdout only)",
    )
    parser.add_argument(
        "--no-fail",
        action="store_true",
        help="Exit 0 even when duplicates are found",
    )
    args = parser.parse_args()

    specs_dir = args.specs_dir if args.specs_dir.is_absolute() else root / args.specs_dir
    if not specs_dir.is_dir():
        print(f"Specs directory not found: {specs_dir}", file=sys.stderr)
        return 2

    duplicates = _find_duplicates(
        specs_dir,
        min_lines=args.min_lines,
        min_line_len=args.min_line_length,
    )
    report_lines = _format_report(duplicates, specs_dir)

    for line in report_lines:
        print(line)

    if args.report is not None:
        report_path = args.report if args.report.is_absolute() else root / args.report
        report_path.parent.mkdir(parents=True, exist_ok=True)
        report_path.write_text("\n".join(report_lines) + "\n", encoding="utf-8")

    if duplicates and not args.no_fail:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
