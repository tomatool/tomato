package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/resource"
)

// These tests drive a local Chrome, started once for the package; each test
// gets its own browser contexts in it, as the resource does in a run. Without
// a Chrome on the machine they are skipped (GitHub's runners have one).

var (
	chromeOnce    sync.Once
	chromeCtx     context.Context
	chromeCancel  context.CancelFunc
	chromeErr     error
	chromeSkipped string
)

// chromePath finds a Chrome or Chromium binary the way chromedp does, plus
// CHROME_PATH for an unusual install.
func chromePath() string {
	if p := os.Getenv("CHROME_PATH"); p != "" {
		return p
	}
	candidates := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome", "headless-shell"}
	if runtime.GOOS == "darwin" {
		candidates = append([]string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}, candidates...)
	}
	for _, c := range candidates {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

func chromeOptions(path string) []chromedp.ExecAllocatorOption {
	return append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(path),
		chromedp.NoSandbox,
	)
}

// sharedChrome starts the package's Chrome on first use.
func sharedChrome(t *testing.T) context.Context {
	t.Helper()
	chromeOnce.Do(func() {
		path := chromePath()
		if path == "" {
			chromeSkipped = "no Chrome or Chromium installed (set CHROME_PATH)"
			return
		}
		alloc, allocCancel := chromedp.NewExecAllocator(context.Background(), chromeOptions(path)...)
		ctx, cancel := chromedp.NewContext(alloc)
		if err := chromedp.Do(ctx); err != nil {
			cancel()
			allocCancel()
			chromeErr = fmt.Errorf("starting %s: %w", path, err)
			return
		}
		chromeCtx = ctx
		chromeCancel = func() { cancel(); allocCancel() }
	})
	if chromeSkipped != "" {
		t.Skip(chromeSkipped)
	}
	if chromeErr != nil {
		t.Fatal(chromeErr)
	}
	return chromeCtx
}

func TestMain(m *testing.M) {
	code := m.Run()
	if chromeCancel != nil {
		chromeCancel()
	}
	os.Exit(code)
}

// site serves each page of pages at its path.
func site(t *testing.T, pages map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		body = strings.ReplaceAll(body, "{{accept-language}}", r.Header.Get("Accept-Language"))
		fmt.Fprint(w, `<!doctype html><html><head><meta charset="utf-8"><title>t</title></head><body>`+body+`</body></html>`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// person is a browser resource on the shared Chrome, against srv.
func person(t *testing.T, srv *httptest.Server, options map[string]any) *Browser {
	t.Helper()
	parent := sharedChrome(t)
	opts := map[string]any{"base_url": srv.URL, "timeout": "1s", "screenshots": t.TempDir()}
	for k, v := range options {
		opts[k] = v
	}
	r, err := New("alice", config.Resource{Type: "browser", Options: opts}, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.browserCtx = parent
	t.Cleanup(r.closeTab)
	return r
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func mustFail(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("passed, want an error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v\nwant it to contain %q", err, want)
	}
}

func TestOpenAndSee(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<h1>7 Zinsen</h1><p hidden>clause_id</p><input value="typed by someone"><input type="password" value="secret">
			<div id="later"></div><div id="soon">Saving…</div>
			<script>
				// Long enough that the "not yet" assertions below cannot lose
				// the race on a slow runner, short enough to stay well inside
				// the 3s waits that follow.
				setTimeout(() => { document.getElementById('later').textContent = 'Neuer Satz.' }, 1500);
				setTimeout(() => { document.getElementById('soon').remove() }, 1500);
			</script>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.sees("7 Zinsen"))
	must(t, r.sees("typed by someone"))
	must(t, r.doesNotSee("clause_id"))
	must(t, r.doesNotSee("secret"))
	mustFail(t, r.sees("Neuer Satz."), `the page shows: "7 Zinsen`)
	mustFail(t, r.doesNotSee("Saving…"), `sees "Saving…"`)
	must(t, r.seesWithin("Neuer Satz.", "3s"))
	must(t, r.noLongerSees("Saving…", "3s"))
	mustFail(t, r.seesWithin("Never", "200ms"), `did not see "Never" within 200ms`)
	mustFail(t, r.noLongerSees("7 Zinsen", "200ms"), `still sees "7 Zinsen" after 200ms`)
	mustFail(t, r.seesWithin("x", "soon"), "invalid duration")
	mustFail(t, r.noLongerSees("x", "soon"), "invalid duration")

	mustFail(t, r.open("/missing"), "the server answered 404")
	mustFail(t, r.open("/%zz"), "invalid URL")
}

func TestURLIs(t *testing.T) {
	srv := site(t, map[string]string{"/master/7.2": `<a href="/history?tab=all#top">History</a>`, "/history": `<h1>Change history</h1>`})
	r := person(t, srv, nil)
	must(t, r.open("/master/7.2"))
	must(t, r.urlIs("/master/7.2"))
	must(t, r.urlIs(srv.URL+"/master/7.2"))
	must(t, r.click("History"))
	must(t, r.seesWithin("Change history", "3s"))
	must(t, r.urlIs("/history?tab=all#top"))
	mustFail(t, r.urlIs("/master"), `is at "/history?tab=all#top", expected "/master"`)
}

func TestClickFindsWhatAPersonCallsIt(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<h2>Apply</h2>
			<button onclick="out.textContent='applied'">Apply</button>
			<button aria-label="Close dialog" onclick="out.textContent='closed'">×</button>
			<span role="button" tabindex="0" onclick="out.textContent='span'">Archive</span>
			<div onclick="out.textContent='plain text'">Just text</div>
			<button onclick="out.textContent='twice'">Delete</button><button>Delete</button>
			<button disabled>Locked</button>
			<button id="later" hidden onclick="out.textContent='appeared'">Appears</button>
			<button style="text-transform: uppercase" onclick="out.textContent='upper'">Loud</button>
			<p id="out"></p>
			<script>setTimeout(() => { document.getElementById('later').hidden = false }, 300)</script>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))

	must(t, r.click(" Apply  "))
	must(t, r.sees("applied"))
	must(t, r.click("Close dialog"))
	must(t, r.sees("closed"))
	must(t, r.click("Archive"))
	must(t, r.sees("span"))
	must(t, r.click("Just text"))
	must(t, r.sees("plain text"))
	must(t, r.click("Appears"))
	must(t, r.sees("appeared"))
	must(t, r.click("Loud"))
	must(t, r.sees("upper"))

	mustFail(t, r.click("Delete"), `2 elements match "Delete": button "Delete", button "Delete"`)
	mustFail(t, r.click("Locked"), `button "Locked" is disabled`)
	mustFail(t, r.click("Nowhere"), `no element "Nowhere" on the page`)
}

func TestClickWaitsForWhatCoversIt(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<button onclick="out.textContent='saved'">Save</button><p id="out"></p>
			<div id="toast" style="position:fixed;inset:0;background:#0003">Loading</div>`,
		"/clears": `<button onclick="out.textContent='saved'">Save</button><p id="out"></p>
			<div id="toast" style="position:fixed;inset:0;background:#0003">Loading</div>
			<script>setTimeout(() => document.getElementById('toast').remove(), 300)</script>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	mustFail(t, r.click("Save"), `button "Save" is covered by div#toast "Loading"`)
	must(t, r.open("/clears"))
	must(t, r.click("Save"))
	must(t, r.sees("saved"))
}

// A page can move between finding where to click and the click arriving. The
// click then lands on something else, which must neither count nor reach it.
func TestAClickThatLandsElsewhereIsStopped(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<button onclick="out.textContent += 'applied '">Apply</button>
			<button onclick="out.textContent += 'deleted '">Delete</button><p id="out"></p>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	ctx, cancel, err := r.stepContext(r.timeout)
	must(t, err)
	defer cancel()

	apply, _, err := r.findClickable(ctx, "Apply", "click")
	must(t, err)
	_, elsewhere, err := r.findClickable(ctx, "Delete", "click")
	must(t, err)
	must(t, errorIf(call[string](ctx, apply, "arm")))
	must(t, chromedp.Do(ctx, chromedp.MouseClickXY(elsewhere.X, elsewhere.Y)))
	doc, err := document(ctx)
	must(t, err)
	missed, err := call[string](ctx, doc, "disarm")
	must(t, err)
	if missed != `the click landed on button "Delete" instead` {
		t.Errorf("disarm = %q", missed)
	}
	must(t, r.doesNotSee("deleted"))

	// Disarmed, the page is the page again, and a guarded click lands.
	must(t, r.click("Delete"))
	must(t, r.sees("deleted"))
	must(t, r.click("Apply"))
	must(t, r.sees("applied"))
	missed, err = call[string](ctx, doc, "disarm")
	must(t, err)
	if missed != "" {
		t.Errorf("disarm with nothing armed = %q", missed)
	}
}

func TestEnabledAndDisabled(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<button disabled>Apply</button><button>Publish</button>
			<div role="button" aria-disabled="true">Archive</div>
			<fieldset disabled><label>Name <input></label></fieldset>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.seesDisabled("Apply"))
	must(t, r.seesEnabled("Publish"))
	must(t, r.seesDisabled("Archive"))
	must(t, r.seesDisabled("Name"))
	mustFail(t, r.seesEnabled("Apply"), `sees "Apply" is disabled, expected enabled`)
	mustFail(t, r.seesDisabled("Publish"), `sees "Publish" is enabled, expected disabled`)
	mustFail(t, r.seesEnabled("Missing"), `does not see "Missing"`)
}

func TestFill(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<label for="q">Search</label><input id="q" value="old" oninput="echo.textContent='input:'+this.value">
			<input placeholder="Your email">
			<label>Notes <textarea>draft</textarea></label>
			<div contenteditable="true" aria-label="Clause 7.2"><p>Die Zinsen</p></div>
			<label>Fixed <input readonly value="x"></label>
			<label>Off <input disabled></label>
			<p id="echo"></p>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.fill("Search", "Zinsen"))
	must(t, r.sees("input:Zinsen"))
	must(t, r.doesNotSee("old"))
	must(t, r.fill("Your email", "ada@example.com"))
	must(t, r.sees("ada@example.com"))
	must(t, r.fill("Notes", "final"))
	must(t, r.sees("final"))
	must(t, r.doesNotSee("draft"))
	must(t, r.fill("Clause 7.2", "Die Gebühren"))
	must(t, r.sees("Die Gebühren"))
	must(t, r.doesNotSee("Die Zinsen"))
	must(t, r.fill("Search", ""))
	must(t, r.sees("input:"))
	must(t, r.doesNotSee("input:Zinsen"))
	mustFail(t, r.fill("Fixed", "y"), "is read-only")
	mustFail(t, r.fill("Off", "y"), "is disabled")
	mustFail(t, r.fill("Nowhere", "y"), `no field "Nowhere" on the page`)
}

func TestTypeAtTheEnd(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<div contenteditable="true" id="doc"><p><strong>7.1</strong> Zinsen werden berechnet.</p><p><strong>7.2</strong> Die Zinsen werden gezahlt</p></div>
			<label>Title <input value="Master"></label>
			<p>7.3 read only text</p>
			<p id="log"></p>
			<script>document.getElementById('doc').addEventListener('keydown', (e) => { log.textContent += e.key })</script>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.typeAtEnd(" und verrechnet.", "7.2"))
	must(t, r.sees("7.2 Die Zinsen werden gezahlt und verrechnet."))
	must(t, r.sees("7.1 Zinsen werden berechnet."))
	// Key by key, as a person types.
	must(t, r.sees(" und verrechnet."))
	must(t, r.typeAtEnd(" v2", "Title"))
	must(t, r.sees("Master v2"))
	mustFail(t, r.typeAtEnd("x", "7.3"), "cannot be edited")
	mustFail(t, r.typeAtEnd("x", "Zinsen"), `2 elements match "Zinsen"`)
}

func TestTypeIntoWhatHasFocus(t *testing.T) {
	srv := site(t, map[string]string{"/": `<label>Comment <textarea></textarea></label>`})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	mustFail(t, r.typeText("hello"), "nothing focused to type into")
	must(t, r.click("Comment"))
	must(t, r.typeText("Bitte prüfen"))
	must(t, r.sees("Bitte prüfen"))
}

func TestPress(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<label>Search <input onkeydown="if (event.key === 'Enter') out.textContent = 'searched ' + this.value;
				if (event.ctrlKey && event.key === 'z') out.textContent = 'undone'"></label><p id="out"></p>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.fill("Search", "Zinsen"))
	must(t, r.press("Enter"))
	must(t, r.sees("searched Zinsen"))
	must(t, r.press("ctrl+z"))
	must(t, r.sees("undone"))
	mustFail(t, r.press("Banana"), "unknown key")
}

func TestCheckAndUncheck(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<label><input type="checkbox" name="c" value="AT"> Austria</label>
			<input type="checkbox" id="nl"><label for="nl">Netherlands</label>
			<label><input type="checkbox" checked> Germany</label>
			<style>.sr-only{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0)}</style>
			<label><input type="checkbox" class="sr-only"><span style="padding:4px;border:1px solid">Remember me</span></label>
			<div role="checkbox" aria-checked="false" tabindex="0" onclick="this.setAttribute('aria-checked', this.getAttribute('aria-checked') === 'true' ? 'false' : 'true')">Newsletter</div>
			<label><input type="checkbox" onclick="return false"> Locked in</label>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.check("Austria"))
	must(t, r.check("Austria")) // already ticked: nothing to do
	must(t, r.check("Netherlands"))
	must(t, r.uncheck("Germany"))
	must(t, r.uncheck("Germany"))
	must(t, r.check("Remember me"))
	must(t, r.check("Newsletter"))
	must(t, r.uncheck("Newsletter"))

	state, err := chromedp.Run(r.tab, chromedp.Evaluate[[]bool](`Array.from(document.querySelectorAll('input[type=checkbox]')).map((c) => c.checked)`))
	must(t, err)
	if fmt.Sprint(state) != "[true true false true false]" {
		t.Errorf("checkboxes = %v", state)
	}
	mustFail(t, r.check("Locked in"), `clicking "Locked in" did not check it`)
	mustFail(t, r.check("Nowhere"), `no checkbox "Nowhere" on the page`)
}

func TestChoose(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<fieldset><legend>Countries</legend>
				<label><input type="radio" name="scope" checked> All countries</label>
				<label><input type="radio" name="scope" onchange="out.textContent='selected'"> Selected countries</label>
			</fieldset>
			<div role="tablist">
				<button role="tab" aria-selected="true" onclick="pick(this)">Draft</button>
				<button role="tab" aria-selected="false" onclick="pick(this)">History</button>
			</div>
			<label>Effective from <select onchange="out.textContent='effective '+this.value">
				<option value="now">Immediately</option><option value="q">Next quarter</option></select></label>
			<p id="out"></p>
			<script>function pick(tab) { for (const t of document.querySelectorAll('[role=tab]')) t.setAttribute('aria-selected', t === tab) }</script>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.choose("Selected countries"))
	must(t, r.sees("selected"))
	must(t, r.choose("History"))
	sel, err := chromedp.Run(r.tab, chromedp.Evaluate[string](`document.querySelector('[aria-selected=true]').textContent`))
	must(t, err)
	if sel != "History" {
		t.Errorf("selected tab = %q", sel)
	}
	// An option of a native select is chosen without opening it.
	must(t, r.choose("Next quarter"))
	must(t, r.sees("effective q"))
	mustFail(t, r.choose("Nowhere"), `no option or radio button "Nowhere" on the page`)
}

func TestSelectFrom(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<label for="country">Country</label>
			<select id="country" onchange="out.textContent='country '+this.value">
				<option value="">Choose…</option><option value="AT">Austria</option><option value="NL" disabled>Netherlands</option>
			</select>
			<label>Fixed <select disabled><option>One</option></select></label>
			<button role="combobox" aria-expanded="false" aria-controls="lb" onclick="lb.hidden = false">Language</button>
			<ul role="listbox" id="lb" hidden>
				<li role="option" aria-selected="false" onclick="out.textContent='language '+this.textContent; lb.hidden = true">Deutsch</li>
				<li role="option" aria-selected="false">English</li>
			</ul>
			<p id="out"></p>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.selectFrom("Austria", "Country"))
	must(t, r.sees("country AT"))
	must(t, r.selectFrom("AT", "Country")) // by value too
	mustFail(t, r.selectFrom("Germany", "Country"), `has no option "Germany" (it has: "Choose…", "Austria", "Netherlands")`)
	mustFail(t, r.selectFrom("Netherlands", "Country"), `option "Netherlands" is disabled`)
	mustFail(t, r.selectFrom("One", "Fixed"), "is disabled")
	must(t, r.selectFrom("Deutsch", "Language"))
	must(t, r.sees("language Deutsch"))
	mustFail(t, r.selectFrom("Deutsch", "Nowhere"), `no select box "Nowhere" on the page`)
	mustFail(t, r.selectFrom("Klingon", "Language"), `cannot select "Klingon" from "Language"`)
}

func TestLocaleAndViewport(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<p id="p"></p><p>heard {{accept-language}}</p>
			<script>p.textContent = navigator.language + ' ' + new Intl.NumberFormat().format(1234.5) + ' ' + innerWidth + 'x' + innerHeight</script>`,
	})
	r := person(t, srv, map[string]any{"locale": "de-DE", "viewport": "1000x700"})
	must(t, r.open("/"))
	must(t, r.sees("de-DE 1.234,5 1000x700"))
	must(t, r.sees("heard de-DE"))
}

func TestDialogsAreAccepted(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<button onclick="out.textContent = 'confirmed ' + confirm('Delete the clause?')">Delete</button><p id="out"></p>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.click("Delete"))
	must(t, r.seesWithin("confirmed true", "3s"))
}

func TestResetStartsAFreshBrowserContext(t *testing.T) {
	srv := site(t, map[string]string{
		"/": `<p id="p"></p><script>p.textContent = localStorage.getItem('seen') ? 'Welcome back' : 'Hello';
			localStorage.setItem('seen', '1'); document.cookie = 'session=1'</script>`,
	})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.sees("Hello"))
	first := r.browserContextID()
	must(t, r.open("/"))
	must(t, r.sees("Welcome back"))

	must(t, r.Reset(context.Background()))
	if r.browserContextID() != "" {
		t.Fatal("the browser context is still open after Reset")
	}
	must(t, r.open("/"))
	must(t, r.sees("Hello"))
	if r.browserContextID() == first || r.browserContextID() == "" {
		t.Errorf("after Reset the browser context is %q, before it was %q", r.browserContextID(), first)
	}

	// A second person on the same Chrome has their own context.
	bob := person(t, srv, nil)
	must(t, bob.open("/"))
	must(t, bob.sees("Hello"))
	if bob.browserContextID() == r.browserContextID() {
		t.Error("two people share a browser context")
	}
}

func TestScreenshots(t *testing.T) {
	srv := site(t, map[string]string{"/": `<h1>Order confirmed</h1>`})
	r := person(t, srv, nil)

	// A failure before any page opened leaves nothing to show.
	ctx := context.Background()
	_, err := r.beforeScenario(ctx, &godog.Scenario{Name: "The order is cancelled"})
	must(t, err)
	_, err = r.afterStep(ctx, nil, godog.StepFailed, fmt.Errorf("boom"))
	must(t, err)
	if entries, _ := os.ReadDir(r.screenshots); len(entries) != 0 {
		t.Fatalf("a screenshot without a page: %v", entries)
	}
	if got := resource.TakeScreenshots("The order is cancelled"); got != nil {
		t.Fatalf("a screenshot recorded without a page: %v", got)
	}

	must(t, r.open("/"))
	must(t, r.saveScreenshot("after-apply"))
	assertPNG(t, filepath.Join(r.screenshots, "after-apply.png"))
	// Each one is recorded for the step that took it, which is how the
	// `tomato` formatter reports it on that step's end.
	got := resource.TakeScreenshots("The order is cancelled")
	if len(got) != 1 || got[0].Resource != "alice" || got[0].Path != filepath.Join(r.screenshots, "after-apply.png") {
		t.Errorf("recorded %v", got)
	}

	// Passing steps leave nothing; the first failure leaves one, attached
	// to the report; later failures in the scenario do not add more.
	_, err = r.afterStep(ctx, nil, godog.StepPassed, nil)
	must(t, err)
	actx, err := r.afterStep(ctx, nil, godog.StepFailed, fmt.Errorf("boom"))
	must(t, err)
	failed := filepath.Join(r.screenshots, "failed-The-order-is-cancelled-alice.png")
	assertPNG(t, failed)
	if a := godog.Attachments(actx); len(a) != 1 || a[0].MediaType != "image/png" || len(a[0].Body) == 0 {
		t.Errorf("attachments = %d", len(a))
	}
	if got := resource.TakeScreenshots("The order is cancelled"); len(got) != 1 || got[0].Path != failed {
		t.Errorf("the failure recorded %v", got)
	}
	must(t, os.Remove(failed))
	_, err = r.afterStep(ctx, nil, godog.StepFailed, fmt.Errorf("again"))
	must(t, err)
	if _, err := os.Stat(failed); err == nil {
		t.Error("a second screenshot for the same scenario")
	}

	// The next scenario takes its own.
	_, err = r.beforeScenario(ctx, &godog.Scenario{Name: "Next"})
	must(t, err)
	_, err = r.afterStep(ctx, nil, godog.StepFailed, fmt.Errorf("boom"))
	must(t, err)
	assertPNG(t, filepath.Join(r.screenshots, "failed-Next-alice.png"))
	if got := resource.TakeScreenshots("Next"); len(got) != 1 || got[0].Path != filepath.Join(r.screenshots, "failed-Next-alice.png") {
		t.Errorf("the next scenario's failure recorded %v", got)
	}

	// A directory that cannot be created fails the step that asked for it.
	blocker := filepath.Join(t.TempDir(), "file")
	must(t, os.WriteFile(blocker, nil, 0o644))
	r.screenshots = filepath.Join(blocker, "dir")
	mustFail(t, r.saveScreenshot("x"), "creating")
}

func assertPNG(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	if !strings.HasPrefix(string(b), "\x89PNG") {
		t.Fatalf("%s is not a PNG", path)
	}
}

// TestInitConnects covers the connection Init makes, on a Chrome of its own.
func TestInitConnects(t *testing.T) {
	sharedChrome(t)
	path := chromePath()
	srv := site(t, map[string]string{"/": `<h1>Connected</h1>`})
	r, err := New("alice", config.Resource{Type: "browser", Options: map[string]any{"base_url": srv.URL}}, nil)
	must(t, err)
	r.connect = func() (context.Context, context.CancelFunc) {
		return chromedp.NewExecAllocator(context.Background(), chromeOptions(path)...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	must(t, r.Init(ctx))
	must(t, r.Init(ctx)) // once is enough
	must(t, r.Ready(ctx))
	must(t, r.open("/"))
	must(t, r.sees("Connected"))
	must(t, r.Cleanup(ctx))
	if err := r.Ready(ctx); err == nil {
		t.Error("ready after Cleanup")
	}
}

func TestInitGivesUpWithTheContext(t *testing.T) {
	r := newFor(t, config.Resource{Type: "browser"})
	r.connect = func() (context.Context, context.CancelFunc) {
		// An allocator that never produces a browser.
		return chromedp.NewExecAllocator(context.Background(), chromedp.ExecPath("/bin/sleep"), chromedp.Flag("x", "100"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := r.Init(ctx); err == nil {
		t.Error("Init passed without a browser")
	}
}
