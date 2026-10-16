package pdf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProfileFor(t *testing.T) {
	for name, id := range map[string]string{"PDF/UA-1": "ua1", "pdf/ua-2": "ua2", " PDF/A-2b ": "2b", "1a": "1a", "PDF/A-4": "4", "wt1a": "wt1a"} {
		p, err := profileFor(name)
		if err != nil || p.id != id {
			t.Errorf("%q: got %+v %v, want %s", name, p, err, id)
		}
	}
	_, err := profileFor("PDF/X-4")
	if err == nil || !strings.Contains(err.Error(), "PDF/UA-1") || !strings.Contains(err.Error(), "PDF/A-3b") {
		t.Errorf("unknown level should list the known ones, got %v", err)
	}
}

// nonCompliant is the shape veraPDF REST 1.30 returns, trimmed.
const nonCompliant = `{"report":{"jobs":[{"itemDetails":{"name":"document.pdf"},"validationResult":[{"details":{"passedRules":100,"failedRules":2,"ruleSummaries":[
{"ruleStatus":"FAILED","specification":"ISO 14289-1:2014","clause":"7.1","testNumber":3,"status":"failed","failedChecks":7,"description":"Content shall be marked as Artifact or tagged as real content","checks":[{"status":"failed","context":"root/document[0]/pages[0](3 0 obj PDPage)/contentStream[0]"}]},
{"ruleStatus":"PASSED","specification":"ISO 14289-1:2014","clause":"7.1","testNumber":5,"status":"passed","failedChecks":0,"description":"MarkInfo","checks":[]},
{"ruleStatus":"FAILED","specification":"ISO 14289-1:2014","clause":"7.21.4.1","testNumber":1,"status":"failed","failedChecks":1,"description":"The font programs for all fonts used for rendering within a conforming file shall be embedded","checks":[{"status":"failed","context":"root/document[0]/pages[0]/resources[0]/fonts[0]","errorMessage":"Helvetica is not embedded"}]}
]},"jobEndStatus":"normal","profileName":"PDF/UA-1 validation profile","compliant":false}]}]}}`

func TestParseReport(t *testing.T) {
	c, err := parseReport([]byte(nonCompliant))
	if err != nil {
		t.Fatal(err)
	}
	if c.Compliant || c.ProfileName != "PDF/UA-1 validation profile" || len(c.Violations) != 2 {
		t.Fatalf("got %+v", c)
	}
	v := c.Violations[1]
	if v.Clause != "7.21.4.1" || v.Test != 1 || v.Checks != 1 || !strings.Contains(v.Context, "Helvetica is not embedded at root/document[0]") {
		t.Errorf("violation: %+v", v)
	}
	msg := c.describe()
	for _, want := range []string{"2 rules failed (PDF/UA-1 validation profile)", "7.1-3 (7 checks): Content shall be marked", "7.21.4.1-1 (1 check)", "at Helvetica is not embedded"} {
		if !strings.Contains(msg, want) {
			t.Errorf("describe lacks %q:\n%s", want, msg)
		}
	}

	// The older object form, compliant.
	older := `{"report":{"jobs":[{"validationResult":{"compliant":true,"profileName":"PDF/A-2b validation profile","details":{"ruleSummaries":[]}}}]}}`
	c, err = parseReport([]byte(older))
	if err != nil || !c.Compliant || len(c.Violations) != 0 {
		t.Errorf("older form: %+v %v", c, err)
	}

	// A document veraPDF could not parse.
	broken := `{"report":{"jobs":[{"taskResult":{"type":"PARSE","isExecuted":true,"isSuccess":false,"exceptionMessage":"Couldn't parse stream caused by exception: Unexpected end of file"}}]}}`
	if _, err := parseReport([]byte(broken)); err == nil || !strings.Contains(err.Error(), "could not validate the document (parse): Couldn't parse stream") {
		t.Errorf("broken: %v", err)
	}
	for name, raw := range map[string]string{"no jobs": `{"report":{"jobs":[]}}`, "not json": `<xml/>`, "no result": `{"report":{"jobs":[{}]}}`, "bad result": `{"report":{"jobs":[{"validationResult":[1]}]}}`} {
		if _, err := parseReport([]byte(raw)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestDescribeTruncates(t *testing.T) {
	c := conformance{ProfileName: "p"}
	for i := 0; i < maxViolationsShown+5; i++ {
		c.Violations = append(c.Violations, violation{Clause: "6.1", Test: i, Description: "d", Checks: 2})
	}
	msg := c.describe()
	if !strings.Contains(msg, "… and 5 more") || strings.Count(msg, "\n  6.1-") != maxViolationsShown {
		t.Errorf("got:\n%s", msg)
	}
}

// fakeVeraPDF serves the REST API's two endpoints and records the upload.
func fakeVeraPDF(t *testing.T, status int, body string) (*httptest.Server, *http.Request, *[]byte) {
	t.Helper()
	var last http.Request
	var upload []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last = *r
		if r.URL.Path == "/api/info" {
			fmt.Fprint(w, `{"os":{}}`)
			return
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		defer f.Close()
		upload, _ = io.ReadAll(f)
		last.Header.Set("X-Upload-Name", hdr.Filename)
		last.Header.Set("X-Upload-Type", hdr.Header.Get("Content-Type"))
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &last, &upload
}

func TestValidatorPostsMultipart(t *testing.T) {
	srv, last, upload := fakeVeraPDF(t, 200, nonCompliant)
	v := &validator{baseURL: srv.URL, client: srv.Client()}
	if err := v.ping(context.Background()); err != nil {
		t.Fatal(err)
	}
	c, err := v.validate(context.Background(), []byte("%PDF-1.7 fake"), profile{"PDF/UA-1", "ua1"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Compliant || len(c.Violations) != 2 {
		t.Errorf("got %+v", c)
	}
	if last.Method != "POST" || last.URL.Path != "/api/validate/ua1" || last.Header.Get("Accept") != "application/json" {
		t.Errorf("request: %s %s %v", last.Method, last.URL.Path, last.Header)
	}
	if last.Header.Get("X-Upload-Name") != "document.pdf" || last.Header.Get("X-Upload-Type") != "application/pdf" || string(*upload) != "%PDF-1.7 fake" {
		t.Errorf("upload: %v %q", last.Header, *upload)
	}
}

func TestValidatorErrors(t *testing.T) {
	srv, _, _ := fakeVeraPDF(t, 500, "boom")
	v := &validator{baseURL: srv.URL, client: srv.Client()}
	_, err := v.validate(context.Background(), []byte("x"), profile{"PDF/UA-1", "ua1"})
	if err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "boom") {
		t.Errorf("got %v", err)
	}

	down := &validator{baseURL: "http://127.0.0.1:1", client: &http.Client{Timeout: time.Second}}
	if _, err := down.validate(context.Background(), []byte("x"), profile{"PDF/UA-1", "ua1"}); err == nil {
		t.Error("a validator nobody answers should fail")
	}
	if err := down.ping(context.Background()); err == nil {
		t.Error("ping of a validator nobody answers should fail")
	}

	notOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer notOK.Close()
	if err := (&validator{baseURL: notOK.URL, client: notOK.Client()}).ping(context.Background()); err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("ping 503: %v", err)
	}
}

func TestExcerpt(t *testing.T) {
	long := strings.Repeat("x", 400)
	if got := excerpt([]byte(" " + long)); len([]rune(got)) != 301 || !strings.HasSuffix(got, "…") {
		t.Errorf("excerpt: %d runes", len([]rune(got)))
	}
	var rep veraReport
	if err := json.Unmarshal([]byte(nonCompliant), &rep); err != nil || len(rep.Report.Jobs) != 1 {
		t.Errorf("report shape: %v", err)
	}
}
