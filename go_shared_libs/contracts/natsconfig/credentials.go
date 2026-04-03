// Package natsconfig holds shared NATS client credential shapes for login and worker bootstrap.
// See docs/tech_specs/nats_messaging.md and docs/tech_specs/worker_node_payloads.md.
package natsconfig

// ClientCredentials mirrors the `nats` object returned by the orchestrator (login and bootstrap).
type ClientCredentials struct {
	URL          string            `json:"url"`
	JWT          string            `json:"jwt"`
	JWTExpiresAt string            `json:"jwt_expires_at"`
	WebSocketURL string            `json:"websocket_url,omitempty"`
	CABundlePEM  string            `json:"ca_bundle_pem,omitempty"`
	Subjects     map[string]string `json:"subjects,omitempty"`
}
