package rsvp

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestNilServer(t *testing.T) {
	if err := Run(nil); err != ErrNilServer {
		t.Fatalf("got error %s but wanted %s", err, ErrNilServer)
	}
}

func TestShutdown(t *testing.T) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	for _, test := range []struct {
		name        string
		addr        string
		delay       time.Duration
		timeout     time.Duration
		expectedErr string
	}{
		{
			name:        "invalid addr",
			addr:        ":bogus",
			expectedErr: "listen tcp: lookup tcp/bogus: unknown port",
		},
		{
			name: "no timeout",
			addr: ":0",
		},
		{
			name:    "good timeout",
			addr:    ":0",
			delay:   100 * time.Millisecond,
			timeout: 200 * time.Millisecond,
		},
		{
			name:        "bad timeout",
			addr:        ":0",
			delay:       300 * time.Millisecond,
			timeout:     100 * time.Millisecond,
			expectedErr: "context deadline exceeded",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := &http.Server{
				Addr: test.addr,
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if test.delay > 0 {
						time.Sleep(test.delay)
					}
					fmt.Fprint(w, "Hello, world!")
				}),
			}
			r, err := newRunner(server, true, WithTimeout(test.timeout))
			if err != nil {
				t.Fatal(err)
			}
			if !testing.Verbose() {
				r.LogFunc = nil
			}
			var wg sync.WaitGroup
			wg.Go(func() {
				time.Sleep(100 * time.Millisecond) // Wait for server to start running
				if r.listenPort == 0 {             // Listen failed
					return
				}
				url := fmt.Sprintf("http://localhost:%d/", r.listenPort)
				resp, err := http.Get(url)
				if err != nil {
					return
				}
				resp.Body.Close()
			})
			wg.Go(func() {
				time.Sleep(150 * time.Millisecond) // Give HTTP request a chance to run
				r.sigChan <- syscall.SIGINT        // Trigger shutdown
			})
			if err := r.run(); err != nil && err.Error() != test.expectedErr {
				t.Errorf("got error %s but wanted %s", err, test.expectedErr)
			}
			wg.Wait()
		})
	}
}
