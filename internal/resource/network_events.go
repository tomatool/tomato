package resource

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// NetworkEvent is something a network resource did to the link between the
// application and a dependency: a fault it applied or healed, or a
// connection it failed.
type NetworkEvent struct {
	At       time.Time `json:"at"`
	Resource string    `json:"resource"`
	// Kind is "applied", "healed" or "connection".
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

var (
	networkMu sync.Mutex
	// networkEvents holds, per scenario, what happened on the network since
	// the formatter last took them, so each event is reported on the step
	// that was running when it happened.
	networkEvents = map[string][]NetworkEvent{}
	// networkFaults holds, per network resource, what describes the faults it
	// has on now: a line each, nil when its network is healthy. A function, so
	// a fault that heals by itself reports the time it has left.
	networkFaults = map[string]func() []string{}
)

// RecordNetworkEvent notes a network event in the scenario, for the `tomato`
// formatter to report on the step that is running.
func RecordNetworkEvent(scenario string, e NetworkEvent) {
	networkMu.Lock()
	defer networkMu.Unlock()
	networkEvents[scenario] = append(networkEvents[scenario], e)
}

// TakeNetworkEvents returns the network events recorded for the scenario since
// the last call, and forgets them. nil when there are none.
func TakeNetworkEvents(scenario string) []NetworkEvent {
	networkMu.Lock()
	defer networkMu.Unlock()
	out := networkEvents[scenario]
	delete(networkEvents, scenario)
	return out
}

// RegisterNetworkFaults sets what a network resource reports as active: a
// function returning one line per fault it has on, nil when its network is
// healthy.
func RegisterNetworkFaults(resource string, active func() []string) {
	networkMu.Lock()
	defer networkMu.Unlock()
	networkFaults[resource] = active
}

// NetworkFaults returns the lines of every network resource with a fault on,
// in resource-name order. nil when the whole network is healthy.
func NetworkFaults() []string {
	networkMu.Lock()
	names := make([]string, 0, len(networkFaults))
	active := make(map[string]func() []string, len(networkFaults))
	for name, fn := range networkFaults {
		names = append(names, name)
		active[name] = fn
	}
	networkMu.Unlock()
	sort.Strings(names)
	var out []string
	for _, name := range names {
		out = append(out, active[name]()...)
	}
	return out
}

// NetworkFaultError is a step's error with the network faults that were
// active when it failed. It unwraps to the step's own error.
type NetworkFaultError struct {
	Err    error
	Faults []string
}

func (e *NetworkFaultError) Error() string {
	return e.Err.Error() + "\nwhile the network was faulty:\n  " + strings.Join(e.Faults, "\n  ")
}

func (e *NetworkFaultError) Unwrap() error { return e.Err }

// WithNetworkFaults adds the active network faults to a step's error, or
// returns it unchanged when the network is healthy.
func WithNetworkFaults(err error) error {
	faults := NetworkFaults()
	var already *NetworkFaultError
	if len(faults) == 0 || errors.As(err, &already) {
		return err
	}
	return &NetworkFaultError{Err: err, Faults: faults}
}
