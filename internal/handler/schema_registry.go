package handler

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/linkedin/goavro/v2"
)

// avroMagicByte starts every value in the Confluent wire format:
// [0x00][4-byte big-endian schema id][Avro binary body].
const avroMagicByte = 0x00

// schemaRegistry is a minimal client for the Confluent Schema Registry REST
// API: register a schema, fetch the latest schema of a subject, and fetch a
// schema by id. It also encodes and decodes Avro values in the Confluent wire
// format, so feature files can speak plain JSON while the app sees the same
// bytes KafkaAvroSerializer and KafkaAvroDeserializer produce.
type schemaRegistry struct {
	baseURL string
	client  *http.Client

	mu     sync.Mutex
	byID   map[int]*goavro.Codec
	latest map[string]int // subject -> schema id, cached per scenario
}

func newSchemaRegistry(baseURL string) *schemaRegistry {
	return &schemaRegistry{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 10 * time.Second},
		byID:    make(map[int]*goavro.Codec),
		latest:  make(map[string]int),
	}
}

// reset forgets which schema is latest for each subject. Codecs by id are
// kept: an id always names the same schema.
func (s *schemaRegistry) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latest = make(map[string]int)
}

// ready checks that the registry answers.
func (s *schemaRegistry) ready(ctx context.Context) error {
	var subjects []string
	return s.do(ctx, http.MethodGet, "/subjects", nil, &subjects)
}

// register registers an Avro schema under subject and returns its id.
// Registering a schema that already exists returns the existing id.
func (s *schemaRegistry) register(ctx context.Context, subject, schema string) (int, error) {
	codec, err := newAvroCodec(schema)
	if err != nil {
		return 0, err
	}
	var resp struct {
		ID int `json:"id"`
	}
	body := map[string]string{"schema": schema, "schemaType": "AVRO"}
	if err := s.do(ctx, http.MethodPost, "/subjects/"+url.PathEscape(subject)+"/versions", body, &resp); err != nil {
		return 0, fmt.Errorf("registering schema for subject %q: %w", subject, err)
	}
	s.mu.Lock()
	s.byID[resp.ID] = codec
	s.latest[subject] = resp.ID
	s.mu.Unlock()
	return resp.ID, nil
}

// latestID returns the id of the latest schema registered under subject.
func (s *schemaRegistry) latestID(ctx context.Context, subject string) (int, error) {
	s.mu.Lock()
	id, ok := s.latest[subject]
	s.mu.Unlock()
	if ok {
		return id, nil
	}

	var resp struct {
		ID     int    `json:"id"`
		Schema string `json:"schema"`
	}
	if err := s.do(ctx, http.MethodGet, "/subjects/"+url.PathEscape(subject)+"/versions/latest", nil, &resp); err != nil {
		return 0, fmt.Errorf("fetching latest schema for subject %q: %w", subject, err)
	}
	codec, err := newAvroCodec(resp.Schema)
	if err != nil {
		return 0, fmt.Errorf("schema %d for subject %q: %w", resp.ID, subject, err)
	}
	s.mu.Lock()
	s.byID[resp.ID] = codec
	s.latest[subject] = resp.ID
	s.mu.Unlock()
	return resp.ID, nil
}

// codec returns the codec for a schema id, fetching the schema if needed.
func (s *schemaRegistry) codec(ctx context.Context, id int) (*goavro.Codec, error) {
	s.mu.Lock()
	codec, ok := s.byID[id]
	s.mu.Unlock()
	if ok {
		return codec, nil
	}

	var resp struct {
		Schema     string `json:"schema"`
		SchemaType string `json:"schemaType"`
	}
	if err := s.do(ctx, http.MethodGet, fmt.Sprintf("/schemas/ids/%d", id), nil, &resp); err != nil {
		return nil, fmt.Errorf("fetching schema %d: %w", id, err)
	}
	if resp.SchemaType != "" && resp.SchemaType != "AVRO" {
		return nil, fmt.Errorf("schema %d is %s; only Avro is supported", id, resp.SchemaType)
	}
	codec, err := newAvroCodec(resp.Schema)
	if err != nil {
		return nil, fmt.Errorf("schema %d: %w", id, err)
	}
	s.mu.Lock()
	s.byID[id] = codec
	s.mu.Unlock()
	return codec, nil
}

// encode turns a JSON document into a Confluent wire-format Avro value using
// the latest schema of subject.
func (s *schemaRegistry) encode(ctx context.Context, subject, jsonText string) ([]byte, error) {
	id, err := s.latestID(ctx, subject)
	if err != nil {
		return nil, err
	}
	codec, err := s.codec(ctx, id)
	if err != nil {
		return nil, err
	}
	return encodeAvro(codec, id, jsonText)
}

// decode turns a Confluent wire-format Avro value into a JSON document.
func (s *schemaRegistry) decode(ctx context.Context, value []byte) (string, error) {
	id, body, err := splitWireFormat(value)
	if err != nil {
		return "", err
	}
	codec, err := s.codec(ctx, id)
	if err != nil {
		return "", err
	}
	return decodeAvro(codec, body)
}

func (s *schemaRegistry) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")
	if in != nil {
		req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("schema registry %s %s: %d %s", method, path, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

// newAvroCodec builds a codec that reads and writes standard JSON: a union
// value is written as `"x"` or `null`, not Avro-JSON's `{"string": "x"}`,
// so feature files look like the JSON the app's DTOs serialise to.
func newAvroCodec(schema string) (*goavro.Codec, error) {
	codec, err := goavro.NewCodecForStandardJSONFull(schema)
	if err != nil {
		return nil, fmt.Errorf("invalid Avro schema: %w", err)
	}
	return codec, nil
}

func encodeAvro(codec *goavro.Codec, id int, jsonText string) ([]byte, error) {
	native, _, err := codec.NativeFromTextual([]byte(jsonText))
	if err != nil {
		return nil, fmt.Errorf("JSON does not match Avro schema %d: %w", id, err)
	}
	out := make([]byte, 5, 5+len(jsonText))
	out[0] = avroMagicByte
	binary.BigEndian.PutUint32(out[1:5], uint32(id))
	out, err = codec.BinaryFromNative(out, native)
	if err != nil {
		return nil, fmt.Errorf("encoding Avro with schema %d: %w", id, err)
	}
	return out, nil
}

func decodeAvro(codec *goavro.Codec, body []byte) (string, error) {
	native, _, err := codec.NativeFromBinary(body)
	if err != nil {
		return "", fmt.Errorf("decoding Avro: %w", err)
	}
	text, err := codec.TextualFromNative(nil, native)
	if err != nil {
		return "", fmt.Errorf("converting Avro to JSON: %w", err)
	}
	return string(text), nil
}

// splitWireFormat returns the schema id and Avro body of a Confluent
// wire-format value.
func splitWireFormat(value []byte) (int, []byte, error) {
	if len(value) < 5 || value[0] != avroMagicByte {
		return 0, nil, fmt.Errorf("value is not in Confluent Avro wire format (no magic byte and schema id)")
	}
	return int(binary.BigEndian.Uint32(value[1:5])), value[5:], nil
}
