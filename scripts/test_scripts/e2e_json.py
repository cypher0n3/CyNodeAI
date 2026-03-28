"""Loose JSON parsing for E2E (cynork / HTTP stdout noise)."""

# Traces: REQ-ORCHES-0120 (E2E parse cynork / gateway JSON output).

import json


def parse_json_loose(text):
    """Parse a JSON object from text; tolerate leading/trailing noise.

    ``cynork -o json`` should print a single object; this recovers when stdout
    contains a prefix/suffix (e.g. stray log lines) by scanning for ``{``.
    """
    if not text or not str(text).strip():
        return None
    s = str(text).strip()
    try:
        o = json.loads(s)
        if isinstance(o, dict):
            return o
    except json.JSONDecodeError:
        pass
    dec = json.JSONDecoder()
    for i, ch in enumerate(s):
        if ch != "{":
            continue
        try:
            obj, _ = dec.raw_decode(s[i:])
            if isinstance(obj, dict):
                return obj
        except json.JSONDecodeError:
            continue
    return None
