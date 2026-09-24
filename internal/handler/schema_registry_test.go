package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const orderSchema = `{
  "type": "record",
  "name": "Order",
  "namespace": "com.example",
  "fields": [
    {"name": "id", "type": "string"},
    {"name": "amount", "type": "long"},
    {"name": "note", "type": ["null", "string"], "default": null}
  ]
}`

// fakeRegistry implements the three Schema Registry endpoints tomato uses.
type fakeRegistry struct {
	mu       sync.Mutex
	schemas  map[int]string
	subjects map[string]int
}

func newFakeRegistry() *fakeRegistry {
	return &fakeRegistry{schemas: map[int]string{}, subjects: map[string]int{}}
}

func (f *fakeRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := req.URL.Path
	switch {
	case req.Method == http.MethodGet && path == "/subjects":
		json.NewEncoder(w).Encode([]string{})
	case req.Method == http.MethodPost && strings.HasSuffix(path, "/versions"):
		subject := strings.TrimSuffix(strings.TrimPrefix(path, "/subjects/"), "/versions")
		var body struct {
			Schema string `json:"schema"`
		}
		json.NewDecoder(req.Body).Decode(&body)
		id := len(f.schemas) + 1
		f.schemas[id] = body.Schema
		f.subjects[subject] = id
		json.NewEncoder(w).Encode(map[string]int{"id": id})
	case req.Method == http.MethodGet && strings.HasSuffix(path, "/versions/latest"):
		subject := strings.TrimSuffix(strings.TrimPrefix(path, "/subjects/"), "/versions/latest")
		id, ok := f.subjects[subject]
		if !ok {
			http.Error(w, `{"error_code":40401,"message":"Subject not found."}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"id": id, "schema": f.schemas[id]})
	case req.Method == http.MethodGet && strings.HasPrefix(path, "/schemas/ids/"):
		var id int
		json.Unmarshal([]byte(strings.TrimPrefix(path, "/schemas/ids/")), &id)
		schema, ok := f.schemas[id]
		if !ok {
			http.Error(w, `{"error_code":40403,"message":"Schema not found"}`, http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"schema": schema})
	default:
		http.NotFound(w, req)
	}
}

func TestSchemaRegistry_RoundTrip(t *testing.T) {
	srv := httptest.NewServer(newFakeRegistry())
	defer srv.Close()
	ctx := context.Background()

	reg := newSchemaRegistry(srv.URL)
	id, err := reg.register(ctx, "orders-value", orderSchema)
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	value, err := reg.encode(ctx, "orders-value", `{"id": "order-1", "amount": 42, "note": "rush"}`)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	gotID, _, err := splitWireFormat(value)
	if err != nil {
		t.Fatalf("encoded value is not wire format: %v", err)
	}
	if gotID != id {
		t.Errorf("wire format carries schema id %d, want %d", gotID, id)
	}

	// A fresh client knows nothing, so decode must fetch the schema by id.
	text, err := newSchemaRegistry(srv.URL).decode(ctx, value)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(text), &got); err != nil {
		t.Fatalf("decoded JSON is invalid: %v (%s)", err, text)
	}
	want := map[string]any{"id": "order-1", "amount": float64(42), "note": "rush"}
	if err := CompareJSON(any(want), any(got), "", false); err != nil {
		t.Errorf("round trip mismatch: %v (decoded %s)", err, text)
	}
}

func TestSchemaRegistry_StandardJSONUnions(t *testing.T) {
	srv := httptest.NewServer(newFakeRegistry())
	defer srv.Close()
	ctx := context.Background()

	reg := newSchemaRegistry(srv.URL)
	if _, err := reg.register(ctx, "orders-value", orderSchema); err != nil {
		t.Fatalf("register: %v", err)
	}
	// Feature files write unions as plain JSON, not Avro-JSON {"string": ...}.
	value, err := reg.encode(ctx, "orders-value", `{"id": "o", "amount": 1, "note": null}`)
	if err != nil {
		t.Fatalf("encode with null union: %v", err)
	}
	text, err := reg.decode(ctx, value)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if strings.Contains(text, `{"string"`) {
		t.Errorf("decoded JSON should use standard unions, got %s", text)
	}
}

func TestSchemaRegistry_Errors(t *testing.T) {
	srv := httptest.NewServer(newFakeRegistry())
	defer srv.Close()
	ctx := context.Background()
	reg := newSchemaRegistry(srv.URL)

	if _, err := reg.encode(ctx, "missing-value", `{}`); err == nil || !strings.Contains(err.Error(), "missing-value") {
		t.Errorf("encoding for an unknown subject should name the subject, got %v", err)
	}
	if _, err := reg.register(ctx, "bad-value", `{"type": "nope"}`); err == nil {
		t.Error("registering an invalid schema should fail")
	}
	if _, err := reg.register(ctx, "orders-value", orderSchema); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := reg.encode(ctx, "orders-value", `{"id": "o"}`); err == nil {
		t.Error("JSON missing a required field should fail to encode")
	}
	if _, err := reg.decode(ctx, []byte(`{"id": "plain json"}`)); err == nil || !strings.Contains(err.Error(), "wire format") {
		t.Errorf("decoding a non-Avro value should explain the wire format, got %v", err)
	}
}

func TestKafkaValueSubject(t *testing.T) {
	k := &Kafka{}
	if got := k.valueSubject("orders"); got != "orders-value" {
		t.Errorf("default subject = %q, want orders-value", got)
	}
	k.config.Options = map[string]any{"schema_registry": map[string]any{
		"url":      "http://registry",
		"subjects": map[string]any{"orders": "com.example.Order"},
	}}
	if got := k.valueSubject("orders"); got != "com.example.Order" {
		t.Errorf("mapped subject = %q, want com.example.Order", got)
	}
	if got := k.valueSubject("payments"); got != "payments-value" {
		t.Errorf("unmapped topic subject = %q, want payments-value", got)
	}
}

func TestKafkaRequiresRegistryForAvroSteps(t *testing.T) {
	k := &Kafka{name: "events"}
	err := k.requireRegistry()
	if err == nil || !strings.Contains(err.Error(), "schema_registry") {
		t.Errorf("expected a hint about options.schema_registry, got %v", err)
	}
}
