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

type WebSocket struct {
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

func NewWebSocket(name string, cfg config.Resource, cm *container.Manager) (*WebSocket, error) {
	return &WebSocket{
		name:      name,
		config:    cfg,
		container: cm,
		headers:   make(http.Header),
		messages:  make([][]byte, 0),
	}, nil
}

func (r *WebSocket) Name() string { return r.name }

func (r *WebSocket) Init(ctx context.Context) error {
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

func (r *WebSocket) Ready(ctx context.Context) error {
	return nil
}

func (r *WebSocket) Reset(ctx context.Context) error {
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

func (r *WebSocket) disconnect() {
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

func (r *WebSocket) RegisterSteps(ctx *godog.ScenarioContext) {
	ctx.Step(fmt.Sprintf(`^I connect to websocket "%s"$`, r.name), r.connect)
	ctx.Step(fmt.Sprintf(`^I connect to websocket "%s" with headers:$`, r.name), r.connectWithHeaders)
	ctx.Step(fmt.Sprintf(`^I disconnect from websocket "%s"$`, r.name), r.disconnectStep)
	ctx.Step(fmt.Sprintf(`^websocket "%s" should be connected$`, r.name), r.shouldBeConnected)
	ctx.Step(fmt.Sprintf(`^websocket "%s" should be disconnected$`, r.name), r.shouldBeDisconnected)

	ctx.Step(fmt.Sprintf(`^I send message to websocket "%s":$`, r.name), r.sendMessage)
	ctx.Step(fmt.Sprintf(`^I send text "([^"]*)" to websocket "%s"$`, r.name), r.sendText)
	ctx.Step(fmt.Sprintf(`^I send JSON to websocket "%s":$`, r.name), r.sendJSON)

	ctx.Step(fmt.Sprintf(`^I should receive message from websocket "%s" within "([^"]*)":$`, r.name), r.shouldReceiveMessage)
	ctx.Step(fmt.Sprintf(`^I should receive message from websocket "%s" within "([^"]*)" containing "([^"]*)"$`, r.name), r.shouldReceiveMessageContaining)
	ctx.Step(fmt.Sprintf(`^I should receive JSON from websocket "%s" within "([^"]*)" matching:$`, r.name), r.shouldReceiveJSONMatching)
	ctx.Step(fmt.Sprintf(`^I should receive "(\d+)" messages from websocket "%s" within "([^"]*)"$`, r.name), r.shouldReceiveNMessages)
	ctx.Step(fmt.Sprintf(`^I should not receive message from websocket "%s" within "([^"]*)"$`, r.name), r.shouldNotReceiveMessage)

	ctx.Step(fmt.Sprintf(`^the last message from websocket "%s" should be:$`, r.name), r.lastMessageShouldBe)
	ctx.Step(fmt.Sprintf(`^the last message from websocket "%s" should contain "([^"]*)"$`, r.name), r.lastMessageShouldContain)
	ctx.Step(fmt.Sprintf(`^the last message from websocket "%s" should be JSON matching:$`, r.name), r.lastMessageShouldBeJSONMatching)
	ctx.Step(fmt.Sprintf(`^websocket "%s" should have received "(\d+)" messages$`, r.name), r.shouldHaveReceivedNMessages)
}

func (r *WebSocket) connect() error {
	return r.connectWithHeaders(nil)
}

func (r *WebSocket) connectWithHeaders(table *godog.Table) error {
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

func (r *WebSocket) readLoop() {
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

func (r *WebSocket) disconnectStep() error {
	r.disconnect()
	return nil
}

func (r *WebSocket) shouldBeConnected() error {
	if !r.connected {
		return fmt.Errorf("websocket is not connected")
	}
	return nil
}

func (r *WebSocket) shouldBeDisconnected() error {
	if r.connected {
		return fmt.Errorf("websocket is still connected")
	}
	return nil
}

func (r *WebSocket) sendMessage(doc *godog.DocString) error {
	if !r.connected {
		if err := r.connect(); err != nil {
			return err
		}
	}
	return r.conn.WriteMessage(websocket.TextMessage, []byte(doc.Content))
}

func (r *WebSocket) sendText(text string) error {
	if !r.connected {
		if err := r.connect(); err != nil {
			return err
		}
	}
	return r.conn.WriteMessage(websocket.TextMessage, []byte(text))
}

func (r *WebSocket) sendJSON(doc *godog.DocString) error {
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

func (r *WebSocket) shouldReceiveMessage(timeout string, doc *godog.DocString) error {
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

func (r *WebSocket) shouldReceiveMessageContaining(timeout, substr string) error {
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

func (r *WebSocket) shouldReceiveJSONMatching(timeout string, doc *godog.DocString) error {
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

func (r *WebSocket) shouldReceiveNMessages(count int, timeout string) error {
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

func (r *WebSocket) shouldNotReceiveMessage(timeout string) error {
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

func (r *WebSocket) waitForMessage(timeout time.Duration) ([]byte, error) {
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

func (r *WebSocket) getMessageCount() int {
	r.messagesMu.RLock()
	defer r.messagesMu.RUnlock()
	return len(r.messages)
}

func (r *WebSocket) lastMessageShouldBe(doc *godog.DocString) error {
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

func (r *WebSocket) lastMessageShouldContain(substr string) error {
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

func (r *WebSocket) lastMessageShouldBeJSONMatching(doc *godog.DocString) error {
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

func (r *WebSocket) shouldHaveReceivedNMessages(count int) error {
	actual := r.getMessageCount()
	if actual != count {
		return fmt.Errorf("expected %d messages, got %d", count, actual)
	}
	return nil
}

// WebSocketClient interface implementation
func (r *WebSocket) Connect(ctx context.Context, headers map[string]string) error {
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

func (r *WebSocket) Send(ctx context.Context, message []byte) error {
	if !r.connected {
		return fmt.Errorf("not connected")
	}
	return r.conn.WriteMessage(websocket.TextMessage, message)
}

func (r *WebSocket) Receive(ctx context.Context, timeout int) ([]byte, error) {
	return r.waitForMessage(time.Duration(timeout) * time.Second)
}

func (r *WebSocket) Disconnect(ctx context.Context) error {
	r.disconnect()
	return nil
}

func (r *WebSocket) Cleanup(ctx context.Context) error {
	r.disconnect()
	return nil
}

var _ Handler = (*WebSocket)(nil)
var _ WebSocketClient = (*WebSocket)(nil)
