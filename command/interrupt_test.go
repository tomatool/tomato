package command

import (
	"bytes"
	"os"
	"reflect"
	"sync"
	"syscall"
	"testing"
	"time"
)

// exitRecorder stands in for os.Exit.
type exitRecorder struct {
	once sync.Once
	code chan int
}

func newExitRecorder() *exitRecorder { return &exitRecorder{code: make(chan int, 1)} }

func (e *exitRecorder) exit(code int) { e.once.Do(func() { e.code <- code }) }

func (e *exitRecorder) wait(t *testing.T) int {
	t.Helper()
	select {
	case code := <-e.code:
		return code
	case <-time.After(10 * time.Second):
		t.Fatal("tomato did not exit after the interrupt")
		return 0
	}
}

func TestInterrupts_CleansUpInReverseAndExits(t *testing.T) {
	var log bytes.Buffer
	in := handleInterrupts(&log)
	defer in.stop()
	exit := newExitRecorder()
	in.exit = exit.exit

	var mu sync.Mutex
	var order []string
	for _, name := range []string{"containers", "resources", "app"} {
		in.onInterrupt(func() { mu.Lock(); order = append(order, name); mu.Unlock() })
	}

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := exit.wait(t); code != interruptExitCode {
		t.Errorf("exit code %d, want %d", code, interruptExitCode)
	}
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"app", "resources", "containers"}; !reflect.DeepEqual(order, want) {
		t.Errorf("cleaned up %v, want %v", order, want)
	}
	if !bytes.Contains(log.Bytes(), []byte("stopping the app")) {
		t.Errorf("the run log should say why the run ended, got %q", log.String())
	}
}

// Keep-alive waits for Ctrl+C itself and shuts down on its own path.
func TestInterrupts_WaitReturnsWithoutExiting(t *testing.T) {
	in := handleInterrupts(nil)
	defer in.stop()
	exit := newExitRecorder()
	in.exit = exit.exit
	cleaned := false
	in.onInterrupt(func() { cleaned = true })

	returned := make(chan struct{})
	go func() { in.wait(); close(returned) }()
	// Let wait register before the signal arrives.
	for {
		in.mu.Lock()
		ready := in.waiting != nil
		in.mu.Unlock()
		if ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case <-returned:
	case <-time.After(10 * time.Second):
		t.Fatal("wait did not return on the interrupt")
	}
	select {
	case code := <-exit.code:
		t.Errorf("exited with %d; keep-alive shuts down itself", code)
	default:
	}
	if cleaned {
		t.Error("the registered cleanup ran; keep-alive runs its own")
	}
}
