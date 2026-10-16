// Package browser provides tomato's `browser` resource: a person using the
// app's web UI in a real Chromium, driven over the Chrome DevTools Protocol.
//
// Steps name what a person sees — a button's accessible name, a field's
// label, a paragraph's text — never a CSS selector, and assertions read what
// the page shows. Each resource is its own browser context, with its own
// cookies and storage, so two resources on one browser are two people.
package browser

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/remote"
	"github.com/cucumber/godog"
	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
	"github.com/tomatool/tomato/internal/resource"
)

//go:embed page.js
var pageJS string

// DevToolsPort is the port Chromium serves the DevTools protocol on inside
// its container, in chromedp/headless-shell and in the chromium preset.
const DevToolsPort = "9222"

const (
	defaultTimeout = 5 * time.Second
	defaultWidth   = 1280
	defaultHeight  = 720
	pollInterval   = 50 * time.Millisecond
)

// Browser is one person's browser: a browser context in a shared Chromium.
type Browser struct {
	name      string
	config    config.Resource
	container *container.Manager

	baseURL     string
	width       int64
	height      int64
	locale      string
	screenshots string
	timeout     time.Duration

	// connect returns a chromedp context bound to a browser. Init points it
	// at the container; the package's tests point it at a local Chrome.
	connect func() (context.Context, context.CancelFunc)

	browserCtx    context.Context
	browserCancel context.CancelFunc

	mu        sync.Mutex
	tab       context.Context
	tabCancel context.CancelFunc

	// scenario is the running scenario's name, for the screenshot taken
	// when it fails; shot is whether that screenshot exists already.
	scenario string
	shot     bool
}

// New creates a browser resource. The options are read here, so a typo in
// one stops the run before any container starts.
func New(name string, cfg config.Resource, cm *container.Manager) (*Browser, error) {
	r := &Browser{
		name:      name,
		config:    cfg,
		container: cm,
		width:     defaultWidth,
		height:    defaultHeight,
		timeout:   defaultTimeout,
	}

	r.baseURL = cfg.BaseURL
	if v, ok := cfg.Options["base_url"].(string); ok && v != "" {
		r.baseURL = v
	}
	if r.baseURL != "" {
		u, err := url.Parse(r.baseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("browser %q: base_url %q is not an absolute URL like http://localhost:8080", name, r.baseURL)
		}
		r.baseURL = strings.TrimRight(loopbackAsLocalhost(u).String(), "/")
	}

	if v, ok := cfg.Options["viewport"]; ok {
		w, h, err := parseViewport(fmt.Sprint(v))
		if err != nil {
			return nil, fmt.Errorf("browser %q: %w", name, err)
		}
		r.width, r.height = w, h
	}

	if v, ok := cfg.Options["locale"].(string); ok {
		r.locale = v
	}
	if v, ok := cfg.Options["screenshots"].(string); ok {
		r.screenshots = v
	}

	if v, ok := cfg.Options["timeout"]; ok {
		d, err := time.ParseDuration(fmt.Sprint(v))
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("browser %q: timeout %q is not a duration like 5s", name, v)
		}
		r.timeout = d
	}

	return r, nil
}

var viewportPattern = regexp.MustCompile(`^\s*(\d+)\s*x\s*(\d+)\s*$`)

// parseViewport reads "1440x900".
func parseViewport(s string) (int64, int64, error) {
	m := viewportPattern.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, fmt.Errorf("viewport %q is not WIDTHxHEIGHT like 1440x900", s)
	}
	w, _ := strconv.ParseInt(m[1], 10, 64)
	h, _ := strconv.ParseInt(m[2], 10, 64)
	if w == 0 || h == 0 {
		return 0, 0, fmt.Errorf("viewport %q has a zero side", s)
	}
	return w, h, nil
}

// loopbackAsLocalhost spells a loopback host as "localhost". The chromium
// preset maps the name localhost to the host tomato runs on; an IP literal
// never reaches a resolver, so 127.0.0.1 would mean the container itself.
func loopbackAsLocalhost(u *url.URL) *url.URL {
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsUnspecified()) {
		out := *u
		if port := u.Port(); port != "" {
			out.Host = "localhost:" + port
		} else {
			out.Host = "localhost"
		}
		return &out
	}
	return u
}

func (r *Browser) Name() string { return r.name }

// Init connects to the Chromium in the resource's container.
func (r *Browser) Init(ctx context.Context) error {
	if r.browserCtx != nil {
		return nil
	}
	if r.connect == nil {
		if r.config.Container == "" {
			return fmt.Errorf("browser %q needs a container to run Chromium in: add a container with `preset: chromium` and point the resource at it with `container:`", r.name)
		}
		port := DevToolsPort
		if p, ok := r.config.Options["port"]; ok {
			port = fmt.Sprint(p)
		}
		host, err := r.container.GetHost(ctx, r.config.Container)
		if err != nil {
			return fmt.Errorf("getting container host: %w", err)
		}
		mapped, err := r.container.GetPort(ctx, r.config.Container, port+"/tcp")
		if err != nil {
			return fmt.Errorf("getting container port: %w", err)
		}
		// Chromium answers DevTools requests only when the Host header is an
		// IP address or localhost.
		host = ipFor(ctx, host)
		endpoint := "ws://" + net.JoinHostPort(host, mapped)
		r.connect = func() (context.Context, context.CancelFunc) {
			return remote.NewAllocator(context.Background(), endpoint)
		}
	}

	allocCtx, allocCancel := r.connect()
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	// The first action connects. It runs on browserCtx itself: a deadline
	// on it would end the connection along with the deadline.
	done := make(chan error, 1)
	go func() { done <- chromedp.Do(browserCtx) }()
	select {
	case err := <-done:
		if err != nil {
			browserCancel()
			allocCancel()
			return fmt.Errorf("connecting to Chromium: %w", err)
		}
	case <-ctx.Done():
		browserCancel()
		allocCancel()
		return fmt.Errorf("connecting to Chromium: %w", ctx.Err())
	}

	r.browserCtx = browserCtx
	r.browserCancel = func() {
		browserCancel()
		allocCancel()
	}
	return nil
}

// ipFor resolves a host name to an address, leaving it as it is when it is
// one already or does not resolve.
func ipFor(ctx context.Context, host string) string {
	if host == "localhost" || net.ParseIP(host) != nil {
		return host
	}
	addrs, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil || len(addrs) == 0 {
		return host
	}
	return addrs[0].String()
}

func (r *Browser) Ready(ctx context.Context) error {
	if r.browserCtx == nil {
		return fmt.Errorf("browser %q is not connected", r.name)
	}
	return nil
}

// Reset closes the person's browser context: its pages, cookies, local and
// session storage and service workers. Chromium keeps running, and the next
// step that needs a page opens a fresh context.
func (r *Browser) Reset(ctx context.Context) error {
	r.closeTab()
	return nil
}

func (r *Browser) closeTab() {
	r.mu.Lock()
	tab, cancel := r.tab, r.tabCancel
	r.tab, r.tabCancel = nil, nil
	r.mu.Unlock()
	if tab == nil {
		return
	}
	// Cancel closes the target and disposes its browser context, and waits
	// for both.
	if err := chromedp.Cancel(tab); err != nil {
		log.Debug().Err(err).Str("resource", r.name).Msg("closing the browser context")
	}
	cancel()
}

func (r *Browser) Cleanup(ctx context.Context) error {
	r.closeTab()
	if r.browserCancel != nil {
		r.browserCancel()
		r.browserCtx, r.browserCancel = nil, nil
	}
	return nil
}

// page returns the person's tab, opening a new browser context for it the
// first time a scenario needs one.
func (r *Browser) page() (context.Context, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tab != nil {
		return r.tab, nil
	}
	if r.browserCtx == nil {
		return nil, fmt.Errorf("browser %q is not connected", r.name)
	}

	tab, cancel := chromedp.NewContext(r.browserCtx, chromedp.WithNewBrowserContext())
	if err := chromedp.Do(tab); err != nil {
		cancel()
		return nil, fmt.Errorf("opening a browser context: %w", err)
	}

	setup, setupCancel := context.WithTimeout(tab, r.timeout+10*time.Second)
	defer setupCancel()
	if _, err := chromedp.Call(setup, emulation.SetDeviceMetricsOverride, emulation.SetDeviceMetricsOverrideParams{
		Width: r.width, Height: r.height, DeviceScaleFactor: 1,
	}); err != nil {
		cancel()
		return nil, fmt.Errorf("setting the viewport: %w", err)
	}
	if r.locale != "" {
		if err := r.applyLocale(setup); err != nil {
			cancel()
			return nil, err
		}
	}

	// alert(), confirm() and prompt() block the page until someone answers.
	// A person clicking through a flow answers OK.
	dialogs := chromedp.Events(tab, page.JavascriptDialogOpening)
	go func() {
		for ev, err := range dialogs {
			if err != nil {
				return
			}
			log.Info().Str("resource", r.name).Str("dialog", string(ev.Type)).Str("message", ev.Message).Msg("accepting a dialog")
			_, _ = chromedp.Call(tab, page.HandleJavaScriptDialog, page.HandleJavaScriptDialogParams{Accept: true})
		}
	}()

	r.tab, r.tabCancel = tab, cancel
	return tab, nil
}

// applyLocale makes the page see the locale everywhere it looks: Intl and
// navigator.language, and the Accept-Language header of every request.
func (r *Browser) applyLocale(ctx context.Context) error {
	if _, err := chromedp.Call(ctx, emulation.SetLocaleOverride, emulation.SetLocaleOverrideParams{Locale: r.locale}); err != nil {
		return fmt.Errorf("setting locale %q: %w", r.locale, err)
	}
	ua, err := chromedp.Run(ctx, chromedp.Evaluate[string](`navigator.userAgent`))
	if err != nil {
		return fmt.Errorf("reading the user agent: %w", err)
	}
	if _, err := chromedp.Call(ctx, emulation.SetUserAgentOverride, emulation.SetUserAgentOverrideParams{
		UserAgent: ua, AcceptLanguage: r.locale,
	}); err != nil {
		return fmt.Errorf("setting Accept-Language %q: %w", r.locale, err)
	}
	return nil
}

// RegisterSteps registers the steps, and the hooks that take a screenshot
// when a step of the scenario fails.
func (r *Browser) RegisterSteps(ctx *godog.ScenarioContext) {
	resource.RegisterStepsToGodog(ctx, r.name, r.Steps())
	ctx.Before(r.beforeScenario)
	ctx.StepContext().After(r.afterStep)
}

func (r *Browser) beforeScenario(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
	r.scenario = sc.Name
	r.shot = false
	return ctx, nil
}

// afterStep saves what the page showed when a step of the scenario failed,
// whichever resource the step belonged to: a red "db" assertion is easier to
// read beside the page that led to it. One screenshot per scenario, of the
// first failure.
func (r *Browser) afterStep(ctx context.Context, _ *godog.Step, status godog.StepResultStatus, _ error) (context.Context, error) {
	if status != godog.StepFailed || r.shot {
		return ctx, nil
	}
	r.mu.Lock()
	open := r.tab != nil
	r.mu.Unlock()
	if !open {
		return ctx, nil
	}
	r.shot = true
	png, path, err := r.capture("failed " + r.scenario + " " + r.name)
	if err != nil {
		log.Warn().Err(err).Str("resource", r.name).Msg("could not take a screenshot of the failed step")
		return ctx, nil
	}
	log.Info().Str("resource", r.name).Str("path", path).Msg("screenshot of the page when the step failed")
	return godog.Attach(ctx, godog.Attachment{Body: png, FileName: filepath.Base(path), MediaType: "image/png"}), nil
}

// capture saves a screenshot of the visible part of the page.
func (r *Browser) capture(name string) ([]byte, string, error) {
	tab, err := r.page()
	if err != nil {
		return nil, "", err
	}
	ctx, cancel := context.WithTimeout(tab, r.timeout+10*time.Second)
	defer cancel()
	png, err := chromedp.Run(ctx, chromedp.CaptureScreenshot())
	if err != nil {
		return nil, "", fmt.Errorf("taking a screenshot: %w", err)
	}
	dir := r.screenshotDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", fmt.Errorf("creating %s: %w", dir, err)
	}
	path := filepath.Join(dir, fileName(name)+".png")
	if err := os.WriteFile(path, png, 0o644); err != nil {
		return nil, "", fmt.Errorf("writing %s: %w", path, err)
	}
	return png, path, nil
}

// screenshotDir is the screenshots option, or the run's directory under
// .tomato/runs, which CI keeps with the run's logs.
func (r *Browser) screenshotDir() string {
	if r.screenshots != "" {
		return r.screenshots
	}
	if r.container != nil {
		if dir := r.container.RunDir(); dir != "" {
			return filepath.Join(dir, "screenshots")
		}
	}
	return filepath.Join(".tomato", "screenshots")
}

var unsafeFileChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// fileName makes a name safe to use as a file name, keeping it readable.
func fileName(name string) string {
	s := strings.Trim(unsafeFileChars.ReplaceAllString(name, "-"), "-.")
	if s == "" {
		return "screenshot"
	}
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

// browserContextID is the id of the person's browser context, which tests use
// to tell two people apart.
func (r *Browser) browserContextID() cdp.BrowserContextID {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tab == nil {
		return ""
	}
	if c := chromedp.FromContext(r.tab); c != nil {
		return c.BrowserContextID
	}
	return ""
}
