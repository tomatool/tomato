// The app the profile fixture measures: each endpoint costs something known,
// so tests/features/profile.feature can check the profile reports it.
package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var (
	mu   sync.Mutex
	held [][]byte // memory /memory holds on to until /release
)

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {})

	// /cpu?ms=400 keeps one core busy for that long.
	http.HandleFunc("/cpu", func(w http.ResponseWriter, r *http.Request) {
		ms, _ := strconv.Atoi(r.URL.Query().Get("ms"))
		end := time.Now().Add(time.Duration(ms) * time.Millisecond)
		x := 0
		for time.Now().Before(end) {
			x++
		}
		fmt.Fprint(w, x%2)
	})

	// /memory?mb=64 allocates that much, touches every page and keeps it.
	http.HandleFunc("/memory", func(w http.ResponseWriter, r *http.Request) {
		mb, _ := strconv.Atoi(r.URL.Query().Get("mb"))
		b := make([]byte, mb<<20)
		for i := 0; i < len(b); i += 4096 {
			b[i] = 1
		}
		mu.Lock()
		held = append(held, b)
		mu.Unlock()
	})

	// /log?level=error writes a line at that level, as a logger would.
	http.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		level := r.URL.Query().Get("level")
		fmt.Printf("%s %s payment provider answered 503\n", time.Now().Format(time.RFC3339), level)
	})

	if err := http.ListenAndServe(":"+os.Getenv("PORT"), nil); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
