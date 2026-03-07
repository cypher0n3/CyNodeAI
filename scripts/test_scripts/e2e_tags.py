"""E2E test tags (aligned with features/README.md suite tags).

Test classes may set a class attribute: tags = ["suite_orchestrator", ...].
Multiple tags are allowed; run_e2e.py --tags X runs any test that has at least one matching tag.
When include_tags is used, prereq modules (config, login, task create) are run first so
dependencies and setup steps are satisfied in the correct sequence.

Targeted tags for subsets:
- full_demo: run during `just setup-dev full-demo` (excludes tests that use a subset of the stack).
- inference: any test that exercises the inference path (Ollama/LLM).
- pma_inference: inference via PMA (gateway chat, worker PMA proxy with inference).
- sba_inference: SBA tasks that use inference.
- auth, task, chat, worker: smaller subsets for focused runs.
"""

import unittest

# Suite tags match features/README.md registry (component ownership)
SUITE_ORCHESTRATOR = "suite_orchestrator"
SUITE_WORKER_NODE = "suite_worker_node"
SUITE_AGENTS = "suite_agents"
SUITE_CYNORK = "suite_cynork"
SUITE_E2E = "suite_e2e"
# Minimal proxy + PMA (no orchestrator; worker proxy and PMA only)
SUITE_PROXY_PMA = "suite_proxy_pma"

# Full-demo and inference subset tags
TAG_FULL_DEMO = "full_demo"
TAG_INFERENCE = "inference"
TAG_PMA_INFERENCE = "pma_inference"
TAG_SBA_INFERENCE = "sba_inference"
TAG_AUTH = "auth"
TAG_TASK = "task"
TAG_CHAT = "chat"
TAG_WORKER = "worker"
TAG_PMA = "pma"

# Modules that provide shared state for other E2E tests. When filtering by include_tags,
# these are always included (in discovery order) so dependencies run before tag-matched tests.
PREREQ_MODULES = (
    "scripts.test_scripts.e2e_010_cli_version_and_status",  # config dir
    "scripts.test_scripts.e2e_020_auth_login",              # auth token
    "scripts.test_scripts.e2e_050_task_create",             # TASK_ID for workflow/task list/etc
)


def get_tags_for_test(test):
    """Return the set of tags for this test (from its class). Empty if no tags attribute."""
    if not hasattr(test, "__class__"):
        return set()
    cls = test.__class__
    if not hasattr(cls, "tags"):
        return set()
    tags = cls.tags
    if isinstance(tags, (list, tuple)):
        return set(t.strip() for t in tags if isinstance(t, str) and t.strip())
    return set()


def _test_module(test):
    """Return the module name of the test's class, or empty string."""
    if not hasattr(test, "__class__"):
        return ""
    return getattr(test.__class__, "__module__", "") or ""


def filter_suite_by_tags(suite, include_tags=None, exclude_tags=None):
    """Return a new suite containing only tests that pass include/exclude tag filters.

    - include_tags: if non-empty, test must have at least one of these tags (class.tags).
      Tests from PREREQ_MODULES are always included when include_tags is set, so setup
      and dependencies run in the correct sequence before tag-matched tests.
    - exclude_tags: if non-empty, test must have none of these tags.
    - Multiple tags on a test: if any of the test's tags are in include_tags, the test runs.
    """
    if not include_tags and not exclude_tags:
        return suite
    include_set = set(t.strip() for t in (include_tags or []))
    exclude_set = set(t.strip() for t in (exclude_tags or []))

    def should_include(test):
        if not isinstance(test, unittest.TestCase):
            return False
        # When filtering by include_tags, always include prereq modules first (order preserved).
        if include_set and _test_module(test) in PREREQ_MODULES:
            if exclude_set and (get_tags_for_test(test) & exclude_set):
                return False
            return True
        tags = get_tags_for_test(test)
        if include_set and not tags & include_set:
            return False
        if exclude_set and (tags & exclude_set):
            return False
        return True

    def iter_cases(suite_or_case):
        try:
            for t in suite_or_case:
                yield from iter_cases(t)
        except TypeError:
            yield suite_or_case

    filtered = unittest.suite.TestSuite()
    for test in iter_cases(suite):
        if should_include(test):
            filtered.addTest(test)
    return filtered
