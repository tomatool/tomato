package network

import (
	"math/rand"
	"sync"
	"sync/atomic"

	"github.com/Shopify/toxiproxy/v2/toxics"
)

// gate is the part of a network that decides, per connection, whether it
// fails, and counts connections. Toxiproxy cannot do either: its toxicity is
// rolled with an unseeded random source, and rolled again for every open
// connection whenever another toxic is added. A gate toxic is installed when a
// proxy starts, before any connection exists, and Toxiproxy creates a stateful
// toxic's state once per connection, when it opens, so that is where the
// gate decides and counts.
type gate struct {
	mu       sync.Mutex
	rng      *rand.Rand
	conns    int // connections opened since the scenario started
	failed   int // of those, how many the gate failed
	failNext int // fail this many new connections, then let them through
	rate     float64
	// onFail is told about each connection the gate fails: its number in the
	// scenario and why.
	onFail func(n int, why string)
	// opened is closed and replaced when a connection opens, for steps that
	// wait for one.
	opened chan struct{}
}

func newGate(seed int64) *gate {
	return &gate{rng: rand.New(rand.NewSource(seed)), opened: make(chan struct{})}
}

// admit counts a new connection and says whether it fails.
func (g *gate) admit() bool {
	g.mu.Lock()
	g.conns++
	n := g.conns
	fail, why := false, ""
	switch {
	case g.failNext > 0:
		g.failNext--
		fail, why = true, "fails the next connections"
	case g.rate > 0 && g.rng.Float64() < g.rate:
		fail, why = true, "fails a share of new connections"
	}
	if fail {
		g.failed++
	}
	close(g.opened)
	g.opened = make(chan struct{})
	onFail := g.onFail
	g.mu.Unlock()
	if fail && onFail != nil {
		onFail(n, why)
	}
	return fail
}

// reset forgets the scenario's connections and failures.
func (g *gate) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.conns, g.failed, g.failNext, g.rate = 0, 0, 0, 0
}

func (g *gate) counts() (conns, failed int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.conns, g.failed
}

// gateToxic is the toxic in each proxy's upstream chain that asks the gate.
// Toxiproxy creates it from JSON, so the gate is handed to it afterwards,
// while connections may already be opening.
type gateToxic struct {
	gate atomic.Pointer[gate]
}

type gateState struct{ fail bool }

// NewState runs once per connection, when it opens.
func (t *gateToxic) NewState() interface{} {
	g := t.gate.Load()
	if g == nil {
		return &gateState{}
	}
	return &gateState{fail: g.admit()}
}

// Pipe closes a failed connection at once, and passes everything else through.
func (t *gateToxic) Pipe(stub *toxics.ToxicStub) {
	if st, ok := stub.State.(*gateState); ok && st.fail {
		stub.Close()
		return
	}
	new(toxics.NoopToxic).Pipe(stub)
}

const gateType = "tomato_gate"

func init() { toxics.Register(gateType, new(gateToxic)) }
