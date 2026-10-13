package kafka

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linkedin/goavro/v2"
)

// AWS Glue Schema Registry, as tomato serves it to the app and uses it itself.
//
// A Glue-serialised value is
//
//	[0x03 header version][0x00 none | 0x05 zlib][16-byte schema version UUID][Avro binary]
//
// and the app's deserializer resolves the UUID by calling Glue's
// GetSchemaVersion through the AWS SDK. tomato holds the schemas itself and
// answers that API on a local endpoint the app is pointed at with
// AWS_ENDPOINT_URL_GLUE, so the app fetches the schema the scenario
// registered, the way it does in production, and no real AWS account is
// involved. It implements the operations the AWS Glue SerDe library calls:
// GetSchemaVersion, GetSchemaByDefinition, CreateSchema, RegisterSchemaVersion
// and GetSchema. Signatures are not verified.

const (
	glueHeaderVersion   = 0x03
	glueCompressNone    = 0x00
	glueCompressZlib    = 0x05
	glueHeaderLen       = 18
	glueDefaultRegistry = "default-registry"
	glueAccount         = "000000000000"
)

type glueVersion struct {
	id        string
	number    int
	schema    *glueSchema
	text      string
	canonical string
	codec     *goavro.Codec
	created   time.Time
}

type glueSchema struct {
	registry string
	name     string
	versions []*glueVersion
}

func (s *glueSchema) arn(region string) string {
	return fmt.Sprintf("arn:aws:glue:%s:%s:schema/%s/%s", region, glueAccount, s.registry, s.name)
}

type glueRegistry struct {
	registry string // the registry steps register into
	region   string
	compress bool // write zlib-compressed values

	mu      sync.Mutex
	schemas map[string]*glueSchema // "registry/name"
	byID    map[string]*glueVersion

	listener net.Listener
	server   *http.Server
}

func newGlueRegistry(registry, region string, compress bool) *glueRegistry {
	if registry == "" {
		registry = glueDefaultRegistry
	}
	if region == "" {
		region = "us-east-1"
	}
	return &glueRegistry{
		registry: registry,
		region:   region,
		compress: compress,
		schemas:  make(map[string]*glueSchema),
		byID:     make(map[string]*glueVersion),
	}
}

// start serves the Glue API on port (0 picks one).
func (g *glueRegistry) start(port int) error {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return fmt.Errorf("starting the Glue Schema Registry endpoint: %w", err)
	}
	g.listener = l
	g.server = &http.Server{Handler: http.HandlerFunc(g.serve), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = g.server.Serve(l) }()
	return nil
}

func (g *glueRegistry) url() string {
	if g.listener == nil {
		return ""
	}
	return "http://" + g.listener.Addr().String()
}

func (g *glueRegistry) stop() {
	if g.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = g.server.Shutdown(ctx)
	}
}

// ready, reset and register make glueRegistry an avroRegistry. Schemas are
// kept across scenarios, as in a real registry: the app caches what it
// fetched by version id.
func (g *glueRegistry) ready(ctx context.Context) error { return nil }
func (g *glueRegistry) reset()                          {}

func (g *glueRegistry) registerSchema(ctx context.Context, subject, schema string) error {
	_, err := g.add(g.registry, subject, schema)
	return err
}

// add registers schema as a version of registry/name and returns it. A
// definition that is already a version of that schema returns that version.
func (g *glueRegistry) add(registry, name, schema string) (*glueVersion, error) {
	codec, err := newAvroCodec(schema)
	if err != nil {
		return nil, err
	}
	canonical := codec.CanonicalSchema()

	g.mu.Lock()
	defer g.mu.Unlock()
	key := registry + "/" + name
	s, ok := g.schemas[key]
	if !ok {
		s = &glueSchema{registry: registry, name: name}
		g.schemas[key] = s
	}
	for _, v := range s.versions {
		if v.canonical == canonical {
			return v, nil
		}
	}
	v := &glueVersion{
		id:        uuid.NewString(),
		number:    len(s.versions) + 1,
		schema:    s,
		text:      schema,
		canonical: canonical,
		codec:     codec,
		created:   time.Now().UTC(),
	}
	s.versions = append(s.versions, v)
	g.byID[v.id] = v
	return v, nil
}

func (g *glueRegistry) latest(registry, name string) (*glueVersion, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	s, ok := g.schemas[registry+"/"+name]
	if !ok || len(s.versions) == 0 {
		return nil, false
	}
	return s.versions[len(s.versions)-1], true
}

// encode turns JSON into a Glue wire-format value with the latest version of
// the schema named subject.
func (g *glueRegistry) encode(ctx context.Context, subject, jsonText string) ([]byte, error) {
	v, ok := g.latest(g.registry, subject)
	if !ok {
		return nil, fmt.Errorf("no schema %q in Glue registry %q; register it first", subject, g.registry)
	}
	native, _, err := v.codec.NativeFromTextual([]byte(jsonText))
	if err != nil {
		return nil, fmt.Errorf("JSON does not match schema %s version %d: %w", subject, v.number, err)
	}
	body, err := v.codec.BinaryFromNative(nil, native)
	if err != nil {
		return nil, fmt.Errorf("encoding Avro with schema %s version %d: %w", subject, v.number, err)
	}
	return glueFrame(v.id, body, g.compress)
}

func glueFrame(versionID string, body []byte, compress bool) ([]byte, error) {
	id, err := uuid.Parse(versionID)
	if err != nil {
		return nil, fmt.Errorf("schema version id %q is not a UUID: %w", versionID, err)
	}
	out := []byte{glueHeaderVersion, glueCompressNone}
	out = append(out, id[:]...)
	if !compress {
		return append(out, body...), nil
	}
	out[1] = glueCompressZlib
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write(body)
	w.Close()
	return append(out, buf.Bytes()...), nil
}

// splitGlueFrame returns the schema version id and the Avro body of a
// Glue wire-format value, decompressing it if needed.
func splitGlueFrame(value []byte) (string, []byte, error) {
	if len(value) < glueHeaderLen || value[0] != glueHeaderVersion {
		return "", nil, fmt.Errorf("value is not in the AWS Glue wire format (no 0x03 header version and schema version id)")
	}
	id, _ := uuid.FromBytes(value[2:glueHeaderLen])
	body := value[glueHeaderLen:]
	switch value[1] {
	case glueCompressNone:
		return id.String(), body, nil
	case glueCompressZlib:
		r, err := zlib.NewReader(bytes.NewReader(body))
		if err != nil {
			return "", nil, fmt.Errorf("decompressing a zlib Glue value: %w", err)
		}
		defer r.Close()
		plain, err := io.ReadAll(r)
		if err != nil {
			return "", nil, fmt.Errorf("decompressing a zlib Glue value: %w", err)
		}
		return id.String(), plain, nil
	default:
		return "", nil, fmt.Errorf("unknown Glue compression byte 0x%02x", value[1])
	}
}

func (g *glueRegistry) decode(ctx context.Context, value []byte) (string, error) {
	id, body, err := splitGlueFrame(value)
	if err != nil {
		return "", err
	}
	g.mu.Lock()
	v, ok := g.byID[id]
	g.mu.Unlock()
	if !ok {
		return "", fmt.Errorf("schema version %s is not in tomato's Glue registry", id)
	}
	return decodeAvro(v.codec, body)
}

// --- the Glue API, for the app ---

type glueSchemaID struct {
	SchemaArn    string `json:"SchemaArn,omitempty"`
	SchemaName   string `json:"SchemaName,omitempty"`
	RegistryName string `json:"RegistryName,omitempty"`
}

type glueRequest struct {
	SchemaVersionID string        `json:"SchemaVersionId"`
	SchemaID        *glueSchemaID `json:"SchemaId"`
	RegistryID      *struct {
		RegistryName string `json:"RegistryName"`
	} `json:"RegistryId"`
	SchemaName          string `json:"SchemaName"`
	SchemaDefinition    string `json:"SchemaDefinition"`
	DataFormat          string `json:"DataFormat"`
	SchemaVersionNumber *struct {
		LatestVersion bool `json:"LatestVersion"`
		VersionNumber int  `json:"VersionNumber"`
	} `json:"SchemaVersionNumber"`
}

// schemaRef resolves the schema a request names, by name or by ARN.
func (g *glueRegistry) schemaRef(id *glueSchemaID) (registry, name string) {
	registry = g.registry
	if id == nil {
		return registry, ""
	}
	if id.SchemaArn != "" {
		// arn:aws:glue:<region>:<account>:schema/<registry>/<name>
		if i := strings.Index(id.SchemaArn, ":schema/"); i >= 0 {
			if parts := strings.SplitN(id.SchemaArn[i+len(":schema/"):], "/", 2); len(parts) == 2 {
				return parts[0], parts[1]
			}
		}
	}
	if id.RegistryName != "" {
		registry = id.RegistryName
	}
	return registry, id.SchemaName
}

func (g *glueRegistry) serve(w http.ResponseWriter, req *http.Request) {
	target := req.Header.Get("X-Amz-Target")
	op := strings.TrimPrefix(target, "AWSGlue.")
	var in glueRequest
	if err := json.NewDecoder(req.Body).Decode(&in); err != nil && err != io.EOF {
		glueError(w, "InvalidInputException", "the request body is not JSON: "+err.Error())
		return
	}

	switch op {
	case "GetSchemaVersion":
		g.getSchemaVersion(w, in)
	case "GetSchemaByDefinition":
		registry, name := g.schemaRef(in.SchemaID)
		v, ok := g.findDefinition(registry, name, in.SchemaDefinition)
		if !ok {
			glueError(w, "EntityNotFoundException", fmt.Sprintf("no version of schema %s/%s has this definition", registry, name))
			return
		}
		glueJSON(w, map[string]any{
			"SchemaVersionId": v.id, "SchemaArn": v.schema.arn(g.region), "DataFormat": "AVRO",
			"Status": "AVAILABLE", "CreatedTime": v.created.Format(time.RFC3339),
		})
	case "CreateSchema":
		registry := g.registry
		if in.RegistryID != nil && in.RegistryID.RegistryName != "" {
			registry = in.RegistryID.RegistryName
		}
		if in.DataFormat != "" && in.DataFormat != "AVRO" {
			glueError(w, "InvalidInputException", "tomato's Glue registry supports AVRO only, not "+in.DataFormat)
			return
		}
		v, err := g.add(registry, in.SchemaName, in.SchemaDefinition)
		if err != nil {
			glueError(w, "InvalidInputException", err.Error())
			return
		}
		glueJSON(w, map[string]any{
			"RegistryName": registry, "SchemaName": in.SchemaName, "SchemaArn": v.schema.arn(g.region),
			"DataFormat": "AVRO", "Compatibility": "BACKWARD", "SchemaStatus": "AVAILABLE",
			"LatestSchemaVersion": v.number, "NextSchemaVersion": v.number + 1,
			"SchemaVersionId": v.id, "SchemaVersionStatus": "AVAILABLE",
		})
	case "RegisterSchemaVersion":
		registry, name := g.schemaRef(in.SchemaID)
		v, err := g.add(registry, name, in.SchemaDefinition)
		if err != nil {
			glueError(w, "InvalidInputException", err.Error())
			return
		}
		glueJSON(w, map[string]any{"SchemaVersionId": v.id, "VersionNumber": v.number, "Status": "AVAILABLE"})
	case "GetSchema":
		registry, name := g.schemaRef(in.SchemaID)
		v, ok := g.latest(registry, name)
		if !ok {
			glueError(w, "EntityNotFoundException", fmt.Sprintf("schema %s/%s not found", registry, name))
			return
		}
		glueJSON(w, map[string]any{
			"RegistryName": registry, "SchemaName": name, "SchemaArn": v.schema.arn(g.region), "DataFormat": "AVRO",
			"Compatibility": "BACKWARD", "SchemaStatus": "AVAILABLE",
			"LatestSchemaVersion": v.number, "NextSchemaVersion": v.number + 1,
		})
	default:
		glueError(w, "UnsupportedOperationException", fmt.Sprintf("tomato's Glue Schema Registry does not implement %q", target))
	}
}

func (g *glueRegistry) getSchemaVersion(w http.ResponseWriter, in glueRequest) {
	var v *glueVersion
	if in.SchemaVersionID != "" {
		g.mu.Lock()
		v = g.byID[in.SchemaVersionID]
		g.mu.Unlock()
	} else {
		registry, name := g.schemaRef(in.SchemaID)
		g.mu.Lock()
		if s, ok := g.schemas[registry+"/"+name]; ok && len(s.versions) > 0 {
			v = s.versions[len(s.versions)-1]
			if n := in.SchemaVersionNumber; n != nil && !n.LatestVersion && n.VersionNumber > 0 {
				v = nil
				if n.VersionNumber <= len(s.versions) {
					v = s.versions[n.VersionNumber-1]
				}
			}
		}
		g.mu.Unlock()
	}
	if v == nil {
		glueError(w, "EntityNotFoundException", "schema version not found")
		return
	}
	glueJSON(w, map[string]any{
		"SchemaVersionId": v.id, "SchemaDefinition": v.text, "DataFormat": "AVRO",
		"SchemaArn": v.schema.arn(g.region), "VersionNumber": v.number, "Status": "AVAILABLE",
		"CreatedTime": v.created.Format(time.RFC3339),
	})
}

func (g *glueRegistry) findDefinition(registry, name, definition string) (*glueVersion, bool) {
	codec, err := newAvroCodec(definition)
	if err != nil {
		return nil, false
	}
	canonical := codec.CanonicalSchema()
	g.mu.Lock()
	defer g.mu.Unlock()
	s, ok := g.schemas[registry+"/"+name]
	if !ok {
		return nil, false
	}
	for _, v := range s.versions {
		if v.canonical == canonical {
			return v, true
		}
	}
	return nil, false
}

func glueJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	json.NewEncoder(w).Encode(v)
}

func glueError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/x-amz-json-1.1")
	w.Header().Set("X-Amzn-ErrorType", code)
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"__type": code, "message": message, "Message": message})
}
