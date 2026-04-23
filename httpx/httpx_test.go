package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewServerSetsReadHeaderTimeout(t *testing.T) {
	srv := NewServer(":0", http.NotFoundHandler())
	if srv.Addr != ":0" {
		t.Errorf("addr: got %q, want :0", srv.Addr)
	}
	if srv.Handler == nil {
		t.Errorf("handler should be set")
	}
	if srv.ReadHeaderTimeout != DefaultReadHeaderTimeout {
		t.Errorf("ReadHeaderTimeout: got %v, want %v", srv.ReadHeaderTimeout, DefaultReadHeaderTimeout)
	}
}

func TestRunEmptySpecsReturnsNil(t *testing.T) {
	if err := Run(t.Context(), time.Second); err != nil {
		t.Errorf("empty specs: got err %v, want nil", err)
	}
}

func TestRunShutsDownOnCtxCancel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	srv := NewServer(addr, handler)
	// Close unused listener — we let ListenAndServe allocate its own.
	_ = ln.Close()

	ctx, cancel := context.WithCancel(t.Context())
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- Run(ctx, 2*time.Second, HTTPServerSpec(srv))
	}()

	// Poll the server until it's accepting connections.
	waitForServer(t, addr)

	resp, err := http.Get("http://" + addr + "/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "ok") {
		t.Errorf("body: got %q", string(body))
	}

	cancel()

	select {
	case err := <-runErrCh:
		if err != nil {
			t.Errorf("run returned err: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Run did not return after ctx cancel")
	}
}

func TestRunAggregatesServeErrors(t *testing.T) {
	startErr := errors.New("boom")
	spec := ServerSpec{
		Server: NewServer(":0", http.NotFoundHandler()),
		Start:  func() error { return startErr },
	}

	ctx, cancel := context.WithCancel(t.Context())
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- Run(ctx, time.Second, spec)
	}()

	// Give the goroutine a moment to record its error, then cancel so Run can return.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-runErrCh:
		if !errors.Is(err, startErr) {
			t.Errorf("want err chain to contain %v, got %v", startErr, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Run did not return")
	}
}

func TestRunIgnoresErrServerClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv := NewServer(addr, http.NotFoundHandler())

	ctx, cancel := context.WithCancel(t.Context())
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- Run(ctx, time.Second, HTTPServerSpec(srv))
	}()

	waitForServer(t, addr)
	cancel()

	select {
	case err := <-runErrCh:
		if err != nil {
			t.Errorf("expected nil (ErrServerClosed ignored), got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("Run did not return")
	}
}

func TestRunStartsAllSpecs(t *testing.T) {
	var count atomic.Int32
	mkSpec := func() ServerSpec {
		return ServerSpec{
			Server: NewServer(":0", http.NotFoundHandler()),
			Start: func() error {
				count.Add(1)
				return http.ErrServerClosed
			},
		}
	}

	ctx, cancel := context.WithCancel(t.Context())
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- Run(ctx, time.Second, mkSpec(), mkSpec(), mkSpec())
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-runErrCh:
		if err != nil {
			t.Errorf("err: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout")
	}
	if got := count.Load(); got != 3 {
		t.Errorf("started: got %d, want 3", got)
	}
}

func TestHTTPServerSpecUsesListenAndServe(t *testing.T) {
	srv := NewServer(":0", http.NotFoundHandler())
	spec := HTTPServerSpec(srv)
	if spec.Server != srv {
		t.Errorf("server mismatch")
	}
	if spec.Start == nil {
		t.Errorf("start must be set")
	}
}

func TestHTTPSServerSpecUsesListenAndServeTLS(t *testing.T) {
	srv := NewServer(":0", http.NotFoundHandler())
	spec := HTTPSServerSpec(srv, "cert.pem", "key.pem")
	if spec.Server != srv {
		t.Errorf("server mismatch")
	}
	if spec.Start == nil {
		t.Errorf("start must be set")
	}
	// Don't invoke Start — it would try to load real files. We only
	// verify wiring here; end-to-end TLS is covered implicitly by
	// consumers that run real TLS servers.
}

// waitForServer polls TCP connectivity to addr until successful or the
// deadline hits. Without this the test races against ListenAndServe's
// listener setup.
func waitForServer(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server at %s did not come up in time", addr)
}
