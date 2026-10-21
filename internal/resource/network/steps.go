package network

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tomatool/tomato/internal/resource"
)

// forDuration is the optional ending every fault step takes.
const forDuration = `(?: for "([^"]*)")?$`

func (n *Network) Steps() resource.StepCategory {
	return resource.StepCategory{
		Name:        "Network",
		Description: "Steps for making the network between the app and a dependency slow, unreliable or down. Every fault lasts until the scenario ends, or for the duration given",
		Steps: []resource.StepDef{
			{
				Group:       "Faults",
				Pattern:     `^"{resource}" adds latency "([^"]*)"(?: with jitter "([^"]*)")?` + forDuration,
				Description: "Delay every response, as a slow or distant dependency does; with jitter, by a random amount either side",
				Example:     `"{resource}" adds latency "300ms" with jitter "100ms" for "10s"`,
				Handler:     n.addLatency,
			},
			{
				Group:       "Faults",
				Pattern:     `^"{resource}" limits bandwidth to "([^"]*)"` + forDuration,
				Description: "Throttle responses to a rate in KB/s or MB/s, as a congested link does",
				Example:     `"{resource}" limits bandwidth to "64KB/s"`,
				Handler:     n.limitBandwidth,
			},
			{
				Group:       "Faults",
				Pattern:     `^"{resource}" stops responding` + forDuration,
				Description: "Accept connections and requests but answer nothing, as a hung dependency or a network partition does. The app waits until its own timeout",
				Example:     `"{resource}" stops responding for "5s"`,
				Handler:     n.stopResponding,
			},
			{
				Group:       "Faults",
				Pattern:     `^"{resource}" resets connections` + forDuration,
				Description: "Reset every connection (TCP RST) as soon as it carries data, as a crash or a failover does",
				Example:     `"{resource}" resets connections for "3s"`,
				Handler:     n.resetConnections,
			},
			{
				Group:       "Faults",
				Pattern:     `^"{resource}" refuses connections` + forDuration,
				Description: "Close the open connections and refuse new ones, as when the dependency is down. The container keeps running and keeps its data",
				Example:     `"{resource}" refuses connections for "5s"`,
				Handler:     n.refuseConnections,
			},
			{
				Group:       "Faults",
				Pattern:     `^"{resource}" flaps every "([^"]*)"` + forDuration,
				Description: "Refuse connections and accept them again in turn, each for the period given, as a dependency restarting in a loop does",
				Example:     `"{resource}" flaps every "2s" for "10s"`,
				Handler:     n.flap,
			},
			{
				Group:       "Faults",
				Pattern:     `^"{resource}" fails the next "(\d+)" connections?$`,
				Description: "Close the next new connections as soon as they open, then let them through, as a dependency that is coming back up does. Open connections are untouched",
				Example:     `"{resource}" fails the next "2" connections`,
				Handler:     n.failNext,
			},
			{
				Group:       "Faults",
				Pattern:     `^"{resource}" fails "(\d+)%" of new connections` + forDuration,
				Description: "Close a share of new connections as soon as they open. Which ones is decided by the run's seed, so a failing run can be repeated",
				Example:     `"{resource}" fails "30%" of new connections`,
				Handler:     n.failShare,
			},
			{
				Group:       "Faults",
				Pattern:     `^"{resource}" loses "(\d+)%" of the data` + forDuration,
				Description: "Drop a share of the data the dependency sends, chunks of the stream rather than IP packets, so the app sees stalls and a garbled protocol",
				Example:     `"{resource}" loses "30%" of the data`,
				Handler:     n.loseData,
			},
			{
				Group:       "Faults",
				Pattern:     `^"{resource}" is healthy$`,
				Description: "Turn every fault off",
				Example:     `"{resource}" is healthy`,
				Handler:     n.heal,
			},
			{
				Group:       "Connections",
				Pattern:     `^"{resource}" has seen "(\d+)" connections?$`,
				Description: "Assert how many connections the app opened through the network since the scenario started, failed ones included: a retry or a reconnect is a new one",
				Example:     `"{resource}" has seen "2" connections`,
				Handler:     n.hasSeen,
			},
			{
				Group:       "Connections",
				Pattern:     `^"{resource}" sees a new connection within "([^"]*)"$`,
				Description: "Wait for the app to open a connection after the network last changed (a fault applied or healed), as an app that reconnects does",
				Example:     `"{resource}" sees a new connection within "10s"`,
				Handler:     n.seesNewConnection,
			},
		},
	}
}

func optionalDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return parseDuration(s)
}

func parseDuration(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%q is not a duration, like 300ms or 5s", s)
	}
	return d, nil
}

func (n *Network) addLatency(ctx context.Context, latency, jitter, dur string) error {
	l, err := parseDuration(latency)
	if err != nil {
		return err
	}
	var j time.Duration
	if jitter != "" {
		if j, err = parseDuration(jitter); err != nil {
			return err
		}
	}
	d, err := optionalDuration(dur)
	if err != nil {
		return err
	}
	n.removeToxic("latency")()
	if err := n.addToxic("latency", "latency", map[string]any{"latency": l.Milliseconds(), "jitter": j.Milliseconds()}); err != nil {
		return err
	}
	desc := "adds latency " + l.String()
	if j > 0 {
		desc += " ± " + j.String()
	}
	n.apply("latency", func() string { return desc }, d, n.removeToxic("latency"))
	return nil
}

var rateRe = regexp.MustCompile(`^(\d+)\s*([KkMm])B/s$`)

func (n *Network) limitBandwidth(ctx context.Context, rate, dur string) error {
	m := rateRe.FindStringSubmatch(strings.TrimSpace(rate))
	if m == nil {
		return fmt.Errorf("%q is not a rate, like 64KB/s or 2MB/s", rate)
	}
	kbs, _ := strconv.Atoi(m[1])
	if strings.EqualFold(m[2], "M") {
		kbs *= 1024
	}
	d, err := optionalDuration(dur)
	if err != nil {
		return err
	}
	n.removeToxic("bandwidth")()
	if err := n.addToxic("bandwidth", "bandwidth", map[string]any{"rate": kbs}); err != nil {
		return err
	}
	n.apply("bandwidth", func() string { return "limits bandwidth to " + rate }, d, n.removeToxic("bandwidth"))
	return nil
}

func (n *Network) stopResponding(ctx context.Context, dur string) error {
	return n.toxicFault("hang", "timeout", map[string]any{"timeout": 0}, "stops responding", dur)
}

func (n *Network) resetConnections(ctx context.Context, dur string) error {
	return n.toxicFault("reset", "reset_peer", map[string]any{"timeout": 0}, "resets connections", dur)
}

func (n *Network) loseData(ctx context.Context, percent, dur string) error {
	p, _ := strconv.Atoi(percent)
	if p < 1 || p > 100 {
		return fmt.Errorf("%s%% is not a share between 1%% and 100%%", percent)
	}
	return n.toxicFault("loss", "packet_loss", map[string]any{"loss_rate": float64(p) / 100}, "loses "+percent+"% of the data", dur)
}

func (n *Network) toxicFault(name, typ string, attrs map[string]any, desc, dur string) error {
	d, err := optionalDuration(dur)
	if err != nil {
		return err
	}
	n.removeToxic(name)()
	if err := n.addToxic(name, typ, attrs); err != nil {
		return err
	}
	n.apply(name, func() string { return desc }, d, n.removeToxic(name))
	return nil
}

func (n *Network) refuseConnections(ctx context.Context, dur string) error {
	d, err := optionalDuration(dur)
	if err != nil {
		return err
	}
	n.refuse()
	n.apply("refuse", func() string { return "refuses connections" }, d, n.accept)
	return nil
}

func (n *Network) flap(ctx context.Context, period, dur string) error {
	p, err := parseDuration(period)
	if err != nil {
		return err
	}
	d, err := optionalDuration(dur)
	if err != nil {
		return err
	}
	stop := make(chan struct{})
	go func() {
		down := true
		for {
			if down {
				n.refuse()
			} else {
				n.accept()
			}
			select {
			case <-stop:
				return
			case <-time.After(p):
				down = !down
			}
		}
	}()
	n.apply("flap", func() string { return "flaps every " + p.String() }, d, func() {
		close(stop)
		n.accept()
	})
	return nil
}

func (n *Network) failNext(ctx context.Context, count string) error {
	c, _ := strconv.Atoi(count)
	n.gate.mu.Lock()
	n.gate.failNext += c
	n.gate.mu.Unlock()
	left := func() string {
		n.gate.mu.Lock()
		defer n.gate.mu.Unlock()
		return fmt.Sprintf("fails the next %d connections (%d left)", c, n.gate.failNext)
	}
	n.apply("next", left, 0, func() {
		n.gate.mu.Lock()
		n.gate.failNext = 0
		n.gate.mu.Unlock()
	})
	return nil
}

func (n *Network) failShare(ctx context.Context, percent, dur string) error {
	p, _ := strconv.Atoi(percent)
	if p < 1 || p > 100 {
		return fmt.Errorf("%s%% is not a share between 1%% and 100%%", percent)
	}
	d, err := optionalDuration(dur)
	if err != nil {
		return err
	}
	n.gate.mu.Lock()
	n.gate.rate = float64(p) / 100
	n.gate.mu.Unlock()
	n.apply("share", func() string { return "fails " + percent + "% of new connections" }, d, func() {
		n.gate.mu.Lock()
		n.gate.rate = 0
		n.gate.mu.Unlock()
	})
	return nil
}

func (n *Network) hasSeen(ctx context.Context, count string) error {
	want, _ := strconv.Atoi(count)
	conns, failed := n.gate.counts()
	if conns != want {
		return fmt.Errorf("%s: expected %s this scenario, the app opened %d (%d failed)", n.name, plural(want, "connection"), conns, failed)
	}
	return nil
}

func (n *Network) seesNewConnection(ctx context.Context, within string) error {
	d, err := parseDuration(within)
	if err != nil {
		return err
	}
	deadline := time.After(d)
	for {
		n.mu.Lock()
		since := n.changedAt
		n.mu.Unlock()
		n.gate.mu.Lock()
		conns, opened := n.gate.conns, n.gate.opened
		n.gate.mu.Unlock()
		if conns > since {
			return nil
		}
		select {
		case <-opened:
		case <-deadline:
			return fmt.Errorf("%s: the app opened no new connection within %s of the network last changing (%d this scenario)", n.name, d, conns)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
