package tui

import (
	"context"
	"log/slog"

	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/cynork/internal/sessionnats"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
)

func (m *Model) stopSessionNats(reason string) {
	if m.sessionNats != nil {
		m.sessionNats.Close(reason)
		m.sessionNats = nil
	}
}

// CleanupSessionNats closes the optional NATS session lifecycle client (call after TUI program exits).
func (m *Model) CleanupSessionNats() {
	m.stopSessionNats("shutdown")
}

func (m *Model) restartSessionNatsFromLogin(client *gateway.Client, login *userapi.LoginResponse) {
	m.stopSessionNats("")
	if client == nil || login == nil {
		return
	}
	rt, err := sessionnats.Start(context.Background(), slog.Default(), client, login)
	if err != nil {
		m.Scrollback = append(m.Scrollback, ScrollbackSystemLinePrefix+"NATS: "+err.Error())
		return
	}
	if rt == nil {
		return
	}
	m.sessionNats = rt
}
