package handler

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v21"
	"github.com/tomatool/tomato/internal/config"
)

func wsDoc(s string) *godog.DocString { return &godog.DocString{Content: s} }

// newWSPair starts a websocket-server stub and a client pointed at it.
func newWSPair(t *testing.T) (*WebSocketServer, *WebSocketClient) {
	t.Helper()
	ctx := context.Background()
	srv, _ := NewWebSocketServer("wsmock", config.Resource{Options: map[string]any{}}, nil)
	if err := srv.Init(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Cleanup(ctx) })

	client, _ := NewWebSocketClient("ws", config.Resource{URL: srv.GetURL() + "/"}, nil)
	if err := client.Init(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Cleanup(ctx) })
	return srv, client
}

// A reply that arrives before the receive step starts waiting must still be
// seen. This was the websocket feature's flake: waitForMessage only counted
// messages that arrived after it was called.
func TestWebSocketClient_ReceivesReplyThatArrivedEarly(t *testing.T) {
	srv, client := newWSPair(t)
	if err := srv.onMessageReplies("ping", wsDoc("pong")); err != nil {
		t.Fatal(err)
	}
	if err := client.connect(); err != nil {
		t.Fatal(err)
	}
	if err := client.sendText("ping"); err != nil {
		t.Fatal(err)
	}
	// Let the reply land before the receive step runs.
	if !eventually(func() bool { return client.getMessageCount() == 1 }) {
		t.Fatal("reply never arrived")
	}
	if err := client.shouldReceiveMessage("1s", wsDoc("pong")); err != nil {
		t.Fatalf("early reply was lost: %v", err)
	}
	// It was consumed: nothing further is pending.
	if err := client.shouldNotReceiveMessage("50ms"); err != nil {
		t.Fatalf("consumed message still pending: %v", err)
	}
}

func TestWebSocketClient_ReceiveStepsConsumeInOrder(t *testing.T) {
	srv, client := newWSPair(t)
	if err := client.connect(); err != nil {
		t.Fatal(err)
	}
	if err := srv.hasConnections(1); err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"a", "b", "c"} {
		if err := srv.broadcastText(m); err != nil {
			t.Fatal(err)
		}
	}
	if err := client.shouldReceiveMessage("1s", wsDoc("a")); err != nil {
		t.Fatal(err)
	}
	if err := client.shouldReceiveNMessages(2, "1s"); err != nil {
		t.Fatal(err)
	}
	if err := client.lastMessageShouldBe(wsDoc("c")); err != nil {
		t.Fatal(err)
	}
	if err := client.shouldReceiveNMessages(1, "100ms"); err == nil {
		t.Error("expected no further messages")
	}
	if err := client.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.consumed != 0 || client.getMessageCount() != 0 {
		t.Error("reset must clear messages and the read cursor")
	}
}

func TestWebSocketClient_ConnectWithHeadersAndJSON(t *testing.T) {
	srv, client := newWSPair(t)
	if err := srv.onConnectSends(wsDoc(`{"type":"welcome"}`)); err != nil {
		t.Fatal(err)
	}
	table := &godog.Table{Rows: []*messages.PickleTableRow{
		{Cells: []*messages.PickleTableCell{{Value: "header"}, {Value: "value"}}},
		{Cells: []*messages.PickleTableCell{{Value: "X-Client-Id"}, {Value: "t1"}}},
	}}
	if err := client.connectWithHeaders(table); err != nil {
		t.Fatal(err)
	}
	if err := client.shouldReceiveJSONMatching("1s", wsDoc(`{"type": "welcome"}`)); err != nil {
		t.Fatal(err)
	}
	if err := client.lastMessageShouldBeJSONMatching(wsDoc(`{"type":"welcome"}`)); err != nil {
		t.Fatal(err)
	}
}

// Rule replies (reader goroutine) and broadcasts (step goroutine) write to
// the same connection; gorilla/websocket panics on concurrent writers.
func TestWebSocketServer_ConcurrentWritesAreSerialised(t *testing.T) {
	srv, client := newWSPair(t)
	if err := srv.onMessageMatchingReplies(".*", wsDoc("reply")); err != nil {
		t.Fatal(err)
	}
	if err := client.connect(); err != nil {
		t.Fatal(err)
	}
	if err := srv.hasConnections(1); err != nil {
		t.Fatal(err)
	}

	const n = 50
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			srv.broadcastText(fmt.Sprintf("b%d", i))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			client.sendText(fmt.Sprintf("m%d", i))
		}
	}()
	wg.Wait()

	if err := client.shouldReceiveNMessages(2*n, "5s"); err != nil {
		t.Fatal(err)
	}
	if err := srv.receivedMessageCount(n); err != nil {
		t.Fatal(err)
	}
}

func TestWebSocketServer_AssertionsWaitForAsyncState(t *testing.T) {
	srv, client := newWSPair(t)
	if err := srv.hasConnections(0); err != nil {
		t.Fatal(err)
	}
	if err := client.connect(); err != nil {
		t.Fatal(err)
	}
	// Called right after connect/send, before the server goroutine has
	// necessarily registered them.
	if err := srv.hasConnections(1); err != nil {
		t.Fatal(err)
	}
	if err := client.sendText("hello"); err != nil {
		t.Fatal(err)
	}
	if err := srv.receivedMessage("hello"); err != nil {
		t.Fatal(err)
	}
	if err := srv.receivedMessage("never sent"); err == nil {
		t.Error("expected an error for a message that was never sent")
	}
	client.disconnect()
	if err := srv.hasConnections(0); err != nil {
		t.Fatal(err)
	}
}

func TestWebSocketServer_BroadcastWithoutClientsFails(t *testing.T) {
	srv, _ := newWSPair(t)
	err := srv.broadcastText("x")
	if err == nil || !strings.Contains(err.Error(), "no clients") {
		t.Errorf("expected a no-clients error, got %v", err)
	}
}

func TestHTTPServer_StoreURL(t *testing.T) {
	srv, _ := NewHTTPServer("mock", config.Resource{Options: map[string]any{}}, nil)
	if err := srv.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer srv.Cleanup(context.Background())
	if err := srv.storeURL("MOCK_URL_TEST"); err != nil {
		t.Fatal(err)
	}
	if got := ReplaceVariables("{{MOCK_URL_TEST}}"); got != srv.GetURL() || !strings.HasPrefix(got, "http://localhost:") {
		t.Errorf("stored url = %q, want %q", got, srv.GetURL())
	}
}
