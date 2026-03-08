# Investigation: Setup-Dev Start - Ollama vs PMA

## Context

**Date:** 2026-03-07  
**Context:** User ran `just setup-dev start` (no flags).
They see `cynodeai-ollama` running but no PMA container.
They want to confirm Ollama was started by the worker node and understand why PMA is not running.

## Findings (No Code Changes)

Summary of causes for observed behavior.

### 1. Ollama: Who Started It?

**Conclusion: Ollama was started by the worker node**, not by compose.

- With plain `just setup-dev start`, the script does **not** pass `--ollama-in-stack`.
  So `start_orchestrator_stack_compose` uses only profile `optional` (no `ollama` profile).
  Compose never starts the `cynodeai-ollama` container.
- The only way `cynodeai-ollama` appears is if the **node-manager** started it.
  In `worker_node/cmd/node-manager/main.go`, `startOllama` uses `getEnv("OLLAMA_CONTAINER_NAME", "cynodeai-ollama")` and runs `podman run -d --name cynodeai-ollama ...`.
- So the control-plane sent **inference_backend** in the node config, and the node ran `maybeStartOllama`, which started the Ollama container.
  That is the prescribed path: node starts inference when instructed.

**Note:** Ollama is the **inference backend** (Phase 1), not a "managed service" in the payload sense.
Managed services are PMA (and any future agent containers).
Both are started by the node when the control-plane sends the corresponding config.

---

### 2. PMA: Why is It Not Running?

PMA is supposed to be started by the node when the control-plane sends **managed_services** (PMA) in the node config.
The flow is:

1. Control-plane **GetConfig** builds payload with `buildManagedServicesDesiredState(ctx, node)`.
2. **buildManagedServicesDesiredState** returns PMA only if:
   - `PMA_IMAGE` and `PMA_SERVICE_ID` are set (compose env sets `PMA_IMAGE=cynodeai-cynode-pma:dev`),
   - `selectedNodeSlug == node.NodeSlug` (this node is chosen to host PMA).
3. **selectPMAHostNodeSlug** picks the node that will run PMA:
   - If `PMA_HOST_NODE_SLUG` is set, that node is used.
   - Else it lists active nodes, sorts by slug, and either returns the first node that has label `PMA_PREFER_HOST_LABEL` (default `orchestrator_host`), or **nodes[0].NodeSlug** if none have that label.
4. The node never sets `Node.Labels` in its capability (see `buildCapability` in nodemanager.go: only `NodeSlug` and `Name` are set).
   So no node has `orchestrator_host`, and with a single node we get `nodes[0].NodeSlug` = this node's slug.
   So **for a single-node dev setup, the control-plane should include PMA in the config**.
5. Node flow: **applyConfigAndStartServices** runs **syncManagedServiceAgentTokens** -> **StartWorkerAPI** -> **maybeStartOllama** -> **maybeStartManagedServices** -> **SendConfigAck**.
   So if Ollama started, the node got past StartWorkerAPI and maybeStartOllama and should have reached **maybeStartManagedServices**.
   If the config had `ManagedServices.Services` with PMA, the node would call **StartManagedServices** (which runs the PMA container).

So in theory the node should receive PMA in config and start it.
Possible reasons it does not:

**A.
Control-plane does not send PMA (buildManagedServicesDesiredState returns nil)**

- **selectedNodeSlug != node.NodeSlug:** With one active node this should not happen unless `PMA_HOST_NODE_SLUG` is set to a different slug.
- **ListActiveNodes empty:** Unlikely at first GetConfig right after register (node is already active).
- **PMA_IMAGE / PMA_SERVICE_ID empty:** Compose env sets `PMA_IMAGE`; default service ID is `pma-main`.
  Unlikely in normal dev.

**B.
Node receives config but fails before or during maybeStartManagedServices**

- **syncManagedServiceAgentTokens** fails (e.g. secure store) -> whole applyConfigAndStartServices fails -> node would exit with error.
  User would likely see node-manager exit.
- **maybeStartManagedServices** fails: e.g. container start fails (image pull 403, socket path length, permission).
  Node would exit with "start managed services: ...".
  If the process is still running, this path was not hit or the error was not fatal in a way that stops the process.
- **maybeStartManagedServices** is a no-op if `nodeConfig.ManagedServices == nil` or `len(Services) == 0`.
  So if the control-plane really did not send PMA for this node, the node would simply not start any managed service and would continue to config ack and the capability loop.

**C.
Node never gets config with ManagedServices**

- First GetConfig happens right after register.
  At that time the only capability snapshot is from the registration body.
  So the control-plane has one active node and (with no labels) selects that node for PMA.
  So the first config *should* include PMA unless there is an env or DB quirk (e.g. different slug, or `PMA_HOST_NODE_SLUG` set elsewhere).

**D.
WORKER_INTERNAL_AGENT_TOKEN**

- Compose does **not** set `WORKER_INTERNAL_AGENT_TOKEN` for the control-plane.
  So `AgentToken` in the PMA managed-service block is empty.
  The node's **resolveManagedServiceToken** returns `hasToken == false` for that service.
  The node still continues: **syncManagedServiceAgentTokens** and **maybeStartManagedServices** do not require a token to start the container.
  So missing token should not prevent the PMA container from starting.

---

### 3. Recommended Next Steps (For the User; No Code Changes in This Investigation)

Steps the user can take to narrow down why PMA is not running.

#### 3.1. Node-Manager Logs

   After `just setup-dev start`, check whether the node-manager process is still running and capture its stdout/stderr (e.g. from the terminal where the script ran, or from the process that started it).
    Look for:

- `"managed services started", "count", 1` -> PMA was started.
- Any error containing `"start managed services"` or `"managed service"` or `"secure store"` or `"listen unix"` (socket path).

#### 3.2. Control-Plane Logs

   `podman logs cynodeai-control-plane 2>&1 | …`  
   Confirm that **GetConfig** is called and that the response is 200.
    There is no current logging of whether `buildManagedServicesDesiredState` returned nil; adding a temporary log there would confirm whether PMA is being included for this node.

#### 3.3. Verify Slug and Pma Host Selection

   In the DB (or via any existing admin/API), confirm the single node's `node_slug` (e.g. `dev-node-1`).
    Ensure no env (e.g. in shell or compose override) sets `PMA_HOST_NODE_SLUG` to something else.

#### 3.4. Reproduce With Logging

   Optionally add a single log line in **buildManagedServicesDesiredState** (orchestrator) when it returns nil, e.g. log `selectedNodeSlug`, `node.NodeSlug`, and whether `ListActiveNodes` was empty, to confirm why PMA might be omitted.

---

### 4. Investigation Summary

- **Item:** **Ollama**
  - conclusion: Started by the **worker node** (inference_backend in config).
    Not started by compose when `--ollama-in-stack` is not used.
- **Item:** **PMA**
  - conclusion: Should be started by the node when the control-plane sends managed_services (PMA).
    Code path suggests PMA should be in config for a single-node dev setup.
    Most likely causes for "no PMA container": (1) control-plane not including PMA for this node (e.g. slug / PMA host selection), or (2) node failing during maybeStartManagedServices (e.g. image, socket, secure store) and exiting, or (3) node receiving config with empty managed_services and correctly doing nothing.
    Checking node-manager and control-plane logs (and optionally one log in buildManagedServicesDesiredState) will narrow it down.
