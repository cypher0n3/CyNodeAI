package chat

// Landmarks are stable strings for PTY and E2E tests to detect UI state transitions
// without depending on exact wording or redraw timing. The TUI (Phase 5) should
// emit or render these where appropriate so automation can assert on them.
const (
	// LandmarkPromptReady indicates the composer is ready for user input (e.g. status bar or footer).
	LandmarkPromptReady = "[CYNRK_PROMPT_READY]"
	// LandmarkPromptReadyShort is a shorter form for E2E so it fits in first PTY read chunk.
	LandmarkPromptReadyShort = "[CYNRK_READY]"
	// LandmarkAssistantInFlight indicates a request is in progress (thinking or streaming).
	LandmarkAssistantInFlight = "[CYNRK_ASSISTANT_IN_FLIGHT]"
	// LandmarkResponseComplete indicates the current assistant turn is finalized.
	LandmarkResponseComplete = "[CYNRK_RESPONSE_COMPLETE]"
	// LandmarkThreadSwitched indicates the active thread has changed.
	LandmarkThreadSwitched = "[CYNRK_THREAD_SWITCHED]"
	// LandmarkAuthRecoveryReady indicates login/re-auth is being prompted.
	LandmarkAuthRecoveryReady = "[CYNRK_AUTH_RECOVERY_READY]"
)
