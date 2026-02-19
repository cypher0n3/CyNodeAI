# BROWSR Requirements

- [1 Overview](#1-overview)
- [2 Requirements](#2-requirements)

## 1 Overview

This document consolidates requirements for the `BROWSR` domain.
It covers secure browser service behavior, rules, and deterministic sanitization requirements.

## 2 Requirements

- **REQ-BROWSR-0001:** Secure browser service: rules and deterministic sanitization for web content.
  [CYNAI.BROWSR.Doc.SecureBrowserService](../tech_specs/secure_browser_service.md#spec-cynai-browsr-doc-securebrowserservice)
  <a id="req-browsr-0001"></a>
- **REQ-BROWSR-0100:** Returned content MUST be treated as untrusted data and MUST NOT be interpreted as instructions.
  [CYNAI.BROWSR.UntrustedContentHandling](../tech_specs/secure_browser_service.md#spec-cynai-browsr-untrustedcontent)
  <a id="req-browsr-0100"></a>
- **REQ-BROWSR-0101:** The agent and orchestrator SHOULD treat all fetched content as untrusted reference material.
  [CYNAI.BROWSR.UntrustedContentHandling](../tech_specs/secure_browser_service.md#spec-cynai-browsr-untrustedcontent)
  <a id="req-browsr-0101"></a>
- **REQ-BROWSR-0102:** The Secure Browser Service SHOULD be configured by preferences stored in PostgreSQL.
  [CYNAI.BROWSR.PreferencesRules](../tech_specs/secure_browser_service.md#spec-cynai-browsr-preferencesrules)
  <a id="req-browsr-0102"></a>
- **REQ-BROWSR-0103:** Effective preferences MUST be resolved using the scope precedence model.
  [CYNAI.BROWSR.PreferencesRules](../tech_specs/secure_browser_service.md#spec-cynai-browsr-preferencesrules)
  <a id="req-browsr-0103"></a>
- **REQ-BROWSR-0104:** The service SHOULD treat missing preference keys as system defaults.
  [CYNAI.BROWSR.PreferencesRules](../tech_specs/secure_browser_service.md#spec-cynai-browsr-preferencesrules)
  <a id="req-browsr-0104"></a>
- **REQ-BROWSR-0105:** A YAML file MAY be used to seed system defaults into PostgreSQL or to export the effective rules for review.
  [CYNAI.BROWSR.PreferencesRules](../tech_specs/secure_browser_service.md#spec-cynai-browsr-preferencesrules)
  <a id="req-browsr-0105"></a>
- **REQ-BROWSR-0106:** The Secure Browser Service MUST implement deterministic handling for `robots.txt` and redirects.
  [CYNAI.BROWSR.RobotsRedirectHandling](../tech_specs/secure_browser_service.md#spec-cynai-browsr-robotsredirects)
  <a id="req-browsr-0106"></a>
- **REQ-BROWSR-0107:** The service SHOULD fetch and cache `robots.txt` per domain.
  [CYNAI.BROWSR.RobotsRedirectHandling](../tech_specs/secure_browser_service.md#spec-cynai-browsr-robotsredirects)
  <a id="req-browsr-0107"></a>
- **REQ-BROWSR-0108:** The default behavior MUST be to respect `robots.txt` when `respect_robots` is true.
  [CYNAI.BROWSR.RobotsRedirectHandling](../tech_specs/secure_browser_service.md#spec-cynai-browsr-robotsredirects)
  <a id="req-browsr-0108"></a>
- **REQ-BROWSR-0109:** If `respect_robots` is false, the service MAY fetch content even when disallowed by `robots.txt`.
  [CYNAI.BROWSR.RobotsRedirectHandling](../tech_specs/secure_browser_service.md#spec-cynai-browsr-robotsredirects)
  <a id="req-browsr-0109"></a>
- **REQ-BROWSR-0110:** The service SHOULD record whether `robots.txt` was applied for each fetch in the audit log.
  [CYNAI.BROWSR.RobotsRedirectHandling](../tech_specs/secure_browser_service.md#spec-cynai-browsr-robotsredirects)
  <a id="req-browsr-0110"></a>
- **REQ-BROWSR-0111:** The service SHOULD follow HTTP redirects up to `max_redirect_hops`.
  [CYNAI.BROWSR.RobotsRedirectHandling](../tech_specs/secure_browser_service.md#spec-cynai-browsr-robotsredirects)
  <a id="req-browsr-0111"></a>
- **REQ-BROWSR-0112:** Redirects SHOULD be constrained by policy, including scheme restrictions and allowlisted domains.
  [CYNAI.BROWSR.RobotsRedirectHandling](../tech_specs/secure_browser_service.md#spec-cynai-browsr-robotsredirects)
  <a id="req-browsr-0112"></a>
- **REQ-BROWSR-0113:** The service SHOULD detect and block common \"AI request\" redirects (bot challenges, consent walls).
  [CYNAI.BROWSR.RobotsRedirectHandling](../tech_specs/secure_browser_service.md#spec-cynai-browsr-robotsredirects)
  <a id="req-browsr-0113"></a>
- **REQ-BROWSR-0114:** The service SHOULD record the final URL and redirect chain length in response metadata.
  [CYNAI.BROWSR.RobotsRedirectHandling](../tech_specs/secure_browser_service.md#spec-cynai-browsr-robotsredirects)
  <a id="req-browsr-0114"></a>
- **REQ-BROWSR-0115:** The Secure Browser Service MUST enforce access control for outbound fetches.
  [CYNAI.BROWSR.AccessControl](../tech_specs/secure_browser_service.md#spec-cynai-browsr-accesscontrol)
  <a id="req-browsr-0115"></a>
- **REQ-BROWSR-0116:** Subject identity MUST be resolved to a user context.
  [CYNAI.BROWSR.AccessControl](../tech_specs/secure_browser_service.md#spec-cynai-browsr-accesscontrol)
  <a id="req-browsr-0116"></a>
- **REQ-BROWSR-0117:** The requested URL MUST pass scheme checks and domain allow policy for that subject.
  [CYNAI.BROWSR.AccessControl](../tech_specs/secure_browser_service.md#spec-cynai-browsr-accesscontrol)
  <a id="req-browsr-0117"></a>
- **REQ-BROWSR-0118:** The service SHOULD enforce maximum response size and concurrency limits by subject and by task.
  [CYNAI.BROWSR.AccessControl](../tech_specs/secure_browser_service.md#spec-cynai-browsr-accesscontrol)
  <a id="req-browsr-0118"></a>
- **REQ-BROWSR-0119:** Policy checks SHOULD include domain allowlists, scheme restrictions, and per-task constraints.
  [CYNAI.BROWSR.PolicyAuditing](../tech_specs/secure_browser_service.md#spec-cynai-browsr-policyauditing)
  <a id="req-browsr-0119"></a>
- **REQ-BROWSR-0120:** Requests SHOULD support rate limiting and concurrency limits.
  [CYNAI.BROWSR.PolicyAuditing](../tech_specs/secure_browser_service.md#spec-cynai-browsr-policyauditing)
  <a id="req-browsr-0120"></a>
- **REQ-BROWSR-0121:** All fetches SHOULD be logged with task context, final URL, and timing information.
  [CYNAI.BROWSR.PolicyAuditing](../tech_specs/secure_browser_service.md#spec-cynai-browsr-policyauditing)
  <a id="req-browsr-0121"></a>
- **REQ-BROWSR-0122:** Responses SHOULD be filtered to avoid leaking secrets or internal network information.
  [CYNAI.BROWSR.PolicyAuditing](../tech_specs/secure_browser_service.md#spec-cynai-browsr-policyauditing)
  <a id="req-browsr-0122"></a>
