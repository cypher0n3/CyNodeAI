package natsutil

import (
	"testing"

	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

func TestBuildEnvelope_marshalFails(t *testing.T) {
	t.Parallel()
	_, err := buildEnvelope("e", EventVersionSessionV1, Producer{}, Scope{}, Correlation{}, make(chan int))
	if err == nil {
		t.Fatal("expected json marshal error")
	}
}

func TestBuildEnvelope_unmarshalToMapFails(t *testing.T) {
	t.Parallel()
	_, err := buildEnvelope("e", EventVersionSessionV1, Producer{}, Scope{}, Correlation{}, 42)
	if err == nil {
		t.Fatal("expected error unmarshaling non-object JSON into map")
	}
}

func TestPublishPayload_nilPayload(t *testing.T) {
	t.Parallel()
	err := publishPayload(nil, nil, "subj", eventSessionActivity, EventVersionSessionV1, Producer{}, Scope{}, Correlation{}, nil)
	if err == nil {
		t.Fatal("expected nil payload error")
	}
}

func TestPublishJSON_marshalFails(t *testing.T) {
	opts := test.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	s := test.RunServer(&opts)
	defer s.Shutdown()
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	env := &Envelope{Payload: map[string]any{"bad": make(chan int)}}
	err = publishJSON(js, nc, "cynode.session.activity.t.s", env)
	if err == nil {
		t.Fatal("expected marshal error")
	}
}
