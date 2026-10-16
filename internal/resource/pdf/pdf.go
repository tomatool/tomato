// Package pdf provides tomato's `pdf` resource: asserting on what is inside
// the PDF documents the application generates.
//
// A document is read from a file, from an object of an s3 resource, or from
// the last response of an http resource. The steps then assert on its page
// count, its text (on the document, on a page, and the order of two pieces
// of it), its metadata, language, outline, and accessibility: whether it is
// tagged, and whether it conforms to PDF/UA or PDF/A, which a veraPDF
// container checks.
//
// Parsing is pdfcpu; text extraction is this package's own (text.go), so a
// damaged font or a generator's quirk never hangs a step.
package pdf

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/resource"
)

// containerAddress is what the resource needs from the container manager.
type containerAddress interface {
	GetHost(ctx context.Context, name string) (string, error)
	GetPort(ctx context.Context, name, port string) (string, error)
}

// PDF asserts on generated PDF documents.
type PDF struct {
	name      string
	config    config.Resource
	container containerAddress
	lookup    resource.HandlerLookup

	validatorName string
	timeout       time.Duration
	validator     *validator

	doc *document
}

// defaultValidatorTimeout bounds one conformance check. veraPDF takes a few
// seconds for a long document, longer under emulation.
const defaultValidatorTimeout = 2 * time.Minute

func New(name string, cfg config.Resource, cm *container.Manager) (*PDF, error) {
	p := &PDF{name: name, config: cfg, timeout: defaultValidatorTimeout}
	if cm != nil {
		p.container = cm
	}
	if v, ok := cfg.Options["validator"].(string); ok {
		p.validatorName = strings.TrimSpace(v)
	}
	if t, ok := cfg.Options["timeout"].(string); ok && t != "" {
		d, err := time.ParseDuration(t)
		if err != nil {
			return nil, fmt.Errorf("pdf resource %q: options.timeout %q: %w", name, t, err)
		}
		p.timeout = d
	}
	return p, nil
}

func (p *PDF) Name() string { return p.name }

// SetHandlerLookup receives the registry's lookup, for reading an s3 object
// or an http response body (resource.HandlerLookupAware).
func (p *PDF) SetHandlerLookup(lookup resource.HandlerLookup) { p.lookup = lookup }

// Init resolves the validator container, when one is configured.
func (p *PDF) Init(ctx context.Context) error {
	if p.validatorName == "" || p.validator != nil {
		return nil
	}
	if p.container == nil {
		return fmt.Errorf("pdf resource %q: options.validator names container %q, but no containers are managed", p.name, p.validatorName)
	}
	host, err := p.container.GetHost(ctx, p.validatorName)
	if err != nil {
		return fmt.Errorf("pdf resource %q: validator container %q: %w", p.name, p.validatorName, err)
	}
	port, err := p.container.GetPort(ctx, p.validatorName, config.VeraPDFPort+"/tcp")
	if err != nil {
		return fmt.Errorf("pdf resource %q: validator container %q has no port %s: %w", p.name, p.validatorName, config.VeraPDFPort, err)
	}
	p.validator = &validator{baseURL: "http://" + host + ":" + port, client: &http.Client{Timeout: p.timeout}}
	return nil
}

// Ready checks the validator answers.
func (p *PDF) Ready(ctx context.Context) error {
	if p.validator == nil {
		return nil
	}
	var err error
	for attempt := 0; attempt < 15; attempt++ {
		if err = p.validator.ping(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("validator %q is not answering: %w", p.validatorName, err)
}

// Reset drops the loaded document. The validator container stays up.
func (p *PDF) Reset(ctx context.Context) error {
	p.doc = nil
	return nil
}

func (p *PDF) Cleanup(ctx context.Context) error { return nil }

func (p *PDF) RegisterSteps(ctx *godog.ScenarioContext) {
	resource.RegisterStepsToGodog(ctx, p.name, p.Steps())
}

func (p *PDF) Steps() resource.StepCategory {
	return resource.StepCategory{
		Name:        "PDF",
		Description: "Steps for asserting on what the PDF documents your application generates contain: pages, text, metadata, outline and accessibility",
		Steps: []resource.StepDef{
			{
				Group:       "Reading",
				Pattern:     `^"{resource}" reads file "([^"]*)"$`,
				Description: "Read a PDF from a file on disk, relative to where tomato runs",
				Example:     `"{resource}" reads file "out/ca.pdf"`,
				Handler:     p.readsFile,
			},
			{
				Group:       "Reading",
				Pattern:     `^"{resource}" reads "([^"]*)" object "([^"]*)"$`,
				Description: "Read a PDF from an object of an s3 resource, as \"bucket/key\"",
				Example:     `"{resource}" reads "files" object "cas/CA_Version_No._07.07_AT-de.pdf"`,
				Handler:     p.readsObject,
			},
			{
				Group:       "Reading",
				Pattern:     `^"{resource}" reads "([^"]*)" response body$`,
				Description: "Read a PDF from the body of the last response an http resource received",
				Example:     `"{resource}" reads "api" response body`,
				Handler:     p.readsResponseBody,
			},
			{
				Group:       "Pages",
				Pattern:     `^"{resource}" has "([^"]*)" pages$`,
				Description: "Assert the number of pages",
				Example:     `"{resource}" has "142" pages`,
				Handler:     p.hasPages,
			},
			{
				Group:       "Text",
				Pattern:     `^"{resource}" text contains "([^"]*)"$`,
				Description: "Assert the document's text contains this, on any page",
				Example:     `"{resource}" text contains "9 Kryptowerte"`,
				Handler:     p.textContains,
			},
			{
				Group:       "Text",
				Pattern:     `^"{resource}" text does not contain "([^"]*)"$`,
				Description: "Assert the document's text does not contain this anywhere, e.g. an unrendered template placeholder",
				Example:     `"{resource}" text does not contain "{{"`,
				Handler:     p.textDoesNotContain,
			},
			{
				Group:       "Text",
				Pattern:     `^"{resource}" page "([^"]*)" contains "([^"]*)"$`,
				Description: "Assert the text of one page, counted from 1, contains this",
				Example:     `"{resource}" page "3" contains "Inhaltsverzeichnis"`,
				Handler:     p.pageContains,
			},
			{
				Group:       "Text",
				Pattern:     `^"{resource}" text "([^"]*)" appears before "([^"]*)"$`,
				Description: "Assert one piece of text comes before another in the document, e.g. the German block of a bilingual contract before the English one",
				Example:     `"{resource}" text "Conditions générales" appears before "General Terms and Conditions"`,
				Handler:     p.textAppearsBefore,
			},
			{
				Group:       "Metadata",
				Pattern:     `^"{resource}" metadata "([^"]*)" is "([^"]*)"$`,
				Description: "Assert a metadata entry (Title, Author, Subject, Producer, Creator, Keywords) from the Info dictionary, or the XMP metadata when the Info dictionary lacks it",
				Example:     `"{resource}" metadata "Title" is "Kundenvereinbarung"`,
				Handler:     p.metadataIs,
			},
			{
				Group:       "Metadata",
				Pattern:     `^"{resource}" language is "([^"]*)"$`,
				Description: "Assert the document's language, the catalog's Lang entry as a BCP 47 tag",
				Example:     `"{resource}" language is "de-AT"`,
				Handler:     p.languageIs,
			},
			{
				Group:       "Metadata",
				Pattern:     `^"{resource}" outline contains "([^"]*)"$`,
				Description: "Assert the outline (the bookmarks a viewer shows as a table of contents) has an entry with this title, at any level",
				Example:     `"{resource}" outline contains "7 Zinsen"`,
				Handler:     p.outlineContains,
			},
			{
				Group:       "Accessibility",
				Pattern:     `^"{resource}" is tagged$`,
				Description: "Assert the document is tagged: MarkInfo says Marked and a structure tree is present",
				Example:     `"{resource}" is tagged`,
				Handler:     p.isTagged,
			},
			{
				Group:       "Accessibility",
				Pattern:     `^"{resource}" conforms to "([^"]*)"$`,
				Description: "Assert the document conforms to PDF/UA-1, PDF/UA-2 or a PDF/A level (1a, 1b, 2a, 2b, 2u, 3a, 3b, 3u, 4, 4e, 4f), checked by the veraPDF container named in options.validator; fails with the rules it breaks",
				Example:     `"{resource}" conforms to "PDF/UA-1"`,
				Handler:     p.conformsTo,
			},
		},
	}
}

// load parses data as the current document.
func (p *PDF) load(data []byte, source string) error {
	doc, err := loadDocument(context.Background(), data, source)
	if err != nil {
		return err
	}
	p.doc = doc
	return nil
}

func (p *PDF) readsFile(path string) error {
	path = resource.ReplaceVariables(path)
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	return p.load(data, fmt.Sprintf("file %q", path))
}

func (p *PDF) readsObject(resName, path string) error {
	path = resource.ReplaceVariables(path)
	h, err := p.resourceNamed(resName)
	if err != nil {
		return err
	}
	store, ok := h.(resource.ObjectReader)
	if !ok {
		return fmt.Errorf("resource %q cannot read objects: it is a %s, not an s3 resource", resName, describeHandler(h))
	}
	data, err := store.ReadObject(context.Background(), path)
	if err != nil {
		return fmt.Errorf("reading %q object %q: %w", resName, path, err)
	}
	return p.load(data, fmt.Sprintf("%q object %q", resName, path))
}

func (p *PDF) readsResponseBody(resName string) error {
	h, err := p.resourceNamed(resName)
	if err != nil {
		return err
	}
	client, ok := h.(resource.ResponseBodyProvider)
	if !ok {
		return fmt.Errorf("resource %q has no response body: it is a %s, not an http client", resName, describeHandler(h))
	}
	data, err := client.LastResponseBody()
	if err != nil {
		return err
	}
	return p.load(data, fmt.Sprintf("%q response body", resName))
}

// resourceNamed finds another configured resource.
func (p *PDF) resourceNamed(name string) (resource.Handler, error) {
	if p.lookup == nil {
		return nil, fmt.Errorf("pdf resource %q cannot look up other resources here", p.name)
	}
	h, ok := p.lookup(name)
	if !ok {
		return nil, fmt.Errorf("no resource named %q in tomato.yml", name)
	}
	return h, nil
}

// describeHandler names a handler's kind for a message: its step category
// when it has one, else its Go type.
func describeHandler(h resource.Handler) string {
	if sp, ok := h.(resource.StepProvider); ok {
		return sp.Steps().Name + " resource"
	}
	return fmt.Sprintf("%T", h)
}

// current returns the loaded document, or says how to load one.
func (p *PDF) current() (*document, error) {
	if p.doc == nil {
		return nil, fmt.Errorf(`no document loaded: use "%s" reads file "...", reads "<s3>" object "..." or reads "<http>" response body first`, p.name)
	}
	return p.doc, nil
}

func (p *PDF) hasPages(want string) error {
	doc, err := p.current()
	if err != nil {
		return err
	}
	want = resource.ReplaceVariables(want)
	n, err := strconv.Atoi(strings.TrimSpace(want))
	if err != nil {
		return fmt.Errorf("page count %q is not a number", want)
	}
	if doc.pages != n {
		return fmt.Errorf("expected %d pages, the document from %s has %d", n, doc.source, doc.pages)
	}
	return nil
}

func (p *PDF) textContains(want string) error {
	doc, err := p.current()
	if err != nil {
		return err
	}
	want = resource.ReplaceVariables(want)
	text, err := doc.text(context.Background())
	if err != nil {
		return err
	}
	if !strings.Contains(fold(text), fold(want)) {
		return fmt.Errorf("text does not contain %q\n%s", want, p.describeText(doc, text))
	}
	return nil
}

func (p *PDF) textDoesNotContain(unwanted string) error {
	doc, err := p.current()
	if err != nil {
		return err
	}
	unwanted = resource.ReplaceVariables(unwanted)
	for n := 1; n <= doc.pages; n++ {
		text, err := doc.pageText(context.Background(), n)
		if err != nil {
			return err
		}
		if i := strings.Index(fold(text), fold(unwanted)); i >= 0 {
			return fmt.Errorf("text contains %q on page %d: %q", unwanted, n, around(fold(text), i, len(fold(unwanted))))
		}
	}
	return nil
}

func (p *PDF) pageContains(page, want string) error {
	doc, err := p.current()
	if err != nil {
		return err
	}
	page = resource.ReplaceVariables(page)
	n, err := strconv.Atoi(strings.TrimSpace(page))
	if err != nil {
		return fmt.Errorf("page %q is not a number", page)
	}
	want = resource.ReplaceVariables(want)
	text, err := doc.pageText(context.Background(), n)
	if err != nil {
		return err
	}
	if !strings.Contains(fold(text), fold(want)) {
		return fmt.Errorf("page %d does not contain %q\n%s", n, want, p.describeText(doc, text))
	}
	return nil
}

func (p *PDF) textAppearsBefore(first, second string) error {
	doc, err := p.current()
	if err != nil {
		return err
	}
	first, second = resource.ReplaceVariables(first), resource.ReplaceVariables(second)
	text, err := doc.text(context.Background())
	if err != nil {
		return err
	}
	folded := fold(text)
	i, j := strings.Index(folded, fold(first)), strings.Index(folded, fold(second))
	switch {
	case i < 0 && j < 0:
		return fmt.Errorf("neither %q nor %q is in the text\n%s", first, second, p.describeText(doc, text))
	case i < 0:
		return fmt.Errorf("%q is not in the text (%q is)\n%s", first, second, p.describeText(doc, text))
	case j < 0:
		return fmt.Errorf("%q is not in the text (%q is)\n%s", second, first, p.describeText(doc, text))
	case i > j:
		return fmt.Errorf("%q appears after %q, not before: %q comes first at %s", first, second, second, p.position(doc, j))
	}
	return nil
}

func (p *PDF) metadataIs(key, want string) error {
	doc, err := p.current()
	if err != nil {
		return err
	}
	key, want = resource.ReplaceVariables(key), resource.ReplaceVariables(want)
	meta, err := doc.metadata()
	if err != nil {
		return err
	}
	for k, v := range meta {
		if strings.EqualFold(k, key) {
			if v != want {
				return fmt.Errorf("metadata %s is %q, not %q", k, v, want)
			}
			return nil
		}
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return fmt.Errorf("the document has no metadata entry %q: it has none at all", key)
	}
	return fmt.Errorf("the document has no metadata entry %q; it has: %s", key, strings.Join(keys, ", "))
}

func (p *PDF) languageIs(want string) error {
	doc, err := p.current()
	if err != nil {
		return err
	}
	want = resource.ReplaceVariables(want)
	lang, err := doc.language()
	if err != nil {
		return err
	}
	if lang == "" {
		return fmt.Errorf("the document declares no language (no Lang entry in the catalog); expected %q", want)
	}
	if !strings.EqualFold(lang, want) {
		return fmt.Errorf("the document's language is %q, not %q", lang, want)
	}
	return nil
}

func (p *PDF) outlineContains(want string) error {
	doc, err := p.current()
	if err != nil {
		return err
	}
	want = strings.TrimSpace(resource.ReplaceVariables(want))
	titles, err := doc.outline(context.Background())
	if err != nil {
		return err
	}
	for _, t := range titles {
		if t == want {
			return nil
		}
	}
	if len(titles) == 0 {
		return fmt.Errorf("the document has no outline; expected an entry %q", want)
	}
	return fmt.Errorf("the outline has no entry %q; it has:\n  %s", want, strings.Join(titles, "\n  "))
}

func (p *PDF) isTagged() error {
	doc, err := p.current()
	if err != nil {
		return err
	}
	marked, structured, err := doc.tagged()
	if err != nil {
		return err
	}
	switch {
	case !marked && !structured:
		return fmt.Errorf("the document is not tagged: no MarkInfo with Marked true, and no structure tree (StructTreeRoot)")
	case !marked:
		return fmt.Errorf("the document has a structure tree but does not declare itself tagged (MarkInfo Marked is not true)")
	case !structured:
		return fmt.Errorf("the document declares itself tagged (MarkInfo Marked) but has no structure tree (StructTreeRoot)")
	}
	return nil
}

func (p *PDF) conformsTo(level string) error {
	doc, err := p.current()
	if err != nil {
		return err
	}
	level = resource.ReplaceVariables(level)
	prof, err := profileFor(level)
	if err != nil {
		return err
	}
	if p.validator == nil {
		return fmt.Errorf("pdf resource %q has no validator for %q: add a container with preset: verapdf and name it in options.validator", p.name, prof.name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	result, err := p.validator.validate(ctx, doc.data, prof)
	if err != nil {
		return err
	}
	if !result.Compliant {
		return fmt.Errorf("the document from %s does not conform to %s: %s", doc.source, prof.name, result.describe())
	}
	return nil
}

// describeText says what text the step looked at, for a failure.
func (p *PDF) describeText(doc *document, text string) string {
	preview := text
	if len(preview) > 300 {
		preview = preview[:300] + "…"
	}
	preview = strings.ReplaceAll(preview, "\n", " ⏎ ")
	return fmt.Sprintf("  document: %d page%s from %s, %d characters of text\n  text: %s", doc.pages, plural(doc.pages), doc.source, len(text), preview)
}

// fold makes whitespace insignificant for matching: a phrase broken across
// a line, or a generator's double space, still matches what a person reads.
func fold(s string) string { return strings.Join(strings.Fields(s), " ") }

// position names the page of an offset into the document's folded text.
func (p *PDF) position(doc *document, offset int) string {
	pos := 0
	for n := 1; n <= doc.pages; n++ {
		t, err := doc.pageText(context.Background(), n)
		if err != nil {
			break
		}
		t = fold(t)
		if t == "" {
			continue
		}
		if pos > 0 {
			pos++ // the separator between pages
		}
		if offset < pos+len(t) {
			return fmt.Sprintf("page %d", n)
		}
		pos += len(t)
	}
	return fmt.Sprintf("offset %d", offset)
}

// around returns the text around a match, for a message.
func around(text string, at, n int) string {
	start, end := at-40, at+n+40
	if start < 0 {
		start = 0
	}
	if end > len(text) {
		end = len(text)
	}
	s := text[start:end]
	if start > 0 {
		s = "…" + s
	}
	if end < len(text) {
		s += "…"
	}
	return strings.ReplaceAll(s, "\n", " ⏎ ")
}
