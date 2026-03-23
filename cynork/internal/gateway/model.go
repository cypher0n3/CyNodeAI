package gateway

// ModelProjectManager is the chat API "model" id that routes to the Project Manager agent
// (cynode-pma) with MCP tool execution. Any other non-empty id is handled as direct
// inference (no PMA, no MCP). The Ollama checkpoint is chosen server-side (PMA INFERENCE_MODEL);
// clients must not send Ollama tag names as the routing model.
// Matches orchestrator/handlers.EffectiveModelPM.
const ModelProjectManager = "cynodeai.pm"
