"""Small JSON helpers for E2E scripts (split from helpers.py for maintainability index)."""

# Traces: REQ-ORCHES-0120 (E2E JSON helpers for task/gateway output).

import json


def parse_json_safe(text):
    """Parse JSON; return dict or None."""
    try:
        return json.loads(text) if text else None
    except json.JSONDecodeError:
        return None


def jq_get(obj, *keys, default=None):
    """Get nested key; e.g. jq_get(d, 'jobs', 0, 'result')."""
    for k in keys:
        if obj is None or not isinstance(obj, (dict, list)):
            return default
        if isinstance(obj, list) and isinstance(k, int):
            obj = obj[k] if 0 <= k < len(obj) else None
        else:
            obj = obj.get(k) if isinstance(obj, dict) else None
    return obj


def get_sba_job_result(result_data):
    """Job result from task result (jobs[0].result or parsed stdout). Return dict or None."""
    job_result = jq_get(result_data, "jobs", 0, "result")
    if not job_result and result_data:
        raw = result_data.get("stdout")
        if isinstance(raw, str):
            job_result = parse_json_safe(raw)
    return job_result
