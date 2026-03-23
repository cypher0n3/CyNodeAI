package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// CopyToClipboard writes text to the system clipboard using OS-specific helpers
// (pbcopy, wl-copy, xclip, xsel, or Windows clip).
func CopyToClipboard(text string) error {
	if text == "" {
		return fmt.Errorf("nothing to copy")
	}
	switch runtime.GOOS {
	case "darwin":
		return copyViaStdin(exec.Command("pbcopy"), text)
	case "windows":
		return copyViaStdin(exec.Command("cmd", "/c", "clip"), text)
	default:
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			if p, err := exec.LookPath("wl-copy"); err == nil {
				return copyViaStdin(exec.Command(p), text)
			}
		}
		if p, err := exec.LookPath("xclip"); err == nil {
			return copyViaStdin(exec.Command(p, "-selection", "clipboard"), text)
		}
		if p, err := exec.LookPath("xsel"); err == nil {
			return copyViaStdin(exec.Command(p, "--clipboard", "--input"), text)
		}
		return fmt.Errorf("no clipboard helper found (install wl-copy, xclip, or xsel)")
	}
}

func copyViaStdin(cmd *exec.Cmd, text string) error {
	cmd.Stdin = strings.NewReader(text)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
		}
		return err
	}
	return nil
}
