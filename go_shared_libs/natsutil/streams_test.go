package natsutil_test

import (
	"testing"

	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"

	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
)

func TestEnsureStreams_CreatesCYNODE_SESSION(t *testing.T) {
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
	if err := natsutil.EnsureStreams(js); err != nil {
		t.Fatal(err)
	}
	if err := natsutil.EnsureStreams(js); err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	info, err := js.StreamInfo(natsutil.StreamCYNODE_SESSION)
	if err != nil {
		t.Fatal(err)
	}
	if info.Config.Name != natsutil.StreamCYNODE_SESSION {
		t.Fatalf("stream name: %s", info.Config.Name)
	}
	found := false
	for _, subj := range info.Config.Subjects {
		if subj == subjectNodeConfigChangedGlob {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("stream subjects missing config_changed: %#v", info.Config.Subjects)
	}
}

func TestEnsureStreams_MergesSubjectsWhenStreamIncomplete(t *testing.T) {
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
	if err := natsutil.EnsureStreams(js); err != nil {
		t.Fatal(err)
	}
	info, err := js.StreamInfo(natsutil.StreamCYNODE_SESSION)
	if err != nil {
		t.Fatal(err)
	}
	up := info.Config
	var subjs []string
	for _, subj := range up.Subjects {
		if subj != subjectNodeConfigChangedGlob {
			subjs = append(subjs, subj)
		}
	}
	up.Subjects = subjs
	if _, err := js.UpdateStream(&up); err != nil {
		t.Fatal(err)
	}
	if err := natsutil.EnsureStreams(js); err != nil {
		t.Fatal(err)
	}
	info2, err := js.StreamInfo(natsutil.StreamCYNODE_SESSION)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, subj := range info2.Config.Subjects {
		if subj == subjectNodeConfigChangedGlob {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("merged stream missing config_changed: %#v", info2.Config.Subjects)
	}
}
