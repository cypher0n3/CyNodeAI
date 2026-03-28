// Package bdd – cynork Godog steps: mock gateway modes, model/project/dispatch, and related HTTP stubs.
// Streaming/in-flight steps and PTY-only steps live in steps_cynork_extra_tui_deferred.go.
package bdd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

func registerCynorkExtraMockGateway(sc *godog.ScenarioContext, state *cynorkState) {

	// ---- Setup steps: mock error modes ----

	sc.Step(`^the TUI is running and the gateway returns an error for GET /v1/models$`, func(ctx context.Context) error {
		getState(ctx).modelsErrorMode = true
		return nil
	})

	sc.Step(`^the TUI is running and the gateway exposes known model ids$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.modelIDs = []string{"cynodeai.pm", "test-model-v2", "other-model"}
		return nil
	})

	sc.Step(`^the TUI is running and the gateway returns an error for the next prefs request$`, func(ctx context.Context) error {
		getState(ctx).prefsErrorMode = true
		return nil
	})

	sc.Step(`^the TUI is running and the gateway will return an auth error for the next auth request$`, func(ctx context.Context) error {
		getState(ctx).authErrorNextReq = true
		return nil
	})

	sc.Step(`^the TUI is running and the gateway returns 404 for the next task request$`, func(ctx context.Context) error {
		getState(ctx).task404Mode = true
		return nil
	})

	// ---- Setup steps: task state ----

	sc.Step(`^the TUI is running and the gateway returns a task list$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		if st.tasks == nil {
			st.tasks = make(map[string]string)
		}
		st.tasks["task-list-1"] = "list test task"
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the TUI is running and the mock gateway has a task with id "([^"]*)"$`, func(ctx context.Context, id string) error {
		st := getState(ctx)
		st.mu.Lock()
		if st.tasks == nil {
			st.tasks = make(map[string]string)
		}
		st.tasks[id] = "task prompt for " + id
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the TUI is running and the mock gateway has a running task with id "([^"]*)"$`, func(ctx context.Context, id string) error {
		st := getState(ctx)
		st.mu.Lock()
		if st.tasks == nil {
			st.tasks = make(map[string]string)
		}
		if st.taskStatuses == nil {
			st.taskStatuses = make(map[string]string)
		}
		st.tasks[id] = "running task prompt"
		st.taskStatuses[id] = "running"
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the TUI is running and the mock gateway has a completed task with id "([^"]*)"$`, func(ctx context.Context, id string) error {
		st := getState(ctx)
		st.mu.Lock()
		if st.tasks == nil {
			st.tasks = make(map[string]string)
		}
		if st.taskStatuses == nil {
			st.taskStatuses = make(map[string]string)
		}
		st.tasks[id] = "echo result"
		st.taskStatuses[id] = "completed"
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the TUI is running and the mock gateway has a task with id "([^"]*)" that has artifacts$`, func(ctx context.Context, id string) error {
		st := getState(ctx)
		st.mu.Lock()
		if st.tasks == nil {
			st.tasks = make(map[string]string)
		}
		if st.taskArtifactsByID == nil {
			st.taskArtifactsByID = make(map[string][]string)
		}
		st.tasks[id] = "artifact task"
		st.taskArtifactsByID[id] = []string{"output.txt", "report.pdf"}
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the TUI is running and the gateway supports task create$`, func(_ context.Context) error {
		return nil // task create endpoint always available
	})

	// ---- Setup steps: skill state ----

	sc.Step(`^the TUI is running and the gateway supports skills list$`, func(_ context.Context) error {
		return nil // skills list endpoint always available
	})

	sc.Step(`^the TUI is running and the gateway has a skill with selector "([^"]*)"$`, func(ctx context.Context, selector string) error {
		getState(ctx).skillSelectorSetup = selector
		return nil
	})

	sc.Step(`^the mock gateway returns a visible skill with selector "([^"]*)"$`, func(ctx context.Context, selector string) error {
		getState(ctx).skillSelectorSetup = selector
		return nil
	})

	sc.Step(`^the TUI is running and the gateway has multiple skills matching "([^"]*)"$`, func(ctx context.Context, _ string) error {
		getState(ctx).multipleSkillsMode = true
		return nil
	})

	sc.Step(`^the gateway supports skills load$`, func(_ context.Context) error {
		return nil // skills load endpoint always available
	})

	sc.Step(`^the TUI is running and a markdown file "([^"]*)" exists with content "([^"]*)"$`, func(ctx context.Context, path, content string) error {
		st := getState(ctx)
		if st.bddRoot != "" && !filepath.IsAbs(path) {
			path = filepath.Join(st.bddRoot, path)
		}
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		return os.WriteFile(path, []byte(content), 0o600)
	})

	sc.Step(`^a markdown file "([^"]*)" exists with updated content$`, func(ctx context.Context, path string) error {
		st := getState(ctx)
		if st.bddRoot != "" && !filepath.IsAbs(path) {
			path = filepath.Join(st.bddRoot, path)
		}
		_ = os.MkdirAll(filepath.Dir(path), 0o755)
		return os.WriteFile(path, []byte("# Updated skill\n\nContent updated.\n"), 0o600)
	})

	// ---- Setup steps: node state ----

	sc.Step(`^the TUI is running and the gateway supports nodes list$`, func(_ context.Context) error {
		return nil // nodes list endpoint always available
	})

	sc.Step(`^the TUI is running and the gateway returns at least one node with id "([^"]*)"$`, func(ctx context.Context, id string) error {
		st := getState(ctx)
		st.mu.Lock()
		if st.nodesByID == nil {
			st.nodesByID = make(map[string]map[string]any)
		}
		st.nodesByID[id] = map[string]any{"id": id, "status": "online", "hostname": "node-host-" + id}
		st.mu.Unlock()
		return nil
	})

	// ---- Setup steps: project state ----

	sc.Step(`^the TUI is running and the gateway supports project listing$`, func(_ context.Context) error {
		return nil // project list endpoint always available
	})

	sc.Step(`^the TUI is running and the gateway supports project get$`, func(_ context.Context) error {
		return nil // project get endpoint always available
	})

	// ---- Setup steps: prefs state ----

	sc.Step(`^the TUI is running and the gateway supports prefs list$`, func(_ context.Context) error {
		return nil
	})
	sc.Step(`^the TUI is running and the gateway supports prefs get$`, func(_ context.Context) error {
		return nil
	})
	sc.Step(`^the TUI is running and the gateway supports prefs set$`, func(_ context.Context) error {
		return nil
	})
	sc.Step(`^the TUI is running and the gateway supports prefs delete$`, func(_ context.Context) error {
		return nil
	})
	sc.Step(`^the TUI is running and the gateway supports prefs effective$`, func(_ context.Context) error {
		return nil
	})

	// ---- Setup steps: model/project gateway config ----

	sc.Step(`^the TUI is running with gateway "([^"]*)"$`, func(ctx context.Context, url string) error {
		getState(ctx).sessionGateway = url
		return nil
	})

	sc.Step(`^the mock gateway exposes GET "([^"]*)"$`, func(_ context.Context, _ string) error {
		return nil // mock always exposes GET /healthz
	})

	// ---- Setup steps: thread state ----

	sc.Step(`^the mock gateway returns multiple chat threads$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		st.mockThreads = []mockThread{
			{ID: "tid-inbox-1", Title: "inbox"},
			{ID: "tid-work-2", Title: "work"},
		}
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the TUI is running and the mock gateway returns a known thread list$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		st.mockThreads = []mockThread{
			{ID: "tid-inbox-1", Title: "inbox"},
			{ID: "tid-work-2", Title: "work"},
		}
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the TUI is running and the mock gateway returns at least one thread with selector "([^"]*)"$`, func(ctx context.Context, selector string) error {
		st := getState(ctx)
		st.mu.Lock()
		st.mockThreads = []mockThread{{ID: "tid-sel-1", Title: selector}}
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the TUI is running and the mock gateway returns multiple chat threads$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		st.mockThreads = []mockThread{
			{ID: "tid-inbox-1", Title: "inbox"},
			{ID: "tid-work-2", Title: "work"},
		}
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the TUI is running with a current thread$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		st.mockThreads = []mockThread{{ID: "tid-cur-1", Title: "current-thread"}}
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the TUI is running with a current thread that has a title or fallback label$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		st.mockThreads = []mockThread{{ID: "tid-titled-1", Title: "My Thread Title"}}
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^the TUI is running with a current thread that is in the mock thread list$`, func(ctx context.Context) error {
		return nil // no-op: covered by other thread setup steps
	})

	// ---- Setup steps: model+project combined ----

	sc.Step(`^the TUI is running with model "([^"]*)" and project "([^"]*)"$`, func(ctx context.Context, model, project string) error {
		st := getState(ctx)
		st.sessionModel = model
		st.sessionProject = project
		return nil
	})

	// ---- Setup steps: session display (scrollback) ----

	sc.Step(`^the TUI has existing messages in the scrollback from a thread$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.mu.Lock()
		st.mockThreads = []mockThread{{ID: "tid-hist-1", Title: "history-thread"}}
		st.mu.Unlock()
		return nil
	})

	// ---- Setup steps: auth config ----

	sc.Step(`^the local cynork config has no token$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.token = ""
		return os.WriteFile(st.configPath, []byte("gateway_url: "+st.mockServer.URL+"\n"), 0o600)
	})

	sc.Step(`^the local cynork config has an expired token$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.token = "expired-tok"
		return os.WriteFile(st.configPath, []byte("gateway_url: "+st.mockServer.URL+"\ntoken: expired-tok\n"), 0o600)
	})

	sc.Step(`^the TUI is running with an expired login token$`, func(ctx context.Context) error {
		st := getState(ctx)
		st.token = "expired-tok"
		return nil
	})

	sc.Step(`^the TUI attempts the initial gateway connection$`, func(_ context.Context) error {
		return nil // no-op: informational step
	})

	sc.Step(`^the TUI attempts the initial gateway connection and the gateway returns 401$`, func(ctx context.Context) error {
		getState(ctx).authErrorNextReq = true
		return nil
	})

	// ---- Setup steps: transcript thinking state (non-streaming; TUI state is opaque in BDD) ----

	sc.Step(`^the TUI is running with transcript containing assistant turns that have retained thinking parts$`, func(_ context.Context) error {
		return nil // no-op: transcript state is opaque without PTY; tested via E2E
	})

	sc.Step(`^the TUI is running with transcript containing expanded thinking blocks$`, func(_ context.Context) error {
		return nil
	})

	sc.Step(`^the TUI is running with transcript containing only assistant turns that have no retained thinking part$`, func(_ context.Context) error {
		return nil
	})

	sc.Step(`^the TUI has two queued drafts$`, func(_ context.Context) error {
		return godog.ErrPending // queue draft feature not yet implemented
	})

	sc.Step(`^the TUI shows enough transcript output to scroll$`, func(_ context.Context) error {
		return godog.ErrPending // requires PTY
	})

	sc.Step(`^the composer has focus$`, func(_ context.Context) error {
		return godog.ErrPending // requires PTY
	})

	// ---- Setup steps: gateway streaming (pending) ----

	sc.Step(`^the gateway returns a structured assistant turn with visible text and thinking$`, bddGatewayStructuredTurn)

	sc.Step(`^the gateway is still generating the current assistant turn$`, bddGatewayStillGenerating)

	sc.Step(`^the gateway supports stream=true and emits real token-by-token visible assistant text$`, func(ctx context.Context) error {
		getState(ctx).bddGatewayTokenStream = true
		return nil
	})

	sc.Step(`^the TUI has sent a message and the gateway is streaming the assistant response$`, bddTUISentMessageAndStreaming)

	sc.Step(`^the connection to the gateway is interrupted before the stream completes$`, bddConnectionInterruptedMidStream)

	// ---- Setup steps: task named ----

	sc.Step(`^I have a task named "([^"]*)"$`, func(ctx context.Context, name string) error {
		st := getState(ctx)
		st.mu.Lock()
		if st.tasks == nil {
			st.tasks = make(map[string]string)
		}
		if st.taskNames == nil {
			st.taskNames = make(map[string]string)
		}
		// Use name as id so GET /v1/tasks/{name} resolves directly.
		st.tasks[name] = "task prompt"
		st.taskNames[name] = name
		st.mu.Unlock()
		return nil
	})

	sc.Step(`^I have created a task with prompt "([^"]*)" and task name "([^"]*)"$`, func(ctx context.Context, prompt, name string) error {
		st := getState(ctx)
		// Pre-populate mock so GET /v1/tasks/{name} resolves by selector name.
		st.mu.Lock()
		if st.tasks == nil {
			st.tasks = make(map[string]string)
		}
		if st.taskNames == nil {
			st.taskNames = make(map[string]string)
		}
		st.tasks[name] = prompt
		st.taskNames[name] = name
		st.mu.Unlock()
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "create", "-p", prompt, "--task-name", name}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		if st.lastExit != 0 {
			return fmt.Errorf("task create exited %d: %s", st.lastExit, st.lastStderr)
		}
		return nil
	})

	// ---- Action steps: cynork tui ----

	sc.Step(`^I run cynork tui$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL}
		if st.token != "" {
			env = append(env, "CYNORK_TOKEN="+st.token)
		}
		args := []string{"--config", st.configPath, "tui"}
		// TUI requires a TTY; without one it exits (bubbletea error or thread error).
		// We capture output to allow assertions on exit reason.
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, "")
		return nil
	})

	sc.Step(`^I run cynork tui without resume-thread$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "tui"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, "")
		return nil
	})

	sc.Step(`^I run cynork tui with resume-thread "([^"]*)"$`, func(ctx context.Context, selector string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		// cynork tui requires a real TTY and cannot run in BDD. Use cynork chat --resume-thread
		// as a proxy: it shares the same thread-resolution logic and can run non-interactively.
		args := []string{"--config", st.configPath, "chat", "--resume-thread", selector, "--message", "hello"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I start a new cynork TUI session$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL}
		if st.token != "" {
			env = append(env, "CYNORK_TOKEN="+st.token)
		}
		args := []string{"--config", st.configPath, "tui"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, "")
		return nil
	})

	// ---- Action steps: cynork chat with message (one-shot) ----

	sc.Step(`^I run cynork chat with message "([^"]*)"$`, func(ctx context.Context, msg string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat", "--message", msg}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	// ---- Action steps: task commands ----

	sc.Step(`^I run cynork task get with task selector "([^"]*)"$`, func(ctx context.Context, selector string) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "get", selector}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	sc.Step(`^I run cynork task result with the stored task id in JSON mode$`, func(ctx context.Context) error {
		st := getState(ctx)
		if st.taskID == "" {
			return fmt.Errorf("no stored task id")
		}
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "task", "result", "--output", "json", st.taskID}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args, env...)
		return nil
	})

	// ---- Action steps: compound type+send ----

	sc.Step(`^I type "([^"]*)" and press Enter and then send a chat message from the composer$`, func(ctx context.Context, text string) error {
		st := getState(ctx)
		gwURL := st.mockServer.URL
		if st.sessionGateway != "" {
			gwURL = st.sessionGateway
		}
		env := []string{"CYNORK_GATEWAY_URL=" + gwURL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		if st.sessionModel != "" {
			args = append(args, "--model", st.sessionModel)
		}
		if st.sessionProject != "" {
			args = append(args, "--project-id", st.sessionProject)
		}
		// slash command, then a real chat message, then exit
		stdin := text + "\necho hello\n/exit\n"
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, stdin)
		return nil
	})

	sc.Step(`^I issue "([^"]*)" in the TUI$`, func(ctx context.Context, text string) error {
		// Alias: equivalent to I type "X" and press Enter.
		st := getState(ctx)
		gwURL := st.mockServer.URL
		if st.sessionGateway != "" {
			gwURL = st.sessionGateway
		}
		env := []string{"CYNORK_GATEWAY_URL=" + gwURL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		if st.sessionModel != "" {
			args = append(args, "--model", st.sessionModel)
		}
		stdin := text + "\n/exit\n"
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, stdin)
		return nil
	})

	// ---- Action steps: project clear ----

	sc.Step(`^I clear the project context via the accepted form$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		stdin := "/project none\necho hello\n/exit\n"
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, stdin)
		return nil
	})

	// ---- Action steps: documentation check ----

	sc.Step(`^I review the documented interactive entrypoints$`, func(ctx context.Context) error {
		st := getState(ctx)
		args := []string{"--help"}
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynork(args)
		return nil
	})

	// ---- Action steps: file reference validation ----

	sc.Step(`^I compose a message with an @ file reference and the referenced file is missing$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		stdin := "@/nonexistent_file_xyz_bdd.txt\n/exit\n"
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, stdin)
		return nil
	})

	// ---- Action steps: thread rename (via /thread rename) ----

	sc.Step(`^I can rename the selected thread$`, func(ctx context.Context) error {
		st := getState(ctx)
		env := []string{"CYNORK_GATEWAY_URL=" + st.mockServer.URL, "CYNORK_TOKEN=" + st.token}
		args := []string{"--config", st.configPath, "chat"}
		stdin := "/thread rename new-title\n/exit\n"
		st.lastExit, st.lastStdout, st.lastStderr = st.runCynorkWithStdin(args, env, stdin)
		combined := st.lastStdout + " " + st.lastStderr
		if !strings.Contains(strings.ToLower(combined), "renamed") && st.lastExit != 0 {
			return fmt.Errorf("expected rename success; got exit %d: %s", st.lastExit, combined)
		}
		return nil
	})

	sc.Step(`^I view the TUI status bar or thread display$`, func(_ context.Context) error {
		return nil // no-op: status bar not visible without PTY
	})

	// ---- Action steps: PTY-required (pending) ----

	sc.Step(`^I open the thread history pane in the TUI$`, func(_ context.Context) error {
		return godog.ErrPending // requires PTY
	})

	sc.Step(`^I scroll with the mouse wheel$`, func(_ context.Context) error {
		return godog.ErrPending // requires PTY
	})

	sc.Step(`^I reorder the queued drafts and choose to send only the first queued draft$`, func(_ context.Context) error {
		return godog.ErrPending // requires PTY
	})

	sc.Step(`^I send a normal interactive chat turn from the TUI$`, bddSendNormalInteractiveStreamTurn)

	sc.Step(`^I send Ctrl-C or otherwise cancel the active stream$`, bddCancelActiveStream)
}
