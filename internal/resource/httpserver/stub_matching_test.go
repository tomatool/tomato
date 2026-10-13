package httpserver

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v21"

	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/resource"
)

type reply struct {
	status int
	body   string
	ctype  string
}

func send(t *testing.T, srv *HTTPServer, method, path, body string, headers map[string]string) reply {
	t.Helper()
	req, _ := http.NewRequest(method, srv.GetURL()+path, strings.NewReader(body))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return reply{resp.StatusCode, string(b), resp.Header.Get("Content-Type")}
}

func table(rows ...[]string) *godog.Table {
	t := &godog.Table{}
	for _, r := range rows {
		row := &messages.PickleTableRow{}
		for _, v := range r {
			row.Cells = append(row.Cells, &messages.PickleTableCell{Value: v})
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

func startStubServer(t *testing.T) *HTTPServer {
	t.Helper()
	srv, _ := New("mock", config.Resource{Options: map[string]any{}}, nil)
	if err := srv.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Cleanup(context.Background()) })
	return srv
}

func doc(s string) *godog.DocString { return &godog.DocString{Content: s} }

func TestStub_ParseConditions(t *testing.T) {
	resource.SetVariable("target", "Kms.Decrypt")
	got := parseConditions(` with header "X-Amz-Target" "{{target}}" with query "page" "2" with body containing "resolveThread"`)
	want := []condition{
		{kind: "header", key: "X-Amz-Target", value: "Kms.Decrypt"},
		{kind: "query", key: "page", value: "2"},
		{kind: "body", value: "resolveThread"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("condition %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	if len(parseConditions("")) != 0 {
		t.Error("no conditions")
	}
	if s := want[0].String() + want[2].String(); !strings.Contains(s, `header "X-Amz-Target"`) || !strings.Contains(s, `body containing "resolveThread"`) {
		t.Errorf("String: %s", s)
	}
}

func TestStub_MatchesOnHeaderQueryAndBody(t *testing.T) {
	srv := startStubServer(t)
	srv.stubReturnsJSON("POST", "/", ` with header "X-Amz-Target" "Kms.Encrypt"`, 200, doc(`{"op":"encrypt"}`))
	srv.stubReturnsJSON("POST", "/", ` with header "X-Amz-Target" "Kms.Decrypt"`, 200, doc(`{"op":"decrypt"}`))
	srv.stubReturnsBody("GET", "/repos", ` with query "page" "2"`, 200, doc(`page two`))
	srv.stubReturnsBody("GET", "/repos", "", 200, doc(`page one`))
	srv.stubReturnsStatus("POST", "/graphql", ` with body containing "resolveThread"`, 204)

	if r := send(t, srv, "POST", "/", "", map[string]string{"X-Amz-Target": "Kms.Decrypt"}); r.body != `{"op":"decrypt"}` || r.ctype != "application/json" {
		t.Errorf("decrypt: %+v", r)
	}
	if r := send(t, srv, "POST", "/", "", map[string]string{"X-Amz-Target": "Kms.Encrypt"}); r.body != `{"op":"encrypt"}` {
		t.Errorf("encrypt: %+v", r)
	}
	if r := send(t, srv, "POST", "/", "", nil); r.status != 404 {
		t.Errorf("no header matches no stub: %+v", r)
	}
	if r := send(t, srv, "GET", "/repos?page=2", "", nil); r.body != "page two" {
		t.Errorf("query: %+v", r)
	}
	if r := send(t, srv, "GET", "/repos?page=3", "", nil); r.body != "page one" {
		t.Errorf("the unconditional stub answers the rest: %+v", r)
	}
	if r := send(t, srv, "POST", "/graphql", `{"query":"mutation resolveThread"}`, nil); r.status != 204 {
		t.Errorf("body: %+v", r)
	}
	if r := send(t, srv, "POST", "/graphql", `{"query":"other"}`, nil); r.status != 404 {
		t.Errorf("body mismatch: %+v", r)
	}
}

func TestStub_PrecedenceSpecificThenRecent(t *testing.T) {
	srv := startStubServer(t)
	srv.stubReturnsBody("GET", "/x", ` with header "A" "1"`, 200, doc(`specific`))
	srv.stubReturnsBody("GET", "/x", "", 200, doc(`background`))
	srv.stubReturnsBody("GET", "/x", "", 200, doc(`override`))

	if r := send(t, srv, "GET", "/x", "", map[string]string{"A": "1"}); r.body != "specific" {
		t.Errorf("more conditions win over a later stub: %+v", r)
	}
	if r := send(t, srv, "GET", "/x", "", nil); r.body != "override" {
		t.Errorf("the most recent of equally specific stubs wins: %+v", r)
	}
}

func TestStub_Sequence(t *testing.T) {
	srv := startStubServer(t)
	resource.SetVariable("id", "o-1")
	err := srv.stubReturnsSequence("POST", "/orders/{{id}}", "", table(
		[]string{"status", "json", "delay"},
		[]string{"503", "", ""},
		[]string{"201", `{"orderId":"o-1"}`, "200ms"},
		[]string{"208", `{"orderId":"o-1"}`, ""},
	))
	if err != nil {
		t.Fatal(err)
	}
	if r := send(t, srv, "POST", "/orders/o-1", "", nil); r.status != 503 {
		t.Errorf("first: %+v", r)
	}
	start := time.Now()
	r := send(t, srv, "POST", "/orders/o-1", "", nil)
	if r.status != 201 || r.ctype != "application/json" || time.Since(start) < 200*time.Millisecond {
		t.Errorf("second should come after 200ms as JSON: %+v after %s", r, time.Since(start))
	}
	for i := 0; i < 2; i++ {
		if r := send(t, srv, "POST", "/orders/o-1", "", nil); r.status != 208 {
			t.Errorf("the last row repeats: %+v", r)
		}
	}
}

func TestStub_SequenceBodyColumn(t *testing.T) {
	srv := startStubServer(t)
	if err := srv.stubReturnsSequence("GET", "/t", "", table([]string{"status", "body"}, []string{"200", "plain"})); err != nil {
		t.Fatal(err)
	}
	if r := send(t, srv, "GET", "/t", "", nil); r.body != "plain" || r.ctype == "application/json" {
		t.Errorf("%+v", r)
	}
}

func TestStub_SequenceTableErrors(t *testing.T) {
	srv := startStubServer(t)
	cases := map[string]*godog.Table{
		"no rows":       table([]string{"status"}),
		"no status":     table([]string{"json"}, []string{"{}"}),
		"json and body": table([]string{"status", "json", "body"}, []string{"200", "{}", "x"}),
		"unknown":       table([]string{"status", "headers"}, []string{"200", "x"}),
		"bad status":    table([]string{"status"}, []string{"ok"}),
		"bad json":      table([]string{"status", "json"}, []string{"200", "{bad"}),
		"bad delay":     table([]string{"status", "delay"}, []string{"200", "soon"}),
	}
	for name, tb := range cases {
		if err := srv.stubReturnsSequence("GET", "/x", "", tb); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if err := srv.stubReturnsJSON("GET", "/x", "", 200, doc(`{bad`)); err == nil {
		t.Error("invalid stub JSON")
	}
}

func TestStub_DelayEndsWhenTheClientGivesUp(t *testing.T) {
	srv := startStubServer(t)
	srv.stubReturnsSequence("GET", "/slow", "", table([]string{"status", "delay"}, []string{"200", "10s"}))

	client := &http.Client{Timeout: 100 * time.Millisecond}
	start := time.Now()
	if _, err := client.Get(srv.GetURL() + "/slow"); err == nil {
		t.Fatal("want a client timeout")
	}
	// The server must not hold the handler for 10s once the client is gone;
	// Cleanup's shutdown would otherwise wait on it.
	done := make(chan struct{})
	go func() { srv.Cleanup(context.Background()); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("a delayed response outlived its client")
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("too slow")
	}
}

func TestStub_RefuseAndAcceptConnections(t *testing.T) {
	srv := startStubServer(t)
	srv.stubReturnsStatus("GET", "/health", "", 200)

	// Leave an idle keep-alive connection behind; refusing must close it too.
	if r := send(t, srv, "GET", "/health", "", nil); r.status != 200 {
		t.Fatalf("%+v", r)
	}
	if err := srv.refuseConnections(); err != nil {
		t.Fatal(err)
	}
	if err := srv.refuseConnections(); err != nil {
		t.Fatalf("refusing twice is a no-op: %v", err)
	}
	if _, err := http.Get(srv.GetURL() + "/health"); err == nil {
		t.Fatal("want connection refused")
	}

	url := srv.GetURL()
	if err := srv.acceptConnections(); err != nil {
		t.Fatal(err)
	}
	if err := srv.acceptConnections(); err != nil {
		t.Fatalf("accepting twice is a no-op: %v", err)
	}
	if srv.GetURL() != url {
		t.Fatalf("the URL changed: %s -> %s", url, srv.GetURL())
	}
	if r := send(t, srv, "GET", "/health", "", nil); r.status != 200 {
		t.Fatalf("stubs are kept: %+v", r)
	}

	// Reset reopens the port for the next scenario.
	srv.refuseConnections()
	if err := srv.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r := send(t, srv, "GET", "/health", "", nil); r.status != 404 {
		t.Fatalf("reset accepts again with no stubs: %+v", r)
	}
}

func TestStub_ReceivedWithExactHeader(t *testing.T) {
	srv := startStubServer(t)
	srv.stubReturnsStatus("POST", "/", "", 200)
	if err := srv.receivedRequestWithExactHeader("POST", "/", "X-Amz-Target", "Kms.Encrypt"); err == nil || !strings.Contains(err.Error(), "no POST / request received") {
		t.Errorf("want none received, got %v", err)
	}
	send(t, srv, "POST", "/", "", map[string]string{"X-Amz-Target": "Kms.Decrypt"})
	resource.SetVariable("op", "Kms.Decrypt")
	if err := srv.receivedRequestWithExactHeader("POST", "/", "X-Amz-Target", "{{op}}"); err != nil {
		t.Fatal(err)
	}
	err := srv.receivedRequestWithExactHeader("POST", "/", "X-Amz-Target", "Kms.Encrypt")
	if err == nil || !strings.Contains(err.Error(), `"Kms.Decrypt"`) {
		t.Errorf("want the header values seen, got %v", err)
	}
	// Exact, not contains.
	if err := srv.receivedRequestWithExactHeader("POST", "/", "X-Amz-Target", "Kms"); err == nil {
		t.Error("a prefix must not match")
	}
}
