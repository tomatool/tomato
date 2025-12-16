package handler

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/cucumber/godog"
	"github.com/gorilla/websocket"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

// WebSocketServer provides a mock WebSocket server for testing
type WebSocketServer struct {
	name      string
	config    config.Resource
	container *container.Manager

	server   *http.Server
	listener net.Listener
	port     int
	upgrader websocket.Upgrader

	connections []*websocket.Conn
	connMu      sync.RWMutex

	onConnectMsg  string
	messageRules  []*MessageRule
	rulesMu       sync.RWMutex
	receivedMsgs  []string
	receivedMu    sync.RWMutex
}

// MessageRule defines how to respond to messages
type MessageRule struct {
	Pattern *regexp.Regexp
	Exact   string
	Reply   string
}

func NewWebSocketServer(name string, cfg config.Resource, cm *container.Manager) (*WebSocketServer, error) {
	return &WebSocketServer{
		name:         name,
		config:       cfg,
		container:    cm,
		connections:  make([]*websocket.Conn, 0),
		messageRules: make([]*MessageRule, 0),
		receivedMsgs: make([]string, 0),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}, nil
}

func (r *WebSocketServer) Name() string { return r.name }

func (r *WebSocketServer) Init(ctx context.Context) error {
	port := 0 // Let system assign a free port
	if p, ok := r.config.Options["port"].(int); ok {
		port = p
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("creating listener: %w", err)
	}
	r.listener = listener
	r.port = listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/", r.handleWebSocket)

	r.server = &http.Server{Handler: mux}

	go func() {
		if err := r.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Log error but don't fail
		}
	}()

	return nil
}

func (r *WebSocketServer) handleWebSocket(w http.ResponseWriter, req *http.Request) {
	conn, err := r.upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
	}

	r.connMu.Lock()
	r.connections = append(r.connections, conn)
	r.connMu.Unlock()

	// Send on-connect message if configured
	if r.onConnectMsg != "" {
		conn.WriteMessage(websocket.TextMessage, []byte(r.onConnectMsg))
	}

	// Handle messages
	go func() {
		defer func() {
			conn.Close()
			r.connMu.Lock()
			for i, c := range r.connections {
				if c == conn {
					r.connections = append(r.connections[:i], r.connections[i+1:]...)
					break
				}
			}
			r.connMu.Unlock()
		}()

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			msgStr := string(message)

			// Record the message
			r.receivedMu.Lock()
			r.receivedMsgs = append(r.receivedMsgs, msgStr)
			r.receivedMu.Unlock()

			// Check for matching rules
			r.rulesMu.RLock()
			for _, rule := range r.messageRules {
				matched := false
				if rule.Exact != "" && rule.Exact == msgStr {
					matched = true
				} else if rule.Pattern != nil && rule.Pattern.MatchString(msgStr) {
					matched = true
				}
				if matched && rule.Reply != "" {
					conn.WriteMessage(websocket.TextMessage, []byte(rule.Reply))
					break
				}
			}
			r.rulesMu.RUnlock()
		}
	}()
}

func (r *WebSocketServer) Ready(ctx context.Context) error {
	return nil
}

func (r *WebSocketServer) Reset(ctx context.Context) error {
	// Close all connections
	r.connMu.Lock()
	for _, conn := range r.connections {
		conn.Close()
	}
	r.connections = make([]*websocket.Conn, 0)
	r.connMu.Unlock()

	r.rulesMu.Lock()
	r.messageRules = make([]*MessageRule, 0)
	r.onConnectMsg = ""
	r.rulesMu.Unlock()

	r.receivedMu.Lock()
	r.receivedMsgs = make([]string, 0)
	r.receivedMu.Unlock()

	return nil
}

func (r *WebSocketServer) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the WebSocket server handler
func (r *WebSocketServer) Steps() StepCategory {
	return StepCategory{
		Name:        "WebSocket Server",
		Description: "Steps for stubbing WebSocket services",
		Steps: []StepDef{
			// Setup
			{
				Group:       "Setup",
				Pattern:     `^"{resource}" on connect sends:$`,
				Description: "Sends a message when a client connects",
				Example:     "\"{resource}\" on connect sends:\n  \"\"\"\n  {\"type\": \"welcome\"}\n  \"\"\"",
				Handler:     r.onConnectSends,
			},
			{
				Group:       "Setup",
				Pattern:     `^"{resource}" on message "([^"]*)" replies:$`,
				Description: "Replies to an exact message",
				Example:     "\"{resource}\" on message \"ping\" replies:\n  \"\"\"\n  pong\n  \"\"\"",
				Handler:     r.onMessageReplies,
			},
			{
				Group:       "Setup",
				Pattern:     `^"{resource}" on message matching "([^"]*)" replies:$`,
				Description: "Replies to messages matching a regex pattern",
				Example:     "\"{resource}\" on message matching \".*subscribe.*\" replies:\n  \"\"\"\n  {\"status\": \"subscribed\"}\n  \"\"\"",
				Handler:     r.onMessageMatchingReplies,
			},

			// Broadcast
			{
				Group:       "Broadcast",
				Pattern:     `^"{resource}" broadcasts:$`,
				Description: "Broadcasts a message to all connected clients",
				Example:     "\"{resource}\" broadcasts:\n  \"\"\"\n  {\"event\": \"update\"}\n  \"\"\"",
				Handler:     r.broadcast,
			},
			{
				Group:       "Broadcast",
				Pattern:     `^"{resource}" broadcasts "([^"]*)"$`,
				Description: "Broadcasts a short message to all connected clients",
				Example:     `"{resource}" broadcasts "ping"`,
				Handler:     r.broadcastText,
			},

			// Assertions
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" has "(\d+)" connections$`,
				Description: "Asserts the number of connected clients",
				Example:     `"{resource}" has "2" connections`,
				Handler:     r.hasConnections,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" received message "([^"]*)"$`,
				Description: "Asserts a specific message was received",
				Example:     `"{resource}" received message "ping"`,
				Handler:     r.receivedMessage,
			},
			{
				Group:       "Assertions",
				Pattern:     `^"{resource}" received "(\d+)" messages$`,
				Description: "Asserts the total number of messages received",
				Example:     `"{resource}" received "3" messages`,
				Handler:     r.receivedMessageCount,
			},
		},
	}
}

func (r *WebSocketServer) onConnectSends(doc *godog.DocString) error {
	r.rulesMu.Lock()
	defer r.rulesMu.Unlock()
	r.onConnectMsg = doc.Content
	return nil
}

func (r *WebSocketServer) onMessageReplies(message string, doc *godog.DocString) error {
	r.rulesMu.Lock()
	defer r.rulesMu.Unlock()
	r.messageRules = append(r.messageRules, &MessageRule{
		Exact: message,
		Reply: doc.Content,
	})
	return nil
}

func (r *WebSocketServer) onMessageMatchingReplies(pattern string, doc *godog.DocString) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}

	r.rulesMu.Lock()
	defer r.rulesMu.Unlock()
	r.messageRules = append(r.messageRules, &MessageRule{
		Pattern: re,
		Reply:   doc.Content,
	})
	return nil
}

func (r *WebSocketServer) broadcast(doc *godog.DocString) error {
	return r.broadcastText(doc.Content)
}

func (r *WebSocketServer) broadcastText(message string) error {
	r.connMu.RLock()
	defer r.connMu.RUnlock()

	for _, conn := range r.connections {
		conn.WriteMessage(websocket.TextMessage, []byte(message))
	}
	return nil
}

func (r *WebSocketServer) hasConnections(count int) error {
	r.connMu.RLock()
	defer r.connMu.RUnlock()

	if len(r.connections) != count {
		return fmt.Errorf("expected %d connections, got %d", count, len(r.connections))
	}
	return nil
}

func (r *WebSocketServer) receivedMessage(message string) error {
	r.receivedMu.RLock()
	defer r.receivedMu.RUnlock()

	for _, msg := range r.receivedMsgs {
		if msg == message {
			return nil
		}
	}
	return fmt.Errorf("message %q was not received", message)
}

func (r *WebSocketServer) receivedMessageCount(count int) error {
	r.receivedMu.RLock()
	defer r.receivedMu.RUnlock()

	if len(r.receivedMsgs) != count {
		return fmt.Errorf("expected %d messages, got %d", count, len(r.receivedMsgs))
	}
	return nil
}

// GetURL returns the server URL for use by other handlers
func (r *WebSocketServer) GetURL() string {
	return fmt.Sprintf("ws://localhost:%d", r.port)
}

func (r *WebSocketServer) Cleanup(ctx context.Context) error {
	// Close all connections first
	r.connMu.Lock()
	for _, conn := range r.connections {
		conn.Close()
	}
	r.connections = nil
	r.connMu.Unlock()

	if r.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return r.server.Shutdown(ctx)
	}
	return nil
}

var _ Handler = (*WebSocketServer)(nil)
