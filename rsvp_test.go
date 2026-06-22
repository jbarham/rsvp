package rsvp

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"syscall"
	"testing"
	"time"
)

const responseBody = "Hello, world!"

func TestNilServer(t *testing.T) {
	if err := Run(nil); err != ErrNilServer {
		t.Fatalf("got error %s but wanted %s", err, ErrNilServer)
	}
}

func TestInvalidAddr(t *testing.T) {
	if err := ListenAndServe(":bogus", nil); err != nil && err.Error() != "listen tcp: lookup tcp/bogus: unknown port" {
		t.Fatal(err)
	}
}

type shutdownTest struct {
	name        string
	delay       time.Duration
	timeout     time.Duration
	expectedErr string
}

func (st shutdownTest) run(t *testing.T, useTLS bool) {
	server := &http.Server{
		Addr: ":0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if st.delay > 0 {
				time.Sleep(st.delay)
			}
			fmt.Fprint(w, responseBody)
		}),
	}
	var opts []Option
	if st.timeout > 0 {
		opts = append(opts, WithTimeout(st.timeout))
	}
	if useTLS {
		opts = append(opts, WithTLS("testdata/cert.pem", "testdata/key.pem"))
	}
	r, err := newRunner(server, true, opts...)
	if err != nil { // Shouldn't happen
		t.Fatal(err)
	}
	if !testing.Verbose() {
		r.LogFunc = nil
	}
	// Start the server and make a request to it, checking that the response is correct, then trigger shutdown.
	var wg sync.WaitGroup
	wg.Go(func() {
		time.Sleep(100 * time.Millisecond) // Wait for server to start running
		if r.listenPort == 0 {
			t.Fatal("listen failed") // Shouldn't happen
		}
		url := fmt.Sprintf("http://localhost:%d/", r.listenPort)
		client := &http.Client{}
		if useTLS {
			url = fmt.Sprintf("https://localhost:%d/", r.listenPort)
			client.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
		resp, err := client.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("got status code %d but wanted %d", resp.StatusCode, http.StatusOK)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("body read failed: %s", err)
		}
		if string(body) != responseBody {
			t.Fatalf("got response body %q but wanted %q", string(body), responseBody)
		}
	})
	wg.Go(func() {
		time.Sleep(150 * time.Millisecond) // Give HTTP request a chance to run
		r.sigChan <- syscall.SIGINT        // Trigger shutdown
	})
	if err := r.run(); err != nil && err.Error() != st.expectedErr {
		t.Errorf("got error %s but wanted %s", err, st.expectedErr)
	}
	wg.Wait()
}

func TestShutdown(t *testing.T) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	for _, useTLS := range []bool{false, true} {
		for _, test := range []shutdownTest{
			{
				name: "no timeout",
			},
			{
				name:    "good timeout",
				delay:   100 * time.Millisecond,
				timeout: 200 * time.Millisecond,
			},
			{
				name:        "bad timeout",
				delay:       300 * time.Millisecond,
				timeout:     100 * time.Millisecond,
				expectedErr: "context deadline exceeded",
			},
		} {
			testName := test.name
			if useTLS {
				testName += " tls"
			}
			t.Run(testName, func(t *testing.T) {
				test.run(t, useTLS)
			})
		}
	}
}
