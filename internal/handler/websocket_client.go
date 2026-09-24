package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cucumber/godog"
	"github.com/gorilla/websocket"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

type WebSocketClient struct {
	name      string
	config    config.Resource
	container *container.Manager

	url     string
	conn    *websocket.Conn
	dialer  *websocket.Dialer
	headers http.Header

	messages    [][]byte
	messagesMu  sync.RWMutex
	lastMessage []byte
	// consumed is how many messages receive steps have already matched.
	// Receive steps look at messages from here on, so a reply that arrives
	// before the step starts waiting is still seen.
	consumed   int
	readCtx    context.Context
	readCancel context.CancelFunc
	connected  atomic.Bool
}

func NewWebSocketClient(name string, cfg config.Resource, cm *container.Manager) (*WebSocketClient, error) {
	return &WebSocketClient{
		name:      name,
		config:    cfg,
		container: cm,
		headers:   make(http.Header),
		messages:  make([][]byte, 0),
	}, nil
}

func (r *WebSocketClient) Name() string { return r.name }

func (r *WebSocketClient) Init(ctx context.Context) error {
	handshakeTimeout := 10 * time.Second
	if t, ok := r.config.Options["handshake_timeout"].(string); ok {
		if d, err := time.ParseDuration(t); err == nil {
			handshakeTimeout = d
		}
	}

	r.dialer = &websocket.Dialer{
		HandshakeTimeout: handshakeTimeout,
	}

	if protocols, ok := r.config.Options["protocols"].([]interface{}); ok {
		for _, p := range protocols {
			if s, ok := p.(string); ok {
				r.dialer.Subprotocols = append(r.dialer.Subprotocols, s)
			}
		}
	}

	if r.config.URL != "" {
		r.url = r.config.URL
	} else if r.config.Container != "" {
		host, err := r.container.GetHost(ctx, r.config.Container)
		if err != nil {
			return fmt.Errorf("getting container host: %w", err)
		}

		port := "8080"
		if p, ok := r.config.Options["port"].(string); ok {
			port = p
		}

		mappedPort, err := r.container.GetPort(ctx, r.config.Container, port+"/tcp")
		if err != nil {
			return fmt.Errorf("getting container port: %w", err)
		}

		path := "/ws"
		if p, ok := r.config.Options["path"].(string); ok {
			path = p
		}

		r.url = fmt.Sprintf("ws://%s:%s%s", host, mappedPort, path)
	}

	if headers, ok := r.config.Options["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				r.headers.Set(k, s)
			}
		}
	}

	return nil
}

func (r *WebSocketClient) Ready(ctx context.Context) error {
	return nil
}

func (r *WebSocketClient) Reset(ctx context.Context) error {
	if r.conn != nil {
		r.disconnect()
	}

	r.messagesMu.Lock()
	r.messages = make([][]byte, 0)
	r.lastMessage = nil
	r.consumed = 0
	r.messagesMu.Unlock()

	r.headers = make(http.Header)
	if headers, ok := r.config.Options["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				r.headers.Set(k, s)
			}
		}
	}

	return nil
}

func (r *WebSocketClient) disconnect() {
	if r.readCancel != nil {
		r.readCancel()
	}
	if r.conn != nil {
		r.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		r.conn.Close()
		r.conn = nil
	}
	r.connected.Store(false)
}

func (r *WebSocketClient) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the WebSocket client handler
func (r *WebSocketClient) Steps() StepCategory {
	return StepCategory{
		Name:        "WebSocket Client",
		Description: "Steps for connecting to WebSocket servers",
		Steps: []StepDef{
			// Connection
			{
				Group:       "Connection",
				Pattern:     `^"{resource}" connects$`,
				Description: "Connect to WebSocket endpoint",
				Example:     `"ws" connects`,
				Handler:     r.connect,
			},
			{
				Group:       "Connection",
				Pattern:     `^"{resource}" connects with headers:$`,
				Description: "Connect with custom headers",
				Example:     `"ws" connects with headers:`,
				Handler:     r.connectWithHeaders,
			},
			{
				Group:       "Connection",
				Pattern:     `^"{resource}" disconnects$`,
				Description: "Disconnect from WebSocket",
				Example:     `"ws" disconnects`,
				Handler:     r.disconnectStep,
			},
			{
				Group:       "Connection",
				Pattern:     `^"{resource}" is connected$`,
				Description: "Assert connected",
				Example:     `"ws" is connected`,
				Handler:     r.shouldBeConnected,
			},
			{
				Group:       "Connection",
				Pattern:     `^"{resource}" is disconnected$`,
				Description: "Assert disconnected",
				Example:     `"ws" is disconnected`,
				Handler:     r.shouldBeDisconnected,
			},

			// Sending
			{
				Group:       "Sending",
				Pattern:     `^"{resource}" sends:$`,
				Description: "Send text message (docstring)",
				Example:     `"ws" sends:`,
				Handler:     r.sendMessage,
			},
			{
				Group:       "Sending",
				Pattern:     `^"{resource}" sends "([^"]*)"$`,
				Description: "Send short text message",
				Example:     `"ws" sends "ping"`,
				Handler:     r.sendText,
			},
			{
				Group:       "Sending",
				Pattern:     `^"{resource}" sends json:$`,
				Description: "Send JSON message",
				Example:     `"ws" sends json:`,
				Handler:     r.sendJSON,
			},

			// Receiving
			{
				Group:       "Receiving",
				Pattern:     `^"{resource}" receives within "([^"]*)":$`,
				Description: "Assert message received within timeout",
				Example:     `"ws" receives within "5s":`,
				Handler:     r.shouldReceiveMessage,
			},
			{
				Group:       "Receiving",
				Pattern:     `^"{resource}" receives within "([^"]*)" containing "([^"]*)"$`,
				Description: "Assert message containing substring received",
				Example:     `"ws" receives within "5s" containing "success"`,
				Handler:     r.shouldReceiveMessageContaining,
			},
			{
				Group:       "Receiving",
				Pattern:     `^"{resource}" receives json within "([^"]*)" matching:$`,
				Description: "Assert JSON message matching structure",
				Example:     `"ws" receives json within "5s" matching:`,
				Handler:     r.shouldReceiveJSONMatching,
			},
			{
				Group:       "Receiving",
				Pattern:     `^"{resource}" receives "(\d+)" messages within "([^"]*)"$`,
				Description: "Assert N messages received within timeout",
				Example:     `"ws" receives "3" messages within "10s"`,
				Handler:     r.shouldReceiveNMessages,
			},
			{
				Group:       "Receiving",
				Pattern:     `^"{resource}" does not receive within "([^"]*)"$`,
				Description: "Assert no message received",
				Example:     `"ws" does not receive within "2s"`,
				Handler:     r.shouldNotReceiveMessage,
			},

			// Assertions
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" last message is:$`,
				Description: "Assert last message matches exactly",
				Example:     `"ws" last message is:`,
				Handler:     r.lastMessageShouldBe,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" last message contains "([^"]*)"$`,
				Description: "Assert last message contains substring",
				Example:     `"ws" last message contains "success"`,
				Handler:     r.lastMessageShouldContain,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" last message is json matching:$`,
				Description: "Assert last message is JSON matching structure",
				Example:     `"ws" last message is json matching:`,
				Handler:     r.lastMessageShouldBeJSONMatching,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" received "(\d+)" messages$`,
				Description: "Assert total message count",
				Example:     `"ws" received "5" messages`,
				Handler:     r.shouldHaveReceivedNMessages,
			},
		},
	}
}

func (r *WebSocketClient) connect() error {
	return r.connectWithHeaders(nil)
}

func (r *WebSocketClient) connectWithHeaders(table *godog.Table) error {
	if r.connected.Load() {
		return nil
	}

	headers := r.headers.Clone()
	if table != nil {
		for _, row := range table.Rows[1:] {
			if len(row.Cells) >= 2 {
				headers.Set(row.Cells[0].Value, row.Cells[1].Value)
			}
		}
	}

	conn, _, err := r.dialer.Dial(r.url, headers)
	if err != nil {
		return fmt.Errorf("connecting to websocket: %w", err)
	}

	r.conn = conn
	r.connected.Store(true)

	r.readCtx, r.readCancel = context.WithCancel(context.Background())
	go r.readLoop(r.readCtx, conn)

	return nil
}

// readLoop reads from conn until it closes. It takes the connection as an
// argument so a disconnect (which nils r.conn) can't race it.
func (r *WebSocketClient) readLoop(ctx context.Context, conn *websocket.Conn) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					r.connected.Store(false)
				}
				return
			}

			r.messagesMu.Lock()
			r.messages = append(r.messages, message)
			r.lastMessage = message
			r.messagesMu.Unlock()
		}
	}
}

func (r *WebSocketClient) disconnectStep() error {
	r.disconnect()
	return nil
}

func (r *WebSocketClient) shouldBeConnected() error {
	if !r.connected.Load() {
		return fmt.Errorf("websocket is not connected")
	}
	return nil
}

func (r *WebSocketClient) shouldBeDisconnected() error {
	if r.connected.Load() {
		return fmt.Errorf("websocket is still connected")
	}
	return nil
}

func (r *WebSocketClient) sendMessage(doc *godog.DocString) error {
	if !r.connected.Load() {
		if err := r.connect(); err != nil {
			return err
		}
	}
	return r.conn.WriteMessage(websocket.TextMessage, []byte(doc.Content))
}

func (r *WebSocketClient) sendText(text string) error {
	if !r.connected.Load() {
		if err := r.connect(); err != nil {
			return err
		}
	}
	return r.conn.WriteMessage(websocket.TextMessage, []byte(text))
}

func (r *WebSocketClient) sendJSON(doc *godog.DocString) error {
	var js json.RawMessage
	if err := json.Unmarshal([]byte(doc.Content), &js); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	if !r.connected.Load() {
		if err := r.connect(); err != nil {
			return err
		}
	}
	return r.conn.WriteMessage(websocket.TextMessage, []byte(doc.Content))
}

func (r *WebSocketClient) shouldReceiveMessage(timeout string, doc *godog.DocString) error {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	if !r.connected.Load() {
		if err := r.connect(); err != nil {
			return err
		}
	}

	message, err := r.waitForMessage(duration)
	if err != nil {
		return err
	}

	expected := strings.TrimSpace(doc.Content)
	actual := strings.TrimSpace(string(message))
	if actual != expected {
		return fmt.Errorf("message mismatch:\nexpected: %s\nactual: %s", expected, actual)
	}

	return nil
}

func (r *WebSocketClient) shouldReceiveMessageContaining(timeout, substr string) error {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	if !r.connected.Load() {
		if err := r.connect(); err != nil {
			return err
		}
	}

	message, err := r.waitForMessage(duration)
	if err != nil {
		return err
	}

	if !strings.Contains(string(message), substr) {
		return fmt.Errorf("message does not contain %q: %s", substr, string(message))
	}

	return nil
}

func (r *WebSocketClient) shouldReceiveJSONMatching(timeout string, doc *godog.DocString) error {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	if !r.connected.Load() {
		if err := r.connect(); err != nil {
			return err
		}
	}

	message, err := r.waitForMessage(duration)
	if err != nil {
		return err
	}

	var expected, actual interface{}
	if err := json.Unmarshal([]byte(doc.Content), &expected); err != nil {
		return fmt.Errorf("invalid expected JSON: %w", err)
	}
	if err := json.Unmarshal(message, &actual); err != nil {
		return fmt.Errorf("invalid message JSON: %w", err)
	}

	expectedJSON, _ := json.Marshal(expected)
	actualJSON, _ := json.Marshal(actual)
	if string(expectedJSON) != string(actualJSON) {
		return fmt.Errorf("JSON mismatch:\nexpected: %s\nactual: %s", string(expectedJSON), string(actualJSON))
	}

	return nil
}

func (r *WebSocketClient) shouldReceiveNMessages(count int, timeout string) error {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	if !r.connected.Load() {
		if err := r.connect(); err != nil {
			return err
		}
	}

	deadline := time.Now().Add(duration)
	for {
		r.messagesMu.Lock()
		unread := len(r.messages) - r.consumed
		if unread >= count {
			r.consumed += count
			r.messagesMu.Unlock()
			return nil
		}
		r.messagesMu.Unlock()
		if !time.Now().Before(deadline) {
			return fmt.Errorf("expected %d messages, received %d within %s", count, unread, timeout)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (r *WebSocketClient) shouldNotReceiveMessage(timeout string) error {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	if !r.connected.Load() {
		if err := r.connect(); err != nil {
			return err
		}
	}

	time.Sleep(duration)

	r.messagesMu.RLock()
	defer r.messagesMu.RUnlock()
	if len(r.messages) > r.consumed {
		return fmt.Errorf("received unexpected message: %s", string(r.messages[r.consumed]))
	}
	return nil
}

// waitForMessage returns the next message no receive step has matched yet,
// waiting up to timeout for one to arrive. Messages that arrived before the
// step started count: a fast echo reply is not lost to the race between the
// send step and this one.
func (r *WebSocketClient) waitForMessage(timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	for {
		r.messagesMu.Lock()
		if len(r.messages) > r.consumed {
			msg := r.messages[r.consumed]
			r.consumed++
			r.messagesMu.Unlock()
			return msg, nil
		}
		r.messagesMu.Unlock()
		if !time.Now().Before(deadline) {
			return nil, fmt.Errorf("no message received within %s", timeout)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (r *WebSocketClient) getMessageCount() int {
	r.messagesMu.RLock()
	defer r.messagesMu.RUnlock()
	return len(r.messages)
}

func (r *WebSocketClient) lastMessageShouldBe(doc *godog.DocString) error {
	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()

	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	expected := strings.TrimSpace(doc.Content)
	actual := strings.TrimSpace(string(lastMsg))
	if actual != expected {
		return fmt.Errorf("message mismatch:\nexpected: %s\nactual: %s", expected, actual)
	}

	return nil
}

func (r *WebSocketClient) lastMessageShouldContain(substr string) error {
	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()

	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	if !strings.Contains(string(lastMsg), substr) {
		return fmt.Errorf("message does not contain %q: %s", substr, string(lastMsg))
	}

	return nil
}

func (r *WebSocketClient) lastMessageShouldBeJSONMatching(doc *godog.DocString) error {
	r.messagesMu.RLock()
	lastMsg := r.lastMessage
	r.messagesMu.RUnlock()

	if lastMsg == nil {
		return fmt.Errorf("no message received")
	}

	var expected, actual interface{}
	if err := json.Unmarshal([]byte(doc.Content), &expected); err != nil {
		return fmt.Errorf("invalid expected JSON: %w", err)
	}
	if err := json.Unmarshal(lastMsg, &actual); err != nil {
		return fmt.Errorf("invalid message JSON: %w", err)
	}

	expectedJSON, _ := json.Marshal(expected)
	actualJSON, _ := json.Marshal(actual)
	if string(expectedJSON) != string(actualJSON) {
		return fmt.Errorf("JSON mismatch:\nexpected: %s\nactual: %s", string(expectedJSON), string(actualJSON))
	}

	return nil
}

func (r *WebSocketClient) shouldHaveReceivedNMessages(count int) error {
	actual := r.getMessageCount()
	if actual != count {
		return fmt.Errorf("expected %d messages, got %d", count, actual)
	}
	return nil
}

// WebSocketClient interface implementation
func (r *WebSocketClient) Connect(ctx context.Context, headers map[string]string) error {
	h := r.headers.Clone()
	for k, v := range headers {
		h.Set(k, v)
	}

	conn, _, err := r.dialer.Dial(r.url, h)
	if err != nil {
		return err
	}

	r.conn = conn
	r.connected.Store(true)
	r.readCtx, r.readCancel = context.WithCancel(ctx)
	go r.readLoop(r.readCtx, conn)

	return nil
}

func (r *WebSocketClient) Send(ctx context.Context, message []byte) error {
	if !r.connected.Load() {
		return fmt.Errorf("not connected")
	}
	return r.conn.WriteMessage(websocket.TextMessage, message)
}

func (r *WebSocketClient) Receive(ctx context.Context, timeout int) ([]byte, error) {
	return r.waitForMessage(time.Duration(timeout) * time.Second)
}

func (r *WebSocketClient) Disconnect(ctx context.Context) error {
	r.disconnect()
	return nil
}

func (r *WebSocketClient) Cleanup(ctx context.Context) error {
	r.disconnect()
	return nil
}

var _ Handler = (*WebSocketClient)(nil)
var _ WebSocketClientInterface = (*WebSocketClient)(nil)
