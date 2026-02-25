# Node Registration: Password-Protected Bundle and Encryption Without TLS (Draft)

- [1. Overview](#1-overview)
- [2. Goals and non-goals](#2-goals-and-non-goals)
- [3. Threat model and assumptions](#3-threat-model-and-assumptions)
- [4. Bundle model](#4-bundle-model)
- [5. Registration flow](#5-registration-flow)
- [6. Node identity at registration](#6-node-identity-at-registration)
- [7. Post-registration: rotating keys](#7-post-registration-rotating-keys)
- [8. Single-node automatic setup](#8-single-node-automatic-setup)
- [9. Node configuration and bundle discovery](#9-node-configuration-and-bundle-discovery)
- [10. Open points for refinement](#10-open-points-for-refinement)

## 1. Overview

- **Document type:** Draft technical specification
- **Status:** Draft, for refinement
- **Applies to:** Worker node registration and initial secure communication with the orchestrator
- **Purpose:** Define a registration and encryption model that (1) does not require users to set up or manage TLS certificates or CAs for CyNodeAI, (2) uses a per-node, password-encrypted bundle as a one-time bearer credential for initial connectivity, and (3) relies on orchestrator-issued rotating (post-quantum) keys for ongoing encrypted traffic after registration.

This draft extends or replaces the current "registration PSK" and TLS-optional model described in [`worker_node.md`](../tech_specs/worker_node.md) and [`worker_node_payloads.md`](../tech_specs/worker_node_payloads.md).

## 2. Goals and Non-Goals

This section states design goals and explicit non-goals for this draft.

### 2.1 Design Goals

- No TLS PKI burden: users do not manage TLS certs, CAs, or hostname-in-cert for CyNodeAI node-orchestrator traffic.
- Traffic remains encrypted and authenticated: initial exchange via bundle-derived material; ongoing traffic via orchestrator-issued rotating keys (post-quantum in target design).
- Per-node bearer credential: each bundle is a one-time credential; no pre-bound node identity at bundle creation time.
- Identity established at registration: the node that presents a valid bundle supplies its identity (hostname, IP, port, node_id, etc.) in the registration request; the orchestrator accepts that and creates the node record.
- Single-node use case: bundle can be generated and ingested by the worker node automatically (e.g. via script or compose) so the admin does not manually download and copy files.

### 2.2 Non-Goals (For This Draft)

- Defining the exact wire format or cryptographic algorithms (to be pinned in a later revision).
- User-gateway or MCP encryption (out of scope; can follow a similar pattern later).
- Mandating removal of existing PSK-based registration; this draft describes the preferred bundle-based model; migration from PSK can be a separate concern.

## 3. Threat Model and Assumptions

- **Eavesdropping:** Mitigated by encrypting registration and all subsequent traffic (bundle-derived key for initial exchange; rotating keys thereafter).
  No reliance on TLS for confidentiality.
- **Tampering:** All relevant payloads are authenticated (e.g. AEAD or MAC).
  Replay of registration is prevented by one-time use of the bundle credential.
- **MITM:** Without TLS, there is no PKI-based server authentication.
  Trust is: (1) the node trusts the orchestrator URL it has (from bundle or config) and (2) only a party that can verify the one-time credential can complete registration.
  The node assumes "whoever successfully completes the registration protocol and returns a valid bootstrap payload" is the intended orchestrator.
  For self-hosted single-orchestrator deployments this is acceptable; optional TLS can be layered later without being required.
- **Bundle at rest:** The bundle file is encrypted with a password chosen by the admin.
  Compromise of the file alone does not allow registration without the password.
  The node must not log or persist the password in plaintext.
- **Assumptions:** The admin provisions the node by giving it the bundle and the password (via secure channel or env).
  The orchestrator (or trusted tooling) is the only issuer of valid bundles.

## 4. Bundle Model

The bundle is a one-time bearer credential, password-encrypted at rest and issued without pre-bound node identity.

### 4.1 One-Time Bearer Credential

- Spec ID: `CYNAI.DRAFT.NodeBundleBearer`

A **node bundle** is a per-node, **one-time bearer credential** issued by the orchestrator (or by trusted tooling that the orchestrator can verify).
No node identity is pre-associated with the bundle when it is created; the orchestrator does not need to create or look up a node record before generating the bundle.

- **Whoever has the bundle** (and the password to decrypt it) **is allowed to register** with the orchestrator and receive configuration (bootstrap payload, JWT, rotating keys).
  The admin decides which system becomes a worker node by giving it the bundle and password.
- **One-time use:** Each bundle is valid for exactly one successful registration.
  After the first successful registration using that bundle, the credential is consumed (orchestrator marks it used).
  Any subsequent attempt to register with the same bundle must be rejected.
  Thus: one bundle yields one node record (the one that registered with it).
- **Identity at registration:** The node identity (node_id, hostname, IP, Worker API port, etc.) is **not** stored when the bundle is created.
  It is **supplied by the node** in the registration request (e.g. in the capability report or registration payload).
  The orchestrator creates (or updates) the node record at registration time using the identity the node reported.

### 4.2 Password-Encrypted Bundle File

- Spec ID: `CYNAI.DRAFT.NodeBundleEncryption`

The bundle is distributed as a **file** that is **encrypted with a password**.
The admin provides the password (e.g. via environment variable or password file) to the worker node; the node uses it to decrypt the bundle at startup.
The orchestrator never sees the password.

- **At rest:** The bundle file may be stored or copied (e.g. to the node host).
  Confidentiality of the file at rest depends on the password; compromise of the file alone does not allow an attacker to register.
- **Algorithm (to be pinned):** The draft assumes a password-based encryption scheme (e.g. password -> KDF -> key for AEAD, or a standard such as the one used for encrypted PEM).
  Exact algorithm and KDF will be specified in a later revision.
- **Contents (after decryption):** The decrypted payload MUST include at least:
  - **Bearer credential:** A one-time token or key material that the orchestrator can verify and mark as consumed (e.g. unique bundle id, or a signed/encrypted token issued by the orchestrator).
  - **Orchestrator base URL (optional but recommended):** So the node knows where to register without requiring a separate config field.
  - Optional: version, issued-at, or other metadata for validation.

The decrypted payload MUST NOT require the node to have any pre-configured node_id or hostname; the node supplies those at registration time.

### 4.3 Bundle Issuance

- Bundles are generated by the orchestrator (e.g. via admin API or UI: "Generate node bundle") or by trusted tooling that can produce credentials the orchestrator will accept.
- At issuance time the orchestrator (or tool) records whatever is needed to **verify and consume** the one-time credential (e.g. a unique bundle id or token stored in a "pending registrations" or "issued bundles" store).
  No node record is created at issuance.
- The issued bundle is encrypted with the **admin-supplied password** (or with a random key that is then encrypted under the password, depending on the chosen scheme).
  The exact flow (e.g. admin enters password in UI vs. tool generates password and displays it once) is left to implementation; the security requirement is that the bundle file is decryptable only with that password.

## 5. Registration Flow

The node uses the decrypted bundle credential once to register; the orchestrator consumes the credential and creates the node record from the reported identity.

### 5.1 High-Level Sequence

- Spec ID: `CYNAI.DRAFT.NodeRegistrationFlow`

1. **Node startup:** Node has the bundle file path and the password (from config or environment).
   It decrypts the bundle and obtains the bearer credential and orchestrator URL (if present).
2. **Registration request:** Node sends a registration request to the orchestrator.
   The request MUST include:
   - **Proof of the one-time credential** (e.g. the token or material from the decrypted bundle, in a form the orchestrator can verify).
   - **Capability report** including node identity and reachability: hostname, primary IP, Worker API port (and optionally listen host, public_base_url).
     See [Node identity at registration](#6-node-identity-at-registration).
   - **Node identity fields:** node_id (from node config or a default), name, labels, etc., as defined in the capability report schema.
3. **Orchestrator validation:** Orchestrator verifies the one-time credential (e.g. looks up the bundle id, checks it is not yet consumed).
   If valid, it **consumes** the credential (marks it used) so it cannot be used again.
4. **Node record creation:** Orchestrator creates (or updates) the node record using the **identity and capability data supplied in the registration request**.
   No pre-existing node record is required.
5. **Bootstrap response:** Orchestrator returns the bootstrap payload (JWT, endpoints, and **initial rotating key material** for ongoing encryption).
   The bootstrap response MUST be encrypted or protected so that only the registering node can use it (e.g. encrypted with a key derived from the registration exchange or with the first rotating key).
6. **Subsequent traffic:** Node uses the JWT and rotating keys for all subsequent communication (config refresh, job dispatch, result reporting).
   See [Post-registration: rotating keys](#7-post-registration-rotating-keys).

### 5.2 No TLS Required

- The registration request and response may be sent over plain TCP (e.g. HTTP on an internal network) or over TLS. **TLS is not required** for the registration or for subsequent node-orchestrator traffic.
  Confidentiality and integrity are provided by (1) the encrypted registration payload and (2) the rotating-key encryption for ongoing traffic.
  If TLS is used, it is an additional layer only; the system must not depend on it for authentication or confidentiality of the registration or bootstrap payloads.

## 6. Node Identity at Registration

- Spec ID: `CYNAI.DRAFT.NodeIdentityAtRegistration`

The node MUST report the following in the registration request (e.g. in the capability report or a dedicated registration payload) so the orchestrator can create the node record and reach the Worker API:

- **Hostname:** The hostname of the machine running the node (e.g. OS hostname).
  Used for inventory and display.
- **Primary IP:** The IP address the orchestrator should use to reach the node (e.g. the node's primary interface address, or the address used for the current connection).
  A single primary IP is sufficient for dispatch; additional addresses may be optional for diagnostics.
- **Worker API port:** The port on which the Worker API is listening (e.g. default 12090 or the value from node startup YAML).
- **Worker API listen host (optional):** If useful for display or validation (e.g. 0.0.0.0 vs a specific interface).
- **Public base URL (optional):** If the node or admin has configured a public URL for the Worker API (e.g. for NAT or load balancers), it may be reported so the orchestrator can store it for dispatch.
- **Node id (node_slug):** A stable identifier for the node (e.g. from node startup YAML `node.id`, or a default such as hostname).
  The orchestrator stores this as the node's primary id for scheduling and config.
- **Name, labels, other metadata:** As in the current capability report schema.

The orchestrator MUST use this reported identity to create or update the node record.
It MUST NOT require a pre-created node record or a pre-bound identity in the bundle.

## 7. Post-Registration: Rotating Keys

- Spec ID: `CYNAI.DRAFT.RotatingKeysPostRegistration`

After successful registration, the orchestrator issues **rotating keys** to the node for encrypting and authenticating all subsequent traffic (config refresh, job dispatch, result reporting).
This draft does not mandate the exact algorithm; the target design is **post-quantum** (e.g. NIST PQC Kyber for key establishment, then symmetric AEAD for payloads).

- **Initial key material:** The bootstrap payload MUST include the first set of key material (e.g. a shared secret or session keys) so that the node and orchestrator can encrypt and authenticate the next messages.
  Delivery of this material MUST be protected (e.g. encrypted within the registration response using a key agreed during the registration exchange).
- **Rotation:** Keys are time-limited or use-limited.
  The orchestrator issues new key material before expiry; the node uses the new keys and discards the old ones.
  Rotation may be push (orchestrator sends new keys in a config refresh or rekey message) or pull (node requests new keys before expiry).
  Exact rotation policy and wire format are left for a later revision.
- **Post-quantum:** The target design uses post-quantum key establishment (e.g. Kyber) so that long-term security does not rely on classical key exchange.
  Symmetric AEAD for payloads may remain classical (e.g. AES-GCM) or use PQ-resistant constructions as they mature.

## 8. Single-Node Automatic Setup

- Spec ID: `CYNAI.DRAFT.SingleNodeAutomaticSetup`

For the single-node use case (orchestrator and worker node on the same host or in the same deployment), the bundle SHOULD be **set up and ingested by the worker node automatically** so the admin does not have to manually download the bundle from the UI and copy it to the node.

### 8.1. Recommended Approach

- A **setup script, compose workflow, or single command** (e.g. `cynork node setup --single-node`, or a compose step that runs after the orchestrator is up):
  1. Calls the orchestrator (or a local tool) to **generate a new node bundle** (and optionally a random password, or accepts an admin-provided password).
  2. Writes the bundle file to a path the node will read (e.g. `/etc/cynode/node.bundle.enc` or a volume mount).
  3. Sets the bundle password in the node's environment (e.g. `CYNODE_NODE_BUNDLE_PASSWORD`) so the node can decrypt the bundle on startup.
- The worker node process starts, reads the bundle path and password from config or environment, decrypts the bundle once, and uses the credential and URL to register.
  No manual copy or download is required.

**Security note:** Automatically setting the password in the node's environment (e.g. from a script) means anyone with access to that environment can decrypt the bundle.
For single-node or trusted-host deployments this is acceptable; for high-security or multi-tenant scenarios, the admin may provide the password via a secret manager or interactive prompt instead.

## 9. Node Configuration and Bundle Discovery

The node discovers the bundle file and password via config or environment; bundle mode takes precedence over PSK when both are present.

### 9.1 Bundle Path and Password

- **Bundle path:** Configurable via node startup YAML (e.g. `orchestrator.bundle_path`) or environment (e.g. `CYNODE_NODE_BUNDLE_PATH`).
  Default path (e.g. `/etc/cynode/node.bundle.enc`) MAY be defined so that a single-node setup can write the bundle to the default and the node finds it without extra config.
- **Password:** Supplied via environment variable (e.g. `CYNODE_NODE_BUNDLE_PASSWORD` or a name from config such as `orchestrator.bundle_password_env`) or via a file (e.g. `orchestrator.bundle_password_file`).
  The node MUST NOT log or persist the password in plaintext; it MAY cache decrypted material in memory or in a local encrypted store (e.g. keyed by a derived key) for the duration of the process or until rekey.

### 9.2 Mode: Bundle Versus PSK

- When the node has a valid **bundle path and password**, it MUST use **bundle mode**: decrypt the bundle and use the one-time credential (and optional orchestrator URL from the bundle) for registration.
  It does not use `registration_psk` from YAML for this registration.
- When the node has **no bundle** (or bundle path/password not set), it MAY fall back to **PSK mode** if supported: use `orchestrator.registration_psk_env` or `registration_psk_file` and `orchestrator.url` from YAML for registration.
  This allows backward compatibility or minimal setups; the preferred path is bundle mode.
- Priority (bundle vs PSK) when both are present: **bundle takes precedence** so that migration to bundles does not require removing the PSK from YAML immediately.

## 10. Open Points for Refinement

- **Wire format:** Exact registration request/response schema (e.g. how the one-time credential is presented, how the bootstrap payload is encrypted).
  Alignment with existing [`worker_node_payloads.md`](../tech_specs/worker_node_payloads.md) and any new "registration v2" payload.
- **Bundle file format:** Choice of password-based encryption (e.g. KDF + AEAD, or a standard like encrypted PEM).
  Version field and upgrade path.
- **Post-quantum algorithms:** Pin Kyber (or other NIST PQC) and key-derivation approach; document library and version.
- **Rotation protocol:** Push vs pull, rekey endpoint or in-band in config refresh, and how to handle key overlap during rotation.
- **Revocation:** Revoking an unused bundle (orchestrator marks bundle id as revoked before any registration).
  Revoking a node after registration (revoke the node record; bundle is already consumed).
- **Cynork / CLI:** Commands or flows for "generate node bundle" and "single-node setup" (e.g. `cynork node bundle generate`, `cynork node setup --single-node`).
- **Requirements and traceability:** Once stable, this draft should be reflected in `docs/requirements/worker.md` (and possibly `bootst.md`) and in the main tech specs (`worker_node.md`, `worker_node_payloads.md`) with proper REQ-* and Spec ID traceability.
