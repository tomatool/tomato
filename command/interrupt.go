package command

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// interruptExitCode is what a shell reports for a process ended by Ctrl+C.
const interruptExitCode = 130

// interrupts stops what a run started when the run is interrupted. Without it
// Ctrl+C, or the UI's Stop, ended tomato before its defers ran: the containers
// were reaped by Docker, but the app runs in its own process group, so nothing
// reached it, and it held the app port and failed the next run.
//
// Scenarios do not stop on a cancelled context, so the run cannot simply
// return: the cleanup registered with onInterrupt runs here, last registered
// first like defers, and tomato exits. A second signal exits at once.
type interrupts struct {
	mu      sync.Mutex
	cleanup []func()
	// waiting is set while the run waits for Ctrl+C itself (keep-alive), which
	// then shuts down on its own path.
	waiting chan struct{}
	signals chan os.Signal
	done    chan struct{}
	log     io.Writer
	exit    func(code int)
}

// handleInterrupts catches SIGINT and SIGTERM until stop is called. What it
// says on an interrupt goes to stderr and to log.
func handleInterrupts(log io.Writer) *interrupts {
	in := &interrupts{
		signals: make(chan os.Signal, 1),
		done:    make(chan struct{}),
		log:     log,
		exit:    os.Exit,
	}
	signal.Notify(in.signals, os.Interrupt, syscall.SIGTERM)
	go in.loop()
	return in
}

func (in *interrupts) loop() {
	var sig os.Signal
	select {
	case sig = <-in.signals:
	case <-in.done:
		return
	}

	in.mu.Lock()
	waiting := in.waiting
	cleanup := in.cleanup
	in.cleanup = nil
	in.mu.Unlock()

	if waiting == nil {
		msg := fmt.Sprintf("\n%s: stopping the app and containers (again to quit now)...\n", sig)
		fmt.Fprint(os.Stderr, msg)
		if in.log != nil {
			fmt.Fprint(in.log, msg)
		}
	}

	// A second signal while shutting down quits without waiting for it.
	go func() {
		select {
		case <-in.signals:
			in.exit(interruptExitCode)
		case <-in.done:
		}
	}()

	if waiting != nil {
		close(waiting)
		return
	}
	for i := len(cleanup) - 1; i >= 0; i-- {
		cleanup[i]()
	}
	in.exit(interruptExitCode)
}

// onInterrupt registers fn to run if the run is interrupted.
func (in *interrupts) onInterrupt(fn func()) {
	in.mu.Lock()
	defer in.mu.Unlock()
	in.cleanup = append(in.cleanup, fn)
}

// wait blocks until the run is interrupted, leaving the shutdown to the caller.
func (in *interrupts) wait() {
	in.mu.Lock()
	in.waiting = make(chan struct{})
	waiting := in.waiting
	in.mu.Unlock()
	<-waiting
}

// stop stops catching signals.
func (in *interrupts) stop() {
	signal.Stop(in.signals)
	close(in.done)
}
