package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

const (
	// The slowest handlers call Bachs for up to 40 s (store onboarding), so
	// writes get longer than that; reads allow image uploads on slow mobile data.
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 60 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second

	// shutdownTimeout is how long requests in progress get to finish once the
	// API is asked to stop. Process managers must wait longer than this before
	// killing it (Docker's default is only 10 s: set stop_grace_period).
	shutdownTimeout = 45 * time.Second
)

// newHTTPServer wraps the router with timeouts, so slow or idle clients
// can't hold connections open forever.
func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    1 << 20,
	}
}

// serve answers requests on ln until ctx is done, then stops taking new
// connections and waits up to timeout for requests in progress to finish.
func serve(ctx context.Context, srv *http.Server, ln net.Listener, timeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// waitFor waits for every channel to close, or until the deadline passes.
func waitFor(deadline time.Time, done ...<-chan struct{}) bool {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	for _, d := range done {
		select {
		case <-d:
		case <-timer.C:
			return false
		}
	}
	return true
}
