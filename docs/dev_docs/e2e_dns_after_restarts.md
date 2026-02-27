# DNS Failing After Repeated Stack Restarts

## Symptom

After multiple `just e2e` runs (or repeated `./scripts/setup-dev.sh stop` then `full-demo`), the E2E fails with:

```text
Failed to pull model tinyllama after 3 attempts
Error: pull model manifest: Get "https://registry.ollama.ai/...": dial tcp: lookup registry.ollama.ai: i/o timeout
```

Other containers that need to resolve external hostnames can show the same kind of lookup timeouts.

## Why It Happens

Containers in the orchestrator stack use the **default DNS** provided by the container runtime (Podman or Docker).
That resolver is typically:

- **Podman:** The default bridge/compose network's embedded DNS (e.g. netavark/dnsname), or the host's resolver passed through slirp4netns/pasta.
- **Docker:** The daemon's embedded DNS or the host's resolver.

Repeated **stop/start** of the stack (`compose down` / `compose up`) tears down and recreates the project network each time.
In some environments this leads to:

- Stale or stuck state in the runtime's DNS proxy.
- Leaked connections or file descriptors in the resolver.
- Interaction issues with the host resolver (e.g. systemd-resolved) after many container restarts.

So the **same host and code** can work on the first run and then see DNS "i/o timeout" after several cycles.
It's an environment/runtime issue, not application logic.

## Fix Applied

The **ollama** service in `orchestrator/docker-compose.yml` is configured with **explicit DNS servers** (8.8.8.8, 8.8.4.4) so it no longer depends on the default resolver.
That makes `ollama pull` (and any other outbound DNS from the Ollama container) stable across repeated restarts.

If other services in the stack start showing lookup timeouts after many restarts, add the same `dns:` block to those services.

## What You Can Do

1. **Use the updated compose** - Ensure you have the `dns:` block on the ollama service (and any other service that hits external hostnames and starts failing).
2. **If you still see DNS timeouts** - Try:
   - Restarting the container runtime (e.g. `systemctl --user restart podman` or restart Docker).
   - Running with explicit DNS for the whole project (e.g. in `~/.config/containers/containers.conf` for Podman, or docker daemon config).
3. **Skip inference smoke when DNS is flaky** - `E2E_SKIP_INFERENCE_SMOKE=1 just e2e` skips the model pull and one-shot chat; the rest of E2E still runs.
