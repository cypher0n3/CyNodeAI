#!/usr/bin/env python3
"""Minimal HTTP client that walks workflow checkpoint node IDs (stdlib only).

This is a **contract reference** for the LangGraph MVP workflow runner: it uses the
orchestrator Workflow Start/Resume API (REQ-ORCHES-0144, REQ-ORCHES-0145) the same
way a Python LangGraph process would persist graph progress.

It does **not** import LangGraph; integrate the real library in a dedicated runner
image when P2-06 is wired to MCP and Worker API.

Environment:
  WORKFLOW_STUB_BASE_URL - e.g. http://127.0.0.1:12082
  WORKFLOW_STUB_TOKEN    - Bearer (admin access token)
  WORKFLOW_STUB_TASK_ID  - uuid task to drive

Exit 0 on success, 1 on failure.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.request


def _post_json(url: str, token: str, body: dict) -> tuple[int, bytes]:
    data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=120) as resp:
            return resp.getcode(), resp.read()
    except urllib.error.HTTPError as e:
        return e.code, e.read()


def main() -> int:
    p = argparse.ArgumentParser(description="Minimal workflow checkpoint walker (stdlib).")
    p.add_argument("--base-url", default=os.environ.get("WORKFLOW_STUB_BASE_URL", ""))
    p.add_argument("--token", default=os.environ.get("WORKFLOW_STUB_TOKEN", ""))
    p.add_argument("--task-id", default=os.environ.get("WORKFLOW_STUB_TASK_ID", ""))
    p.add_argument("--holder-id", default="workflow-stub-runner")
    args = p.parse_args()
    if not args.base_url or not args.token or not args.task_id:
        print("base-url, token, and task-id required (flags or env)", file=sys.stderr)
        return 1
    base = args.base_url.rstrip("/")
    tok = args.token.strip()
    tid = args.task_id.strip()

    code, body = _post_json(
        f"{base}/v1/workflow/start",
        tok,
        {"task_id": tid, "holder_id": args.holder_id},
    )
    if code not in (200, 409):
        print(f"start failed {code}: {body!r}", file=sys.stderr)
        return 1
    start = json.loads(body.decode("utf-8") or "{}")
    lease_id = (start or {}).get("lease_id") or ""

    verify_state = json.dumps(
        {
            "pma_tasked_paa": True,
            "paa_outcome": "accepted",
            "findings": "criteria met (stub runner)",
        }
    )
    cp_body = {"task_id": tid, "last_node_id": "verify_step_result", "state": verify_state}
    code2, _ = _post_json(f"{base}/v1/workflow/checkpoint", tok, cp_body)
    if code2 != 204:
        print(f"checkpoint failed {code2}", file=sys.stderr)
        return 1

    code3, rb = _post_json(
        f"{base}/v1/workflow/resume", tok, {"task_id": tid}
    )
    if code3 != 200:
        print(f"resume failed {code3}: {rb!r}", file=sys.stderr)
        return 1
    out = json.loads(rb.decode("utf-8") or "{}")
    if (out or {}).get("last_node_id") != "verify_step_result":
        print(f"unexpected last_node_id: {out!r}", file=sys.stderr)
        return 1
    st = (out or {}).get("state")
    if not st or "paa_outcome" not in str(st):
        print(f"state missing review payload: {out!r}", file=sys.stderr)
        return 1

    if lease_id:
        code4, _ = _post_json(
            f"{base}/v1/workflow/release",
            tok,
            {"task_id": tid, "lease_id": lease_id},
        )
        if code4 != 204:
            print(f"release failed {code4}", file=sys.stderr)
            return 1

    print("workflow stub runner: verify_step_result checkpoint ok")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
