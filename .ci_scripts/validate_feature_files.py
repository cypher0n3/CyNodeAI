#!/usr/bin/env python3
"""
Validate Gherkin feature file conventions under features/.

This enforces the project conventions documented in:
- features/README.md
- docs/docs_standards/spec_authoring_writing_and_validation.md

Exit code:
- 0 when all feature files are valid
- 1 when any validation errors are found
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class ValidationError:
    path: Path
    line_no: int
    message: str


_SUITE_TAG_RE = re.compile(r"^@suite_[a-z0-9_]+$")
_FEATURE_RE = re.compile(r"^\s*Feature:\s+.+\S\s*$")
_SCENARIO_RE = re.compile(r"^\s*Scenario(?: Outline)?:\s+.+\S\s*$")
_BACKGROUND_RE = re.compile(r"^\s*Background:\s*$")
_COMMENT_RE = re.compile(r"^\s*#")

_REQ_TAG_RE = re.compile(r"@req_[a-z0-9]+_[0-9]{4}\b")
_SPEC_TAG_RE = re.compile(r"@spec_[a-z0-9_]+\b")


def _repo_root() -> Path:
    """Return repo root. Assume this script lives in repo/.ci_scripts."""
    return Path(__file__).resolve().parent.parent


def _load_allowed_suite_tags(repo_root: Path) -> set[str]:
    """
    Parse the suite tag registry from features/README.md.

    The registry is kept in prose so the docs remain the source of truth.
    """
    readme = repo_root / "features" / "README.md"
    if not readme.is_file():
        sys.exit(f"features suite tag registry not found: {readme}")
    text = readme.read_text(encoding="utf-8", errors="replace")
    tags = set(re.findall(r"`(@suite_[a-z0-9_]+)`", text))
    if not tags:
        sys.exit(f"no @suite_* tags found in registry: {readme}")
    return tags


def _iter_feature_files(repo_root: Path) -> list[Path]:
    features_dir = repo_root / "features"
    if not features_dir.is_dir():
        sys.exit(f"features directory not found: {features_dir}")
    return sorted(features_dir.rglob("*.feature"))


def _is_meaningful_line(line: str) -> bool:
    if not line.strip():
        return False
    if _COMMENT_RE.match(line):
        return False
    return True


def _meaningful_lines_with_numbers(lines: list[str]) -> list[tuple[int, str]]:
    return [(i, line) for i, line in enumerate(lines, start=1) if _is_meaningful_line(line)]


def _validate_top_matter(
    path: Path,
    lines: list[str],
    allowed_suite_tags: set[str],
) -> list[ValidationError]:
    errors: list[ValidationError] = []
    suite_tag: str | None = None
    meaningful = _meaningful_lines_with_numbers(lines)
    if len(meaningful) < 2:
        errors.append(ValidationError(path, 1, "file must contain @suite_* tag and Feature line"))
        return errors

    suite_line_no, suite_line = meaningful[0]
    suite = suite_line.strip()
    suite_tag = suite if suite.startswith("@suite_") else None
    if not _SUITE_TAG_RE.match(suite):
        errors.append(ValidationError(
            path,
            suite_line_no,
            "first meaningful line must be exactly one @suite_* tag",
        ))
    elif suite not in allowed_suite_tags:
        errors.append(ValidationError(
            path,
            suite_line_no,
            f"unknown suite tag {suite!r} (not in features/README.md registry)",
        ))

    feature_line_no, feature_line = meaningful[1]
    if not _FEATURE_RE.match(feature_line):
        errors.append(ValidationError(
            path,
            feature_line_no,
            "second meaningful line must be 'Feature: ...'",
        ))

    # Enforce "immediately above Feature:" (no blank/comment/other meaningful lines).
    # If there are any meaningful lines between suite_ln and feature_ln, it's a violation.
    for i in range(suite_line_no + 1, feature_line_no):
        if _is_meaningful_line(lines[i - 1]):
            errors.append(ValidationError(
                path,
                i,
                "@suite_* tag must be immediately above the Feature line",
            ))
            break

    # Enforce exactly one suite tag in the file.
    for i, line in enumerate(lines, start=1):
        s = line.strip()
        if s.startswith("@suite_") and i != suite_line_no:
            errors.append(ValidationError(
                path,
                i,
                "feature file must contain exactly one @suite_* tag",
            ))

    # Enforce suite directory rule: @suite_<name> lives under features/<name>/.
    if suite_tag and suite_tag in allowed_suite_tags:
        root = _repo_root()
        try:
            rel = path.resolve().relative_to(root.resolve())
        except ValueError:
            errors.append(ValidationError(
                path, suite_line_no, "feature file path must be under repo root"
            ))
            return errors
        if len(rel.parts) < 3 or rel.parts[0] != "features":
            errors.append(ValidationError(
                path,
                suite_line_no,
                "feature file must live under features/<suite>/",
            ))
        else:
            suite_dir = suite_tag[len("@suite_"):]
            if rel.parts[1] != suite_dir:
                errors.append(ValidationError(
                    path,
                    suite_line_no,
                    f"suite tag {suite_tag!r} requires directory features/{suite_dir}/",
                ))
    return errors


def _validate_narrative_block(path: Path, lines: list[str]) -> list[ValidationError]:
    errors: list[ValidationError] = []
    meaningful = _meaningful_lines_with_numbers(lines)
    if len(meaningful) < 2:
        return errors

    # Find the Feature line (expected to be meaningful[1] if top matter is valid).
    feature_idx = None
    for idx, (line_no, line) in enumerate(meaningful):
        if _FEATURE_RE.match(line):
            feature_idx = idx
            break
    if feature_idx is None:
        return errors

    story_lines: list[tuple[int, str]] = []
    for line_no, line in meaningful[feature_idx + 1:]:
        if _BACKGROUND_RE.match(line) or _SCENARIO_RE.match(line):
            break
        story_lines.append((line_no, line.strip()))

    if not story_lines:
        errors.append(ValidationError(
            path,
            meaningful[feature_idx][0],
            "missing user story narrative block",
        ))
        return errors

    want = [
        (re.compile(r"^As (a|an) .+"), "As a ..."),
        (re.compile(r"^I want .+"), "I want ..."),
        (re.compile(r"^So that .+"), "So that ..."),
    ]
    got = [s for _, s in story_lines if s]
    for pos, (pattern, label) in enumerate(want):
        if len(got) <= pos or not pattern.match(got[pos]):
            line_no = story_lines[pos][0] if len(story_lines) > pos else story_lines[-1][0]
            errors.append(ValidationError(
                path,
                line_no,
                f"user story narrative line {pos + 1} must match {label}",
            ))
    return errors


def _validate_scenarios_have_traceability_tags(
    path: Path,
    lines: list[str],
) -> list[ValidationError]:
    errors: list[ValidationError] = []
    pending_tags: list[tuple[int, str]] = []
    for line_no, line in enumerate(lines, start=1):
        stripped = line.strip()
        if not stripped or _COMMENT_RE.match(line):
            continue
        if stripped.startswith("@"):
            pending_tags.append((line_no, stripped))
            continue
        if _SCENARIO_RE.match(line):
            tags_text = " ".join(tag for _, tag in pending_tags)
            if not _REQ_TAG_RE.search(tags_text):
                errors.append(ValidationError(
                    path,
                    line_no,
                    "Scenario must include at least one @req_* tag",
                ))
            if not _SPEC_TAG_RE.search(tags_text):
                errors.append(ValidationError(
                    path,
                    line_no,
                    "Scenario must include at least one @spec_* tag",
                ))
            for tag_line_no, tag in pending_tags:
                if tag.startswith("@suite_"):
                    errors.append(ValidationError(
                        path,
                        tag_line_no,
                        "@suite_* tags must be Feature-level only",
                    ))
            pending_tags = []
            continue
        if _BACKGROUND_RE.match(line):
            pending_tags = []
            continue
        pending_tags = []
    return errors


def validate_feature_file(path: Path, allowed_suite_tags: set[str]) -> list[ValidationError]:
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError as e:
        return [ValidationError(path, 0, f"failed to read file: {e}")]

    lines = text.splitlines()
    errors: list[ValidationError] = []

    for line_no, line in enumerate(lines, start=1):
        if "\t" in line:
            errors.append(ValidationError(path, line_no, "tabs are not allowed in .feature files"))

    errors.extend(_validate_top_matter(path, lines, allowed_suite_tags))
    errors.extend(_validate_narrative_block(path, lines))
    errors.extend(_validate_scenarios_have_traceability_tags(path, lines))
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate Gherkin feature file conventions.")
    parser.add_argument(
        "--report",
        type=Path,
        metavar="PATH",
        help="Write report to this path (e.g. dev_docs/feature_files_validation_report.txt)",
    )
    parser.add_argument(
        "paths",
        nargs="*",
        help="Optional feature file paths to validate (default: all under features/)",
    )
    args = parser.parse_args()

    root = _repo_root()
    allowed_suites = _load_allowed_suite_tags(root)
    targets = [Path(p) for p in args.paths] if args.paths else _iter_feature_files(root)

    all_errors: list[ValidationError] = []
    for target in targets:
        path = (root / target).resolve() if not target.is_absolute() else target
        if not path.is_file():
            all_errors.append(ValidationError(path, 0, "target is not a file"))
            continue
        all_errors.extend(validate_feature_file(path, allowed_suites))

    report_lines: list[str] = []
    if all_errors:
        report_lines.append("Feature file validation: FAILED")
        report_lines.append("")
        for err in sorted(all_errors, key=lambda e: (str(e.path), e.line_no, e.message)):
            report_lines.append(f"  {err.path}:{err.line_no}: {err.message}")
        for line in report_lines:
            print(line)
        if args.report:
            args.report.parent.mkdir(parents=True, exist_ok=True)
            args.report.write_text("\n".join(report_lines) + "\n", encoding="utf-8")
        return 1

    report_lines.append("Feature file validation: OK")
    for line in report_lines:
        print(line)
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text("\n".join(report_lines) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
