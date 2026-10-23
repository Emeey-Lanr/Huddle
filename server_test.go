package main

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// startServe runs serve on a free local port and returns its address and result.
func startServe(t *testing.T, ctx context.Context, handler http.Handler, timeout time.Duration) (string, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- serve(ctx, newHTTPServer(handler), ln, timeout) }()
	return "http://" + ln.Addr().String(), result
}

func TestServe_FinishesRequestsInProgress(t *testing.T) {
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		io.WriteString(w, "done")
	})
	ctx, cancel := context.WithCancel(context.Background())
	addr, result := startServe(t, ctx, handler, 5*time.Second)

	type reply struct {
		code int
		body string
		err  error
	}
	got := make(chan reply, 1)
	go func() {
		resp, err := http.Get(addr + "/slow")
		if err != nil {
			got <- reply{err: err}
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		got <- reply{code: resp.StatusCode, body: string(b)}
	}()

	<-started
	cancel() // asked to stop while the request is still running

	r := <-got
	if r.err != nil || r.code != http.StatusOK || r.body != "done" {
		t.Errorf("request in progress: %d %q %v, want 200 \"done\"", r.code, r.body, r.err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Errorf("serve returned %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after shutdown")
	}

	// New connections are refused once it has stopped.
	if _, err := http.Get(addr + "/slow"); err == nil {
		t.Error("server still accepts requests after shutdown")
	}
}

func TestServe_GivesUpAfterTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	})
	ctx, cancel := context.WithCancel(context.Background())
	addr, result := startServe(t, ctx, handler, 100*time.Millisecond)

	go http.Get(addr + "/stuck")
	<-started
	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("serve returned %v, want context.DeadlineExceeded", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve waited past its shutdown timeout")
	}
}

func TestNewHTTPServer_HasTimeouts(t *testing.T) {
	srv := newHTTPServer(http.NotFoundHandler())
	if srv.ReadHeaderTimeout == 0 || srv.ReadTimeout == 0 || srv.WriteTimeout == 0 || srv.IdleTimeout == 0 {
		t.Errorf("missing timeouts: %+v", srv)
	}
	// Bachs onboarding can take 40 s; writes must not be cut off before it answers.
	if srv.WriteTimeout <= 40*time.Second {
		t.Errorf("WriteTimeout %v is shorter than the slowest handler", srv.WriteTimeout)
	}
	if shutdownTimeout <= 40*time.Second {
		t.Errorf("shutdownTimeout %v would cut off the slowest handler", shutdownTimeout)
	}
}

func TestWaitFor(t *testing.T) {
	closed := make(chan struct{})
	close(closed)
	if !waitFor(time.Now().Add(time.Second), closed, closed) {
		t.Error("closed channels: want true")
	}

	open := make(chan struct{})
	start := time.Now()
	if waitFor(time.Now().Add(50*time.Millisecond), closed, open) {
		t.Error("open channel: want false")
	}
	if time.Since(start) > time.Second {
		t.Error("waitFor ignored its deadline")
	}
}
