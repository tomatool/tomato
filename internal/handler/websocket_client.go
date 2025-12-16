package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
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
	readCtx     context.Context
	readCancel  context.CancelFunc
	connected   bool
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
	r.connected = false
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
			// Connection management
			{
				Pattern:     `^"{resource}" connects$`,
				Description: "Connects to the WebSocket endpoint",
				Example:     `"{resource}" connects`,
				Handler:     r.connect,
			},
			{
				Pattern:     `^"{resource}" connects with headers:$`,
				Description: "Connects with custom headers",
				Example:     "\"{resource}\" connects with headers:\n  | header        | value       |\n  | Authorization | Bearer xyz  |",
				Handler:     r.connectWithHeaders,
			},
			{
				Pattern:     `^"{resource}" disconnects$`,
				Description: "Disconnects from the WebSocket",
				Example:     `"{resource}" disconnects`,
				Handler:     r.disconnectStep,
			},
			{
				Pattern:     `^"{resource}" is connected$`,
				Description: "Asserts the WebSocket is connected",
				Example:     `"{resource}" is connected`,
				Handler:     r.shouldBeConnected,
			},
			{
				Pattern:     `^"{resource}" is disconnected$`,
				Description: "Asserts the WebSocket is disconnected",
				Example:     `"{resource}" is disconnected`,
				Handler:     r.shouldBeDisconnected,
			},

			// Sending messages
			{
				Pattern:     `^"{resource}" sends:$`,
				Description: "Sends a text message",
				Example:     "\"{resource}\" sends:\n  \"\"\"\n  Hello Server\n  \"\"\"",
				Handler:     r.sendMessage,
			},
			{
				Pattern:     `^"{resource}" sends "([^"]*)"$`,
				Description: "Sends a short text message",
				Example:     `"{resource}" sends "ping"`,
				Handler:     r.sendText,
			},
			{
				Pattern:     `^"{resource}" sends json:$`,
				Description: "Sends a JSON message",
				Example:     "\"{resource}\" sends json:\n  \"\"\"\n  {\"action\": \"subscribe\"}\n  \"\"\"",
				Handler:     r.sendJSON,
			},

			// Receiving messages
			{
				Pattern:     `^"{resource}" receives within "([^"]*)":$`,
				Description: "Asserts a specific message is received within timeout",
				Example:     "\"{resource}\" receives within \"5s\":\n  \"\"\"\n  Hello Client\n  \"\"\"",
				Handler:     r.shouldReceiveMessage,
			},
			{
				Pattern:     `^"{resource}" receives within "([^"]*)" containing "([^"]*)"$`,
				Description: "Asserts a message containing substring is received",
				Example:     `"{resource}" receives within "5s" containing "success"`,
				Handler:     r.shouldReceiveMessageContaining,
			},
			{
				Pattern:     `^"{resource}" receives json within "([^"]*)" matching:$`,
				Description: "Asserts a JSON message matching structure is received",
				Example:     "\"{resource}\" receives json within \"5s\" matching:\n  \"\"\"\n  {\"status\": \"ok\"}\n  \"\"\"",
				Handler:     r.shouldReceiveJSONMatching,
			},
			{
				Pattern:     `^"{resource}" receives "(\d+)" messages within "([^"]*)"$`,
				Description: "Asserts N messages are received within timeout",
				Example:     `"{resource}" receives "3" messages within "10s"`,
				Handler:     r.shouldReceiveNMessages,
			},
			{
				Pattern:     `^"{resource}" does not receive within "([^"]*)"$`,
				Description: "Asserts no message is received within timeout",
				Example:     `"{resource}" does not receive within "2s"`,
				Handler:     r.shouldNotReceiveMessage,
			},

			// Last message assertions
			{
				Pattern:     `^"{resource}" last message is:$`,
				Description: "Asserts the last message matches exactly",
				Example:     "\"{resource}\" last message is:\n  \"\"\"\n  pong\n  \"\"\"",
				Handler:     r.lastMessageShouldBe,
			},
			{
				Pattern:     `^"{resource}" last message contains "([^"]*)"$`,
				Description: "Asserts the last message contains substring",
				Example:     `"{resource}" last message contains "success"`,
				Handler:     r.lastMessageShouldContain,
			},
			{
				Pattern:     `^"{resource}" last message is json matching:$`,
				Description: "Asserts the last message is JSON matching structure",
				Example:     "\"{resource}\" last message is json matching:\n  \"\"\"\n  {\"type\": \"response\"}\n  \"\"\"",
				Handler:     r.lastMessageShouldBeJSONMatching,
			},
			{
				Pattern:     `^"{resource}" received "(\d+)" messages$`,
				Description: "Asserts total messages received count",
				Example:     `"{resource}" received "5" messages`,
				Handler:     r.shouldHaveReceivedNMessages,
			},
		},
	}
}

func (r *WebSocketClient) connect() error {
	return r.connectWithHeaders(nil)
}

func (r *WebSocketClient) connectWithHeaders(table *godog.Table) error {
	if r.connected {
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
	r.connected = true

	r.readCtx, r.readCancel = context.WithCancel(context.Background())
	go r.readLoop()

	return nil
}

func (r *WebSocketClient) readLoop() {
	for {
		select {
		case <-r.readCtx.Done():
			return
		default:
			_, message, err := r.conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					r.connected = false
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
	if !r.connected {
		return fmt.Errorf("websocket is not connected")
	}
	return nil
}

func (r *WebSocketClient) shouldBeDisconnected() error {
	if r.connected {
		return fmt.Errorf("websocket is still connected")
	}
	return nil
}

func (r *WebSocketClient) sendMessage(doc *godog.DocString) error {
	if !r.connected {
		if err := r.connect(); err != nil {
			return err
		}
	}
	return r.conn.WriteMessage(websocket.TextMessage, []byte(doc.Content))
}

func (r *WebSocketClient) sendText(text string) error {
	if !r.connected {
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

	if !r.connected {
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

	if !r.connected {
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

	if !r.connected {
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

	if !r.connected {
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

	if !r.connected {
		if err := r.connect(); err != nil {
			return err
		}
	}

	deadline := time.Now().Add(duration)
	initialCount := r.getMessageCount()

	for time.Now().Before(deadline) {
		currentCount := r.getMessageCount()
		if currentCount-initialCount >= count {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	received := r.getMessageCount() - initialCount
	return fmt.Errorf("expected %d messages, received %d within %s", count, received, timeout)
}

func (r *WebSocketClient) shouldNotReceiveMessage(timeout string) error {
	duration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	if !r.connected {
		if err := r.connect(); err != nil {
			return err
		}
	}

	initialCount := r.getMessageCount()
	time.Sleep(duration)

	if r.getMessageCount() > initialCount {
		r.messagesMu.RLock()
		lastMsg := r.lastMessage
		r.messagesMu.RUnlock()
		return fmt.Errorf("received unexpected message: %s", string(lastMsg))
	}

	return nil
}

func (r *WebSocketClient) waitForMessage(timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	initialCount := r.getMessageCount()

	for time.Now().Before(deadline) {
		r.messagesMu.RLock()
		if len(r.messages) > initialCount {
			msg := r.messages[len(r.messages)-1]
			r.messagesMu.RUnlock()
			return msg, nil
		}
		r.messagesMu.RUnlock()
		time.Sleep(50 * time.Millisecond)
	}

	return nil, fmt.Errorf("no message received within %s", timeout)
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
	r.connected = true
	r.readCtx, r.readCancel = context.WithCancel(ctx)
	go r.readLoop()

	return nil
}

func (r *WebSocketClient) Send(ctx context.Context, message []byte) error {
	if !r.connected {
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
