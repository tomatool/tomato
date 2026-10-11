package resource

import "time"

// Eventually polls cond until it holds or timeout passes.
//
// Assertions need this whenever the state they check arrives asynchronously
// to the step that caused it: a message is still in flight, a connection has
// not registered yet. Each resource passes its own timeout, because what
// counts as "too slow" differs between a local websocket and a Kafka round
// trip.
func Eventually(timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}
