package handlers

import (
	"sync"

	"github.com/nats-io/nats.go"
)

var (
	jetStreamBumpMu sync.RWMutex
	jetStreamBump   nats.JetStreamContext
)

// SetJetStreamForConfigBump sets the JetStream context used when publishing node.config_changed after config bumps.
// Call from user-gateway and control-plane mains when NATS is enabled; pass nil to disable.
func SetJetStreamForConfigBump(js nats.JetStreamContext) {
	jetStreamBumpMu.Lock()
	jetStreamBump = js
	jetStreamBumpMu.Unlock()
}

func getJetStreamForConfigBump() nats.JetStreamContext {
	jetStreamBumpMu.RLock()
	defer jetStreamBumpMu.RUnlock()
	return jetStreamBump
}
