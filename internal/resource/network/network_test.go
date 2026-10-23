package network

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/resource"
)

// echoServer stands in for a dependency container: it answers every line
// with the same line.
func echoServer(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}()
		}
	}()
	return l.Addr().String()
}

type fakeContainers struct{ addr string }

func (f fakeContainers) GetHost(context.Context, string) (string, error) {
	h, _, _ := net.SplitHostPort(f.addr)
	return h, nil
}
func (f fakeContainers) GetPort(context.Context, string, string) (string, error) {
	_, p, _ := net.SplitHostPort(f.addr)
	return p, nil
}
func (f fakeContainers) ExposedPorts(string) []string { return []string{"5432/tcp"} }

func newNetwork(t *testing.T) *Network {
	t.Helper()
	n, err := New("db-network", config.Resource{Type: "network", Container: "db", Options: map[string]any{"seed": 42}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	n.cm = fakeContainers{addr: echoServer(t)}
	if err := n.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	n.scenario = t.Name()
	t.Cleanup(func() { _ = n.Cleanup(context.Background()) })
	return n
}

func (n *Network) appAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", n.AppPorts()["db"]["5432/tcp"])
}

// roundTrip opens a connection through the network, sends a line and reads it back.
func roundTrip(addr string, timeout time.Duration) (time.Duration, error) {
	start := time.Now()
	c, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return 0, err
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(timeout))
	if _, err := c.Write([]byte("ping\n")); err != nil {
		return 0, err
	}
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		return 0, err
	}
	if line != "ping\n" {
		return 0, fmt.Errorf("got %q", line)
	}
	return time.Since(start), nil
}

func TestHealthyNetworkIsTransparent(t *testing.T) {
	n := newNetwork(t)
	if _, err := roundTrip(n.appAddr(), time.Second); err != nil {
		t.Fatal(err)
	}
	if lines := n.activeLines(); lines != nil {
		t.Errorf("a healthy network reports %v", lines)
	}
}

func TestLatency(t *testing.T) {
	n := newNetwork(t)
	if err := n.addLatency(context.Background(), "200ms", "", ""); err != nil {
		t.Fatal(err)
	}
	took, err := roundTrip(n.appAddr(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if took < 200*time.Millisecond {
		t.Errorf("round trip took %s with 200ms of latency", took)
	}
}

func TestRefuseAndHeal(t *testing.T) {
	n := newNetwork(t)
	if err := n.refuseConnections(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := roundTrip(n.appAddr(), time.Second); err == nil {
		t.Fatal("a refusing network let a connection through")
	}
	if err := n.heal(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := roundTrip(n.appAddr(), time.Second); err != nil {
		t.Fatalf("healed network, same port: %v", err)
	}
}

func TestStopsResponding(t *testing.T) {
	n := newNetwork(t)
	if err := n.stopResponding(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	_, err := roundTrip(n.appAddr(), 300*time.Millisecond)
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("want a timeout from a hung network, got %v", err)
	}
}

func TestFailNextConnections(t *testing.T) {
	n := newNetwork(t)
	if err := n.failNext(context.Background(), "2"); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		if _, err := roundTrip(n.appAddr(), time.Second); err == nil {
			t.Fatalf("connection %d went through", i)
		}
	}
	if _, err := roundTrip(n.appAddr(), time.Second); err != nil {
		t.Fatalf("connection 3: %v", err)
	}
	if err := n.hasSeen(context.Background(), "3"); err != nil {
		t.Error(err)
	}
	events := resource.TakeNetworkEvents(t.Name())
	var failed int
	for _, e := range events {
		if e.Kind == "connection" {
			failed++
		}
	}
	if failed != 2 {
		t.Errorf("want 2 connection events, got %+v", events)
	}
}

func TestShareIsSeeded(t *testing.T) {
	pattern := func() string {
		g := newGate(42)
		g.rate = 0.3
		var b strings.Builder
		for i := 0; i < 30; i++ {
			if g.admit() {
				b.WriteByte('x')
			} else {
				b.WriteByte('.')
			}
		}
		return b.String()
	}
	if a, b := pattern(), pattern(); a != b {
		t.Errorf("the same seed failed different connections:\n%s\n%s", a, b)
	}
}

func TestFaultHealsAfterItsDuration(t *testing.T) {
	n := newNetwork(t)
	if err := n.refuseConnections(context.Background(), "200ms"); err != nil {
		t.Fatal(err)
	}
	if lines := n.activeLines(); len(lines) == 0 || !strings.Contains(lines[0], "refuses connections (step 0, ") {
		t.Errorf("active: %v", lines)
	}
	time.Sleep(400 * time.Millisecond)
	if _, err := roundTrip(n.appAddr(), time.Second); err != nil {
		t.Fatalf("after its duration: %v", err)
	}
	var kinds []string
	for _, e := range resource.TakeNetworkEvents(t.Name()) {
		kinds = append(kinds, e.Kind)
	}
	if strings.Join(kinds, ",") != "applied,healed" {
		t.Errorf("events: %v", kinds)
	}
}

func TestSeesNewConnection(t *testing.T) {
	n := newNetwork(t)
	if err := n.resetConnections(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if err := n.heal(context.Background()); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		_, _ = roundTrip(n.appAddr(), time.Second)
	}()
	if err := n.seesNewConnection(context.Background(), "2s"); err != nil {
		t.Fatal(err)
	}
	// A new change: the connection before it no longer counts.
	if err := n.addLatency(context.Background(), "10ms", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := n.seesNewConnection(context.Background(), "200ms"); err == nil {
		t.Error("no connection since the last change, yet it saw one")
	}
}

func TestResetHealsAndForgets(t *testing.T) {
	n := newNetwork(t)
	_ = n.refuseConnections(context.Background(), "")
	_ = n.failNext(context.Background(), "5")
	if err := n.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := roundTrip(n.appAddr(), time.Second); err != nil {
		t.Fatalf("after Reset: %v", err)
	}
	if err := n.hasSeen(context.Background(), "1"); err != nil {
		t.Error(err)
	}
	if lines := n.activeLines(); lines != nil {
		t.Errorf("after Reset: %v", lines)
	}
}

func TestFailingStepNamesTheFaults(t *testing.T) {
	n := newNetwork(t)
	_ = n.addLatency(context.Background(), "300ms", "", "")
	err := resource.WithNetworkFaults(errors.New("expected status 201, got 502"))
	want := "expected status 201, got 502\nwhile the network was faulty:\n  db-network adds latency 300ms (step 0)"
	if !strings.HasPrefix(err.Error(), want) {
		t.Errorf("got:\n%s", err)
	}
	_ = n.heal(context.Background())
	if err := resource.WithNetworkFaults(errors.New("x")); err.Error() != "x" {
		t.Errorf("healthy network changed the error: %q", err)
	}
}

func TestValidateOptions(t *testing.T) {
	for _, opts := range []map[string]any{
		{"ports": []any{"abc"}},
		{"ports": "5432"},
		{"seed": "x"},
	} {
		if err := ValidateOptions(opts); err == nil {
			t.Errorf("%v: want an error", opts)
		}
	}
	if err := ValidateOptions(map[string]any{"ports": []any{5432, "6379/tcp"}, "seed": 1}); err != nil {
		t.Error(err)
	}
}

// slowEcho answers every line after 100ms, as a dependency doing work does.
func slowEcho(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					time.Sleep(100 * time.Millisecond)
					_, _ = c.Write([]byte(line))
				}
			}()
		}
	}()
	return l.Addr().String()
}

func TestMeterCountsTrafficRoundTripsAndWait(t *testing.T) {
	n, err := New("db-network", config.Resource{Type: "network", Container: "db"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	n.cm = fakeContainers{addr: slowEcho(t)}
	if err := n.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = n.Cleanup(context.Background()) })

	c, err := net.Dial("tcp", n.appAddr())
	if err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(c)
	for i := 0; i < 3; i++ {
		if _, err := c.Write([]byte("hello\n")); err != nil {
			t.Fatal(err)
		}
		if _, err := r.ReadString('\n'); err != nil {
			t.Fatal(err)
		}
	}
	got := n.MeterDependency()
	if got.Container != "db" || got.Connections != 1 || got.Open != 1 {
		t.Errorf("connections: %+v", got)
	}
	if got.Sent != 18 || got.Received != 18 {
		t.Errorf("bytes: sent %d received %d, want 18 each", got.Sent, got.Received)
	}
	if got.RoundTrips != 3 || got.WaitMs < 300 || got.WaitMs > 600 {
		t.Errorf("round trips %d, wait %dms; want 3 and about 300ms", got.RoundTrips, got.WaitMs)
	}

	c.Close()
	deadline := time.Now().Add(2 * time.Second)
	for n.MeterDependency().Open != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if open := n.MeterDependency().Open; open != 0 {
		t.Errorf("a closed connection still counts as open: %d", open)
	}

	// Latency a scenario adds is the network's, not the dependency's.
	if err := n.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = n.addLatency(context.Background(), "200ms", "", "")
	if _, err := roundTrip(n.appAddr(), 2*time.Second); err != nil {
		t.Fatal(err)
	}
	if got := n.MeterDependency(); got.WaitMs >= 200 || got.RoundTrips != 1 {
		t.Errorf("with 200ms of added latency: %+v; want the dependency's own ~100ms", got)
	}
}
