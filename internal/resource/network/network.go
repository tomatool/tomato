// Package network provides tomato's `network` resource: a proxy between the
// application under test and one of its dependency containers, which a
// scenario makes slow, cuts, hangs or takes down, to see what the
// application does.
//
// It works on TCP, so it works the same for every dependency tomato runs in a
// container: databases, caches, brokers, object stores. Only the application
// goes through it: its {{.<container>.port.<port>}} resolves to the proxy,
// while tomato's own resources keep connecting to the container, so the
// steps that check a result never see the faults.
//
// The proxy is Toxiproxy, embedded: tomato drives it in-process, without its
// HTTP API, and adds one toxic of its own, the gate (gate.go).
package network

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	toxiproxy "github.com/Shopify/toxiproxy/v2"
	"github.com/cucumber/godog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/resource"
)

// containerAccess is what Network needs from the container manager.
type containerAccess interface {
	GetHost(ctx context.Context, name string) (string, error)
	GetPort(ctx context.Context, name, port string) (string, error)
	ExposedPorts(name string) []string
}

// Network is the `network` resource.
type Network struct {
	name      string
	container string
	cm        containerAccess
	ports     []string       // the container ports it stands in front of, "5432/tcp"
	listen    map[string]int // a fixed port to listen on per container port (the kafka preset's), else random
	seed      int64

	mu          sync.Mutex
	initialized bool
	links       []*link
	gate        *gate
	faults      map[string]*fault
	order       []string // fault keys in the order they were applied
	scenario    string
	step        int // the running step's 1-based number in its scenario
	changedAt   int // connections seen when a fault was last applied or healed
	stopFlap    chan struct{}
}

// link is one proxied port.
type link struct {
	port     string // container port, "5432/tcp"
	upstream string // the container, as tomato reaches it
	proxy    *toxiproxy.Proxy
}

// fault is one thing wrong with the network.
type fault struct {
	desc  func() string // "adds latency 300ms"
	step  int
	until time.Time // zero: until the scenario heals it or ends
	off   func()
	timer *time.Timer
}

// New creates a network resource. Options: ports (default every port the
// container exposes) and seed (default random, printed at the start).
func New(name string, cfg config.Resource, cm *container.Manager) (*Network, error) {
	n := &Network{
		name:      name,
		container: cfg.Container,
		listen:    map[string]int{},
		seed:      rand.Int63n(1_000_000),
		faults:    map[string]*fault{},
	}
	if cm != nil {
		n.cm = cm
	}
	if err := n.parseOptions(cfg.Options); err != nil {
		return nil, err
	}
	return n, nil
}

// ValidateOptions checks the options without starting anything.
func ValidateOptions(opts map[string]any) error {
	return (&Network{listen: map[string]int{}}).parseOptions(opts)
}

func (n *Network) parseOptions(opts map[string]any) error {
	if raw, ok := opts["ports"]; ok {
		list, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("ports: want a list of container ports, like [5432]")
		}
		for _, p := range list {
			port, err := normalizePort(p)
			if err != nil {
				return fmt.Errorf("ports: %w", err)
			}
			n.ports = append(n.ports, port)
		}
	}
	if raw, ok := opts["seed"]; ok {
		seed, ok := raw.(int)
		if !ok {
			return fmt.Errorf("seed: want a whole number, got %v", raw)
		}
		n.seed = int64(seed)
	}
	// listen is set by the kafka preset, which has to advertise the proxy's
	// port to the app before the broker starts.
	if raw, ok := opts["listen"].(map[string]any); ok {
		for port, v := range raw {
			p, err := normalizePort(port)
			if err != nil {
				return fmt.Errorf("listen: %w", err)
			}
			listen, ok := v.(int)
			if !ok {
				return fmt.Errorf("listen: want a port number for %s, got %v", port, v)
			}
			n.listen[p] = listen
		}
	}
	return nil
}

func normalizePort(v any) (string, error) {
	s := fmt.Sprint(v)
	num := strings.TrimSuffix(s, "/tcp")
	if _, err := strconv.Atoi(num); err != nil {
		return "", fmt.Errorf("%q is not a TCP port", s)
	}
	return num + "/tcp", nil
}

func (n *Network) Name() string { return n.name }

// ResetsFirst: other resources reset over the link, so it is healed first.
func (n *Network) ResetsFirst() bool { return true }

// Init starts a proxy per port. It runs before the app starts, so the app's
// addresses can point at the proxies, and again with every other resource,
// when it does nothing.
func (n *Network) Init(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.initialized {
		return nil
	}
	if n.cm == nil {
		return fmt.Errorf("network %s: no container manager", n.name)
	}
	ports := n.ports
	if len(ports) == 0 {
		ports = n.cm.ExposedPorts(n.container)
		// A port the kafka preset gave its own listener for tomato is not the app's.
		ports = filterPorts(ports, n.listen)
	}
	if len(ports) == 0 {
		return fmt.Errorf("network %s: container %q exposes no ports to stand in front of", n.name, n.container)
	}

	host, err := n.cm.GetHost(ctx, n.container)
	if err != nil {
		return fmt.Errorf("network %s: %w", n.name, err)
	}
	server := toxiproxy.NewServer(toxiproxy.NewMetricsContainer(prometheus.NewRegistry()), zerolog.Nop())
	n.gate = newGate(n.seed)
	n.gate.onFail = n.connectionFailed

	for _, port := range ports {
		mapped, err := n.cm.GetPort(ctx, n.container, port)
		if err != nil {
			return fmt.Errorf("network %s: container %q port %s: %w", n.name, n.container, port, err)
		}
		upstream := net.JoinHostPort(host, mapped)
		p := toxiproxy.NewProxy(server, n.name+"-"+strings.TrimSuffix(port, "/tcp"), fmt.Sprintf("127.0.0.1:%d", n.listen[port]), upstream)
		if err := p.Start(); err != nil {
			return fmt.Errorf("network %s: listening for %s: %w", n.name, port, err)
		}
		w, err := p.Toxics.AddToxicJson(strings.NewReader(toxicJSON("gate", gateType, "upstream", nil)))
		if err != nil {
			return fmt.Errorf("network %s: installing the gate: %w", n.name, err)
		}
		w.Toxic.(*gateToxic).gate.Store(n.gate)
		n.links = append(n.links, &link{port: port, upstream: upstream, proxy: p})
	}
	resource.RegisterNetworkFaults(n.name, n.activeLines)
	n.initialized = true
	return nil
}

// filterPorts drops the ports a preset listed for tomato's own use: when
// listen is set, only its ports are the app's.
func filterPorts(ports []string, listen map[string]int) []string {
	if len(listen) == 0 {
		return ports
	}
	var out []string
	for _, p := range ports {
		if _, ok := listen[p]; ok {
			out = append(out, p)
		}
	}
	return out
}

func (n *Network) Ready(ctx context.Context) error { return nil }

// AppPorts points the app's {{.<container>.port.<port>}} at the proxies.
func (n *Network) AppPorts() map[string]map[string]int {
	n.mu.Lock()
	defer n.mu.Unlock()
	ports := map[string]int{}
	for _, l := range n.links {
		_, p, _ := net.SplitHostPort(l.proxy.Listen)
		port, _ := strconv.Atoi(p)
		ports[l.port] = port
	}
	return map[string]map[string]int{n.container: ports}
}

// Describe is the line printed at the start of a run.
func (n *Network) Describe() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	var out []string
	for _, l := range n.links {
		out = append(out, fmt.Sprintf("%s  app → %s → %s:%s  (seed %d)", n.name, l.proxy.Listen, n.container, strings.TrimSuffix(l.port, "/tcp"), n.seed))
	}
	return out
}

// Reset heals the network before every scenario and forgets its connections.
func (n *Network) Reset(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.healAll(ctx)
	if n.gate != nil {
		n.gate.reset()
	}
	n.changedAt = 0
	return nil
}

func (n *Network) Cleanup(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.healAll(ctx)
	for _, l := range n.links {
		l.proxy.Stop()
	}
	return nil
}

// healAll turns every fault off. The caller holds n.mu.
func (n *Network) healAll(ctx context.Context) {
	for _, key := range n.order {
		f := n.faults[key]
		if f.timer != nil {
			f.timer.Stop()
		}
		f.off()
	}
	n.faults = map[string]*fault{}
	n.order = nil
}

func (n *Network) RegisterSteps(ctx *godog.ScenarioContext) {
	ctx.Before(func(c context.Context, sc *godog.Scenario) (context.Context, error) {
		n.mu.Lock()
		n.scenario, n.step = sc.Name, 0
		n.mu.Unlock()
		return c, nil
	})
	ctx.StepContext().Before(func(c context.Context, st *godog.Step) (context.Context, error) {
		n.mu.Lock()
		n.step++
		n.mu.Unlock()
		return c, nil
	})
	resource.RegisterStepsToGodog(ctx, n.name, n.Steps())
}

// ── faults ──────────────────────────────────────────────────────────────────

// apply turns a fault on, replacing one with the same key, for d when d is
// set. on has already been called; off undoes it.
func (n *Network) apply(key string, desc func() string, d time.Duration, off func()) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if old, ok := n.faults[key]; ok {
		if old.timer != nil {
			old.timer.Stop()
		}
		n.removeKey(key)
	}
	f := &fault{desc: desc, step: n.step, off: off}
	if d > 0 {
		f.until = time.Now().Add(d)
		f.timer = time.AfterFunc(d, func() { n.healAfter(key, f, d) })
	}
	n.faults[key] = f
	n.order = append(n.order, key)
	n.changedAt, _ = n.gate.counts()
	detail := desc()
	if d > 0 {
		detail += " for " + d.String()
	}
	n.event("applied", detail)
}

// healAfter is a fault's timer running out.
func (n *Network) healAfter(key string, f *fault, d time.Duration) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.faults[key] != f {
		return
	}
	f.off()
	n.removeKey(key)
	n.changedAt, _ = n.gate.counts()
	n.event("healed", fmt.Sprintf("%s (its %s ran out)", f.desc(), d))
}

// heal turns every fault off, from a step.
func (n *Network) heal(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.order) == 0 {
		return nil
	}
	var healed []string
	for _, key := range n.order {
		healed = append(healed, n.faults[key].desc())
	}
	n.healAll(ctx)
	n.changedAt, _ = n.gate.counts()
	n.event("healed", strings.Join(healed, ", "))
	return nil
}

func (n *Network) removeKey(key string) {
	delete(n.faults, key)
	for i, k := range n.order {
		if k == key {
			n.order = append(n.order[:i], n.order[i+1:]...)
			break
		}
	}
}

// event records a network event on the running step. The caller holds n.mu.
func (n *Network) event(kind, detail string) {
	resource.RecordNetworkEvent(n.scenario, resource.NetworkEvent{
		At: time.Now(), Resource: n.name, Kind: kind, Detail: detail,
	})
}

func (n *Network) connectionFailed(num int, why string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.event("connection", fmt.Sprintf("failed connection %d: %s", num, why))
}

// activeLines is what a failing step's error says about this network.
func (n *Network) activeLines() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if len(n.order) == 0 {
		return nil
	}
	var out []string
	for _, key := range n.order {
		f := n.faults[key]
		when := fmt.Sprintf("step %d", f.step)
		if !f.until.IsZero() {
			left := time.Until(f.until).Round(100 * time.Millisecond)
			when += fmt.Sprintf(", %s left", left)
		}
		out = append(out, fmt.Sprintf("%s %s (%s)", n.name, f.desc(), when))
	}
	// What the faults did to connections, once there is something to say.
	if conns, failed := n.gate.counts(); conns > 0 {
		out = append(out, fmt.Sprintf("%s has seen %s this scenario, %d failed", n.name, plural(conns, "connection"), failed))
	}
	return out
}

// addToxic adds the same toxic to every proxy.
func (n *Network) addToxic(name, typ string, attrs map[string]any) error {
	for _, l := range n.links {
		if _, err := l.proxy.Toxics.AddToxicJson(strings.NewReader(toxicJSON(name, typ, "downstream", attrs))); err != nil {
			return fmt.Errorf("network %s: %w", n.name, err)
		}
	}
	return nil
}

func (n *Network) removeToxic(name string) func() {
	return func() {
		for _, l := range n.links {
			_ = l.proxy.Toxics.RemoveToxic(context.Background(), name)
		}
	}
}

// toxicJSON builds a toxic from typed values: Toxiproxy ignores an attribute
// it does not know, so a free-form one would be a fault that does nothing.
func toxicJSON(name, typ, stream string, attrs map[string]any) string {
	b, _ := json.Marshal(map[string]any{
		"name": name, "type": typ, "stream": stream, "toxicity": 1.0, "attributes": attrs,
	})
	return string(b)
}

func (n *Network) refuse() {
	for _, l := range n.links {
		l.proxy.Stop()
	}
}

func (n *Network) accept() {
	for _, l := range n.links {
		_ = l.proxy.Start() // already started is fine
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
