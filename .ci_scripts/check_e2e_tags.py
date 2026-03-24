#!/usr/bin/env python3
"""
Check E2E test scripts for required tags and convention (tags after docstring).

- Discovers scripts/test_scripts/e2e_*.py (excludes e2e_state.py, e2e_tags.py).
- Each test class (unittest.TestCase subclass) must have a non-empty `tags` attribute.
- Multiple tags per class are allowed; if any requested tag matches, the test is run.
- Tags must appear after the class docstring in source order.

Exit code: 0 when all checks pass; 1 when any validation fails.
"""

from __future__ import annotations

import ast
import sys
from pathlib import Path


def _repo_root() -> Path:
    return Path(__file__).resolve().parent.parent


# Allowed suite tags (must match e2e_tags.py and features/README.md).
# Includes suite_* plus descriptive tags used by e2e test classes.
ALLOWED_TAGS = frozenset({
    "suite_orchestrator",
    "suite_worker_node",
    "suite_agents",
    "suite_cynork",
    "suite_e2e",
    "suite_proxy_pma",
    "full_demo",
    "auth",
    "task",
    "inference",
    "no_inference",
    "sba_inference",
    "pma_inference",
    "pma",
    "chat",
    "chat_capable",
    "worker",
    "uds_routing",
    "tui_pty",
    "tui",
    "streaming",
    "control_plane",
    "sba",
    "gateway",
    "gpu_variant",
    "artifacts",
})


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


def _is_test_case_base(node: ast.ClassDef) -> bool:
    for base in node.bases:
        if isinstance(base, ast.Attribute):
            if base.attr == "TestCase":
                return True
        if isinstance(base, ast.Name):
            if base.id == "TestCase":
                return True
    return False


def _validate_one_tag(node: ast.ClassDef, el: ast.expr, lineno: int) -> list[tuple[int, str]]:
    """Check one tag element; return issues (0 or 1 item)."""
    if isinstance(el, ast.Constant) and isinstance(el.value, str):
        if el.value.strip() and el.value not in ALLOWED_TAGS:
            allowed = sorted(ALLOWED_TAGS)
            msg = f"test class {node.name}: unknown tag {el.value!r} (allowed: {allowed})"
            return [(lineno, msg)]
    elif not (isinstance(el, ast.Constant) and isinstance(el.value, str)):
        return [(lineno, f"test class {node.name}: tags must contain only strings")]
    return []


def _check_tags_value(node: ast.ClassDef) -> list[tuple[int, str]]:
    """Return list of (line_no, message) for invalid tags value (missing, empty, or invalid)."""
    for stmt in node.body:
        if not isinstance(stmt, ast.Assign):
            continue
        for t in stmt.targets:
            if not isinstance(t, ast.Name) or t.id != "tags":
                continue
            if not isinstance(stmt.value, (ast.List, ast.Tuple)):
                msg = f"test class {node.name}: tags must be a list or tuple"
                return [(stmt.lineno, msg)]
            elts = stmt.value.elts
            if not elts:
                return [(stmt.lineno, f"test class {node.name}: tags must be non-empty")]
            issues = []
            for el in elts:
                issues.extend(_validate_one_tag(node, el, stmt.lineno))
            return issues
    return []


def _has_tags_attr(node: ast.ClassDef) -> bool:
    for stmt in node.body:
        if isinstance(stmt, ast.Assign):
            for t in stmt.targets:
                if isinstance(t, ast.Name) and t.id == "tags":
                    return True
    return False


def check_file(path: Path) -> list[tuple[int, str]]:
    """Run all tag checks on one file. Return list of (line_no, message)."""
    issues = []
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as e:
        return [(0, f"read error: {e}")]
    try:
        tree = ast.parse(text)
    except SyntaxError as e:
        return [(e.lineno or 0, f"syntax error: {e}")]

    for node in ast.walk(tree):
        if not isinstance(node, ast.ClassDef):
            continue
        if not _is_test_case_base(node):
            continue
        if not _has_tags_attr(node):
            msg = f"test class {node.name}: missing required 'tags' attribute"
            issues.append((node.lineno, msg))
        else:
            issues.extend(_check_tags_value(node))
        issues.extend(_class_body_order_issues_for_class(node))

    return issues


def _class_body_order_issues_for_class(node: ast.ClassDef) -> list[tuple[int, str]]:
    """Return issues for this class only (tags after docstring)."""
    docstring_lineno = None
    tags_lineno = None
    for stmt in node.body:
        if isinstance(stmt, ast.Expr) and isinstance(stmt.value, ast.Constant):
            if isinstance(stmt.value.value, str) and docstring_lineno is None:
                docstring_lineno = stmt.lineno
        if isinstance(stmt, ast.Assign):
            for t in stmt.targets:
                if isinstance(t, ast.Name) and t.id == "tags":
                    if tags_lineno is None:
                        tags_lineno = stmt.lineno
                    break
    if tags_lineno is None:
        return []
    if docstring_lineno is None:
        msg = f"test class {node.name}: no class docstring (required before tags)"
        return [(node.lineno, msg)]
    if tags_lineno <= docstring_lineno:
        msg = f"test class {node.name}: tags must appear after the class docstring"
        return [(tags_lineno, msg)]
    return []


def main() -> int:
    repo_root = _repo_root()
    paths = _e2e_script_paths(repo_root)
    all_issues: list[tuple[Path, int, str]] = []
    for path in paths:
        rel = path.relative_to(repo_root)
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
