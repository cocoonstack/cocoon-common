// Package httpx provides shared HTTP server lifecycle helpers so that
// cocoonstack binaries don't each reinvent graceful-shutdown plumbing.
package httpx

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"
)

const (
	// DefaultReadHeaderTimeout is the conservative default for
	// http.Server.ReadHeaderTimeout. 10s is what every cocoonstack
	// consumer was already using and mitigates Slowloris attacks
	// (gosec G112) by capping how long a client may take to send headers.
	DefaultReadHeaderTimeout = 10 * time.Second
)

// StartFunc is the listen-and-serve entry point for a single server,
// invoked in its own goroutine by Run. Typical implementations are
// (*http.Server).ListenAndServe or (*http.Server).ListenAndServeTLS.
type StartFunc func() error

// ServerSpec pairs an http.Server with the StartFunc that boots it.
// Start is called in a goroutine; Server is what Run calls Shutdown on
// when the parent context is canceled.
type ServerSpec struct {
	Server *http.Server
	Start  StartFunc
}

// NewServer returns an *http.Server with Addr, Handler, and the safe
// ReadHeaderTimeout default set. Use this in preference to composing an
// http.Server literal so every server in the stack carries the same
// Slowloris-mitigation timeout.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: DefaultReadHeaderTimeout,
	}
}

// HTTPServerSpec wraps srv with srv.ListenAndServe as its StartFunc.
func HTTPServerSpec(srv *http.Server) ServerSpec {
	return ServerSpec{
		Server: srv,
		Start:  srv.ListenAndServe,
	}
}

// HTTPSServerSpec wraps srv with srv.ListenAndServeTLS(cert, key) as its
// StartFunc. If cert and key are empty, the server is expected to have
// its TLSConfig.Certificates populated and srv.ListenAndServeTLS("", "")
// will still work.
func HTTPSServerSpec(srv *http.Server, cert, key string) ServerSpec {
	return ServerSpec{
		Server: srv,
		Start: func() error {
			return srv.ListenAndServeTLS(cert, key)
		},
	}
}

// Run starts every spec in its own goroutine and blocks until ctx is
// canceled. On cancellation it calls Shutdown on each server using a
// fresh timeout context that is *not* derived from the canceled parent
// (shutdown must outlive the signal-derived ctx). Errors from both
// serve and shutdown are aggregated with errors.Join. http.ErrServerClosed
// is treated as a clean shutdown and never returned.
func Run(ctx context.Context, shutdownTimeout time.Duration, specs ...ServerSpec) error {
	if len(specs) == 0 {
		return nil
	}

	serveErrs := make([]error, len(specs))
	var wg sync.WaitGroup

	// Capture a ctx-independent parent for shutdown so that when the
	// caller's signal-derived ctx is canceled we still have a live
	// parent to derive WithTimeout from.
	shutdownParent := context.WithoutCancel(ctx)

	for i, spec := range specs {
		wg.Go(func() {
			if err := spec.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serveErrs[i] = err
			}
		})
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(shutdownParent, shutdownTimeout)
	defer cancel()

	shutdownErrs := make([]error, len(specs))
	var shutdownWG sync.WaitGroup
	for i, spec := range specs {
		shutdownWG.Go(func() {
			if err := spec.Server.Shutdown(shutdownCtx); err != nil {
				shutdownErrs[i] = err
			}
		})
	}
	shutdownWG.Wait()

	// Wait for serve goroutines to return after Shutdown so we collect
	// any ListenAndServe errors produced during the shutdown window.
	wg.Wait()

	return errors.Join(append(serveErrs, shutdownErrs...)...)
}
