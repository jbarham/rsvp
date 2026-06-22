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
