package pdf

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/pdffixture"
	"github.com/tomatool/tomato/internal/resource"
)

// baseHandler is a resource that does nothing, to embed in fakes.
type baseHandler struct{ name string }

func (b baseHandler) Name() string                         { return b.name }
func (b baseHandler) Init(context.Context) error           { return nil }
func (b baseHandler) Ready(context.Context) error          { return nil }
func (b baseHandler) Reset(context.Context) error          { return nil }
func (b baseHandler) RegisterSteps(*godog.ScenarioContext) {}
func (b baseHandler) Cleanup(context.Context) error        { return nil }
func (b baseHandler) Steps() resource.StepCategory         { return resource.StepCategory{Name: "Fake"} }

// fakeStore holds objects, like the s3 resource.
type fakeStore struct {
	baseHandler
	objects map[string][]byte
	err     error
}

func (s *fakeStore) ReadObject(_ context.Context, path string) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}
	b, ok := s.objects[path]
	if !ok {
		return nil, fmt.Errorf("object %q does not exist", path)
	}
	return b, nil
}

// fakeClient keeps a last response body, like the http client.
type fakeClient struct {
	baseHandler
	body []byte
	err  error
}

func (c *fakeClient) LastResponseBody() ([]byte, error) { return c.body, c.err }

// plainHandler is a resource with neither.
type plainHandler struct{ baseHandler }

func (plainHandler) Steps() resource.StepCategory { return resource.StepCategory{Name: "Redis"} }

func lookupOf(handlers map[string]resource.Handler) resource.HandlerLookup {
	return func(name string) (resource.Handler, bool) {
		h, ok := handlers[name]
		return h, ok
	}
}

func newPDF(t *testing.T, opts map[string]any) *PDF {
	t.Helper()
	p, err := New("pdf", config.Resource{Type: "pdf", Options: opts}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p.SetHandlerLookup(lookupOf(map[string]resource.Handler{
		"files": &fakeStore{objects: map[string][]byte{"reports/cas/x.pdf": pdffixture.Contract(), "reports/broken.pdf": []byte("not a pdf")}},
		"api":   &fakeClient{body: pdffixture.Accessible()},
		"cache": plainHandler{},
		"empty": &fakeClient{err: errors.New(`"empty" has no response yet`)},
		"down":  &fakeStore{err: errors.New("connection refused")},
	}))
	return p
}

func withContract(t *testing.T) *PDF {
	t.Helper()
	p := newPDF(t, nil)
	if err := p.readsObject("files", "reports/cas/x.pdf"); err != nil {
		t.Fatal(err)
	}
	return p
}

func expectError(t *testing.T, err error, wants ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error mentioning %q", wants)
	}
	for _, w := range wants {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error %q lacks %q", err, w)
		}
	}
}

func TestNewOptions(t *testing.T) {
	p := newPDF(t, nil)
	if p.Name() != "pdf" || p.validatorName != "" || p.timeout != defaultValidatorTimeout {
		t.Errorf("defaults: %+v", p)
	}
	p = newPDF(t, map[string]any{"validator": " verapdf ", "timeout": "30s"})
	if p.validatorName != "verapdf" || p.timeout != 30*time.Second {
		t.Errorf("options: %+v", p)
	}
	if _, err := New("pdf", config.Resource{Options: map[string]any{"timeout": "soon"}}, nil); err == nil || !strings.Contains(err.Error(), "options.timeout") {
		t.Errorf("bad timeout: %v", err)
	}
	// The registry builds a throwaway handler for docs with a dummy config.
	if _, err := New("resource", resource.DummyConfig(), nil); err != nil {
		t.Error(err)
	}
	if err := p.Cleanup(context.Background()); err != nil {
		t.Error(err)
	}
}

func TestStepsAreDocumentedAndExamplesMatch(t *testing.T) {
	p := newPDF(t, nil)
	cat := p.Steps()
	if cat.Name != "PDF" || cat.Description == "" || len(cat.Steps) != 13 {
		t.Fatalf("category %q with %d steps", cat.Name, len(cat.Steps))
	}
	for _, s := range cat.Steps {
		if s.Group == "" || s.Description == "" || s.Example == "" || s.Handler == nil || !strings.Contains(s.Pattern, "{resource}") {
			t.Errorf("incomplete step %+v", s)
		}
		re := regexp.MustCompile(strings.ReplaceAll(s.Pattern, "{resource}", "pdf"))
		if !re.MatchString(strings.ReplaceAll(s.Example, "{resource}", "pdf")) {
			t.Errorf("example %q does not match its pattern %s", s.Example, s.Pattern)
		}
	}
}

func TestReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ca.pdf")
	if err := os.WriteFile(path, pdffixture.Contract(), 0o644); err != nil {
		t.Fatal(err)
	}
	p := newPDF(t, nil)
	resource.SetVariable("out", dir)
	if err := p.readsFile("{{out}}/ca.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := p.hasPages("5"); err != nil {
		t.Error(err)
	}
	expectError(t, p.readsFile(filepath.Join(dir, "missing.pdf")), "reading", "missing.pdf")
	os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hello"), 0o644)
	expectError(t, p.readsFile(filepath.Join(dir, "x.txt")), "is not a PDF")
	// A failed read keeps the previous document.
	if err := p.hasPages("5"); err != nil {
		t.Error(err)
	}
}

func TestReadsObjectAndResponseBody(t *testing.T) {
	p := newPDF(t, nil)
	if err := p.readsObject("files", "reports/cas/x.pdf"); err != nil {
		t.Fatal(err)
	}
	if err := p.hasPages("5"); err != nil {
		t.Error(err)
	}
	if err := p.readsResponseBody("api"); err != nil {
		t.Fatal(err)
	}
	if err := p.hasPages("1"); err != nil {
		t.Error(err)
	}
	expectError(t, p.readsObject("nope", "a/b"), `no resource named "nope"`)
	expectError(t, p.readsObject("cache", "a/b"), `resource "cache" cannot read objects`, "Redis resource, not an s3 resource")
	expectError(t, p.readsObject("api", "a/b"), `resource "api" cannot read objects`, "Fake resource")
	expectError(t, p.readsObject("files", "reports/none.pdf"), `reading "files" object "reports/none.pdf"`, "does not exist")
	expectError(t, p.readsObject("down", "a/b"), "connection refused")
	expectError(t, p.readsObject("files", "reports/broken.pdf"), `"files" object "reports/broken.pdf" is not a PDF`)
	expectError(t, p.readsResponseBody("cache"), `resource "cache" has no response body`, "not an http client")
	expectError(t, p.readsResponseBody("empty"), "has no response yet")
	expectError(t, p.readsResponseBody("missing"), `no resource named "missing"`)

	unwired := &PDF{name: "pdf"}
	expectError(t, unwired.readsObject("files", "a/b"), "cannot look up other resources")
	if describeHandler(plainHandler{}) != "Redis resource" {
		t.Error(describeHandler(plainHandler{}))
	}
}

func TestStepsWithoutADocument(t *testing.T) {
	p := newPDF(t, nil)
	for name, err := range map[string]error{
		"pages": p.hasPages("1"), "contains": p.textContains("x"), "not contains": p.textDoesNotContain("x"),
		"page": p.pageContains("1", "x"), "before": p.textAppearsBefore("a", "b"), "metadata": p.metadataIs("Title", "x"),
		"language": p.languageIs("de"), "outline": p.outlineContains("x"), "tagged": p.isTagged(), "conforms": p.conformsTo("PDF/UA-1"),
	} {
		if err == nil || !strings.Contains(err.Error(), `no document loaded: use "pdf" reads file`) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestResetDropsTheDocument(t *testing.T) {
	p := withContract(t)
	if err := p.Reset(context.Background()); err != nil {
		t.Fatal(err)
	}
	expectError(t, p.hasPages("5"), "no document loaded")
}

func TestPagesAndText(t *testing.T) {
	p := withContract(t)
	resource.SetVariable("pages", "5")
	if err := p.hasPages("{{pages}}"); err != nil {
		t.Error(err)
	}
	expectError(t, p.hasPages("142"), `expected 142 pages, the document from "files" object "reports/cas/x.pdf" has 5`)
	expectError(t, p.hasPages("many"), "is not a number")

	for _, want := range []string{"9 Kryptowerte", "Absichtlich frei gelassen", "Version 07.07", "1,00 €"} {
		if err := p.textContains(want); err != nil {
			t.Error(err)
		}
	}
	expectError(t, p.textContains("Kryptowährung"), `text does not contain "Kryptowährung"`, "document: 5 pages from", "characters of text", "text: Kundenvereinbarung")
	// Whitespace is not significant: a phrase across a line break, or spaced differently, matches.
	if err := p.textContains("Kundinnen und Kunden in Österreich. 1 Allgemeines"); err != nil {
		t.Error("across a line break:", err)
	}
	if err := p.pageContains("2", "9   Kryptowerte\nAbsichtlich"); err != nil {
		t.Error("spaced differently:", err)
	}
	if err := p.textDoesNotContain("{{"); err != nil {
		t.Error(err)
	}
	expectError(t, p.textDoesNotContain("frei gelassen"), `text contains "frei gelassen" on page 2`, "Absichtlich frei gelassen")

	if err := p.pageContains("3", "Inhaltsverzeichnis"); err != nil {
		t.Error(err)
	}
	expectError(t, p.pageContains("1", "Inhaltsverzeichnis"), `page 1 does not contain "Inhaltsverzeichnis"`)
	expectError(t, p.pageContains("9", "x"), "page 9 does not exist: the document has 5 pages")
	expectError(t, p.pageContains("three", "x"), `page "three" is not a number`)

	if err := p.textAppearsBefore("Conditions générales", "General Terms and Conditions"); err != nil {
		t.Error(err)
	}
	expectError(t, p.textAppearsBefore("General Terms and Conditions", "Allgemeine Geschäftsbedingungen"), `appears after "Allgemeine Geschäftsbedingungen", not before`, "comes first at page 1")
	expectError(t, p.textAppearsBefore("nope", "General Terms and Conditions"), `"nope" is not in the text ("General Terms and Conditions" is)`)
	expectError(t, p.textAppearsBefore("General Terms and Conditions", "nope"), `"nope" is not in the text ("General Terms and Conditions" is)`)
	expectError(t, p.textAppearsBefore("nope", "nada"), `neither "nope" nor "nada" is in the text`)
}

func TestMetadataLanguageOutline(t *testing.T) {
	p := withContract(t)
	if err := p.metadataIs("Title", "Kundenvereinbarung"); err != nil {
		t.Error(err)
	}
	if err := p.metadataIs("producer", "tomato fixtures"); err != nil {
		t.Error("keys are case-insensitive:", err)
	}
	expectError(t, p.metadataIs("Title", "AGB"), `metadata Title is "Kundenvereinbarung", not "AGB"`)
	expectError(t, p.metadataIs("Trapped", "False"), `no metadata entry "Trapped"; it has: Author, CreationDate`)

	if err := p.languageIs("de-at"); err != nil {
		t.Error(err)
	}
	expectError(t, p.languageIs("de-DE"), `language is "de-AT", not "de-DE"`)

	for _, title := range []string{"7 Zinsen", "7.1 Sollzinsen", " Inhaltsverzeichnis "} {
		if err := p.outlineContains(title); err != nil {
			t.Error(err)
		}
	}
	expectError(t, p.outlineContains("8 Gebühren"), `outline has no entry "8 Gebühren"; it has:`, "7 Zinsen")
	expectError(t, p.isTagged(), "is not tagged: no MarkInfo with Marked true, and no structure tree")

	// A document with neither language nor outline nor metadata.
	bare := newPDF(t, nil)
	if err := bare.load(onePage("", "BT /F1 10 Tf 10 100 Td (x) Tj ET"), "test"); err != nil {
		t.Fatal(err)
	}
	expectError(t, bare.languageIs("de"), "declares no language")
	expectError(t, bare.outlineContains("x"), "has no outline")
	expectError(t, bare.metadataIs("Title", "x"), `no metadata entry "Title": it has none at all`)
	if err := bare.load(onePage("/MarkInfo << /Marked true >>", ""), "test"); err != nil {
		t.Fatal(err)
	}
	expectError(t, bare.isTagged(), "declares itself tagged (MarkInfo Marked) but has no structure tree")
	if err := bare.load(onePage("/StructTreeRoot << /Type /StructTreeRoot >>", ""), "test"); err != nil {
		t.Fatal(err)
	}
	expectError(t, bare.isTagged(), "has a structure tree but does not declare itself tagged")
}

func TestAccessibleDocumentSteps(t *testing.T) {
	p := newPDF(t, nil)
	if err := p.readsResponseBody("api"); err != nil {
		t.Fatal(err)
	}
	for name, err := range map[string]error{
		"tagged": p.isTagged(), "language": p.languageIs("de-AT"), "title": p.metadataIs("Title", "Kundenvereinbarung"),
		"text": p.textContains("barrierefreie Fassung"), "producer": p.metadataIs("Producer", "tomato fixtures"),
	} {
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestConformsTo(t *testing.T) {
	p := withContract(t)
	expectError(t, p.conformsTo("PDF/UA-1"), `has no validator for "PDF/UA-1"`, "preset: verapdf", "options.validator")
	expectError(t, p.conformsTo("PDF/X-1a"), "unknown conformance level")

	srv, last, _ := fakeVeraPDF(t, 200, nonCompliant)
	p.validator = &validator{baseURL: srv.URL, client: srv.Client()}
	expectError(t, p.conformsTo("PDF/UA-1"), `does not conform to PDF/UA-1: 2 rules failed`, "7.21.4.1-1 (1 check): The font programs")
	if last.URL.Path != "/api/validate/ua1" {
		t.Errorf("posted to %s", last.URL.Path)
	}

	ok, _, _ := fakeVeraPDF(t, 200, `{"report":{"jobs":[{"validationResult":[{"compliant":true,"profileName":"PDF/A-2b validation profile","details":{"ruleSummaries":[]}}]}]}}`)
	p.validator = &validator{baseURL: ok.URL, client: ok.Client()}
	resource.SetVariable("level", "PDF/A-2b")
	if err := p.conformsTo("{{level}}"); err != nil {
		t.Error(err)
	}
	if err := p.conformsTo("2b"); err != nil {
		t.Error("veraPDF's own id is accepted:", err)
	}

	down, _, _ := fakeVeraPDF(t, 502, "bad gateway")
	p.validator = &validator{baseURL: down.URL, client: down.Client()}
	expectError(t, p.conformsTo("PDF/UA-1"), "502")
}

// fakeContainers answers host and port lookups.
type fakeContainers struct {
	host, port string
	hostErr    error
	portErr    error
	asked      []string
}

func (c *fakeContainers) GetHost(_ context.Context, name string) (string, error) {
	c.asked = append(c.asked, "host "+name)
	return c.host, c.hostErr
}

func (c *fakeContainers) GetPort(_ context.Context, name, port string) (string, error) {
	c.asked = append(c.asked, "port "+name+" "+port)
	return c.port, c.portErr
}

func TestInitAndReady(t *testing.T) {
	// No validator configured: nothing to do.
	p := newPDF(t, nil)
	if err := p.Init(context.Background()); err != nil || p.validator != nil {
		t.Errorf("init without validator: %v %v", err, p.validator)
	}
	if err := p.Ready(context.Background()); err != nil {
		t.Error(err)
	}

	// A validator named, but no containers (tomato validate, docs).
	p = newPDF(t, map[string]any{"validator": "verapdf"})
	expectError(t, p.Init(context.Background()), `options.validator names container "verapdf"`, "no containers are managed")

	srv, _, _ := fakeVeraPDF(t, 200, "{}")
	host, port, _ := strings.Cut(strings.TrimPrefix(srv.URL, "http://"), ":")
	cs := &fakeContainers{host: host, port: port}
	p.container = cs
	if err := p.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p.validator == nil || p.validator.baseURL != srv.URL || strings.Join(cs.asked, ",") != "host verapdf,port verapdf 8080/tcp" {
		t.Errorf("validator %+v asked %v", p.validator, cs.asked)
	}
	if err := p.Ready(context.Background()); err != nil {
		t.Error(err)
	}
	// Init is idempotent: resources are initialized again after the app starts.
	if err := p.Init(context.Background()); err != nil || len(cs.asked) != 2 {
		t.Errorf("second init: %v %v", err, cs.asked)
	}

	p = newPDF(t, map[string]any{"validator": "verapdf"})
	p.container = &fakeContainers{hostErr: errors.New("no such container")}
	expectError(t, p.Init(context.Background()), `validator container "verapdf": no such container`)
	p.container = &fakeContainers{host: "h", portErr: errors.New("port not exposed")}
	expectError(t, p.Init(context.Background()), `has no port 8080`)

	// Ready gives up when the context does.
	p.validator = &validator{baseURL: "http://127.0.0.1:1", client: &http.Client{Timeout: 200 * time.Millisecond}}
	p.validatorName = "verapdf"
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := p.Ready(ctx); err == nil {
		t.Error("Ready should fail when nobody answers")
	}
}

func TestAround(t *testing.T) {
	text := strings.Repeat("a", 50) + "MATCH" + strings.Repeat("b", 50)
	got := around(text, 50, 5)
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") || !strings.Contains(got, "MATCH") || len([]rune(got)) != 87 {
		t.Errorf("got %q (%d)", got, len([]rune(got)))
	}
	if got := around("short\nline", 0, 5); got != "short ⏎ line" {
		t.Errorf("got %q", got)
	}
}
