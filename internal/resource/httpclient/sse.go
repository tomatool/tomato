package httpclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/jsonmatch"
	"github.com/tomatool/tomato/internal/resource"
)

// Server-Sent Events: a GET whose response never ends, carrying events as
// `id:`, `event:` and `data:` lines (https://html.spec.whatwg.org/multipage/server-sent-events.html).
//
// A stream is read in the background into a list of events and a list of
// comments. Receive steps consume them in order, so an event that arrived
// before the step started waiting still counts, and an unexpected event fails
// the step that meets it instead of being skipped over. There is no automatic
// reconnect: a test resumes explicitly, with "opens stream ... after event id",
// so a broken resume can't hide behind a client that quietly retries.

// sseEvent is one dispatched event.
type sseEvent struct {
	ID    string
	HasID bool
	Type  string
	Data  string
}

// describe renders an event for failure messages.
func (e sseEvent) describe() string {
	id := "no id"
	if e.HasID {
		id = "id " + e.ID
	}
	return fmt.Sprintf("%s (%s): %s", e.Type, id, e.Data)
}

// eventStream is one open stream and everything read from it.
type eventStream struct {
	path   string
	cancel context.CancelFunc
	done   chan struct{}
	gate   *readGate

	mu       sync.Mutex
	events   []sseEvent
	comments []string
	// consumed and commentsConsumed are how many events and comments receive
	// steps have already matched.
	consumed         int
	commentsConsumed int
	// closedByServer is set when the body ended without the test closing it.
	closedByServer bool
	closedByClient bool
	readErr        error
}

// newStreamClient returns the client streams are opened with. A stream must
// outlive the `timeout` option, which bounds a whole request including its
// body, so here it bounds only the wait for response headers. How long a step
// waits for an event is the step's own `within`.
func newStreamClient(timeout time.Duration, checkRedirect func(*http.Request, []*http.Request) error) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = timeout
	return &http.Client{Transport: transport, CheckRedirect: checkRedirect}
}

func (r *HTTPClient) streamSteps() []resource.StepDef {
	return []resource.StepDef{
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" opens stream "([^"]*)"$`,
			Description: "Open a Server-Sent Events stream with the scenario's headers, cookies and query params, and fail unless it answers 200 with text/event-stream",
			Example:     `"api" opens stream "/events"`,
			Handler:     r.openStream,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" opens stream "([^"]*)" after event id "([^"]*)"$`,
			Description: "Resume a stream: open it with a Last-Event-ID header, as a reconnecting client does",
			Example:     `"api" opens stream "/events" after event id "{{last_id}}"`,
			Handler:     r.openStreamAfter,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" closes stream$`,
			Description: "Close the stream from the client side",
			Example:     `"api" closes stream`,
			Handler:     r.closeStreamStep,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" stream skips events "([^"]*)"$`,
			Description: "Leave events of these comma-separated types out of every receive and assertion for the rest of the scenario, e.g. heartbeats sent as events",
			Example:     `"api" stream skips events "heartbeat"`,
			Handler:     r.skipEventTypes,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" receives event "([^"]*)" within "([^"]*)"$`,
			Description: "Assert the next event has this type, waiting for it up to the duration",
			Example:     `"api" receives event "order.created" within "5s"`,
			Handler:     r.receiveEvent,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" receives event "([^"]*)" within "([^"]*)" with json containing:$`,
			Description: "Assert the next event has this type and its data contains the JSON fields given (extra fields ignored, same matchers as \"response json contains\")",
			Example:     `"api" receives event "order.created" within "5s" with json containing:`,
			Handler:     r.receiveEventWithJSON,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" receives events within "([^"]*)":$`,
			Description: "Assert the next events, in order, from a table with an event column and optional id and data columns",
			Example:     `"api" receives events within "5s":`,
			Handler:     r.receiveEvents,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" does not receive an event within "([^"]*)"$`,
			Description: "Assert no further event arrives for the duration (comments don't count)",
			Example:     `"api" does not receive an event within "2s"`,
			Handler:     r.noEventWithin,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" receives a comment within "([^"]*)"$`,
			Description: "Assert a comment line (\": ...\"), the usual keep-alive, arrives within the duration",
			Example:     `"api" receives a comment within "2s"`,
			Handler:     r.receiveComment,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" stream event ids are consecutive from "(\d+)"$`,
			Description: "Assert the ids of every event received on this stream so far are integers counting up by one from the number given, with no gap or repeat. Events without an id are left out",
			Example:     `"api" stream event ids are consecutive from "1"`,
			Handler:     r.eventIDsConsecutive,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" stops reading the stream$`,
			Description: "Stop reading the stream without closing it, as a stalled client does: what the server sends piles up in the connection's buffers until the server blocks, and a server that protects itself disconnects the slow consumer",
			Example:     `"api" stops reading the stream`,
			Handler:     r.pauseStream,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" resumes reading the stream$`,
			Description: "Read the stream again: what arrived while paused is delivered in order, then whatever the server did meanwhile (sent more, or closed the stream)",
			Example:     `"api" resumes reading the stream`,
			Handler:     r.resumeStream,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" stream is closed by the server within "([^"]*)"$`,
			Description: "Assert the server ends the stream within the duration",
			Example:     `"api" stream is closed by the server within "5s"`,
			Handler:     r.closedByServerWithin,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" stream is disconnected by the server within "([^"]*)"$`,
			Description: "Assert the server ends the stream within the duration, cleanly or by dropping the connection, as a server that disconnects a slow consumer does",
			Example:     `"api" stream is disconnected by the server within "5s"`,
			Handler:     r.disconnectedByServerWithin,
		},
		{
			Group:       "Event Streams",
			Pattern:     `^"{resource}" last stream event id saved as "\{\{([^}]+)\}\}"$`,
			Description: "Save the id of the last event a receive step matched, the position a client would resume from",
			Example:     `"api" last stream event id saved as "{{last_id}}"`,
			Handler:     r.saveLastEventID,
		},
	}
}

func (r *HTTPClient) openStream(path string) error {
	return r.startStream(path, "")
}

func (r *HTTPClient) openStreamAfter(path, lastEventID string) error {
	return r.startStream(path, resource.ReplaceVariables(lastEventID))
}

func (r *HTTPClient) startStream(path, lastEventID string) error {
	// Opening again is a reconnect: the old stream goes first.
	r.closeStream()

	ctx, cancel := context.WithCancel(context.Background())
	req, err := r.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		cancel()
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if lastEventID != "" {
		req.Header.Set("Last-Event-ID", lastEventID)
	}
	// Query params are single-use, as for a request.
	r.requestParams = make(url.Values)

	resp, err := r.streamClient.Do(req)
	if err != nil {
		cancel()
		return fmt.Errorf("opening stream %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		cancel()
		return fmt.Errorf("opening stream %s: expected status 200, got %d\nBody: %s", path, resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		resp.Body.Close()
		cancel()
		return fmt.Errorf("opening stream %s: expected Content-Type text/event-stream, got %q", path, ct)
	}

	s := &eventStream{path: path, cancel: cancel, done: make(chan struct{}), gate: newReadGate()}
	r.stream = s
	go s.read(resp.Body)
	return nil
}

// read parses the body until it ends or the stream is closed.
func (s *eventStream) read(body io.ReadCloser) {
	defer close(s.done)
	defer body.Close()

	err := parseEventStream(s.gate.wrap(body), func(e sseEvent) {
		s.mu.Lock()
		s.events = append(s.events, e)
		s.mu.Unlock()
	}, func(c string) {
		s.mu.Lock()
		s.comments = append(s.comments, c)
		s.mu.Unlock()
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closedByClient {
		s.closedByServer = true
		if err != nil && !errors.Is(err, io.EOF) {
			s.readErr = err
		}
	}
}

// close ends the stream from the client side and waits for the reader to stop.
func (s *eventStream) close() {
	s.mu.Lock()
	if !s.closedByServer {
		s.closedByClient = true
	}
	s.mu.Unlock()
	s.cancel()
	// A paused reader is blocked before its next read; let it see the cancel.
	s.gate.open()
	<-s.done
}

func (r *HTTPClient) closeStream() {
	if r.stream != nil {
		r.stream.close()
		r.stream = nil
	}
}

func (r *HTTPClient) closeStreamStep() error {
	if r.stream == nil {
		return fmt.Errorf("no stream is open")
	}
	r.closeStream()
	return nil
}

func (r *HTTPClient) pauseStream() error {
	s, err := r.openedStream()
	if err != nil {
		return err
	}
	if !s.gate.close() {
		return fmt.Errorf("the stream is already paused")
	}
	return nil
}

func (r *HTTPClient) resumeStream() error {
	s, err := r.openedStream()
	if err != nil {
		return err
	}
	if !s.gate.open() {
		return fmt.Errorf("the stream is not paused")
	}
	return nil
}

// readGate blocks reads while closed. Nothing is read from the connection
// then, so the kernel's buffers fill and the server's writes block: the
// server sees what a stalled client looks like on the wire.
type readGate struct {
	mu     sync.Mutex
	cond   *sync.Cond
	paused bool
}

func newReadGate() *readGate {
	g := &readGate{}
	g.cond = sync.NewCond(&g.mu)
	return g
}

// close pauses reading; it reports false if reading was already paused.
func (g *readGate) close() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.paused {
		return false
	}
	g.paused = true
	return true
}

// open resumes reading; it reports false if reading was not paused.
func (g *readGate) open() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.paused {
		return false
	}
	g.paused = false
	g.cond.Broadcast()
	return true
}

func (g *readGate) wrap(r io.Reader) io.Reader { return gatedReader{g, r} }

type gatedReader struct {
	g *readGate
	r io.Reader
}

func (gr gatedReader) Read(p []byte) (int, error) {
	gr.g.mu.Lock()
	for gr.g.paused {
		gr.g.cond.Wait()
	}
	gr.g.mu.Unlock()
	return gr.r.Read(p)
}

func (r *HTTPClient) skipEventTypes(types string) error {
	for _, t := range strings.Split(types, ",") {
		if t = strings.TrimSpace(t); t != "" {
			r.skipEvents[t] = true
		}
	}
	return nil
}

func (r *HTTPClient) openedStream() (*eventStream, error) {
	if r.stream == nil {
		return nil, fmt.Errorf("no stream is open: use \"%s\" opens stream first", r.name)
	}
	return r.stream, nil
}

// nextEvent waits up to timeout for the next event no receive step has
// matched, skipping the scenario's skipped types, and consumes it.
func (r *HTTPClient) nextEvent(timeout time.Duration) (sseEvent, error) {
	s, err := r.openedStream()
	if err != nil {
		return sseEvent{}, err
	}
	var next sseEvent
	found := resource.Eventually(timeout, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		for s.consumed < len(s.events) {
			e := s.events[s.consumed]
			s.consumed++
			if !r.skipEvents[e.Type] {
				next = e
				return true
			}
		}
		return false
	})
	if found {
		return next, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closedByServer {
		if s.readErr != nil {
			return sseEvent{}, fmt.Errorf("no event within %s: the stream failed: %v", timeout, s.readErr)
		}
		return sseEvent{}, fmt.Errorf("no event within %s: the server closed the stream", timeout)
	}
	return sseEvent{}, fmt.Errorf("no event within %s", timeout)
}

func (r *HTTPClient) receiveEvent(eventType, within string) error {
	timeout, err := time.ParseDuration(within)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	e, err := r.nextEvent(timeout)
	if err != nil {
		return fmt.Errorf("expected event %q: %w", eventType, err)
	}
	if e.Type != eventType {
		return fmt.Errorf("expected event %q, got %s", eventType, e.describe())
	}
	return nil
}

func (r *HTTPClient) receiveEventWithJSON(eventType, within string, doc *godog.DocString) error {
	timeout, err := time.ParseDuration(within)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	e, err := r.nextEvent(timeout)
	if err != nil {
		return fmt.Errorf("expected event %q: %w", eventType, err)
	}
	if e.Type != eventType {
		return fmt.Errorf("expected event %q, got %s", eventType, e.describe())
	}
	return matchEventJSON(e, resource.ReplaceVariables(doc.Content))
}

func matchEventJSON(e sseEvent, expectedJSON string) error {
	var expected, actual interface{}
	if err := json.Unmarshal([]byte(expectedJSON), &expected); err != nil {
		return fmt.Errorf("invalid expected JSON: %w", err)
	}
	if err := json.Unmarshal([]byte(e.Data), &actual); err != nil {
		return fmt.Errorf("event %s: data is not JSON: %w", e.describe(), err)
	}
	if err := jsonmatch.Compare(expected, actual, "", true); err != nil {
		return fmt.Errorf("event %s: %w", e.describe(), err)
	}
	return nil
}

func (r *HTTPClient) receiveEvents(within string, table *godog.Table) error {
	timeout, err := time.ParseDuration(within)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	if len(table.Rows) < 2 {
		return fmt.Errorf("the table needs a header row and at least one event")
	}
	cols := map[string]int{}
	for i, c := range table.Rows[0].Cells {
		cols[strings.TrimSpace(c.Value)] = i
	}
	if _, ok := cols["event"]; !ok {
		return fmt.Errorf("the table needs an event column")
	}
	for name := range cols {
		if name != "event" && name != "id" && name != "data" {
			return fmt.Errorf("unknown column %q: use event, id and data", name)
		}
	}

	// One deadline for the whole table: "within" is how long the events
	// together may take to arrive.
	deadline := time.Now().Add(timeout)
	for i, row := range table.Rows[1:] {
		cell := func(name string) (string, bool) {
			idx, ok := cols[name]
			if !ok {
				return "", false
			}
			return resource.ReplaceVariables(row.Cells[idx].Value), true
		}
		wantType, _ := cell("event")
		e, err := r.nextEvent(time.Until(deadline))
		if err != nil {
			return fmt.Errorf("row %d: expected event %q: %w", i+1, wantType, err)
		}
		if e.Type != wantType {
			return fmt.Errorf("row %d: expected event %q, got %s", i+1, wantType, e.describe())
		}
		if wantID, ok := cell("id"); ok && (!e.HasID || e.ID != wantID) {
			return fmt.Errorf("row %d: expected id %q, got %s", i+1, wantID, e.describe())
		}
		if wantData, ok := cell("data"); ok && wantData != "" {
			if err := matchEventJSON(e, wantData); err != nil {
				return fmt.Errorf("row %d: %w", i+1, err)
			}
		}
	}
	return nil
}

func (r *HTTPClient) noEventWithin(within string) error {
	timeout, err := time.ParseDuration(within)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	s, err := r.openedStream()
	if err != nil {
		return err
	}
	time.Sleep(timeout)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.events[s.consumed:] {
		if !r.skipEvents[e.Type] {
			return fmt.Errorf("received unexpected event %s", e.describe())
		}
	}
	return nil
}

func (r *HTTPClient) receiveComment(within string) error {
	timeout, err := time.ParseDuration(within)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	s, err := r.openedStream()
	if err != nil {
		return err
	}
	found := resource.Eventually(timeout, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.commentsConsumed < len(s.comments) {
			s.commentsConsumed++
			return true
		}
		return false
	})
	if !found {
		return fmt.Errorf("no comment within %s", timeout)
	}
	return nil
}

func (r *HTTPClient) eventIDsConsecutive(from int) error {
	s, err := r.openedStream()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	want := from
	seen := 0
	for _, e := range s.events {
		if !e.HasID || r.skipEvents[e.Type] {
			continue
		}
		got, err := strconv.Atoi(e.ID)
		if err != nil {
			return fmt.Errorf("event %s: id is not an integer", e.describe())
		}
		if got != want {
			return fmt.Errorf("expected id %d, got %s", want, e.describe())
		}
		want++
		seen++
	}
	if seen == 0 {
		return fmt.Errorf("no event with an id received on stream %s", s.path)
	}
	return nil
}

func (r *HTTPClient) closedByServerWithin(within string) error {
	timeout, err := time.ParseDuration(within)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	s, err := r.openedStream()
	if err != nil {
		return err
	}
	select {
	case <-s.done:
	case <-time.After(timeout):
		return fmt.Errorf("stream %s still open after %s", s.path, timeout)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.readErr != nil {
		return fmt.Errorf("stream %s failed instead of being closed: %v", s.path, s.readErr)
	}
	return nil
}

func (r *HTTPClient) disconnectedByServerWithin(within string) error {
	timeout, err := time.ParseDuration(within)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	s, err := r.openedStream()
	if err != nil {
		return err
	}
	select {
	case <-s.done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("stream %s still open after %s", s.path, timeout)
	}
}

func (r *HTTPClient) saveLastEventID(varName string) error {
	s, err := r.openedStream()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := s.consumed - 1; i >= 0; i-- {
		if e := s.events[i]; e.HasID && !r.skipEvents[e.Type] {
			resource.SetVariable(varName, e.ID)
			return nil
		}
	}
	return fmt.Errorf("no event with an id has been received on stream %s", s.path)
}

// parseEventStream reads an event stream, calling onEvent for every
// dispatched event and onComment for every comment line, until the reader
// ends. It follows the parsing rules of the HTML standard: lines end in CRLF,
// LF or CR; a blank line dispatches; multiple data lines join with "\n"; an
// event with no data is not dispatched; the type defaults to "message"; one
// space after the colon is dropped. retry is accepted and ignored, since
// tomato never reconnects on its own.
func parseEventStream(r io.Reader, onEvent func(sseEvent), onComment func(string)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	scanner.Split(scanEventLines)

	var (
		data    strings.Builder
		hasData bool
		evType  string
		id      string
		hasID   bool
	)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if hasData {
				t := evType
				if t == "" {
					t = "message"
				}
				onEvent(sseEvent{ID: id, HasID: hasID, Type: t, Data: data.String()})
			}
			data.Reset()
			hasData = false
			evType = ""
			id, hasID = "", false
			continue
		}
		if strings.HasPrefix(line, ":") {
			onComment(strings.TrimPrefix(line[1:], " "))
			continue
		}
		field, value, found := strings.Cut(line, ":")
		if found {
			value = strings.TrimPrefix(value, " ")
		}
		switch field {
		case "data":
			if hasData {
				data.WriteByte('\n')
			}
			data.WriteString(value)
			hasData = true
		case "event":
			evType = value
		case "id":
			if !strings.ContainsRune(value, 0) {
				id, hasID = value, true
			}
		}
	}
	return scanner.Err()
}

// scanEventLines splits on CRLF, LF or a lone CR.
func scanEventLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		if data[i] == '\r' {
			if i+1 < len(data) {
				if data[i+1] == '\n' {
					return i + 2, data[:i], nil
				}
				return i + 1, data[:i], nil
			}
			if !atEOF {
				// A CR at the end of the buffer may be the first half of CRLF.
				return 0, nil, nil
			}
		}
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
