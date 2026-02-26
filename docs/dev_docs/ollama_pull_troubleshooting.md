# Ollama Model Pull (E2E)

- [Behavior](#behavior)
- [Correct URL / Model Name](#correct-url--model-name)
- [If Pull Fails (Model Not Present)](#if-pull-fails-model-not-present)

## Behavior

The E2E script ensures the inference model (e.g. `tinyllama`) is available in the Ollama container before running inference tests.

1. **Model already present** - If `ollama list` shows the model, the script tries to update it (`ollama pull` with a 30s timeout).
   If the update succeeds, the model is up to date.
   If it fails or times out (e.g. no network), the script continues using the existing model and does not fail.
2. **Model not present** - The script pulls the model with up to 3 attempts (5s between attempts).
   If all fail, E2E fails and reports the last pull output.

So E2E only fails on pull when the model is missing and cannot be fetched.
A pre-loaded model (e.g. from a previous run or manual `ollama pull tinyllama`) is always used, and the script only attempts an optional update.

## Correct URL / Model Name

- **Registry:** `https://registry.ollama.ai`
- **TinyLlama manifest:** `https://registry.ollama.ai/v2/library/tinyllama/manifests/latest`
- **CLI model name:** `tinyllama`

## If Pull Fails (Model Not Present)

- Check that the host can reach the registry, e.g.  
  `curl -sI --connect-timeout 10 https://registry.ollama.ai/v2/library/tinyllama/manifests/latest`  
  (expect HTTP 200.)
- Pre-pull when you have network:  
  `podman exec cynodeai-ollama ollama pull tinyllama`  
  (or your runtime).
    Later E2E runs will see the model and only try an optional update.
- To skip the pull and inference smoke entirely (e.g. no registry access), set  
  `E2E_SKIP_INFERENCE_SMOKE=1`.
