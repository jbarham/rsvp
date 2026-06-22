# rsvp

`rsvp` is a small Go library that makes it easy to run HTTP servers with graceful shutdown functionality.

The API is just two functions: `rsvp.ListenAndServe` is a drop-in replacement for the standard
library `http.ListenAndServe` function but adds graceful shutdown.
For more advanced use cases, `rsvp.Run` runs an `http.Server` with graceful shutdown.

The default behavior can be customized with options to set trigger signals, shutdown timeout and context,
and a logging function.

## Installation

`go get github.com/jbarham/rsvp`

## Motivation and Usage

Why does graceful shutdown matter? Because by default, Go HTTP servers will terminate in-flight requests when they're stopped
by signals like SIGTERM or SIGINT. This can have unwanted consequences in production systems if those terminated requests were
midway through operations such as updating a customer account.

The following sample code demonstrates how `rsvp` works to gracefully shut down Go HTTP servers without terminating
in-flight requests:

```go
package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jbarham/rsvp"
)

var runGracefully = flag.Bool("graceful", false, "set to shut down gracefully")

func main() {
	flag.Parse()

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

	if *runGracefully {
		rsvp.ListenAndServe(":8080", nil)
	} else {
		http.ListenAndServe(":8080", nil)
	}
}
```

Run `go run github.com/jbarham/rsvp/demo@latest` to start the above HTTP server
without graceful shutdown. Load http://localhost:8080/5 to trigger a 5 second
request, then stop the server with _Ctrl-C_ and note that the in-flight request
is terminated.

Run `go run github.com/jbarham/rsvp/demo@latest -graceful` to enable graceful shutdown,
repeat the above steps and note that the in-flight request is allowed to finish
before the server shuts down.

## Reference

https://pkg.go.dev/github.com/jbarham/rsvp