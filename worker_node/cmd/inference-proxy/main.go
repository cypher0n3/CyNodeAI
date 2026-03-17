// inference-proxy is a minimal HTTP reverse proxy that forwards requests to Ollama.
// REQ-WORKER-0270: when INFERENCE_PROXY_SOCKET is set, the proxy listens on a Unix domain
// socket instead of TCP 127.0.0.1:11434, so that sandboxes receive inference via UDS.
// Per docs/tech_specs/worker_node.md: enforces request size (10 MiB) and per-request timeout;
// MUST NOT expose credentials.
package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cypher0n3/cynodeai/worker_node/internal/inferenceproxy"
)

const (
	listenAddr      = "127.0.0.1:11434"
	defaultUpstream = "http://host.containers.internal:11434"
)

func main() {
	os.Exit(runMain(context.Background(), os.Args[1:]))
}

// runMain is extracted from main() for testability.
func runMain(ctx context.Context, args []string) int {
	if healthURL := parseHealthcheckURL(args); healthURL != "" {
		return runHealthcheck(ctx, healthURL)
	}
	// REQ-WORKER-0270: prefer UDS when INFERENCE_PROXY_SOCKET is set.
	if sockPath := os.Getenv("INFERENCE_PROXY_SOCKET"); sockPath != "" {
		return runUDS(ctx, sockPath)
	}
	return run(ctx, nil, "")
}

// runUDS delegates to inferenceproxy.RunUDS.
// REQ-WORKER-0270: UDS mode; TCP port 11434 is NOT bound.
func runUDS(ctx context.Context, sockPath string) int {
	return inferenceproxy.RunUDS(ctx, sockPath)
}

// run starts the proxy server. If INFERENCE_PROXY_SOCKET is set and listener is nil, delegates to
// UDS mode (REQ-WORKER-0270). Otherwise uses TCP. If listener is non-nil it is used as-is (tests).
func run(ctx context.Context, listener net.Listener, listenAddrOverride string) int {
	if sockPath := os.Getenv("INFERENCE_PROXY_SOCKET"); sockPath != "" && listener == nil {
		return runUDS(ctx, sockPath)
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
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", inferenceproxy.NewProxy(u))

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  inferenceproxy.PerRequestTimeout + 10*time.Second,
		WriteTimeout: inferenceproxy.PerRequestTimeout + 10*time.Second,
	}

	if listener == nil {
		addr := listenAddr
		if listenAddrOverride != "" {
			addr = listenAddrOverride
		}
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

func parseHealthcheckURL(args []string) string {
	fs := flag.NewFlagSet("inference-proxy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var healthURL string
	fs.StringVar(&healthURL, "healthcheck-url", "", "perform one health check request and exit")
	_ = fs.Parse(args)
	return healthURL
}

func runHealthcheck(ctx context.Context, rawURL string) int {
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	// Support http+unix:// for UDS health checks (REQ-WORKER-0270).
	httpClient := http.DefaultClient
	effectiveURL := rawURL
	if strings.HasPrefix(rawURL, "http+unix://") {
		encoded := strings.TrimPrefix(rawURL, "http+unix://")
		sockPath := encoded
		urlPath := "/healthz"
		if idx := strings.Index(encoded, "/"); idx > 0 {
			sockPath = encoded[:idx]
			urlPath = encoded[idx:]
		}
		decoded, err := url.PathUnescape(sockPath)
		if err != nil {
			slog.Error("invalid healthcheck UDS path", "url", rawURL, "error", err)
			return 1
		}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", decoded)
			},
		}
		httpClient = &http.Client{Timeout: 2 * time.Second, Transport: transport}
		effectiveURL = "http://localhost" + urlPath
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, effectiveURL, http.NoBody)
	if err != nil {
		slog.Error("build healthcheck request failed", "error", err)
		return 1
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		slog.Error("healthcheck request failed", "error", err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		slog.Error("healthcheck status not ok", "status", resp.StatusCode)
		return 1
	}
	return 0
}
