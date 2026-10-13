package kafka

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/tomatool/tomato/internal/config"
)

const tradeSchema = `{"type":"record","name":"TradeEventV2","namespace":"portfolio","fields":[
  {"name":"tradeId","type":"string"},
  {"name":"quantity","type":"string"},
  {"name":"venue","type":["null","string"],"default":null}]}`

// The same schema, written differently: Glue must treat it as the same version.
const tradeSchemaReformatted = `{"namespace":"portfolio","name":"TradeEventV2","type":"record","fields":[{"name":"tradeId","type":"string"},{"name":"quantity","type":"string"},{"name":"venue","type":["null","string"],"default":null}]}`

func TestGlueFrame(t *testing.T) {
	id := uuid.NewString()
	for _, compress := range []bool{false, true} {
		value, err := glueFrame(id, []byte("avro-body"), compress)
		if err != nil {
			t.Fatal(err)
		}
		if value[0] != 0x03 {
			t.Errorf("header version: got 0x%02x", value[0])
		}
		wantComp := byte(0x00)
		if compress {
			wantComp = 0x05
		}
		if value[1] != wantComp {
			t.Errorf("compression byte: got 0x%02x, want 0x%02x", value[1], wantComp)
		}
		if got, _ := uuid.FromBytes(value[2:18]); got.String() != id {
			t.Errorf("schema version id: got %s", got)
		}
		gotID, body, err := splitGlueFrame(value)
		if err != nil || gotID != id || string(body) != "avro-body" {
			t.Errorf("round trip (compress %v): %s %q %v", compress, gotID, body, err)
		}
	}
	if _, err := glueFrame("not-a-uuid", nil, false); err == nil {
		t.Error("want an id error")
	}
}

func TestSplitGlueFrameErrors(t *testing.T) {
	id := uuid.New()
	cases := map[string][]byte{
		"too short":        {0x03, 0x00, 0x01},
		"confluent value":  append([]byte{0x00, 0, 0, 0, 1}, make([]byte, 20)...),
		"unknown compress": append(append([]byte{0x03, 0x07}, id[:]...), 'x'),
		"bad zlib":         append(append([]byte{0x03, 0x05}, id[:]...), 'x'),
	}
	for name, value := range cases {
		if _, _, err := splitGlueFrame(value); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestGlueRegistry_VersionsAndRoundTrip(t *testing.T) {
	g := newGlueRegistry("portfolio", "eu-central-1", false)
	ctx := context.Background()

	if err := g.registerSchema(ctx, "trade-events", tradeSchema); err != nil {
		t.Fatal(err)
	}
	v1, _ := g.latest("portfolio", "trade-events")
	if err := g.registerSchema(ctx, "trade-events", tradeSchemaReformatted); err != nil {
		t.Fatal(err)
	}
	if v, _ := g.latest("portfolio", "trade-events"); v != v1 {
		t.Fatalf("the same schema in another layout must stay version 1, got version %d", v.number)
	}
	if err := g.registerSchema(ctx, "trade-events", `{"type":"string"}`); err != nil {
		t.Fatal(err)
	}
	v2, _ := g.latest("portfolio", "trade-events")
	if v2.number != 2 || v2.id == v1.id {
		t.Fatalf("a new schema is version 2 with its own id: %+v", v2)
	}
	if err := g.registerSchema(ctx, "x", `{not a schema`); err == nil {
		t.Fatal("want an invalid schema error")
	}

	// Encode with the latest (version 2: a plain string), decode by its id.
	value, err := g.encode(ctx, "trade-events", `"hello"`)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := g.decode(ctx, value); err != nil || got != `"hello"` {
		t.Fatalf("decode: %s %v", got, err)
	}
	// A value written with version 1 still decodes.
	v1Value, _ := glueFrame(v1.id, mustBinary(t, v1, `{"tradeId":"t-1","quantity":"10","venue":"XETR"}`), true)
	got, err := g.decode(ctx, v1Value)
	if err != nil || !strings.Contains(got, `"venue":"XETR"`) {
		t.Fatalf("decode version 1: %s %v", got, err)
	}

	if _, err := g.encode(ctx, "missing", `{}`); err == nil || !strings.Contains(err.Error(), "register it first") {
		t.Errorf("want an unregistered schema error, got %v", err)
	}
	if _, err := g.encode(ctx, "trade-events", `{"wrong":1}`); err == nil {
		t.Error("want a JSON that does not match the schema error")
	}
	unknown, _ := glueFrame(uuid.NewString(), []byte{0}, false)
	if _, err := g.decode(ctx, unknown); err == nil || !strings.Contains(err.Error(), "not in tomato's Glue registry") {
		t.Errorf("want an unknown version error, got %v", err)
	}
	if _, err := g.decode(ctx, []byte{0}); err == nil {
		t.Error("want a wire format error")
	}
	if err := g.ready(ctx); err != nil {
		t.Error(err)
	}
	g.reset()
	if _, ok := g.latest("portfolio", "trade-events"); !ok {
		t.Error("schemas outlive a reset, as in a real registry")
	}
}

func mustBinary(t *testing.T, v *glueVersion, jsonText string) []byte {
	t.Helper()
	native, _, err := v.codec.NativeFromTextual([]byte(jsonText))
	if err != nil {
		t.Fatal(err)
	}
	b, err := v.codec.BinaryFromNative(nil, native)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// call invokes one Glue API operation the way the AWS SDK does.
func call(t *testing.T, g *glueRegistry, op string, in any) (int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(in)
	req := httptest.NewRequest("POST", "/", strings.NewReader(string(body)))
	req.Header.Set("X-Amz-Target", "AWSGlue."+op)
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	rec := httptest.NewRecorder()
	g.serve(rec, req)
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestGlueAPI(t *testing.T) {
	g := newGlueRegistry("portfolio", "eu-central-1", false)
	schemaID := map[string]string{"SchemaName": "trade-events", "RegistryName": "portfolio"}

	// The app's producer creates the schema, then looks it up by definition.
	code, out := call(t, g, "CreateSchema", map[string]any{
		"RegistryId": map[string]string{"RegistryName": "portfolio"}, "SchemaName": "trade-events",
		"DataFormat": "AVRO", "SchemaDefinition": tradeSchema,
	})
	if code != 200 || out["SchemaVersionId"] == "" || out["LatestSchemaVersion"] != float64(1) {
		t.Fatalf("CreateSchema: %d %v", code, out)
	}
	id := out["SchemaVersionId"].(string)
	arn := out["SchemaArn"].(string)
	if arn != "arn:aws:glue:eu-central-1:000000000000:schema/portfolio/trade-events" {
		t.Errorf("SchemaArn: %s", arn)
	}

	code, out = call(t, g, "GetSchemaByDefinition", map[string]any{"SchemaId": schemaID, "SchemaDefinition": tradeSchemaReformatted})
	if code != 200 || out["SchemaVersionId"] != id || out["Status"] != "AVAILABLE" {
		t.Errorf("GetSchemaByDefinition: %d %v", code, out)
	}
	code, out = call(t, g, "GetSchemaByDefinition", map[string]any{"SchemaId": schemaID, "SchemaDefinition": `{"type":"int"}`})
	if code != 400 || out["__type"] != "EntityNotFoundException" {
		t.Errorf("GetSchemaByDefinition, unknown: %d %v", code, out)
	}

	// The app's consumer resolves the id in a message.
	code, out = call(t, g, "GetSchemaVersion", map[string]any{"SchemaVersionId": id})
	if code != 200 || out["SchemaDefinition"] != tradeSchema || out["DataFormat"] != "AVRO" || out["VersionNumber"] != float64(1) {
		t.Errorf("GetSchemaVersion by id: %d %v", code, out)
	}

	code, out = call(t, g, "RegisterSchemaVersion", map[string]any{"SchemaId": map[string]string{"SchemaArn": arn}, "SchemaDefinition": `{"type":"string"}`})
	if code != 200 || out["VersionNumber"] != float64(2) {
		t.Fatalf("RegisterSchemaVersion by ARN: %d %v", code, out)
	}
	code, out = call(t, g, "GetSchemaVersion", map[string]any{"SchemaId": schemaID, "SchemaVersionNumber": map[string]any{"LatestVersion": true}})
	if code != 200 || out["VersionNumber"] != float64(2) {
		t.Errorf("GetSchemaVersion latest: %d %v", code, out)
	}
	code, out = call(t, g, "GetSchemaVersion", map[string]any{"SchemaId": schemaID, "SchemaVersionNumber": map[string]any{"VersionNumber": 1}})
	if code != 200 || out["SchemaVersionId"] != id {
		t.Errorf("GetSchemaVersion number 1: %d %v", code, out)
	}
	for name, in := range map[string]any{
		"unknown id":     map[string]any{"SchemaVersionId": uuid.NewString()},
		"unknown number": map[string]any{"SchemaId": schemaID, "SchemaVersionNumber": map[string]any{"VersionNumber": 9}},
		"unknown schema": map[string]any{"SchemaId": map[string]string{"SchemaName": "nope"}},
	} {
		if code, out := call(t, g, "GetSchemaVersion", in); code != 400 || out["__type"] != "EntityNotFoundException" {
			t.Errorf("GetSchemaVersion %s: %d %v", name, code, out)
		}
	}

	code, out = call(t, g, "GetSchema", map[string]any{"SchemaId": schemaID})
	if code != 200 || out["LatestSchemaVersion"] != float64(2) {
		t.Errorf("GetSchema: %d %v", code, out)
	}
	if code, out := call(t, g, "GetSchema", map[string]any{"SchemaId": map[string]string{"SchemaName": "nope"}}); code != 400 || out["__type"] != "EntityNotFoundException" {
		t.Errorf("GetSchema unknown: %d %v", code, out)
	}

	// Errors name what went wrong.
	if code, out := call(t, g, "CreateSchema", map[string]any{"SchemaName": "j", "DataFormat": "JSON", "SchemaDefinition": "{}"}); code != 400 || !strings.Contains(out["message"].(string), "AVRO only") {
		t.Errorf("CreateSchema JSON: %d %v", code, out)
	}
	if code, _ := call(t, g, "CreateSchema", map[string]any{"SchemaName": "bad", "SchemaDefinition": "{bad"}); code != 400 {
		t.Errorf("CreateSchema invalid schema: %d", code)
	}
	if code, _ := call(t, g, "RegisterSchemaVersion", map[string]any{"SchemaId": schemaID, "SchemaDefinition": "{bad"}); code != 400 {
		t.Errorf("RegisterSchemaVersion invalid schema: %d", code)
	}
	if code, out := call(t, g, "PutSchemaVersionMetadata", map[string]any{}); code != 400 || out["__type"] != "UnsupportedOperationException" || !strings.Contains(out["message"].(string), "PutSchemaVersionMetadata") {
		t.Errorf("unsupported operation: %d %v", code, out)
	}
	req := httptest.NewRequest("POST", "/", strings.NewReader("{not json"))
	req.Header.Set("X-Amz-Target", "AWSGlue.GetSchemaVersion")
	rec := httptest.NewRecorder()
	g.serve(rec, req)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "InvalidInputException") {
		t.Errorf("a body that is not JSON: %d %s", rec.Code, rec.Body)
	}
}

// The endpoint the app's SDK is pointed at answers over HTTP.
func TestGlueEndpoint(t *testing.T) {
	g := newGlueRegistry("", "", false)
	if err := g.start(0); err != nil {
		t.Fatal(err)
	}
	defer g.stop()
	if g.registry != "default-registry" || g.region != "us-east-1" {
		t.Errorf("defaults: %s %s", g.registry, g.region)
	}
	g.registerSchema(context.Background(), "s", `{"type":"string"}`)
	v, _ := g.latest("default-registry", "s")

	req, _ := http.NewRequest("POST", g.url(), strings.NewReader(`{"SchemaVersionId":"`+v.id+`"}`))
	req.Header.Set("X-Amz-Target", "AWSGlue.GetSchemaVersion")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "application/x-amz-json-1.1" {
		t.Fatalf("%d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if (&glueRegistry{}).url() != "" {
		t.Error("no URL before start")
	}
	busy := newGlueRegistry("", "", false)
	port := g.listener.Addr().(*net.TCPAddr).Port
	if err := busy.start(port); err == nil {
		t.Error("want a port in use error")
	}
}

func TestKafka_GlueConfig(t *testing.T) {
	glue, _ := New("events", config.Resource{Options: map[string]any{
		"schema_registry": map[string]any{"format": "glue"},
	}}, nil)
	confluent, _ := New("events", config.Resource{Options: map[string]any{
		"schema_registry": map[string]any{"url": "http://localhost:8081"},
	}}, nil)
	plain, _ := New("events", config.Resource{Options: map[string]any{}}, nil)

	if !glue.ProvidesAppEnv() || confluent.ProvidesAppEnv() || plain.ProvidesAppEnv() {
		t.Error("only a Glue registry provides app environment")
	}
	if glue.valueSubject("trades") != "trades" || confluent.valueSubject("trades") != "trades-value" {
		t.Errorf("subjects: glue %s, confluent %s", glue.valueSubject("trades"), confluent.valueSubject("trades"))
	}
	if len(glue.AppEnv().Set) != 0 {
		t.Error("no environment before Init")
	}
	glue.glue = newGlueRegistry("", "", false)
	glue.glue.start(0)
	defer glue.glue.stop()
	if got := glue.AppEnv().Set["AWS_ENDPOINT_URL_GLUE"]; got != glue.glue.url() || got == "" {
		t.Errorf("AWS_ENDPOINT_URL_GLUE = %q", got)
	}
}
