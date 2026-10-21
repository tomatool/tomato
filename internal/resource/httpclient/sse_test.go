package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v21"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/resource"
)

func TestParseEventStream(t *testing.T) {
	input := ": hello\r\n" +
		"id: 1\r\nevent: order.created\r\ndata: {\"a\":1}\r\n\r\n" +
		"data: line one\ndata: line two\n\n" + // no event field: type "message"
		"event: no-data\n\n" + // no data: not dispatched
		"id: 3\revent: cr\rdata:no space\r\r" + // lone CR line endings
		"retry: 1000\nid\ndata: empty id\n\n" + // "id" alone sets an empty id
		"data: unterminated" // no blank line before EOF: not dispatched

	var events []sseEvent
	var comments []string
	err := parseEventStream(strings.NewReader(input),
		func(e sseEvent) { events = append(events, e) },
		func(c string) { comments = append(comments, c) })
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := []sseEvent{
		{ID: "1", HasID: true, Type: "order.created", Data: `{"a":1}`},
		{Type: "message", Data: "line one\nline two"},
		{ID: "3", HasID: true, Type: "cr", Data: "no space"},
		{ID: "", HasID: true, Type: "message", Data: "empty id"},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(want), events)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Errorf("event %d: got %+v, want %+v", i, events[i], want[i])
		}
	}
	if len(comments) != 1 || comments[0] != "hello" {
		t.Errorf("comments: got %q, want [hello]", comments)
	}
}

func TestScanEventLines_CRSplitAcrossReads(t *testing.T) {
	// A CR at the end of one read must wait for the next byte: it may be CRLF.
	advance, token, err := scanEventLines([]byte("data: x\r"), false)
	if err != nil || advance != 0 || token != nil {
		t.Fatalf("got advance=%d token=%q err=%v, want a request for more data", advance, token, err)
	}
	advance, token, _ = scanEventLines([]byte("data: x\r\n"), false)
	if advance != 9 || string(token) != "data: x" {
		t.Fatalf("got advance=%d token=%q", advance, token)
	}
	advance, token, _ = scanEventLines([]byte("data: x\r"), true)
	if advance != 8 || string(token) != "data: x" {
		t.Fatalf("at EOF: got advance=%d token=%q", advance, token)
	}
}

// sseServer streams the frames written to its channel, and records each
// request it got.
type sseServer struct {
	*httptest.Server
	frames chan string
	mu     sync.Mutex
	reqs   []*http.Request
}

func newSSEServer(t *testing.T) *sseServer {
	s := &sseServer{frames: make(chan string, 100)}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.reqs = append(s.reqs, r)
		s.mu.Unlock()
		switch r.URL.Path {
		case "/unauthorized":
			http.Error(w, "no token", http.StatusUnauthorized)
			return
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{}`)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case f, ok := <-s.frames:
				if !ok {
					return // closed: end the stream from the server side
				}
				fmt.Fprint(w, f)
				w.(http.Flusher).Flush()
			}
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *sseServer) lastRequest() *http.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reqs[len(s.reqs)-1]
}

func newStreamTestClient(t *testing.T, baseURL string) *HTTPClient {
	t.Helper()
	c, err := New("api", config.Resource{BaseURL: baseURL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Cleanup(context.Background()) })
	return c
}

func eventsTable(rows ...[]string) *godog.Table {
	t := &godog.Table{}
	for _, r := range rows {
		row := &messages.PickleTableRow{}
		for _, v := range r {
			row.Cells = append(row.Cells, &messages.PickleTableCell{Value: v})
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

func TestStream_SendsScenarioRequestSetup(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)

	c.setHeader("Authorization", "Bearer t")
	c.setCookie("session", "s1")
	c.setQueryParam("account", "a-1")
	if err := c.openStreamAfter("/events", "41"); err != nil {
		t.Fatal(err)
	}

	req := srv.lastRequest()
	checks := map[string]string{
		"Authorization": "Bearer t",
		"Accept":        "text/event-stream",
		"Last-Event-ID": "41",
	}
	for h, want := range checks {
		if got := req.Header.Get(h); got != want {
			t.Errorf("header %s: got %q, want %q", h, got, want)
		}
	}
	if ck, err := req.Cookie("session"); err != nil || ck.Value != "s1" {
		t.Errorf("cookie session: got %v, %v", ck, err)
	}
	if got := req.URL.Query().Get("account"); got != "a-1" {
		t.Errorf("query account: got %q", got)
	}
	if len(c.requestParams) != 0 {
		t.Errorf("query params should be single-use, still set: %v", c.requestParams)
	}
}

func TestStream_OpenFailsOnWrongStatusOrContentType(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)

	err := c.openStream("/unauthorized")
	if err == nil || !strings.Contains(err.Error(), "got 401") || !strings.Contains(err.Error(), "no token") {
		t.Errorf("want a 401 error with the body, got %v", err)
	}
	err = c.openStream("/json")
	if err == nil || !strings.Contains(err.Error(), "text/event-stream") {
		t.Errorf("want a content type error, got %v", err)
	}
	if c.stream != nil {
		t.Error("a failed open must not leave a stream behind")
	}
}

func TestStream_OutlivesRequestTimeout(t *testing.T) {
	srv := newSSEServer(t)
	c, err := New("api", config.Resource{BaseURL: srv.URL, Options: map[string]any{"timeout": "100ms"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Cleanup(context.Background())

	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	srv.frames <- "id: 1\nevent: late\ndata: {}\n\n"
	if err := c.receiveEvent("late", "2s"); err != nil {
		t.Fatalf("the stream should outlive the 100ms request timeout: %v", err)
	}
}

func TestStream_ReceiveInOrder(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}

	srv.frames <- "id: 1\nevent: created\ndata: {\"order\":{\"id\":\"o-1\",\"qty\":10}}\n\n"
	srv.frames <- "id: 2\nevent: filled\ndata: {\"filled\":10}\n\n"
	srv.frames <- "id: 3\nevent: settled\ndata: {}\n\n"

	if err := c.receiveEventWithJSON("created", "2s", &godog.DocString{Content: `{"order":{"id":"o-1"}}`}); err != nil {
		t.Fatal(err)
	}
	// The next event is "filled", so expecting "settled" fails and names what came.
	err := c.receiveEvent("settled", "2s")
	if err == nil || !strings.Contains(err.Error(), `got filled (id 2)`) {
		t.Fatalf("want an order mismatch naming the event, got %v", err)
	}
	if err := c.receiveEvents("2s", eventsTable(
		[]string{"event", "id"},
		[]string{"settled", "3"},
	)); err != nil {
		t.Fatal(err)
	}
	if err := c.eventIDsConsecutive(1); err != nil {
		t.Fatal(err)
	}
	if err := c.saveLastEventID("last"); err != nil {
		t.Fatal(err)
	}
	if got := resource.ReplaceVariables("{{last}}"); got != "3" {
		t.Errorf("last id: got %q, want 3", got)
	}
}

func TestStream_ReceiveEventsTable(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	srv.frames <- "id: 1\nevent: a\ndata: {\"n\":1}\n\n"
	srv.frames <- "id: 2\nevent: b\ndata: {\"n\":2}\n\n"

	err := c.receiveEvents("1s", eventsTable(
		[]string{"event", "id", "data"},
		[]string{"a", "1", `{"n":1}`},
		[]string{"b", "2", `{"n":3}`},
	))
	if err == nil || !strings.Contains(err.Error(), "row 2") {
		t.Fatalf("want row 2 to fail on data, got %v", err)
	}

	for _, bad := range [][]string{{"id"}, {"event", "status"}} {
		if err := c.receiveEvents("1s", eventsTable(bad, make([]string, len(bad)))); err == nil {
			t.Errorf("columns %v: want an error", bad)
		}
	}
	if err := c.receiveEvents("1s", eventsTable([]string{"event"})); err == nil {
		t.Error("a table without rows should fail")
	}
}

func TestStream_TimeoutAndServerClose(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}

	err := c.receiveEvent("x", "100ms")
	if err == nil || !strings.Contains(err.Error(), "no event within 100ms") {
		t.Fatalf("want a timeout, got %v", err)
	}
	if err := c.closedByServerWithin("50ms"); err == nil {
		t.Fatal("the stream is still open")
	}

	srv.frames <- "id: 1\nevent: last\ndata: {}\n\n"
	close(srv.frames)
	if err := c.closedByServerWithin("2s"); err != nil {
		t.Fatal(err)
	}
	// Events that arrived before the close are still delivered.
	if err := c.receiveEvent("last", "1s"); err != nil {
		t.Fatal(err)
	}
	err = c.receiveEvent("more", "100ms")
	if err == nil || !strings.Contains(err.Error(), "the server closed the stream") {
		t.Fatalf("want the close named, got %v", err)
	}
}

func TestStream_CommentsAreNotEvents(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}

	srv.frames <- ": heartbeat\n\n"
	if err := c.receiveComment("2s"); err != nil {
		t.Fatal(err)
	}
	if err := c.noEventWithin("100ms"); err != nil {
		t.Fatalf("a comment is not an event: %v", err)
	}
	if err := c.receiveComment("100ms"); err == nil {
		t.Fatal("the only comment was already consumed")
	}

	srv.frames <- "event: surprise\ndata: {}\n\n"
	err := c.noEventWithin("200ms")
	if err == nil || !strings.Contains(err.Error(), "surprise") {
		t.Fatalf("want the unexpected event named, got %v", err)
	}
}

func TestStream_SkippedEventTypes(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	if err := c.skipEventTypes("heartbeat, ping"); err != nil {
		t.Fatal(err)
	}

	srv.frames <- "event: heartbeat\ndata: {}\n\n"
	srv.frames <- "id: 7\nevent: ping\ndata: {}\n\n"
	srv.frames <- "id: 1\nevent: real\ndata: {}\n\n"
	srv.frames <- "event: heartbeat\ndata: {}\n\n"

	if err := c.receiveEvent("real", "2s"); err != nil {
		t.Fatal(err)
	}
	if err := c.noEventWithin("100ms"); err != nil {
		t.Fatalf("skipped types are not events: %v", err)
	}
	if err := c.eventIDsConsecutive(1); err != nil {
		t.Fatalf("a skipped event's id is left out: %v", err)
	}
}

func TestStream_EventIDsConsecutive(t *testing.T) {
	cases := []struct {
		name    string
		frames  string
		from    int
		wantErr string
	}{
		{"gap", "id: 1\ndata: x\n\nid: 3\ndata: x\n\n", 1, "expected id 2"},
		{"repeat", "id: 1\ndata: x\n\nid: 1\ndata: x\n\n", 1, "expected id 2"},
		{"wrong start", "id: 2\ndata: x\n\n", 1, "expected id 1"},
		{"not a number", "id: abc\ndata: x\n\n", 1, "not an integer"},
		{"resumed", "id: 46\ndata: x\n\nid: 47\ndata: x\n\n", 46, ""},
		{"ids skip events without one", "id: 1\ndata: x\n\ndata: hb\n\nid: 2\ndata: x\n\n", 1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := newSSEServer(t)
			c := newStreamTestClient(t, srv.URL)
			if err := c.openStream("/events"); err != nil {
				t.Fatal(err)
			}
			srv.frames <- tc.frames
			close(srv.frames)
			if err := c.closedByServerWithin("2s"); err != nil {
				t.Fatal(err)
			}
			err := c.eventIDsConsecutive(tc.from)
			if tc.wantErr == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("want %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestStream_ResetAndReopen(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)

	for _, step := range []func() error{
		func() error { return c.receiveEvent("x", "10ms") },
		func() error { return c.closeStreamStep() },
		func() error { return c.eventIDsConsecutive(1) },
		func() error { return c.saveLastEventID("v") },
	} {
		if err := step(); err == nil || !strings.Contains(err.Error(), "no stream is open") {
			t.Fatalf("want 'no stream is open', got %v", err)
		}
	}

	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	srv.frames <- "event: a\ndata: {}\n\n"
	if err := c.receiveEvent("a", "2s"); err != nil {
		t.Fatal(err)
	}
	if err := c.saveLastEventID("v"); err == nil {
		t.Fatal("no event with an id was received")
	}

	// Opening again replaces the stream: nothing of the old one carries over.
	first := c.stream
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	if c.stream == first {
		t.Fatal("want a new stream")
	}
	select {
	case <-first.done:
	case <-time.After(2 * time.Second):
		t.Fatal("the old stream was not closed")
	}

	c.skipEventTypes("a")
	if err := c.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.stream != nil || len(c.skipEvents) != 0 {
		t.Fatal("reset must close the stream and forget skipped types")
	}
}

func TestStream_StepDefinitions(t *testing.T) {
	c, _ := New("api", resource.DummyConfig(), nil)
	steps := c.Steps().Steps

	stream := c.streamSteps()
	if len(stream) == 0 {
		t.Fatal("no stream steps")
	}
	registered := map[string]bool{}
	for _, s := range steps {
		registered[s.Pattern] = true
	}
	for _, s := range stream {
		if !registered[s.Pattern] {
			t.Errorf("stream step %q is not part of the http client's steps", s.Pattern)
		}
		if s.Group != "Event Streams" || s.Description == "" || s.Handler == nil {
			t.Errorf("step %q: needs the Event Streams group, a description and a handler", s.Pattern)
		}
		re := regexp.MustCompile(strings.ReplaceAll(s.Pattern, "{resource}", "app-client"))
		if !re.MatchString(s.Example) {
			t.Errorf("example %q does not match its pattern %q", s.Example, s.Pattern)
		}
	}
}

func TestStream_InvalidDurations(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	doc := &godog.DocString{Content: `{}`}
	table := eventsTable([]string{"event"}, []string{"a"})
	for name, step := range map[string]func() error{
		"receives event":       func() error { return c.receiveEvent("a", "soon") },
		"with json":            func() error { return c.receiveEventWithJSON("a", "soon", doc) },
		"receives events":      func() error { return c.receiveEvents("soon", table) },
		"does not receive":     func() error { return c.noEventWithin("soon") },
		"receives a comment":   func() error { return c.receiveComment("soon") },
		"closed by the server": func() error { return c.closedByServerWithin("soon") },
		"disconnected":         func() error { return c.disconnectedByServerWithin("soon") },
	} {
		if err := step(); err == nil || !strings.Contains(err.Error(), "invalid duration") {
			t.Errorf("%s: want an invalid duration error, got %v", name, err)
		}
	}
}

func TestStream_StepsNeedAnOpenStream(t *testing.T) {
	c := newStreamTestClient(t, "http://127.0.0.1:1")
	doc := &godog.DocString{Content: `{}`}
	table := eventsTable([]string{"event"}, []string{"a"})
	for name, step := range map[string]func() error{
		"with json":            func() error { return c.receiveEventWithJSON("a", "10ms", doc) },
		"receives events":      func() error { return c.receiveEvents("10ms", table) },
		"does not receive":     func() error { return c.noEventWithin("10ms") },
		"receives a comment":   func() error { return c.receiveComment("10ms") },
		"closed by the server": func() error { return c.closedByServerWithin("10ms") },
	} {
		if err := step(); err == nil || !strings.Contains(err.Error(), "no stream is open") {
			t.Errorf("%s: want 'no stream is open', got %v", name, err)
		}
	}
}

func TestStream_OpenErrors(t *testing.T) {
	// Nothing listens on port 1.
	c := newStreamTestClient(t, "http://127.0.0.1:1")
	if err := c.openStream("/events"); err == nil || !strings.Contains(err.Error(), "opening stream") {
		t.Errorf("want a connection error, got %v", err)
	}
	if err := c.openStream("/bad\x7fpath"); err == nil || !strings.Contains(err.Error(), "creating request") {
		t.Errorf("want a request error, got %v", err)
	}
}

func TestStream_ReceiveEventWithJSONFailures(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	doc := func(s string) *godog.DocString { return &godog.DocString{Content: s} }

	if err := c.receiveEventWithJSON("a", "50ms", doc(`{}`)); err == nil || !strings.Contains(err.Error(), "no event within") {
		t.Errorf("want a timeout, got %v", err)
	}

	srv.frames <- "event: b\ndata: {}\n\n"
	if err := c.receiveEventWithJSON("a", "2s", doc(`{}`)); err == nil || !strings.Contains(err.Error(), `expected event "a", got b`) {
		t.Errorf("want a type mismatch, got %v", err)
	}

	srv.frames <- "event: a\ndata: {}\n\n"
	if err := c.receiveEventWithJSON("a", "2s", doc(`{not json`)); err == nil || !strings.Contains(err.Error(), "invalid expected JSON") {
		t.Errorf("want an invalid expected JSON error, got %v", err)
	}

	srv.frames <- "event: a\ndata: plain text\n\n"
	if err := c.receiveEventWithJSON("a", "2s", doc(`{}`)); err == nil || !strings.Contains(err.Error(), "data is not JSON") {
		t.Errorf("want a data is not JSON error, got %v", err)
	}

	resource.SetVariable("want", "o-2")
	srv.frames <- "event: a\ndata: {\"id\":\"o-1\"}\n\n"
	err := c.receiveEventWithJSON("a", "2s", doc(`{"id":"{{want}}"}`))
	if err == nil || !strings.Contains(err.Error(), "o-1") {
		t.Errorf("want a mismatch naming the event, with variables replaced, got %v", err)
	}
}

func TestStream_ReceiveEventsTimeout(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	srv.frames <- "id: 1\nevent: a\ndata: {}\n\n"
	srv.frames <- "id: 9\nevent: b\ndata: {}\n\n"

	err := c.receiveEvents("2s", eventsTable(
		[]string{"event", "id"},
		[]string{"a", "1"},
		[]string{"b", "2"},
	))
	if err == nil || !strings.Contains(err.Error(), `row 2: expected id "2"`) {
		t.Fatalf("want an id mismatch, got %v", err)
	}
	err = c.receiveEvents("100ms", eventsTable([]string{"event"}, []string{"c"}))
	if err == nil || !strings.Contains(err.Error(), "no event within") {
		t.Fatalf("want a timeout, got %v", err)
	}
	if err := c.receiveEvents("1s", eventsTable([]string{"event"}, []string{"c"})); err == nil {
		t.Fatal("want a timeout")
	}
}

func TestStream_ClientClose(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	s := c.stream
	if err := c.closeStreamStep(); err != nil {
		t.Fatal(err)
	}
	if s.closedByServer || !s.closedByClient || s.readErr != nil {
		t.Fatalf("a client close is not a server close: %+v", s)
	}
}

// A body cut off mid-chunk is a failure, not a clean close.
func TestStream_BrokenStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		buf.WriteString("HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nTransfer-Encoding: chunked\r\n\r\n")
		buf.WriteString("ff\r\nid: 1\ndata: x\n\n") // promises 255 bytes, sends fewer
		buf.Flush()
	}))
	defer srv.Close()

	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	err := c.closedByServerWithin("2s")
	if err == nil || !strings.Contains(err.Error(), "failed instead of being closed") {
		t.Fatalf("want a stream failure, got %v", err)
	}
	// The complete event before the cut is still delivered; then the failure.
	if err := c.receiveEvent("message", "1s"); err != nil {
		t.Fatal(err)
	}
	err = c.receiveEvent("message", "50ms")
	if err == nil || !strings.Contains(err.Error(), "the stream failed") {
		t.Fatalf("want the failure named, got %v", err)
	}
}

func TestStream_ReceiveEventsWrongType(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	srv.frames <- "event: b\ndata: {}\n\n"
	err := c.receiveEvents("2s", eventsTable([]string{"event"}, []string{"a"}))
	if err == nil || !strings.Contains(err.Error(), `row 1: expected event "a", got b`) {
		t.Fatalf("want a type mismatch, got %v", err)
	}
}

func TestStream_EventIDsConsecutiveNeedsAnID(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	err := c.eventIDsConsecutive(1)
	if err == nil || !strings.Contains(err.Error(), "no event with an id") {
		t.Fatalf("an empty stream must not pass as gapless, got %v", err)
	}
}

// slowConsumerServer streams large events as fast as it can and disconnects
// a client whose connection stops draining: a write that cannot complete
// within 300ms ends the stream, as a server protecting itself does.
func slowConsumerServer(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		rc := http.NewResponseController(w)
		payload := strings.Repeat("x", 16*1024)
		for i := 1; ; i++ {
			rc.SetWriteDeadline(time.Now().Add(300 * time.Millisecond))
			if _, err := fmt.Fprintf(w, "id: %d\nevent: tick\ndata: {\"p\":%q}\n\n", i, payload); err != nil {
				return // slow consumer: disconnect
			}
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestStream_PausedReaderIsDisconnectedAndResumesWithoutLoss(t *testing.T) {
	srv := slowConsumerServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.openStream("/events"); err != nil {
		t.Fatal(err)
	}
	if err := c.receiveEvent("tick", "2s"); err != nil {
		t.Fatal(err)
	}
	if err := c.pauseStream(); err != nil {
		t.Fatal(err)
	}
	// While paused nothing is read, so the buffers fill and the server's
	// write deadline passes.
	time.Sleep(1500 * time.Millisecond)
	if err := c.closedByServerWithin("10ms"); err == nil {
		t.Fatal("a paused reader cannot have seen the close yet")
	}
	if err := c.resumeStream(); err != nil {
		t.Fatal(err)
	}
	// What arrived before the disconnect is delivered, then the end. The
	// server dropped the connection mid-write, so it is a disconnect, not a
	// clean close.
	if err := c.disconnectedByServerWithin("10s"); err != nil {
		t.Fatal(err)
	}
	if err := c.closedByServerWithin("10ms"); err == nil || !strings.Contains(err.Error(), "failed instead of being closed") {
		t.Fatalf("a dropped connection is not a clean close: %v", err)
	}
	if err := c.eventIDsConsecutive(1); err != nil {
		t.Fatalf("the events delivered must have no gap: %v", err)
	}
	if n := len(c.stream.events); n < 2 {
		t.Fatalf("want the buffered events delivered after resuming, got %d", n)
	}
}

func TestStream_DisconnectedByServer(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.disconnectedByServerWithin("10ms"); err == nil || !strings.Contains(err.Error(), "no stream is open") {
		t.Errorf("no stream: %v", err)
	}
	c.openStream("/events")
	if err := c.disconnectedByServerWithin("soon"); err == nil || !strings.Contains(err.Error(), "invalid duration") {
		t.Errorf("bad duration: %v", err)
	}
	if err := c.disconnectedByServerWithin("50ms"); err == nil || !strings.Contains(err.Error(), "still open") {
		t.Errorf("open stream: %v", err)
	}
	close(srv.frames) // a clean close counts too
	if err := c.disconnectedByServerWithin("2s"); err != nil {
		t.Error(err)
	}
}

func TestStream_PauseErrors(t *testing.T) {
	srv := newSSEServer(t)
	c := newStreamTestClient(t, srv.URL)
	if err := c.pauseStream(); err == nil || !strings.Contains(err.Error(), "no stream is open") {
		t.Errorf("pause without a stream: %v", err)
	}
	if err := c.resumeStream(); err == nil || !strings.Contains(err.Error(), "no stream is open") {
		t.Errorf("resume without a stream: %v", err)
	}
	c.openStream("/events")
	if err := c.resumeStream(); err == nil || !strings.Contains(err.Error(), "not paused") {
		t.Errorf("resume a running stream: %v", err)
	}
	c.pauseStream()
	if err := c.pauseStream(); err == nil || !strings.Contains(err.Error(), "already paused") {
		t.Errorf("pause twice: %v", err)
	}

	// Closing a paused stream must not hang on the blocked reader.
	done := make(chan struct{})
	go func() { c.closeStreamStep(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("closing a paused stream hung")
	}
}
