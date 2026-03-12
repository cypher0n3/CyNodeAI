# Secure Browser Service

- [Document Overview](#document-overview)
- [Service Purpose](#service-purpose)
- [Agent Interaction Model](#agent-interaction-model)
- [Deterministic Sanitization](#deterministic-sanitization)
- [Configuration and Rules](#configuration-and-rules)
- [Access Control](#access-control)
- [Policy and Auditing](#policy-and-auditing)

## Document Overview

- Spec ID: `CYNAI.BROWSR.Doc.SecureBrowserService` <a id="spec-cynai-browsr-doc-securebrowserservice"></a>

This document defines the Secure Browser Service, a service that retrieves web content on behalf of agents.
It returns sanitized plain text and strips common prompt-injection patterns using deterministic rules, not AI.

## Service Purpose

- Provide web access without granting direct internet egress to sandbox containers.
- Reduce prompt-injection risk by removing common instruction-like content from fetched pages.
- Return plain text suitable for downstream use as untrusted reference material.

## Agent Interaction Model

- Spec ID: `CYNAI.BROWSR.UntrustedContentHandling` <a id="spec-cynai-browsr-untrustedcontent"></a>

Agents do not browse the open web directly.
Instead, they submit a structured request to the orchestrator, which routes approved requests to the Secure Browser Service.

Minimum request fields

- `url`: absolute URL to retrieve
- `task_id`: task context for auditing and traceability
- `mode`: content extraction mode (e.g. auto, article, full_text)
- `max_chars`: maximum returned character count

Minimum response fields

- `status`: success|error
- `content_text`: sanitized plain text
- `metadata`: json object (final_url, fetched_at, content_type, title when available)
- `error`: structured error object when `status` is error

Security note

- Returned content MUST be treated as untrusted data and MUST NOT be interpreted as instructions.

Traces To:

- [REQ-BROWSR-0100](../requirements/browsr.md#req-browsr-0100)
- [REQ-BROWSR-0101](../requirements/browsr.md#req-browsr-0101)

## Deterministic Sanitization

- Spec ID: `CYNAI.BROWSR.DeterministicSanitization` <a id="spec-cynai-browsr-deterministicsanitization"></a>

The Secure Browser Service performs a deterministic sanitization pipeline that does not use AI.
The goal is to strip non-content and reduce common prompt-injection vectors before returning text.

### Extraction Steps

- Fetch and render content
  - Retrieve the page and follow redirects within policy.
  - Optionally render with a headless browser when required for meaningful text extraction.
- Parse HTML and remove non-content elements
  - Remove `script`, `style`, `noscript`, `svg`, `canvas`, `iframe`, and `form` elements.
  - Remove HTML comments and elements that are hidden (e.g. `display:none`, `aria-hidden=true`).
  - Remove common boilerplate containers by heuristic class or id matches (e.g. nav, footer, header, sidebar, ads, cookie).
- Convert to text
  - Convert remaining DOM to plain text with stable whitespace rules.
  - Collapse repeated whitespace and normalize line breaks.
- Apply injection-stripping rules
  - Remove lines matching a configured denylist of instruction-like patterns, such as:
    - "ignore previous instructions"
    - "system prompt"
    - "developer message"
    - "jailbreak"
    - "you are chatgpt"
  - Remove sections labeled as prompts, instructions, or policies when detected by deterministic markers.
- Truncate and annotate
  - Enforce `max_chars` and add a suffix indicating truncation when applicable.

### Sanitization Limitations

- Sanitization reduces risk but cannot guarantee removal of all malicious instructions.
- The agent and orchestrator SHOULD treat all fetched content as untrusted reference material.

## Configuration and Rules

- Spec ID: `CYNAI.BROWSR.PreferencesRules` <a id="spec-cynai-browsr-preferencesrules"></a>

The Secure Browser Service SHOULD be configured by user task-execution preferences and constraints stored in PostgreSQL.
This enables deterministic behavior changes without modifying code and allows per-user, per-project, and per-task overrides.
These preferences are not deployment or service configuration.

Preference source of truth

- Effective preferences MUST be resolved using the precedence model in [`docs/tech_specs/user_preferences.md`](user_preferences.md).
- The service SHOULD treat missing preference keys as system-scoped defaults.

Traces To:

- [REQ-BROWSR-0102](../requirements/browsr.md#req-browsr-0102)
- [REQ-BROWSR-0103](../requirements/browsr.md#req-browsr-0103)
- [REQ-BROWSR-0104](../requirements/browsr.md#req-browsr-0104)
- [REQ-BROWSR-0105](../requirements/browsr.md#req-browsr-0105)

Optional YAML import and export

- A YAML file MAY be used to seed system-scoped defaults into PostgreSQL or to export the effective rules for review.
- The prototype YAML format is defined by `docs/tech_specs/secure_browser_rules.yaml`.

Recommended preference controls

- Fetch controls
  - `respect_robots`: whether to respect `robots.txt` (default: true)
  - `user_agent`: user agent string used for fetching
  - `timeout_seconds`: request timeout
  - `max_bytes`: maximum fetched bytes before extraction
- Redirect controls
  - `max_redirect_hops`: maximum number of redirects (default: small, e.g. 5)
  - `allow_cross_domain_redirects`: whether redirects may change domains
  - `blocked_redirect_url_patterns`: regex patterns for known bot challenges and consent flows
- Extraction controls
  - `mode_defaults`: default extraction mode mapping
  - `strip_elements`: HTML tags to remove
  - `strip_selectors`: heuristic selectors to remove (nav, footer, ads, cookie banners)
- Injection stripping controls
  - `denylist_line_patterns`: regex patterns for instruction-like lines to remove
  - `denylist_section_markers`: deterministic markers for sections to drop
  - `max_output_chars`: maximum returned characters after sanitization

## Robots and Redirect Handling

- Spec ID: `CYNAI.BROWSR.RobotsRedirectHandling` <a id="spec-cynai-browsr-robotsredirects"></a>

The Secure Browser Service MUST implement deterministic handling for `robots.txt` and redirects.

Traces To:

- [REQ-BROWSR-0106](../requirements/browsr.md#req-browsr-0106)
- [REQ-BROWSR-0107](../requirements/browsr.md#req-browsr-0107)
- [REQ-BROWSR-0108](../requirements/browsr.md#req-browsr-0108)
- [REQ-BROWSR-0109](../requirements/browsr.md#req-browsr-0109)
- [REQ-BROWSR-0110](../requirements/browsr.md#req-browsr-0110)
- [REQ-BROWSR-0111](../requirements/browsr.md#req-browsr-0111)
- [REQ-BROWSR-0112](../requirements/browsr.md#req-browsr-0112)
- [REQ-BROWSR-0113](../requirements/browsr.md#req-browsr-0113)
- [REQ-BROWSR-0114](../requirements/browsr.md#req-browsr-0114)

### Robots Policy

- The service SHOULD fetch and cache `robots.txt` per domain.
- The default behavior MUST be to respect `robots.txt` when `respect_robots` is true.
- If `respect_robots` is false, the service MAY fetch content even when disallowed by `robots.txt`.
- The service SHOULD record whether `robots.txt` was applied for each fetch in the audit log.

### Redirect Policy

- The service SHOULD follow HTTP redirects up to `max_redirect_hops`.
- Redirects SHOULD be constrained by policy, including scheme restrictions and allowlisted domains.
- The service SHOULD detect and block common "AI request" redirects, such as bot challenges and consent walls,
  using deterministic URL pattern matching from configuration.
- The service SHOULD record the final URL and redirect chain length in response metadata.

## Access Control

- Spec ID: `CYNAI.BROWSR.AccessControl` <a id="spec-cynai-browsr-accesscontrol"></a>

The Secure Browser Service MUST enforce access control for outbound fetches.
Access control rules are defined in [`docs/tech_specs/access_control.md`](access_control.md).

Recommended checks

- Subject identity MUST be resolved to a user context.
- The requested URL MUST pass scheme checks and domain allow policy for that subject.
- The service SHOULD enforce maximum response size and concurrency limits by subject and by task.
- The service SHOULD restrict redirect behavior within policy.

Traces To:

- [REQ-BROWSR-0115](../requirements/browsr.md#req-browsr-0115)
- [REQ-BROWSR-0116](../requirements/browsr.md#req-browsr-0116)
- [REQ-BROWSR-0117](../requirements/browsr.md#req-browsr-0117)
- [REQ-BROWSR-0118](../requirements/browsr.md#req-browsr-0118)

## Policy and Auditing

- Spec ID: `CYNAI.BROWSR.PolicyAuditing` <a id="spec-cynai-browsr-policyauditing"></a>

The orchestrator and Secure Browser Service enforce outbound browsing policy.

- Policy checks SHOULD include domain allowlists, scheme restrictions, and per-task constraints.
- Requests SHOULD support rate limiting and concurrency limits.
- All fetches SHOULD be logged with task context, final URL, and timing information.
- Responses SHOULD be filtered to avoid leaking secrets or internal network information.

Traces To:

- [REQ-BROWSR-0119](../requirements/browsr.md#req-browsr-0119)
- [REQ-BROWSR-0120](../requirements/browsr.md#req-browsr-0120)
- [REQ-BROWSR-0121](../requirements/browsr.md#req-browsr-0121)
- [REQ-BROWSR-0122](../requirements/browsr.md#req-browsr-0122)
