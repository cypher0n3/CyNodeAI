package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const goosLinux = "linux"

func TestCopyToClipboard_Empty(t *testing.T) {
	err := CopyToClipboard("")
	if err == nil {
		t.Fatal("expected error for empty text")
	}
	if !strings.Contains(err.Error(), "nothing to copy") {
		t.Errorf("err = %v", err)
	}
}

func TestCopyViaStdin_Success(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("true")
	if err := copyViaStdin(cmd, "hello"); err != nil {
		t.Fatalf("copyViaStdin: %v", err)
	}
}

func TestCopyViaStdin_ErrorCombinesStderr(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("sh", "-c", "echo failmsg >&2; exit 7")
	err := copyViaStdin(cmd, "x")
	if err == nil {
		t.Fatal("expected error from failing helper")
	}
	if !strings.Contains(err.Error(), "failmsg") {
		t.Errorf("expected stderr in error, got %v", err)
	}
}

func TestCopyViaStdin_ErrorNoOutput(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("sh", "-c", "exit 3")
	err := copyViaStdin(cmd, "x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func writeExecutable(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestCopyToClipboard_Linux_NoHelper(t *testing.T) {
	if runtime.GOOS != goosLinux {
		t.Skip("linux-specific clipboard fallback")
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("WAYLAND_DISPLAY", "")
	err := CopyToClipboard("hello")
	if err == nil || !strings.Contains(err.Error(), "no clipboard helper") {
		t.Fatalf("got %v", err)
	}
}

func TestCopyToClipboard_Linux_FakeClipboardHelpers(t *testing.T) {
	if runtime.GOOS != goosLinux {
		t.Skip("linux-specific clipboard paths")
	}
	script := "#!/bin/sh\nexec /bin/cat >/dev/null\n"
	tests := []struct {
		name   string
		bin    string
		path   string // full PATH value
		wayEnv string
		text   string
	}{
		{
			name:   "xclip",
			bin:    "xclip",
			path:   "", // filled below: dir + PATH
			wayEnv: "",
			text:   "hello",
		},
		{
			name:   "wl-copy",
			bin:    "wl-copy",
			path:   "",
			wayEnv: "wayland-1",
			text:   "hello",
		},
		{
			name:   "xsel",
			bin:    "xsel",
			path:   "", // dir only — no system xclip
			wayEnv: "",
			text:   "paste-me",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeExecutable(t, dir, tt.bin, script)
			pathVal := tt.path
			if pathVal == "" && tt.bin == "xsel" {
				pathVal = dir
			} else if pathVal == "" {
				pathVal = dir + string(os.PathListSeparator) + os.Getenv("PATH")
			}
			t.Setenv("PATH", pathVal)
			t.Setenv("WAYLAND_DISPLAY", tt.wayEnv)
			if err := CopyToClipboard(tt.text); err != nil {
				t.Fatal(err)
			}
		})
	}
}
