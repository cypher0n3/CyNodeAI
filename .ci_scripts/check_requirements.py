#!/usr/bin/env python3
"""
Validate docs/requirements/*.md: no duplicate REQ ids, sequential ids per file,
each REQ has a spec ref.

Checks:
- No duplicate REQ ids across all requirement files.
- Within each file, REQ numeric suffixes appear in strictly ascending order.
- Every REQ entry has at least one spec reference (markdown link to tech_specs).

Exit code: 0 if all checks pass, 1 otherwise. Output to stdout; optional --report to file.
With --fix-ordering, reorder REQ blocks so numeric ids are ascending.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


def _find_repo_root() -> Path:
    """Assume script lives in repo/.ci_scripts."""
    return Path(__file__).resolve().parent.parent


def _collect_requirement_files(reqs_dir: Path) -> list[Path]:
    """Return sorted .md files under reqs_dir, excluding README.md."""
    if not reqs_dir.is_dir():
        return []
    return sorted(
        p for p in reqs_dir.glob("*.md")
        if p.name != "README.md"
    )


# Match REQ-<DOMAIN>-<NNNN> at start of a list item line (optional **).
_REQ_LINE_RE = re.compile(
    r"^-\s*\*?\*?REQ-([A-Z]+)-(\d{4})\s*:",
    re.IGNORECASE,
)
# Match markdown link whose target contains tech_specs (spec reference).
_SPEC_REF_RE = re.compile(
    r"\[[^\]]+\]\([^)]*tech_specs[^)]*\)",
)


def _parse_requirement_entries(path: Path) -> list[tuple[str, int, list[str]]]:
    """
    Return list of (req_id, line_no, block_lines) for each REQ in file.

    Block is from the REQ line up to (excl) the next REQ line or end of file.
    """
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return []

    lines = text.splitlines()
    entries: list[tuple[str, int, list[str]]] = []
    i = 0
    while i < len(lines):
        line = lines[i]
        match = _REQ_LINE_RE.search(line)
        if match:
            domain, num = match.group(1), match.group(2)
            req_id = f"REQ-{domain.upper()}-{num}"
            line_no = i + 1
            block = [line]
            j = i + 1
            while j < len(lines) and not _REQ_LINE_RE.search(lines[j]):
                block.append(lines[j])
                j += 1
            entries.append((req_id, line_no, block))
            i = j
        else:
            i += 1
    return entries


def _block_has_spec_ref(block: list[str]) -> bool:
    """True if any line in block contains a markdown link to tech_specs."""
    return any(_SPEC_REF_RE.search(ln) for ln in block)


def _check_duplicate_ids(
    file_entries: list[tuple[Path, list[tuple[str, int, list[str]]]]],
) -> list[tuple[str, list[tuple[Path, int]]]]:
    """Return list of (req_id, [(path, line_no), ...]) for ids that appear more than once."""
    id_to_locations: dict[str, list[tuple[Path, int]]] = {}
    for path, entries in file_entries:
        for req_id, line_no, _ in entries:
            id_to_locations.setdefault(req_id, []).append((path, line_no))
    return [
        (req_id, locs)
        for req_id, locs in id_to_locations.items()
        if len(locs) > 1
    ]


def _req_id_to_num(req_id: str) -> int:
    """Extract numeric suffix from REQ-DOMAIN-NNNN."""
    return int(req_id.split("-")[-1], 10)


def _check_sequential_per_file(
    _path: Path,
    entries: list[tuple[str, int, list[str]]],
) -> list[tuple[str, int, int]]:
    """
    Return (req_id, line_no, expected_num) for entries where numeric part is not sequential.
    Report all violations: only advance expected sequence when we see a strictly greater num.
    """
    errors: list[tuple[str, int, int]] = []
    prev_num = -1
    for req_id, line_no, _ in entries:
        num = _req_id_to_num(req_id)
        if num <= prev_num:
            errors.append((req_id, line_no, prev_num + 1))
        else:
            prev_num = num
    return errors


def _parse_file_into_blocks(path: Path) -> tuple[list[str], list[tuple[int, list[str]]], list[str]]:
    """
    Split file into (preamble_lines, [(num, block_lines), ...], postamble_lines).

    Preamble is everything before the first REQ list item; postamble is everything after the last
    REQ block. Blocks are (numeric_id, lines) for each REQ entry.
    """
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return [], [], []
    lines = text.splitlines()
    preamble: list[str] = []
    blocks: list[tuple[int, list[str]]] = []
    postamble: list[str] = []
    i = 0
    while i < len(lines):
        match = _REQ_LINE_RE.search(lines[i])
        if match:
            _, num_str = match.group(1), match.group(2)
            num = int(num_str, 10)
            block = [lines[i]]
            j = i + 1
            while j < len(lines) and not _REQ_LINE_RE.search(lines[j]):
                block.append(lines[j])
                j += 1
            blocks.append((num, block))
            i = j
        else:
            if not blocks:
                preamble.append(lines[i])
            else:
                postamble.append(lines[i])
            i += 1
    return preamble, blocks, postamble


def _fix_ordering_in_file(path: Path) -> bool:
    """Rewrite path so REQ blocks are sorted by numeric id. Return True if changed."""
    preamble, blocks, postamble = _parse_file_into_blocks(path)
    if not blocks:
        return False
    sorted_blocks = sorted(blocks, key=lambda x: x[0])
    if sorted_blocks == blocks:
        return False
    new_lines = preamble + [ln for _, b in sorted_blocks for ln in b] + postamble
    path.write_text("\n".join(new_lines) + "\n", encoding="utf-8")
    return True


def _fix_ordering(_reqs_dir: Path, paths_with_errors: list[Path]) -> list[Path]:
    """Fix ordering in each path that has sequential errors. Return paths that were changed."""
    changed: list[Path] = []
    for path in paths_with_errors:
        if _fix_ordering_in_file(path):
            changed.append(path)
    return changed


def _check_missing_spec_refs(
    _path: Path,
    entries: list[tuple[str, int, list[str]]],
) -> list[tuple[str, int]]:
    """Return list of (req_id, line_no) for entries with no spec reference in their block."""
    return [
        (req_id, line_no)
        for req_id, line_no, block in entries
        if not _block_has_spec_ref(block)
    ]


def _run_checks(reqs_dir: Path) -> tuple[
    list[tuple[str, list[tuple[Path, int]]]],
    list[tuple[Path, str, int, int]],
    list[tuple[Path, str, int]],
]:
    """Run all checks. Return (duplicates, sequential_errors, missing_spec_refs)."""
    files = _collect_requirement_files(reqs_dir)
    file_entries: list[tuple[Path, list[tuple[str, int, list[str]]]]] = []
    for path in files:
        entries = _parse_requirement_entries(path)
        file_entries.append((path, entries))

    duplicates = _check_duplicate_ids(file_entries)
    sequential_errors: list[tuple[Path, str, int, int]] = []
    missing_refs: list[tuple[Path, str, int]] = []
    for path, entries in file_entries:
        for req_id, line_no, expected in _check_sequential_per_file(path, entries):
            sequential_errors.append((path, req_id, line_no, expected))
        for req_id, line_no in _check_missing_spec_refs(path, entries):
            missing_refs.append((path, req_id, line_no))

    return duplicates, sequential_errors, missing_refs


def _format_report(
    reqs_dir: Path,
    duplicates: list[tuple[str, list[tuple[Path, int]]]],
    sequential_errors: list[tuple[Path, str, int, int]],
    missing_refs: list[tuple[Path, str, int]],
) -> list[str]:
    """Produce human-readable report lines."""
    lines = [
        "Requirements validation report",
        "=============================",
        f"Requirements dir: {reqs_dir}",
        "",
    ]
    failed = False

    if duplicates:
        failed = True
        lines.append("Duplicate REQ ids (must be unique):")
        for req_id, locs in duplicates:
            lines.append(f"  {req_id}:")
            for path, line_no in locs:
                try:
                    rel = path.relative_to(reqs_dir)
                except ValueError:
                    rel = path
                lines.append(f"    {rel}:{line_no}")
        lines.append("")
    else:
        lines.append("Duplicate REQ ids: none")
        lines.append("")

    if sequential_errors:
        failed = True
        lines.append("Non-sequential REQ ids (numeric part must increase within file):")
        for path, req_id, line_no, expected in sequential_errors:
            try:
                rel = path.relative_to(reqs_dir)
            except ValueError:
                rel = path
            lines.append(f"  {rel}:{line_no} {req_id} (expected next >= {expected})")
        lines.append("")
    else:
        lines.append("Sequential REQ ids: OK")
        lines.append("")

    if missing_refs:
        failed = True
        lines.append("REQ entries missing at least one spec ref (link to tech_specs):")
        for path, req_id, line_no in missing_refs:
            try:
                rel = path.relative_to(reqs_dir)
            except ValueError:
                rel = path
            lines.append(f"  {rel}:{line_no} {req_id}")
        lines.append("")
    else:
        lines.append("Spec refs: all REQ entries have at least one")
        lines.append("")

    lines.append("Result: FAIL" if failed else "Result: PASS")
    return lines


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Validate requirement files: no duplicate ids, sequential ids, spec refs.",
    )
    root = _find_repo_root()
    default_reqs = root / "docs" / "requirements"
    parser.add_argument(
        "--reqs-dir",
        type=Path,
        default=default_reqs,
        metavar="DIR",
        help=f"Requirements directory (default: {default_reqs})",
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
        help="Exit 0 even when validation fails",
    )
    parser.add_argument(
        "--fix-ordering",
        action="store_true",
        help="Reorder REQ blocks in place so numeric ids are ascending",
    )
    args = parser.parse_args()

    reqs_dir = args.reqs_dir if args.reqs_dir.is_absolute() else root / args.reqs_dir
    if not reqs_dir.is_dir():
        print(f"Requirements directory not found: {reqs_dir}", file=sys.stderr)
        return 2

    duplicates, sequential_errors, missing_refs = _run_checks(reqs_dir)

    if args.fix_ordering and sequential_errors:
        paths_to_fix = sorted(set(p for p, _r, _l, _e in sequential_errors))
        changed = _fix_ordering(reqs_dir, paths_to_fix)
        if changed:
            try:
                rels = [p.relative_to(reqs_dir) for p in changed]
            except ValueError:
                rels = changed
            print("Fixed ordering in: " + ", ".join(str(r) for r in rels), file=sys.stderr)
            duplicates, sequential_errors, missing_refs = _run_checks(reqs_dir)
        report_lines = _format_report(
            reqs_dir, duplicates, sequential_errors, missing_refs
        )
    else:
        report_lines = _format_report(
            reqs_dir, duplicates, sequential_errors, missing_refs
        )
    for line in report_lines:
        print(line)

    if args.report is not None:
        report_path = args.report if args.report.is_absolute() else root / args.report
        report_path.parent.mkdir(parents=True, exist_ok=True)
        report_path.write_text("\n".join(report_lines) + "\n", encoding="utf-8")

    has_errors = bool(duplicates or sequential_errors or missing_refs)
    if has_errors and not args.no_fail:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
