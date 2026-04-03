"""Small JSON helpers for E2E scripts (split from helpers.py for maintainability index)."""

# Traces: REQ-ORCHES-0120 (E2E JSON helpers for task/gateway output).

import json
import urllib.error
import urllib.request

from scripts.test_scripts import config


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


def fetch_gateway_refresh_tokens(refresh_token, timeout=30):
    """POST /v1/auth/refresh; return (access_token, refresh_token) or (None, None)."""
    if not refresh_token or not str(refresh_token).strip():
        return None, None
    url = config.USER_API.rstrip("/") + "/v1/auth/refresh"
    body = json.dumps({"refresh_token": refresh_token}).encode()
    req = urllib.request.Request(
        url,
        data=body,
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            if resp.status != 200:
                return None, None
            data = json.loads(resp.read().decode())
            acc = data.get("access_token")
            ref = data.get("refresh_token")
            if isinstance(acc, str) and acc.strip():
                return acc.strip(), ref.strip() if isinstance(ref, str) else None
            return None, None
    except (urllib.error.URLError, OSError, json.JSONDecodeError, ValueError, TypeError):
        return None, None


def fetch_gateway_login_json(timeout=30):
    """POST /v1/auth/login; return decoded JSON body on 200, else None."""
    url = config.USER_API.rstrip("/") + "/v1/auth/login"
    body = json.dumps(
        {"handle": "admin", "password": config.ADMIN_PASSWORD}
    ).encode()
    req = urllib.request.Request(
        url,
        data=body,
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            if resp.status != 200:
                return None
            return json.loads(resp.read().decode())
    except (urllib.error.URLError, OSError, json.JSONDecodeError, ValueError, TypeError):
        return None


def get_sba_job_result(result_data):
    """Job result from task result (jobs[0].result or parsed stdout). Return dict or None."""
    job_result = jq_get(result_data, "jobs", 0, "result")
    if not job_result and result_data:
        raw = result_data.get("stdout")
        if isinstance(raw, str):
            job_result = parse_json_safe(raw)
    return job_result
