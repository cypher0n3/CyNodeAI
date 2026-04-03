package natsutil

import (
	"errors"
	"slices"
	"time"

	"github.com/nats-io/nats.go"
)

// StreamCYNODE_SESSION is the JetStream stream name for session lifecycle subjects (Phase 1).
const StreamCYNODE_SESSION = "CYNODE_SESSION"

// DefaultSessionStreamMaxAge matches docs/tech_specs/nats_messaging.md operational defaults.
const DefaultSessionStreamMaxAge = 6 * time.Hour

// defaultSessionStreamSubjects are JetStream subjects for session lifecycle and node config notifications (Phase 1).
func defaultSessionStreamSubjects() []string {
	return []string{
		"cynode.session.activity.*.*",
		"cynode.session.attached.*.*",
		"cynode.session.detached.*.*",
		"cynode.node.config_changed.*.*",
	}
}

// EnsureStreams creates or updates the CYNODE_SESSION stream so required subjects are present.
// CYNODE_CHAT stream creation is deferred (Phase 2 chat helpers).
func EnsureStreams(js nats.JetStreamContext) error {
	want := defaultSessionStreamSubjects()
	_, err := js.AddStream(&nats.StreamConfig{
		Name:     StreamCYNODE_SESSION,
		Subjects: want,
		MaxAge:   DefaultSessionStreamMaxAge,
		Storage:  nats.FileStorage,
	})
	if err == nil {
		return nil
	}
	if !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
		return err
	}
	info, err := js.StreamInfo(StreamCYNODE_SESSION)
	if err != nil {
		return err
	}
	if streamCoversSubjects(info.Config.Subjects, want) {
		return nil
	}
	merged := mergeSubjectLists(info.Config.Subjects, want)
	up := info.Config
	up.Subjects = merged
	_, err = js.UpdateStream(&up)
	return err
}

func streamCoversSubjects(have, need []string) bool {
	for _, n := range need {
		if !slices.Contains(have, n) {
			return false
		}
	}
	return true
}

func mergeSubjectLists(have, add []string) []string {
	out := slices.Clone(have)
	for _, s := range add {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}
