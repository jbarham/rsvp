// Package rsvp provides a simple way to run an HTTP server with graceful shutdown functionality.
//
// It listens for specified OS signals (SIGTERM and SIGINT by default) and shuts down the server gracefully,
// by calling [http.Server.Shutdown], when one of those signals is received.
// In particular, this allows the server to finish handling any in-flight HTTP requests before shutting down, instead of terminating immediately.
//
// The simplest way to use rsvp is to replace calls to [http.ListenAndServe] with [rsvp.ListenAndServe],
// which has the same signature but adds graceful shutdown functionality.
//
// For more advanced use cases, you can create an [http.Server] and call [rsvp.Run] directly.
//
// The default behavior can be customized with the provided [Option] functions.
package rsvp

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var ErrNilServer = errors.New("rsvp: server is nil")

type runner struct {
	Timeout           time.Duration
	Signals           []os.Signal
	ShutdownContext   context.Context
	CertFile, KeyFile string
	LogFunc           func(msg string)

	server  *http.Server
	sigChan chan os.Signal

	// Testing only
	isTest     bool
	listenPort int
}

func newRunner(s *http.Server, isTest bool, opts ...Option) (*runner, error) {
	if s == nil {
		return nil, ErrNilServer
	}
	r := &runner{
		Signals: []os.Signal{syscall.SIGTERM, syscall.SIGINT},
		LogFunc: func(msg string) { log.Println(msg) },
		server:  s,
		sigChan: make(chan os.Signal, 1),
		isTest:  isTest,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

func (r *runner) log(msg string) {
	if r.LogFunc != nil {
		r.LogFunc(msg)
	}
}

func (r *runner) run() error {
	// Return immediately if we can't bind to the listening address
	ln, err := net.Listen("tcp", r.server.Addr)
	if err != nil {
		return err
	}

	defer ln.Close()

	shutdownErrChan, shutdownCancel := func() (chan error, context.CancelFunc) {
		cancelCtx, cancel := context.WithCancel(context.Background())
		ch := make(chan error, 1)
		go func() {
			if !r.isTest {
				// Set up signal handler
				signal.Notify(r.sigChan, r.Signals...)
				defer signal.Stop(r.sigChan)
			}

			// Wait for signal or context cancellation
			select {
			case sig := <-r.sigChan:
				r.log(fmt.Sprintf("Got signal %s, shutting down...", sig))
			case <-cancelCtx.Done():
				return // Shutdown was canceled, so exit goroutine without shutting down server
			}

			// Shut down server, with optional timeout
			ctx := r.ShutdownContext
			if ctx == nil {
				ctx = context.Background()
			}
			if r.Timeout > 0 {
				var timeoutCancel context.CancelFunc
				ctx, timeoutCancel = context.WithTimeout(ctx, r.Timeout)
				defer timeoutCancel()
			}
			ch <- r.server.Shutdown(ctx)
			r.log("Server shutdown complete")
		}()
		return ch, cancel
	}()
	defer shutdownCancel() // Ensure shutdown goroutine is cleaned up when server exits

	listenAddr := r.server.Addr
	if r.isTest {
		r.listenPort = ln.Addr().(*net.TCPAddr).Port
		listenAddr = fmt.Sprintf(":%d", r.listenPort)
	}

	var serveErr error
	if r.CertFile != "" && r.KeyFile != "" {
		r.log(fmt.Sprintf("Starting TLS server on %s...", listenAddr))
		serveErr = r.server.ServeTLS(ln, r.CertFile, r.KeyFile)
	} else {
		r.log(fmt.Sprintf("Starting server on %s...", listenAddr))
		serveErr = r.server.Serve(ln)
	}
	if serveErr != nil && serveErr != http.ErrServerClosed {
		// Unexpected error, so return it and skip waiting for shutdown to complete
		return serveErr
	}

	// Wait until shutdown has finished, returning error, if any
	return <-shutdownErrChan
}

// Option functions configure the behavior of the graceful shutdown process.
type Option func(*runner)

// WithTimeout sets the timeout for the shutdown process.
// If the shutdown takes longer than this, shutdown will be canceled and [Run] will return the [context.DeadlineExceeded] error.
//
// Without a timeout (the default case), the shutdown process will wait indefinitely for in-flight requests to finish before shutting down the server.
// In practice, service managers like systemd will send a final non-catchable SIGKILL signal after a certain timeout if the process has not exited.
func WithTimeout(t time.Duration) Option {
	return func(r *runner) { r.Timeout = t }
}

// WithSignals sets the signals that will trigger the shutdown process.
// The default signals are SIGTERM and SIGINT.
func WithSignals(sig os.Signal, extra ...os.Signal) Option {
	return func(r *runner) { r.Signals = append([]os.Signal{sig}, extra...) }
}

// WithShutdownContext sets the context that will be used for the shutdown process.
// The default context is [context.Background].
//
// If a timeout is also set by [WithTimeout], the shutdown will be canceled if it takes longer than the timeout, even if the context itself has not been canceled.
func WithShutdownContext(ctx context.Context) Option {
	return func(r *runner) { r.ShutdownContext = ctx }
}

// WithTLS sets the TLS certificate and key files to use for the server.
//
// If both certFile and keyFile are provided, the server will use TLS. Otherwise, it will use plain text HTTP.
func WithTLS(certFile, keyFile string) Option {
	return func(r *runner) {
		r.CertFile = certFile
		r.KeyFile = keyFile
	}
}

// WithLogFunc sets the logging function that will be used to log messages during the server lifecycle.
//
// The default logging function calls log.Println. Setting the logging function to nil will disable logging.
func WithLogFunc(logger func(msg string)) Option {
	return func(r *runner) { r.LogFunc = logger }
}

// Run starts the HTTP server and shuts it down gracefully when a signal is received.
//
// Run returns any error that occurs during startup or shutdown. It returns nil on successful shutdown.
func Run(server *http.Server, opts ...Option) error {
	r, err := newRunner(server, false, opts...)
	if err != nil {
		return err
	}
	return r.run()
}

// ListenAndServe is a convenience wrapper around [Run] that creates an http.Server with the given address and handler.
//
// ListenAndServe is intended primarily as a drop-in replacement for [http.ListenAndServe] which adds graceful shutdown functionality.
func ListenAndServe(addr string, handler http.Handler, opts ...Option) error {
	s := &http.Server{Addr: addr, Handler: handler}
	return Run(s, opts...)
}
