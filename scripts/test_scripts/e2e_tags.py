"""E2E test tags (aligned with features/README.md suite tags).

Test classes may set:
- tags = ["suite_orchestrator", ...] for filtering (run_e2e.py --tags X).
- prereqs = ["gateway", "config", "auth"] for setup; only whitelisted names are run.
Prereqs are defined in the test class; the runner runs only those setup steps
(no global prereq list).

Targeted tags for subsets:
- full_demo: run during `just setup-dev full-demo`
  (excludes tests that use a subset of the stack).
- inference: any test that exercises the inference path (Ollama/LLM).
- no_inference: test does not require a running LLM/Ollama; use for fast or CI runs
  without inference.
- pma_inference: inference via PMA (gateway chat, worker PMA proxy with inference).
- PMA warm pool (cynodeai-managed-pma-pool-*): orchestrator keeps spare slots (REQ-ORCHES-0192);
  login assigns slots to sessions; pool shrinks on logout or admin revoke_sessions (e2e_0831,
  e2e_0840). Class e2e_0831 uses no_inference only (no LLM required for container lifecycle).
- sba_inference: SBA tasks that use inference.
- auth, task, chat, worker: smaller subsets for focused runs.

Logical group tags (run related tests together):
- tui: all TUI/PTY tests (slash commands, PTY harness, TUI streaming).
- streaming: SSE, gateway streaming contract, cynork transport, PMA path, TUI streaming.
- control_plane: node register, capability, workflow API.
- sba: all SBA (sandbox agent) tests.
- gateway: user gateway health and streaming contract.
- nats: JetStream session lifecycle / connectivity (gateway + control-plane NATS paths).
- uds_routing: inference proxy UDS and run-args contract tests (worker_node).
- worker_node: use suite_worker_node for all worker/node tests.
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
TAG_GPU_VARIANT = "gpu_variant"
TAG_NO_INFERENCE = "no_inference"
# Logical group tags
TAG_TUI = "tui"
TAG_STREAMING = "streaming"
TAG_CONTROL_PLANE = "control_plane"
TAG_SBA = "sba"
TAG_GATEWAY = "gateway"
TAG_NATS = "nats"
TAG_UDS_ROUTING = "uds_routing"

# Prereq names (whitelist). Tests declare prereqs = ["gateway", "config", ...];
# runner runs only these.
PREREQ_GATEWAY = "gateway"
PREREQ_CONFIG = "config"
PREREQ_AUTH = "auth"
PREREQ_TASK_ID = "task_id"
PREREQ_OLLAMA = "ollama"
# Gateway PMA chat path ready (after host Ollama smoke); only for tests tagged pma_inference.
PREREQ_PMA_CHAT = "pma_chat"
PREREQ_NODE_REGISTER = "node_register"

PREREQ_WHITELIST = frozenset[str]({
    PREREQ_GATEWAY,
    PREREQ_CONFIG,
    PREREQ_AUTH,
    PREREQ_TASK_ID,
    PREREQ_OLLAMA,
    PREREQ_PMA_CHAT,
    PREREQ_NODE_REGISTER,
})

# Order in which runner runs prereq setup (config before auth before task_id, etc.).
PREREQ_ORDER = (
    PREREQ_GATEWAY,
    PREREQ_CONFIG,
    PREREQ_AUTH,
    PREREQ_NODE_REGISTER,
    PREREQ_TASK_ID,
    PREREQ_OLLAMA,
    PREREQ_PMA_CHAT,
)

# Prereqs that must be re-run before every test that needs them (e.g. login token state).
PREREQ_ALWAYS_RERUN = frozenset({PREREQ_AUTH})


def get_prereqs_for_test(test):
    """Return the set of whitelisted prereq names for this test (from its class.prereqs)."""
    if not hasattr(test, "__class__"):
        return set()
    cls = test.__class__
    if not hasattr(cls, "prereqs"):
        return set()
    p = cls.prereqs
    if isinstance(p, (list, tuple)):
        return PREREQ_WHITELIST & set(str(x).strip() for x in p if x)
    return set()


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
    - exclude_tags: if non-empty, test must have none of these tags.
    - No prereq injection; runner provides config/auth/TASK_ID via helpers.
    """
    if not include_tags and not exclude_tags:
        return suite
    include_set = set(t.strip() for t in (include_tags or []))
    exclude_set = set(t.strip() for t in (exclude_tags or []))

    def should_include(test):
        if not isinstance(test, unittest.TestCase):
            return False
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
