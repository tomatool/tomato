package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

var (
	db  *sql.DB
	rdb *redis.Client

	// WebSocket
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	wsClients   = make(map[*websocket.Conn]bool)
	wsClientsMu sync.Mutex
)

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	// Initialize database
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://test:test@localhost:5432/test?sslmode=disable"
	}

	var err error
	db, err = sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize Redis
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}
	rdb = redis.NewClient(opt)
	defer rdb.Close()

	// Create tables
	if err := initDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/users", usersHandler)
	mux.HandleFunc("/users/", userHandler)
	mux.HandleFunc("/cache", cacheHandler)
	mux.HandleFunc("/echo", echoHandler)
	mux.HandleFunc("/ws", wsHandler)
	mux.HandleFunc("/text", textHandler)
	mux.HandleFunc("/empty", emptyHandler)
	mux.HandleFunc("/login", loginHandler)
	mux.HandleFunc("/logout", logoutHandler)
	mux.HandleFunc("/profile", profileHandler)

	startGRPC()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	log.Printf("Server starting on port %s", port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}

// startGRPC serves grpc.health.v1.Health with reflection enabled, which is
// what the grpc resource's feature file drives.
//
// The health service is used rather than a bespoke one on purpose: it ships
// inside grpc-go, so the test app gains a real gRPC surface — unary method,
// enum response, NOT_FOUND path, and a streaming method to refuse — without
// adding a .proto file, protoc, or generated code to this repository.
func startGRPC() {
	addr := os.Getenv("GRPC_PORT")
	if addr == "" {
		addr = "9090"
	}

	lis, err := net.Listen("tcp", ":"+addr)
	if err != nil {
		log.Fatalf("gRPC listen error: %v", err)
	}

	srv := grpc.NewServer()
	hs := health.NewServer()
	hs.SetServingStatus("tomato", healthpb.HealthCheckResponse_SERVING)
	hs.SetServingStatus("degraded", healthpb.HealthCheckResponse_NOT_SERVING)
	healthpb.RegisterHealthServer(srv, hs)
	reflection.Register(srv)

	go func() {
		log.Printf("gRPC server starting on port %s", addr)
		if err := srv.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()
}

func initDB() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	// Check database
	if err := db.Ping(); err != nil {
		http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
		return
	}

	// Check Redis
	if err := rdb.Ping(r.Context()).Err(); err != nil {
		http.Error(w, "Redis unhealthy", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func usersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		listUsers(w, r)
	case http.MethodPost:
		createUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path /users/{id}
	idStr := r.URL.Path[len("/users/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		getUser(w, r, id)
	case http.MethodDelete:
		deleteUser(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func listUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, name, email FROM users ORDER BY id")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		users = append(users, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func getUser(w http.ResponseWriter, r *http.Request, id int) {
	// Try cache first
	ctx := r.Context()
	cacheKey := fmt.Sprintf("user:%d", id)

	cached, err := rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		w.Write([]byte(cached))
		return
	}

	// Query database
	var u User
	err = db.QueryRow("SELECT id, name, email FROM users WHERE id = $1", id).Scan(&u.ID, &u.Name, &u.Email)
	if err == sql.ErrNoRows {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Cache result
	data, _ := json.Marshal(u)
	rdb.Set(ctx, cacheKey, data, 5*time.Minute)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	w.Write(data)
}

func createUser(w http.ResponseWriter, r *http.Request) {
	var u User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err := db.QueryRow(
		"INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
		u.Name, u.Email,
	).Scan(&u.ID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(u)
}

func deleteUser(w http.ResponseWriter, r *http.Request, id int) {
	result, err := db.Exec("DELETE FROM users WHERE id = $1", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Invalidate cache
	rdb.Del(r.Context(), fmt.Sprintf("user:%d", id))

	w.WriteHeader(http.StatusNoContent)
}

func cacheHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "key parameter required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		val, err := rdb.Get(ctx, key).Result()
		if err == redis.Nil {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"key": key, "value": val})

	case http.MethodPost:
		var body struct {
			Value string `json:"value"`
			TTL   int    `json:"ttl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		ttl := time.Duration(body.TTL) * time.Second
		if ttl == 0 {
			ttl = time.Hour
		}
		if err := rdb.Set(ctx, key, body.Value, ttl).Err(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)

	case http.MethodDelete:
		rdb.Del(ctx, key)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"method":  r.Method,
		"path":    r.URL.Path,
		"query":   r.URL.Query(),
		"headers": r.Header,
	}

	cookies := map[string]string{}
	for _, c := range r.Cookies() {
		cookies[c.Name] = c.Value
	}
	response["cookies"] = cookies

	if r.Method == http.MethodPost || r.Method == http.MethodPut {
		raw, _ := io.ReadAll(r.Body)
		response["raw"] = string(raw)
		var body interface{}
		if err := json.Unmarshal(raw, &body); err == nil {
			response["body"] = body
		}
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			if form, err := url.ParseQuery(string(raw)); err == nil {
				flat := map[string]string{}
				for k := range form {
					flat[k] = form.Get(k)
				}
				response["form"] = flat
			}
		}
	}

	json.NewEncoder(w).Encode(response)
}

// textHandler returns a fixed plain-text body.
func textHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Tomato", "fresh")
	fmt.Fprint(w, "Hello from the \"tomato\" test app\nline two")
}

// emptyHandler answers with no body at all.
func emptyHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// loginHandler sets a session cookie and points at the session resource.
func loginHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "sess-42", Path: "/"})
	w.Header().Set("Location", "/echo?session=sess-42")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "logged in"})
}

// logoutHandler clears the session cookie.
func logoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1})
	w.WriteHeader(http.StatusNoContent)
}

// profileHandler returns a fixed profile with typed fields.
func profileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         "6f1c2a8e-3b4d-4c5e-9f60-7a8b9c0d1e2f",
		"email":      "ada@example.com",
		"name":       "Ada",
		"created_at": "2026-09-24T10:00:00Z",
		"tags":       []string{"admin", "beta"},
		"verified":   true,
		"age":        36,
	})
}

// WebSocket handler - echo server with broadcast support
func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Register client
	wsClientsMu.Lock()
	wsClients[conn] = true
	wsClientsMu.Unlock()

	defer func() {
		wsClientsMu.Lock()
		delete(wsClients, conn)
		wsClientsMu.Unlock()
	}()

	// Clients that identify themselves get a welcome with the id they sent,
	// which lets features check headers sent on the handshake.
	if id := r.Header.Get("X-Client-Id"); id != "" {
		welcome, _ := json.Marshal(map[string]string{"action": "welcome", "client": id})
		conn.WriteMessage(websocket.TextMessage, welcome)
	}

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Try to parse as JSON command
		var cmd struct {
			Action  string          `json:"action"`
			Payload json.RawMessage `json:"payload"`
		}

		if err := json.Unmarshal(message, &cmd); err == nil {
			switch cmd.Action {
			case "echo":
				// Echo back the payload
				response := map[string]interface{}{
					"action":  "echo",
					"payload": json.RawMessage(cmd.Payload),
				}
				respBytes, _ := json.Marshal(response)
				conn.WriteMessage(websocket.TextMessage, respBytes)

			case "broadcast":
				// Broadcast to all clients
				wsClientsMu.Lock()
				for client := range wsClients {
					if err := client.WriteMessage(messageType, message); err != nil {
						client.Close()
						delete(wsClients, client)
					}
				}
				wsClientsMu.Unlock()

			case "ping":
				// Respond with pong
				response := map[string]string{"action": "pong"}
				respBytes, _ := json.Marshal(response)
				conn.WriteMessage(websocket.TextMessage, respBytes)

			default:
				// Unknown action, echo back
				conn.WriteMessage(messageType, message)
			}
		} else {
			// Not JSON, just echo back
			conn.WriteMessage(messageType, message)
		}
	}
}
