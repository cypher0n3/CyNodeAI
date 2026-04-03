package natsutil

import "github.com/nats-io/nats.go"

// CloseConn drains and closes a NATS connection (graceful disconnect).
func CloseConn(nc *nats.Conn) error {
	if nc == nil {
		return nil
	}
	return nc.Drain()
}
