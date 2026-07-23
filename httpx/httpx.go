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

// DefaultReadHeaderTimeout caps client header send time to mitigate Slowloris attacks (gosec G112).
const DefaultReadHeaderTimeout = 10 * time.Second

// StartFunc is a server's listen-and-serve entry point, invoked in its own goroutine by Run.
type StartFunc func() error

// ServerSpec pairs an http.Server with the StartFunc that boots it; Run calls Shutdown on Server when ctx is canceled.
type ServerSpec struct {
	Server *http.Server
	Start  StartFunc
}

// NewServer returns an *http.Server with Addr, Handler, and DefaultReadHeaderTimeout set.
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

// HTTPSServerSpec wraps srv with srv.ListenAndServeTLS(cert, key); empty cert and key require srv.TLSConfig.Certificates to be populated.
func HTTPSServerSpec(srv *http.Server, cert, key string) ServerSpec {
	return ServerSpec{
		Server: srv,
		Start: func() error {
			return srv.ListenAndServeTLS(cert, key)
		},
	}
}

// Run starts every spec, waits for ctx cancellation or a Start failure, then shuts down all servers and joins the errors.
func Run(ctx context.Context, shutdownTimeout time.Duration, specs ...ServerSpec) error {
	if len(specs) == 0 {
		return nil
	}

	serveErrs := make([]error, len(specs))
	var wg sync.WaitGroup

	// shutdownParent must outlive ctx cancellation so shutdown can still run.
	shutdownParent := context.WithoutCancel(ctx)

	// runCtx trips shutdown on a Start failure, so a bind/TLS error at startup doesn't hang until SIGTERM.
	runCtx, cancelRun := context.WithCancel(ctx)
	defer cancelRun()

	for i, spec := range specs {
		wg.Go(func() {
			if err := spec.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serveErrs[i] = err
				cancelRun()
			}
		})
	}

	<-runCtx.Done()

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

	// Wait after Shutdown so ListenAndServe errors from the shutdown window are collected too.
	wg.Wait()

	return errors.Join(append(serveErrs, shutdownErrs...)...)
}
