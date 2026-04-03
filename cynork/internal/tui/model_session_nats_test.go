package tui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/cynork/internal/chat"
	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/natsconfig"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

func TestModel_SessionNats_RestartAndCleanup(t *testing.T) {
	t.Parallel()
	s := &chat.Session{Client: gateway.NewClient("http://127.0.0.1:9")}
	m := NewModel(s)
	m.restartSessionNatsFromLogin(nil, nil)
	m.restartSessionNatsFromLogin(s.Client, nil)
	m.CleanupSessionNats()
}

func TestModel_RestartSessionNats_GetMeError(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	s := &chat.Session{Client: gateway.NewClient(ts.URL)}
	s.Client.SetToken("tok")
	m := NewModel(s)
	m.restartSessionNatsFromLogin(s.Client, &userapi.LoginResponse{
		InteractiveSessionID: "550e8400-e29b-41d4-a716-446655440033",
		SessionBindingKey:    "bk",
		Nats: &natsconfig.ClientCredentials{
			URL:          "nats://127.0.0.1:4222",
			JWT:          "x",
			JWTExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	})
	found := false
	for _, line := range m.Scrollback {
		if strings.Contains(line, "NATS:") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected NATS error in scrollback: %v", m.Scrollback)
	}
}
