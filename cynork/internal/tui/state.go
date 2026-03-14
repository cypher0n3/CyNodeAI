// Package tui – structured transcript and session state for the Cynork TUI.
// See docs/tech_specs/cynork_tui.md and chat_threads_and_messages.md (Structured Turns).
package tui

import "time"

// Role is the role of a transcript turn (user, assistant, system).
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// PartKind is the kind of a structured transcript part.
// See chat_threads_and_messages.md § Structured Representation.
type PartKind string

const (
	PartKindText          PartKind = "text"
	PartKindThinking      PartKind = "thinking"
	PartKindToolCall      PartKind = "tool_call"
	PartKindToolResult    PartKind = "tool_result"
	PartKindAttachmentRef PartKind = "attachment_ref"
	PartKindDownloadRef   PartKind = "download_ref"
)

// TranscriptPart is one structured part of a turn (text, thinking, tool_call, etc.).
// Collapsed and HiddenByDefault apply to thinking: collapsed placeholder shows until /show-thinking.
type TranscriptPart struct {
	Kind            PartKind
	Text            string
	Meta            map[string]any
	Collapsed       bool
	HiddenByDefault bool
}

// TranscriptTurn is one logical turn in the transcript.
// InFlight is true for the single active streaming assistant turn; StreamingState holds phase and amendment flag.
type TranscriptTurn struct {
	MessageID      string
	ThreadID       string
	Role           Role
	Content        string
	Parts          []TranscriptPart
	CreatedAt      time.Time
	InFlight       bool
	Interrupted    bool
	StreamingState StreamingState
}

// StreamingState is attached to the in-flight turn during streaming.
type StreamingState struct {
	Phase         StreamingPhase
	SpinnerFrame  int
	AmendmentSeen bool
}

// StreamingPhase is the current phase of an in-flight turn.
type StreamingPhase string

const (
	StreamingPhaseWorking  StreamingPhase = "Working"
	StreamingPhaseThinking StreamingPhase = "Thinking"
	StreamingPhaseToolCall StreamingPhase = "Calling tool"
	StreamingPhaseToolWait StreamingPhase = "Waiting for tool result"
)

// ThreadListItem is one thread in the list (for /thread list and switch).
// Selector is the user-typeable form (ordinal, short id, or title) shown and accepted by /thread switch.
type ThreadListItem struct {
	ID        string
	Selector  string
	Title     string
	Summary   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Archived  bool
	ProjectID string
}

// ConnectionState is the gateway connection state for the status bar.
type ConnectionState string

const (
	ConnectionStateUnknown      ConnectionState = ""
	ConnectionStateConnected    ConnectionState = "Connected"
	ConnectionStateReconnecting ConnectionState = "Reconnecting..."
	ConnectionStateDisconnected ConnectionState = "Disconnected"
)

// AuthState is the auth state for the status bar.
type AuthState string

const (
	AuthStateUnknown AuthState = ""
	AuthStateOK      AuthState = "ok"
	AuthStateFailed  AuthState = "failed"
)

// ContextPaneTab is the active tab when the context pane is visible.
type ContextPaneTab string

const (
	ContextPaneTabThreads ContextPaneTab = "threads"
	ContextPaneTabHelp    ContextPaneTab = "help"
)

// Draft is a queued unsent draft (local only).
type Draft struct {
	Text      string
	CreatedAt time.Time
}

// SessionState holds session-level state for the TUI (gateway, project, model, thread, UI prefs).
// Used to drive status bar and thread UX; transcript is separate (TranscriptTurn slice).
type SessionState struct {
	GatewayURL           string
	EffectiveProjectID   string
	SelectedModelID      string
	CurrentThreadID      string
	CurrentThreadTitle   string
	CurrentThreadSummary string
	ShowThinking         bool
	ConnectionState      ConnectionState
	AuthState            AuthState
	ContextPaneVisible   bool
	ContextPaneTab       ContextPaneTab
	DraftQueue           []Draft
	ThreadIndex          []ThreadListItem
}
