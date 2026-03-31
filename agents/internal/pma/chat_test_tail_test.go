package pma

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"syscall"
	"testing"
)

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate(short,10)=%q", got)
	}
	if got := truncate("longer than five", 5); got != "longe…" {
		t.Errorf("truncate(longer...,5)=%q", got)
	}
}

func TestIsClientDisconnect(t *testing.T) {
	if isClientDisconnect(nil) {
		t.Error("nil is not disconnect")
	}
	if !isClientDisconnect(syscall.EPIPE) {
		t.Error("EPIPE")
	}
	if !isClientDisconnect(syscall.ECONNRESET) {
		t.Error("ECONNRESET")
	}
	if !isClientDisconnect(&net.OpError{Err: syscall.EPIPE}) {
		t.Error("OpError EPIPE")
	}
	if !isClientDisconnect(errors.New("broken pipe")) {
		t.Error("broken pipe string")
	}
	if !isClientDisconnect(errors.New("connection reset by peer")) {
		t.Error("reset string")
	}
	if isClientDisconnect(errors.New("other")) {
		t.Error("other error")
	}
}

func TestLogStreamCompletionInferenceError(t *testing.T) {
	logStreamCompletionInferenceError(nil, errors.New("x"), false)
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	logStreamCompletionInferenceError(log, errors.New("pre"), false)
	logStreamCompletionInferenceError(log, syscall.EPIPE, true)
	logStreamCompletionInferenceError(log, errors.New("post"), true)
}
