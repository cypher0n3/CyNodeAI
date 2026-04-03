package sessionnats

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cypher0n3/cynodeai/cynork/internal/gateway"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/natsconfig"
	"github.com/cypher0n3/cynodeai/go_shared_libs/contracts/userapi"
	"github.com/cypher0n3/cynodeai/go_shared_libs/natsutil"
	"github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
)

func TestMain(m *testing.M) {
	activityTickerInterval = 25 * time.Millisecond
	code := m.Run()
	activityTickerInterval = 2 * time.Minute
	os.Exit(code)
}

func TestCynorkNats_StartNilLogin(t *testing.T) {
	t.Parallel()
	r, err := Start(t.Context(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r != nil {
		t.Fatal("expected nil runtime")
	}
}

func TestCynorkNats_PublishAttachedJetStream(t *testing.T) {
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

	sid := "550e8400-e29b-41d4-a716-446655440000"
	subj := "cynode.session.attached." + defaultTenantID + "." + sid
	ch := make(chan *nats.Msg, 2)
	sub, err := nc.ChanSubscribe(subj, ch)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	r := &Runtime{
		js:         js,
		tenantID:   defaultTenantID,
		sessionID:  sid,
		userID:     "22222222-2222-4222-8222-222222222222",
		bindingKey: "bk1",
	}
	if err := r.publishAttached(); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-ch:
		if len(msg.Data) == 0 {
			t.Fatal("empty message")
		}
		var env struct {
			EventType string `json:"event_type"`
		}
		if err := json.Unmarshal(msg.Data, &env); err != nil {
			t.Fatal(err)
		}
		if env.EventType != "session.attached" {
			t.Fatalf("event_type %q", env.EventType)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for attached")
	}
}

func TestCynorkNats_PublishActivityJetStream(t *testing.T) {
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

	sid := "550e8400-e29b-41d4-a716-446655440001"
	subj := "cynode.session.activity." + defaultTenantID + "." + sid
	ch := make(chan *nats.Msg, 2)
	sub, err := nc.ChanSubscribe(subj, ch)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	r := &Runtime{
		js:         js,
		tenantID:   defaultTenantID,
		sessionID:  sid,
		userID:     "u1",
		bindingKey: "bk2",
	}
	if err := r.publishActivity(); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-ch:
		if !strings.Contains(string(msg.Data), "session.activity") {
			t.Fatalf("payload: %s", msg.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for activity")
	}
}

func TestCynorkNats_PublishDetachedJetStream(t *testing.T) {
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

	sid := "550e8400-e29b-41d4-a716-446655440002"
	subj := "cynode.session.detached." + defaultTenantID + "." + sid
	ch := make(chan *nats.Msg, 2)
	sub, err := nc.ChanSubscribe(subj, ch)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	r := &Runtime{
		js:         js,
		tenantID:   defaultTenantID,
		sessionID:  sid,
		userID:     "u1",
		bindingKey: "bk3",
	}
	if err := r.publishDetached("logout"); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-ch:
		if !strings.Contains(string(msg.Data), "session.detached") {
			t.Fatalf("payload: %s", msg.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for detached")
	}
}

func TestStartWithDial_FullFlow(t *testing.T) {
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
	dial := func(_ *natsutil.NatsConfig) (*nats.Conn, nats.JetStreamContext, error) {
		return nc, js, nil
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users/me" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "22222222-2222-4222-8222-222222222222", "handle": "alice", "is_active": true,
		})
	}))
	defer ts.Close()

	client := gateway.NewClient(ts.URL)
	client.SetToken("tok")
	login := &userapi.LoginResponse{
		InteractiveSessionID: "550e8400-e29b-41d4-a716-446655440099",
		SessionBindingKey:    "bk",
		Nats: &natsconfig.ClientCredentials{
			URL:          s.ClientURL(),
			JWT:          "unused",
			JWTExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	rt, err := startWithDial(t.Context(), nil, client, login, dial)
	if err != nil {
		t.Fatal(err)
	}
	if rt == nil {
		t.Fatal("expected runtime")
	}
	defer rt.Close("")

	subj := "cynode.session.activity." + defaultTenantID + "." + login.InteractiveSessionID
	ch := make(chan *nats.Msg, 4)
	sub, err := nc.ChanSubscribe(subj, ch)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	select {
	case msg := <-ch:
		if !strings.Contains(string(msg.Data), "session.activity") {
			t.Fatalf("got %s", msg.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for activity tick")
	}
}

func TestStartWithDial_DialError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "u1", "handle": "a", "is_active": true})
	}))
	defer ts.Close()
	client := gateway.NewClient(ts.URL)
	client.SetToken("t")
	login := &userapi.LoginResponse{
		InteractiveSessionID: "550e8400-e29b-41d4-a716-446655440088",
		SessionBindingKey:    "bk",
		Nats: &natsconfig.ClientCredentials{
			URL:          "nats://127.0.0.1:4222",
			JWT:          "x",
			JWTExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	dial := func(_ *natsutil.NatsConfig) (*nats.Conn, nats.JetStreamContext, error) {
		return nil, nil, errors.New("dial failed")
	}
	_, err := startWithDial(t.Context(), nil, client, login, dial)
	if err == nil || !strings.Contains(err.Error(), "dial failed") {
		t.Fatalf("got %v", err)
	}
}

func TestStartWithDial_GetMeError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()
	client := gateway.NewClient(ts.URL)
	client.SetToken("t")
	login := &userapi.LoginResponse{
		InteractiveSessionID: "550e8400-e29b-41d4-a716-446655440077",
		SessionBindingKey:    "bk",
		Nats: &natsconfig.ClientCredentials{
			URL:          "nats://127.0.0.1:4222",
			JWT:          "x",
			JWTExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	_, err := startWithDial(t.Context(), nil, client, login, func(_ *natsutil.NatsConfig) (*nats.Conn, nats.JetStreamContext, error) {
		return nil, nil, errors.New("should not dial")
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestStart_InvalidNatsValidate(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "u1", "handle": "a", "is_active": true})
	}))
	defer ts.Close()
	client := gateway.NewClient(ts.URL)
	login := &userapi.LoginResponse{
		InteractiveSessionID: "550e8400-e29b-41d4-a716-446655440066",
		SessionBindingKey:    "bk",
		Nats: &natsconfig.ClientCredentials{
			URL:          "",
			JWT:          "",
			JWTExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	_, err := Start(t.Context(), nil, client, login)
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestRuntime_Close_Nil(t *testing.T) {
	var r *Runtime
	r.Close("x")
}

func TestPublishAttached_JSNil(t *testing.T) {
	r := &Runtime{}
	if err := r.publishAttached(); err != nil {
		t.Fatal(err)
	}
}

func TestPublishActivity_JSNil(t *testing.T) {
	r := &Runtime{}
	if err := r.publishActivity(); err != nil {
		t.Fatal(err)
	}
}

func TestPublishDetached_JSNil(t *testing.T) {
	r := &Runtime{}
	if err := r.publishDetached("logout"); err != nil {
		t.Fatal(err)
	}
}

func TestClose_DetachedWhenJSNil(t *testing.T) {
	r := &Runtime{}
	r.Close("logout")
}

func TestStartWithDial_NilDialUsesConnect(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "u1", "handle": "a", "is_active": true})
	}))
	defer ts.Close()
	client := gateway.NewClient(ts.URL)
	client.SetToken("t")
	login := &userapi.LoginResponse{
		InteractiveSessionID: "550e8400-e29b-41d4-a716-446655440055",
		SessionBindingKey:    "bk",
		Nats: &natsconfig.ClientCredentials{
			URL:          "nats://127.0.0.1:1",
			JWT:          "x",
			JWTExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}
	_, err := startWithDial(t.Context(), nil, client, login, nil)
	if err == nil {
		t.Fatal("expected connect error")
	}
}

func TestRunActivityLoop_LogsPublishError(t *testing.T) {
	opts := test.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	s := test.RunServer(&opts)
	defer s.Shutdown()
	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if err := natsutil.EnsureStreams(js); err != nil {
		t.Fatal(err)
	}
	nc.Close()
	r := &Runtime{
		js:         js,
		tenantID:   defaultTenantID,
		sessionID:  "550e8400-e29b-41d4-a716-446655440044",
		userID:     "u1",
		bindingKey: "bk",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	log := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
	go r.runActivityLoop(ctx, log)
	time.Sleep(60 * time.Millisecond)
	cancel()
}

func TestRuntime_Close_ReasonPublishesDetached(t *testing.T) {
	opts := test.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	s := test.RunServer(&opts)
	defer s.Shutdown()

	nc, err := nats.Connect(s.ClientURL())
	if err != nil {
		t.Fatal(err)
	}
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if err := natsutil.EnsureStreams(js); err != nil {
		t.Fatal(err)
	}

	sid := "550e8400-e29b-41d4-a716-446655440003"
	subj := "cynode.session.detached." + defaultTenantID + "." + sid
	ch := make(chan *nats.Msg, 2)
	sub, err := nc.ChanSubscribe(subj, ch)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	_, cancelLoop := context.WithCancel(t.Context())
	r := &Runtime{
		nc:         nc,
		js:         js,
		cancelLoop: cancelLoop,
		tenantID:   defaultTenantID,
		sessionID:  sid,
		userID:     "u1",
		bindingKey: "bk4",
	}
	r.Close("logout")
	select {
	case msg := <-ch:
		if !strings.Contains(string(msg.Data), "session.detached") {
			t.Fatalf("payload: %s", msg.Data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for detached from Close")
	}
}

func TestCloseConn_Idempotent(t *testing.T) {
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
	r := &Runtime{nc: nc}
	if err := r.closeConn(); err != nil {
		t.Fatal(err)
	}
	if err := r.closeConn(); err != nil {
		t.Fatal(err)
	}
}
