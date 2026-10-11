package command

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Two goroutines broadcasting at once used to write to the same connection
// concurrently, which gorilla/websocket answers with
// "panic: concurrent write to websocket connection" — taking the whole
// `tomato ui` process down mid-run. The run goroutine and the file watcher
// both broadcast, so it needed no unusual timing to happen.
func TestBroadcast_ConcurrentWritesDoNotPanic(t *testing.T) {
	s := &UIServer{clients: make(map[*websocket.Conn]*sync.Mutex)}

	srv := httptest.NewServer(http.HandlerFunc(s.handleWebSocket))
	defer srv.Close()

	// Two clients, so the fan-out loop has more than one connection to hold
	// open while it writes.
	var conns []*websocket.Conn
	for i := 0; i < 2; i++ {
		c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer c.Close()
		conns = append(conns, c)
	}

	// Let both registrations land before broadcasting.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.clientsMux.RLock()
		n := len(s.clients)
		s.clientsMux.RUnlock()
		if n == len(conns) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Drain, so a client never blocks the writers.
	for _, c := range conns {
		go func(c *websocket.Conn) {
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					return
				}
			}
		}(c)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 25; j++ {
				s.broadcastRunStatus("run_output", "", "", "line")
			}
		}(i)
	}
	wg.Wait() // a concurrent write panics the process rather than failing here
}
