package rsvp_test

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/jbarham/rsvp"
)

func ExampleListenAndServe() {
	http.HandleFunc("GET /{delay}", func(w http.ResponseWriter, r *http.Request) {
		delay, err := strconv.Atoi(r.PathValue("delay"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Starting request with %ds delay...", delay)
		time.Sleep(time.Duration(delay) * time.Second)
		log.Print("Finished request")
	})
	rsvp.ListenAndServe(":8080", nil)
}

func ExampleRun() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, world!")
	})
	server := &http.Server{Addr: ":8080"}
	// Registered shutdown functions are run in separate goroutines,
	// so we need to wait for them to finish before exiting main.
	var wg sync.WaitGroup
	wg.Add(1)
	server.RegisterOnShutdown(func() {
		log.Print("Server is shutting down, cleaning up...")
		// Close database connections, WebSockets etc
		wg.Done()
	})
	rsvp.Run(server)
	wg.Wait()
	log.Print("Shutdown functions complete, exiting")
}

func ExampleWithLogFunc() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, world!")
	})
	logFunc := func(msg string) {
		slog.Info(msg)
	}
	rsvp.ListenAndServe(":8080", nil, rsvp.WithLogFunc(logFunc))
}

func ExampleWithTimeout() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Print("A slow request is being processed...")
		time.Sleep(10 * time.Second)
		log.Print("Slow request completed")
	})
	if err := rsvp.ListenAndServe(":8080", nil, rsvp.WithTimeout(5*time.Second)); err != nil {
		log.Fatalf("Shutdown error: %s", err)
	}
}
