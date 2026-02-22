#!/usr/bin/env python3
"""
Validate internal file links in docs/ markdown files.

Finds all markdown links that point to other files (not same-document # anchors
or external URLs) and checks that target files exist and, when present, that
fragment identifiers exist in the target file.

Same-document (internal) links may use regular heading refs. For cross-doc
links from docs/tech_specs, docs/requirements, and docs/draft_specs, fragment
identifiers must use stable anchor patterns from .markdownlint.yml
allow-custom-anchors. Other docs may use non-anchor links.

Exit code: 0 if all links valid, 1 if any broken. Outputs to stdout; report
can be written to dev_docs when run via justfile.
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path


def _find_docs_root() -> Path:
    """Return docs directory (repo/docs). Assume script lives in repo/.ci_scripts."""
    script_dir = Path(__file__).resolve().parent
    root = script_dir.parent
    docs = root / "docs"
    if not docs.is_dir():
        sys.exit(f"docs directory not found: {docs}")
    return docs


def _collect_md_files(docs_root: Path) -> list[Path]:
    """Return all .md files under docs_root."""
    return sorted(docs_root.rglob("*.md"))


# Match markdown link: [text](url). Captures url (may include fragment).
_LINK_RE = re.compile(r"\[([^\]]*)\]\(([^)\s]+)(?:\s+[^)]*)?\)")

# Subpaths under docs/ where cross-doc links must use stable anchors only.
_STABLE_ANCHOR_SCOPE = ("tech_specs", "requirements", "draft_specs")


def _load_stable_anchor_patterns(repo_root: Path) -> tuple[re.Pattern[str], ...]:
    """Read allow-custom-anchors.allowedIdPatterns from .markdownlint.yml."""
    yaml_path = repo_root / ".markdownlint.yml"
    if not yaml_path.is_file():
        return ()
    text = yaml_path.read_text(encoding="utf-8", errors="replace")
    patterns: list[re.Pattern[str]] = []
    in_allowed = False
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.startswith("allowedIdPatterns:"):
            in_allowed = True
            continue
        if in_allowed:
            # End of list: sibling key at 2-space indent (e.g. strictPlacement:)
            if re.match(r"^  [a-zA-Z]", line):
                in_allowed = False
                continue
            # List item: "    - pattern: '...'" or "    - pattern: \"...\""
            m = re.match(r"^\s*-\s*pattern:\s*(?:(['\"])(.+)\1|(.+?))\s*$", stripped)
            if m:
                raw = m.group(2) if m.group(2) is not None else (m.group(3) or "").strip()
                if raw:
                    try:
                        patterns.append(re.compile(raw))
                    except re.error:
                        continue
    return tuple(patterns)


def _extract_links(content: str) -> list[tuple[str, int]]:
    """Return list of (href, 1-based line number) for each link in content."""
    out: list[tuple[str, int]] = []
    for i, line in enumerate(content.splitlines(), start=1):
        for m in _LINK_RE.finditer(line):
            href = m.group(2).strip()
            out.append((href, i))
    return out


def _is_internal_ref(href: str) -> bool:
    """True if href is same-document (starts with #) or external (http/mailto)."""
    h = href.strip()
    if h.startswith("#"):
        return True
    if h.startswith(("http://", "https://", "mailto:")):
        return True
    return False


def _resolve_path(from_file: Path, href: str) -> Path:
    """Resolve href (path part only) relative to from_file. Returns absolute path."""
    path_part = href.split("#", 1)[0].strip()
    from_dir = from_file.parent
    return (from_dir / path_part).resolve()


def _under_stable_anchor_scope(docs_root: Path, source: Path) -> bool:
    """True if source is under tech_specs, requirements, or draft_specs."""
    try:
        rel = source.resolve().relative_to(docs_root.resolve())
    except ValueError:
        return False
    parts = rel.parts
    return len(parts) >= 1 and parts[0] in _STABLE_ANCHOR_SCOPE


def _fragment_matches_stable_pattern(
    fragment: str, patterns: tuple[re.Pattern[str], ...]
) -> bool:
    """True if fragment matches one of the allow-custom-anchors id patterns."""
    return any(p.fullmatch(fragment) for p in patterns)


def _slug_from_heading(line: str) -> str:
    """Derive GitHub-style anchor slug from a markdown heading line."""
    # Strip leading # and whitespace, take rest of line
    s = line.lstrip("#").strip()
    s = s.lower()
    # Replace spaces and runs of non-alphanumeric with single hyphen
    s = re.sub(r"[^\w\s-]", "", s)
    s = re.sub(r"[-\s]+", "-", s).strip("-")
    return s


def _anchors_in_file(path: Path) -> set[str]:
    """Return set of anchor ids present in file (explicit id= and heading-derived)."""
    ids: set[str] = set()
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return ids
    # Explicit HTML anchors: <a id="..."></a> or <a id='...'></a>; allow content before </a>
    for m in re.finditer(r'<a\s+id=["\']([^"\']+)["\']\s*>[^<]*</a>', text, re.DOTALL):
        ids.add(m.group(1))
    # Heading-derived slugs (## or ### etc.)
    for line in text.splitlines():
        if line.startswith("#") and not line.startswith("# "):
            slug = _slug_from_heading(line)
            if slug:
                ids.add(slug)
    return ids


def validate_links(
    docs_root: Path,
    *,
    check_fragments: bool = True,
) -> list[tuple[Path, int, str, str]]:
    """
    Validate all file links in docs/*.md.

    Returns list of (source_file, line_no, href, error_message) for broken links.
    When check_fragments is True, every link that includes a #fragment is validated
    against explicit <a id="..."> anchors and heading-derived slugs in the target file.
    """
    errors: list[tuple[Path, int, str, str]] = []
    repo_root = docs_root.parent
    stable_patterns = _load_stable_anchor_patterns(repo_root)
    md_files = _collect_md_files(docs_root)
    anchor_cache: dict[Path, set[str]] = {}

    def get_anchors(path: Path) -> set[str]:
        if path not in anchor_cache:
            anchor_cache[path] = _anchors_in_file(path)
        return anchor_cache[path]

    for md_path in md_files:
        try:
            content = md_path.read_text(encoding="utf-8", errors="replace")
        except OSError as e:
            errors.append((md_path, 0, "", str(e)))
            continue

        for href, line_no in _extract_links(content):
            if _is_internal_ref(href):
                continue

            path_part = href.split("#", 1)[0].strip()
            if not path_part:
                continue
            target_path = _resolve_path(md_path, href)

            if not target_path.exists():
                errors.append((
                    md_path,
                    line_no,
                    href,
                    f"target does not exist: {target_path}",
                ))
                continue

            # Allow links to directories (e.g. ../tech_specs/); they resolve in GitHub.
            if target_path.is_dir():
                continue

            if not target_path.is_file():
                errors.append((
                    md_path,
                    line_no,
                    href,
                    f"target is not a file or directory: {target_path}",
                ))
                continue

            # Cross-doc links from tech_specs/requirements/draft_specs must use stable anchors.
            # Same-document links may use regular heading refs.
            if "#" in href:
                fragment = href.split("#", 1)[1].strip()
                if fragment:
                    is_cross_doc = target_path.resolve() != md_path.resolve()
                    if (
                        is_cross_doc
                        and _under_stable_anchor_scope(docs_root, md_path)
                        and stable_patterns
                        and not _fragment_matches_stable_pattern(fragment, stable_patterns)
                    ):
                        errors.append((
                            md_path,
                            line_no,
                            href,
                            "cross-doc links in tech_specs/requirements/draft_specs must use "
                            "stable anchor ids from .markdownlint.yml allow-custom-anchors",
                        ))
                        continue
            # Always validate fragment when href contains # (file links only).
            if check_fragments and "#" in href:
                fragment = href.split("#", 1)[1].strip()
                if fragment:
                    anchors = get_anchors(target_path)
                    if fragment not in anchors:
                        errors.append((
                            md_path,
                            line_no,
                            href,
                            f"fragment '{fragment}' not found in {target_path}",
                        ))

    return errors


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Validate internal file links in docs/ markdown.",
    )
    parser.add_argument(
        "--no-fragments",
        action="store_true",
        help="Do not validate fragment identifiers in target files",
    )
    parser.add_argument(
        "--report",
        type=Path,
        metavar="PATH",
        help="Write report to this path (e.g. dev_docs/doc_links_validation_report.txt)",
    )
    args = parser.parse_args()

    docs_root = _find_docs_root()
    errors = validate_links(
        docs_root,
        check_fragments=not args.no_fragments,
    )

    report_lines: list[str] = []
    if errors:
        report_lines.append("Doc link validation: FAILED")
        report_lines.append("")
        for path, line_no, href, msg in errors:
            report_lines.append(f"  {path}:{line_no}: {href}")
            report_lines.append(f"    -> {msg}")
        for line in report_lines:
            print(line)
        if args.report:
            args.report.parent.mkdir(parents=True, exist_ok=True)
            args.report.write_text("\n".join(report_lines) + "\n", encoding="utf-8")
        return 1

    report_lines.append("Doc link validation: OK (all file links valid)")
    for line in report_lines:
        print(line)
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text("\n".join(report_lines) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
