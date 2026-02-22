// inference-proxy is a minimal HTTP reverse proxy that forwards requests to Ollama.
// It listens on localhost:11434 inside a pod and forwards to the node's Ollama container.
// Per docs/tech_specs/worker_node.md: enforces request size (10 MiB) and per-request timeout (120s);
// MUST NOT expose credentials.
package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/cypher0n3/cynodeai/worker_node/internal/inferenceproxy"
)

const (
	listenAddr      = ":11434"
	defaultUpstream = "http://host.containers.internal:11434"
)

func main() {
	os.Exit(run(context.Background(), nil, ""))
}

// run starts the proxy server. If listener is nil, one is created at listenAddr (or default when empty).
// When ctx is done, the server shuts down. Returns 0 on normal shutdown, 1 on error.
func run(ctx context.Context, listener net.Listener, listenAddrOverride string) int {
	addr := listenAddr
	if listenAddrOverride != "" {
		addr = listenAddrOverride
	}
	upstream := os.Getenv("OLLAMA_UPSTREAM_URL")
	if upstream == "" {
		upstream = defaultUpstream
	}
	u, err := url.Parse(upstream)
	if err != nil {
		slog.Error("invalid OLLAMA_UPSTREAM_URL", "url", upstream, "error", err)
		return 1
	}

	srv := &http.Server{
		Addr:         listenAddr,
		Handler:      inferenceproxy.NewProxy(u),
		ReadTimeout:  inferenceproxy.PerRequestTimeout + 10*time.Second,
		WriteTimeout: inferenceproxy.PerRequestTimeout + 10*time.Second,
	}

	if listener == nil {
		var listenErr error
		listener, listenErr = net.Listen("tcp", addr)
		if listenErr != nil {
			slog.Error("listen failed", "error", listenErr)
			return 1
		}
	}

	done := make(chan struct{})
	var serveErr error
	go func() {
		serveErr = srv.Serve(listener)
		close(done)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
		<-done
		return 0
	case <-done:
		if serveErr != nil && serveErr != http.ErrServerClosed {
			slog.Error("server failed", "error", serveErr)
			return 1
		}
		return 0
	}
}
