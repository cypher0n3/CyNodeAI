package natsutil_test

import (
	"testing"

	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"

	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
)

func TestCloseConn_Nil(t *testing.T) {
	t.Parallel()
	if err := natsutil.CloseConn(nil); err != nil {
		t.Fatal(err)
	}
}

func TestCloseConn_Drains(t *testing.T) {
	opts := test.DefaultTestOptions
	opts.Port = -1
	s := test.RunServer(&opts)
	defer s.Shutdown()
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	if err := natsutil.CloseConn(nc); err != nil {
		t.Fatal(err)
	}
}
