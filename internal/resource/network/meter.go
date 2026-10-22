package network

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/Shopify/toxiproxy/v2/toxics"
)

// meter counts what goes over a network's connections: bytes each way, how
// many requests the dependency answered, and how long it took to start
// answering each one.
//
// A connection's two directions are separate Toxiproxy links, each with its
// own toxic state. Toxiproxy's accept loop starts a connection's upstream link
// and then its downstream one, one connection at a time, and creates a
// stateful toxic's state as it starts a link, so the upstream meter's state is
// handed to the downstream meter's through pending, and the two directions of
// one connection share a meterConn.
//
// The meters are installed with the gate, before any fault, so the time is
// the dependency's own: latency a scenario adds on the way back is not in it.
type meter struct {
	mu      sync.Mutex
	pending *meterConn

	sent, received   atomic.Int64
	roundTrips, open atomic.Int64
	waitNs           atomic.Int64
}

type meterConn struct {
	mu       sync.Mutex
	awaiting bool
	sentAt   time.Time
	closed   int // directions that ended; the connection is closed at 2
}

func (m *meter) reset() {
	m.sent.Store(0)
	m.received.Store(0)
	m.roundTrips.Store(0)
	m.waitNs.Store(0)
}

// request notes bytes on their way to the dependency.
func (m *meter) request(c *meterConn, n int) {
	m.sent.Add(int64(n))
	c.mu.Lock()
	if !c.awaiting {
		c.awaiting, c.sentAt = true, time.Now()
	}
	c.mu.Unlock()
}

// response notes bytes on their way back: the first after a request ends
// the wait for it.
func (m *meter) response(c *meterConn, n int) {
	m.received.Add(int64(n))
	c.mu.Lock()
	if c.awaiting {
		m.waitNs.Add(int64(time.Since(c.sentAt)))
		m.roundTrips.Add(1)
		c.awaiting = false
	}
	c.mu.Unlock()
}

func (m *meter) closeHalf(c *meterConn) {
	c.mu.Lock()
	c.closed++
	done := c.closed == 2
	c.mu.Unlock()
	if done {
		m.open.Add(-1)
	}
}

// meterToxic is one direction's meter in a proxy's chain. Toxiproxy creates
// a toxic as the zero value of its registered type, so each direction is a
// type of its own rather than a field.
type meterToxic struct {
	meter atomic.Pointer[meter]
}

type upstreamMeter struct{ meterToxic }
type downstreamMeter struct{ meterToxic }

func (t *upstreamMeter) NewState() interface{}         { return t.newState(true) }
func (t *downstreamMeter) NewState() interface{}       { return t.newState(false) }
func (t *upstreamMeter) Pipe(stub *toxics.ToxicStub)   { t.pipe(stub, true) }
func (t *downstreamMeter) Pipe(stub *toxics.ToxicStub) { t.pipe(stub, false) }

func (t *meterToxic) newState(upstream bool) interface{} {
	m := t.meter.Load()
	if m == nil {
		return (*meterConn)(nil)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if upstream {
		c := &meterConn{}
		m.pending = c
		m.open.Add(1)
		return c
	}
	c := m.pending
	m.pending = nil
	if c == nil {
		// A downstream link without its upstream half: an existing
		// connection after the chain changed. Meter it alone.
		c = &meterConn{closed: 1}
	}
	return c
}

func (t *meterToxic) pipe(stub *toxics.ToxicStub, upstream bool) {
	m := t.meter.Load()
	c, _ := stub.State.(*meterConn)
	for {
		select {
		case <-stub.Interrupt:
			return
		case chunk := <-stub.Input:
			if chunk == nil {
				if m != nil && c != nil {
					m.closeHalf(c)
				}
				stub.Close()
				return
			}
			if m != nil && c != nil {
				if upstream {
					m.request(c, len(chunk.Data))
				} else {
					m.response(c, len(chunk.Data))
				}
			}
			stub.Output <- chunk
		}
	}
}

const (
	meterUpType   = "tomato_meter_upstream"
	meterDownType = "tomato_meter_downstream"
)

func init() {
	toxics.Register(meterUpType, new(upstreamMeter))
	toxics.Register(meterDownType, new(downstreamMeter))
}
