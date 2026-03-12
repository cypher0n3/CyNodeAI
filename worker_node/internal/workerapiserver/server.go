// Package workerapiserver provides the Worker API HTTP server lifecycle for both
// standalone worker-api and embedded use by the node-manager (single-process topology).
// See docs/tech_specs/worker_node.md CYNAI.WORKER.SingleProcessHostBinary.
package workerapiserver

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CallerServiceIDContextKey is the context key for the managed-service caller identity (service_id) on internal proxy requests.
// Handlers that need the caller service ID should read it from the request context.
var CallerServiceIDContextKey = contextKey("caller_service_id")

type contextKey string

// WithCallerServiceID wraps the handler so that requests receive the given serviceID in context under CallerServiceIDContextKey.
// Used when serving the internal proxy on per-service UDS listeners.
func WithCallerServiceID(next http.Handler, serviceID string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), CallerServiceIDContextKey, serviceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RunConfig holds the handlers and listen options for the Worker API server.
// Callers (worker-api main or node-manager) build the handlers via BuildMuxes and pass them here.
type RunConfig struct {
	// PublicHandler is the main Worker API mux (healthz, readyz, jobs, telemetry, managed-service proxy).
	PublicHandler http.Handler
	// InternalHandler is the internal orchestrator proxy mux (mcp:call, agent:ready).
	InternalHandler http.Handler
	// ListenAddr is the TCP address for the public API (e.g. ":12090" or "0.0.0.0:12090").
	ListenAddr string
	// InternalListenAddr is the optional TCP address for the internal server (e.g. "127.0.0.1:9191"). If empty, internal TCP is not used.
	InternalListenAddr string
	// StateDir is the worker state directory (for UDS paths).
	StateDir string
	// SocketByService maps service_id to UDS path for per-service internal proxy. Optional.
	SocketByService map[string]string
	// InternalListenUnix is an optional single unix socket path for the internal server (WORKER_INTERNAL_LISTEN_UNIX).
	InternalListenUnix string
	// Logger for server lifecycle and errors.
	Logger *slog.Logger
}

// Server holds the HTTP servers and listeners; call Run (blocking) or Start (non-blocking) then Shutdown.
type Server struct {
	cfg           RunConfig
	publicSrv     *http.Server
	internalSrv   *http.Server
	publicLn      net.Listener
	internalLn    net.Listener
	internalUnix  net.Listener
	internalUDSS  []*http.Server
	internalUDSLn []net.Listener
	readyOnce     sync.Once
	readyCh       chan struct{}
	serverErr     chan error
}

// NewServer creates a Server from the run config. It does not start listening; call Run or Start.
func NewServer(cfg *RunConfig) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("RunConfig is required")
	}
	cfgCopy := *cfg
	if cfgCopy.PublicHandler == nil {
		return nil, errors.New("PublicHandler is required")
	}
	if cfgCopy.InternalHandler == nil {
		cfgCopy.InternalHandler = http.NewServeMux()
	}
	if cfgCopy.ListenAddr == "" {
		cfgCopy.ListenAddr = getEnv("LISTEN_ADDR", ":9190")
	}
	if cfgCopy.InternalListenAddr == "" {
		cfgCopy.InternalListenAddr = getEnv("WORKER_INTERNAL_LISTEN_ADDR", "127.0.0.1:9191")
	}
	if cfgCopy.Logger == nil {
		cfgCopy.Logger = slog.Default()
	}
	s := &Server{
		cfg:       cfgCopy,
		readyCh:   make(chan struct{}),
		serverErr: make(chan error, 1),
	}
	s.publicSrv = &http.Server{
		Addr:              cfgCopy.ListenAddr,
		Handler:           cfgCopy.PublicHandler,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	s.internalSrv = &http.Server{
		Addr:              cfgCopy.InternalListenAddr,
		Handler:           cfgCopy.InternalHandler,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	return s, nil
}

// ListenAddr returns the resolved public API listen address (after env defaults).
func (s *Server) ListenAddr() string { return s.cfg.ListenAddr }

// InternalListenAddr returns the resolved internal API listen address (after env defaults).
func (s *Server) InternalListenAddr() string { return s.cfg.InternalListenAddr }

// Run starts all listeners and blocks until ctx is canceled, then shuts down.
// Returns nil after graceful shutdown, or an error if startup (bind) fails.
func (s *Server) Run(ctx context.Context) error {
	if err := s.startListeners(); err != nil {
		return err
	}
	defer s.shutdownListeners(context.WithoutCancel(ctx))
	select {
	case <-ctx.Done():
	case err := <-s.serverErr:
		if err != nil {
			return err
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	return s.Shutdown(shutdownCtx)
}

// Start starts all listeners in goroutines and returns a channel that closes when the public API is listening.
// The caller must call Shutdown when done. Non-blocking; returns (readyCh, nil) or (nil, err) if bind fails.
func (s *Server) Start(ctx context.Context) (<-chan struct{}, error) {
	if err := s.startListeners(); err != nil {
		return nil, err
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = s.Shutdown(shutdownCtx)
		s.shutdownListeners(shutdownCtx)
	}()
	return s.readyCh, nil
}

func (s *Server) startListeners() error {
	if err := s.startPublicListener(); err != nil {
		return err
	}
	if err := s.startInternalTCPListener(); err != nil {
		return err
	}
	if err := s.startInternalUnixListener(); err != nil {
		return err
	}
	return s.startPerServiceUDSListeners()
}

func (s *Server) startPublicListener() error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return err
	}
	s.publicLn = ln
	s.signalReady()
	go func() {
		if err := s.publicSrv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.serverErr <- err
		}
	}()
	return nil
}

func (s *Server) startInternalTCPListener() error {
	if s.cfg.InternalListenAddr == "" {
		return nil
	}
	internalLn, err := net.Listen("tcp", s.cfg.InternalListenAddr)
	if err != nil {
		return err
	}
	s.internalLn = internalLn
	go func() {
		if err := s.internalSrv.Serve(internalLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.serverErr <- err
		}
	}()
	return nil
}

func (s *Server) startInternalUnixListener() error {
	path := strings.TrimSpace(s.cfg.InternalListenUnix)
	if path == "" {
		return nil
	}
	_ = os.RemoveAll(path)
	unixLn, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	s.internalUnix = unixLn
	go func() {
		if err := s.internalSrv.Serve(unixLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.serverErr <- err
		}
	}()
	return nil
}

func (s *Server) startPerServiceUDSListeners() error {
	for serviceID, socketPath := range s.cfg.SocketByService {
		if err := s.startOneServiceUDSListener(serviceID, socketPath); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) startOneServiceUDSListener(serviceID, socketPath string) error {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return err
	}
	_ = os.RemoveAll(socketPath)
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return err
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = l.Close()
		return err
	}
	serviceHandler := WithCallerServiceID(s.cfg.InternalHandler, serviceID)
	serviceSrv := &http.Server{
		Handler:           serviceHandler,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	s.internalUDSS = append(s.internalUDSS, serviceSrv)
	s.internalUDSLn = append(s.internalUDSLn, l)
	go func() {
		if err := serviceSrv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.serverErr <- err
		}
	}()
	return nil
}

func (s *Server) signalReady() {
	s.readyOnce.Do(func() { close(s.readyCh) })
}

func (s *Server) shutdownListeners(ctx context.Context) {
	if s.publicLn != nil {
		_ = s.publicLn.Close()
	}
	if s.internalLn != nil {
		_ = s.internalLn.Close()
	}
	if s.internalUnix != nil {
		_ = s.internalUnix.Close()
		_ = os.Remove(s.cfg.InternalListenUnix)
	}
	for _, l := range s.internalUDSLn {
		_ = l.Close()
	}
	for _, path := range s.cfg.SocketByService {
		_ = os.Remove(path)
	}
}

// Shutdown gracefully shuts down all HTTP servers. Call after Run returns or when the process is exiting.
func (s *Server) Shutdown(ctx context.Context) error {
	var errs []error
	if s.publicSrv != nil {
		if err := s.publicSrv.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if s.internalSrv != nil {
		if err := s.internalSrv.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	for _, serv := range s.internalUDSS {
		if err := serv.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
