package tui

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
)

func TestModel_applyStreamDone_UnauthorizedOpensLoginOverlay(t *testing.T) {
	t.Parallel()
	cl := gateway.NewClient("http://gw")
	sess := chat.NewSession(cl)
	m := NewModel(sess)
	m.Scrollback = []string{assistantPrefix}
	m.seedTranscriptAssistantInFlight()
	m.applyStreamDone(streamDoneMsg{
		err: &gateway.HTTPError{Status: http.StatusUnauthorized, Err: fmt.Errorf("401 Unauthorized")},
	})
	if !m.ShowLoginForm {
		t.Fatal("expected ShowLoginForm after 401 stream end")
	}
	combined := strings.Join(m.Scrollback, "\n")
	if !strings.Contains(combined, chat.LandmarkAuthRecoveryReady) {
		t.Fatalf("scrollback should contain auth recovery landmark; got %q", combined)
	}
}

func TestModel_applySendResult_UnauthorizedOpensLoginOverlay(t *testing.T) {
	t.Parallel()
	cl := gateway.NewClient("http://gw")
	sess := chat.NewSession(cl)
	m := NewModel(sess)
	m.Loading = true
	m.applySendResult(sendResult{
		err: &gateway.HTTPError{Status: http.StatusUnauthorized, Err: fmt.Errorf("401 Unauthorized")},
	})
	if !m.ShowLoginForm {
		t.Fatal("expected ShowLoginForm after 401 send result")
	}
	combined := strings.Join(m.Scrollback, "\n")
	if !strings.Contains(combined, chat.LandmarkAuthRecoveryReady) {
		t.Fatalf("scrollback should contain auth recovery landmark; got %q", combined)
	}
}
