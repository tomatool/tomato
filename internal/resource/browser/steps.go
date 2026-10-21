package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/rs/zerolog/log"
	"github.com/tomatool/tomato/internal/resource"
)

// Steps returns the structured step definitions for the browser resource.
func (r *Browser) Steps() resource.StepCategory {
	return resource.StepCategory{
		Name:        "Browser",
		Description: "Steps for using a web UI in a real Chromium, the way a person does: by what the page shows",
		Steps: []resource.StepDef{
			// Navigation
			{
				Group:       "Navigation",
				Pattern:     `^"{resource}" opens "([^"]*)"$`,
				Description: "Open a page and wait until it has loaded. A path is joined to base_url",
				Example:     `"alice-browser" opens "/master"`,
				Handler:     r.open,
			},

			// Using the page
			{
				Group:       "Using the Page",
				Pattern:     `^"{resource}" clicks "([^"]*)"$`,
				Description: "Click what is called this: a button, link or tab by its accessible name, or else the element showing this text. Waits until it is there, enabled and not covered",
				Example:     `"alice-browser" clicks "Apply"`,
				Handler:     r.click,
			},
			{
				Group:       "Using the Page",
				Pattern:     `^"{resource}" fills "([^"]*)" with "([^"]*)"$`,
				Description: "Replace what a field holds. The field is named by its label, placeholder or accessible name; a contenteditable editor counts as a field",
				Example:     `"alice-browser" fills "Search" with "Zinsen"`,
				Handler:     r.fill,
			},
			{
				Group:       "Using the Page",
				Pattern:     `^"{resource}" types "([^"]*)" at the end of "([^"]*)"$`,
				Description: "Put the caret at the end of the paragraph or field showing this text, and type there key by key",
				Example:     `"alice-browser" types " und Gebühren" at the end of "7.2"`,
				Handler:     r.typeAtEnd,
			},
			{
				Group:       "Using the Page",
				Pattern:     `^"{resource}" types "([^"]*)"$`,
				Description: "Type key by key into whatever has focus",
				Example:     `"alice-browser" types "Neuer Satz."`,
				Handler:     r.typeText,
			},
			{
				Group:       "Using the Page",
				Pattern:     `^"{resource}" presses "([^"]*)"$`,
				Description: "Press a key or a shortcut, such as Enter, Escape, Tab, ArrowDown or ctrl+z",
				Example:     `"alice-browser" presses "ctrl+z"`,
				Handler:     r.press,
			},
			{
				Group:       "Using the Page",
				Pattern:     `^"{resource}" checks "([^"]*)"$`,
				Description: "Tick a checkbox or switch by its label, unless it is ticked already",
				Example:     `"alice-browser" checks "Austria"`,
				Handler:     r.check,
			},
			{
				Group:       "Using the Page",
				Pattern:     `^"{resource}" unchecks "([^"]*)"$`,
				Description: "Clear a checkbox or switch by its label, unless it is clear already",
				Example:     `"alice-browser" unchecks "Netherlands"`,
				Handler:     r.uncheck,
			},
			{
				Group:       "Using the Page",
				Pattern:     `^"{resource}" chooses "([^"]*)"$`,
				Description: "Choose a radio button, tab or option by its label",
				Example:     `"alice-browser" chooses "Selected countries"`,
				Handler:     r.choose,
			},
			{
				Group:       "Using the Page",
				Pattern:     `^"{resource}" selects "([^"]*)" from "([^"]*)"$`,
				Description: "Pick an option of a select box or combobox, both by what they show",
				Example:     `"alice-browser" selects "Austria" from "Country"`,
				Handler:     r.selectFrom,
			},

			// What the page shows
			{
				Group:       "What the Page Shows",
				Pattern:     `^"{resource}" sees "([^"]*)"$`,
				Description: "Assert the page shows this text now, in its content or in a field",
				Example:     `"alice-browser" sees "7 Zinsen"`,
				Handler:     r.sees,
			},
			{
				Group:       "What the Page Shows",
				Pattern:     `^"{resource}" sees "([^"]*)" within "([^"]*)"$`,
				Description: "Wait until the page shows this text, for at most the duration",
				Example:     `"bob-browser" sees "Neuer Satz." within "3s"`,
				Handler:     r.seesWithin,
			},
			{
				Group:       "What the Page Shows",
				Pattern:     `^"{resource}" does not see "([^"]*)"$`,
				Description: "Assert the page does not show this text now",
				Example:     `"alice-browser" does not see "clause_id"`,
				Handler:     r.doesNotSee,
			},
			{
				Group:       "What the Page Shows",
				Pattern:     `^"{resource}" no longer sees "([^"]*)" within "([^"]*)"$`,
				Description: "Wait until the page stops showing this text, for at most the duration",
				Example:     `"alice-browser" no longer sees "Saving…" within "5s"`,
				Handler:     r.noLongerSees,
			},
			{
				Group:       "What the Page Shows",
				Pattern:     `^"{resource}" sees "([^"]*)" is disabled$`,
				Description: "Assert a button, field or other control is disabled",
				Example:     `"alice-browser" sees "Apply" is disabled`,
				Handler:     r.seesDisabled,
			},
			{
				Group:       "What the Page Shows",
				Pattern:     `^"{resource}" sees "([^"]*)" is enabled$`,
				Description: "Assert a button, field or other control is enabled",
				Example:     `"alice-browser" sees "Apply" is enabled`,
				Handler:     r.seesEnabled,
			},
			{
				Group:       "What the Page Shows",
				Pattern:     `^"{resource}" url is "([^"]*)"$`,
				Description: "Assert the page's address: the path with its query and fragment, or the whole URL when one is given",
				Example:     `"alice-browser" url is "/master/7.2"`,
				Handler:     r.urlIs,
			},

			// Screenshots
			{
				Group:       "Screenshots",
				Pattern:     `^"{resource}" saves screenshot "([^"]*)"$`,
				Description: "Save what the page shows as <name>.png in the screenshots directory",
				Example:     `"alice-browser" saves screenshot "after-apply"`,
				Handler:     r.saveScreenshot,
			},
		},
	}
}

// stepContext bounds one step: long enough for its waits, so a browser that
// stops answering fails the step instead of hanging the run.
func (r *Browser) stepContext(wait time.Duration) (context.Context, context.CancelFunc, error) {
	tab, err := r.page()
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(tab, wait+10*time.Second)
	return ctx, cancel, nil
}

// retry runs attempt until it succeeds or wait has passed, and returns its
// last error. Everything after a click is asynchronous, so a step that names
// an element waits for it rather than failing on the first look.
func retry(ctx context.Context, wait time.Duration, attempt func() error) error {
	deadline := time.Now().Add(wait)
	for {
		err := attempt()
		if err == nil {
			return nil
		}
		if !time.Now().Before(deadline) || ctx.Err() != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return err
		case <-time.After(pollInterval):
		}
	}
}

func parseWait(s string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: %w", s, err)
	}
	return d, nil
}

// --- the page helpers -------------------------------------------------------

// document returns the page's document, which the page helpers run on.
func document(ctx context.Context) (runtime.RemoteObjectID, error) {
	doc, err := chromedp.Run(ctx, chromedp.Evaluate[*runtime.RemoteObject](`document`))
	if err != nil {
		return "", err
	}
	return doc.ObjectID, nil
}

// call runs a page.js helper on the object, which is the document or an
// element, and decodes its result into T.
func call[T any](ctx context.Context, on runtime.RemoteObjectID, op string, args ...any) (T, error) {
	all := append([]any{op}, args...)
	return chromedp.Run(ctx, chromedp.CallFunctionOn[T](pageJS, func(p *runtime.CallFunctionOnParams) {
		p.ObjectID = on
	}, all...))
}

// textRoles are accessibility nodes that only carry text: a button's label
// is a StaticText named "Apply" inside the button named "Apply".
var textRoles = map[string]bool{
	"StaticText": true, "InlineTextBox": true, "LineBreak": true,
	"RootWebArea": true, "WebArea": true, "none": true,
}

// locate finds the one element that a person would call target, as a kind
// of control: what the accessibility tree names target first, then the
// element whose visible text is target. It fails when there is none, or more
// than one with nothing to choose between them.
func locate(ctx context.Context, target, kind string) (runtime.RemoteObjectID, error) {
	// Accessible names have their whitespace collapsed, as page.js does to
	// visible text.
	target = normalize(target)
	doc, err := document(ctx)
	if err != nil {
		return "", err
	}
	tree, err := chromedp.Call(ctx, accessibility.QueryAXTree, accessibility.QueryAXTreeParams{
		ObjectID: doc, AccessibleName: target,
	})
	if err != nil {
		return "", fmt.Errorf("reading the accessibility tree: %w", err)
	}

	args := []*runtime.CallArgument{jsArg("locate"), jsArg(kind), jsArg(target)}
	seen := map[int64]bool{}
	for _, n := range tree.Nodes {
		if n.Ignored || n.BackendDOMNodeID == 0 || seen[int64(n.BackendDOMNodeID)] {
			continue
		}
		if n.Role != nil && textRoles[jsonString(n.Role.Value)] {
			continue
		}
		seen[int64(n.BackendDOMNodeID)] = true
		node, err := chromedp.Call(ctx, dom.ResolveNode, dom.ResolveNodeParams{BackendNodeID: n.BackendDOMNodeID})
		if err != nil || node.Object == nil {
			continue
		}
		args = append(args, &runtime.CallArgument{ObjectID: node.Object.ObjectID})
	}

	res, err := chromedp.Call(ctx, runtime.CallFunctionOn, runtime.CallFunctionOnParams{
		FunctionDeclaration: pageJS,
		ObjectID:            doc,
		Arguments:           args,
	})
	if err != nil {
		return "", err
	}
	if res.ExceptionDetails != nil {
		return "", &chromedp.ExceptionError{ExceptionDetails: res.ExceptionDetails}
	}
	if res.Result == nil {
		return "", errors.New("locating an element returned nothing")
	}
	if res.Result.Type == runtime.RemoteObjectTypeString {
		return "", errors.New(jsonString(res.Result.Value))
	}
	return res.Result.ObjectID, nil
}

func jsArg(v any) *runtime.CallArgument {
	b, _ := json.Marshal(v)
	return &runtime.CallArgument{Value: b}
}

func jsonString(v []byte) string {
	var s string
	if err := json.Unmarshal(v, &s); err != nil {
		return string(v)
	}
	return s
}

// errorIf turns a helper's error message into an error; "" is success.
func errorIf(msg string, err error) error {
	if err != nil {
		return err
	}
	if msg != "" {
		return errors.New(msg)
	}
	return nil
}

// clickPoint is where page.js says to click, or why it cannot be clicked.
type clickPoint struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Error string  `json:"error"`
}

// findClickable finds the element target names once, and where to click it
// if it is enabled and uncovered.
func (r *Browser) findClickable(ctx context.Context, target, kind string) (runtime.RemoteObjectID, clickPoint, error) {
	id, err := locate(ctx, target, kind)
	if err != nil {
		return "", clickPoint{}, err
	}
	at, err := call[clickPoint](ctx, id, "point")
	if err != nil {
		return "", clickPoint{}, err
	}
	if at.Error != "" {
		return "", clickPoint{}, errors.New(at.Error)
	}
	return id, at, nil
}

// clickOn clicks the element target names, as a person would: it waits until
// the element is there, enabled and uncovered, clicks the middle of it, and
// tries again when the click landed on something else because the page moved
// in between.
func (r *Browser) clickOn(ctx context.Context, target, kind string) (runtime.RemoteObjectID, error) {
	var id runtime.RemoteObjectID
	err := retry(ctx, r.timeout, func() error {
		var at clickPoint
		var err error
		if id, at, err = r.findClickable(ctx, target, kind); err != nil {
			return err
		}
		if err := errorIf(call[string](ctx, id, "arm")); err != nil {
			return err
		}
		if err := chromedp.Do(ctx, chromedp.MouseClickXY(at.X, at.Y)); err != nil {
			return err
		}
		doc, err := document(ctx)
		if err != nil {
			// The click navigated away, so it landed.
			return nil
		}
		missed, err := call[string](ctx, doc, "disarm")
		if err != nil {
			return nil
		}
		return errorIf(missed, nil)
	})
	return id, err
}

// --- navigation ---------------------------------------------------------------

// resolve turns what a step names into the URL to open: a path is joined to
// base_url, and a loopback host is spelled localhost (see loopbackAsLocalhost).
func (r *Browser) resolve(target string) (string, error) {
	target = resource.ReplaceVariables(target)
	u, err := url.Parse(target)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", target, err)
	}
	// http://…, and data: or about: pages, which have no host.
	if u.IsAbs() && (u.Host != "" || u.Opaque != "") {
		return loopbackAsLocalhost(u).String(), nil
	}
	if r.baseURL == "" {
		return "", fmt.Errorf("%q is a path, and %q has no base_url to open it on", target, r.name)
	}
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}
	return r.baseURL + target, nil
}

func (r *Browser) open(target string) error {
	address, err := r.resolve(target)
	if err != nil {
		return err
	}
	ctx, cancel, err := r.stepContext(30 * time.Second)
	if err != nil {
		return err
	}
	defer cancel()
	resp, err := chromedp.RunResponse(ctx, chromedp.Navigate(address))
	if err != nil {
		return fmt.Errorf("opening %s: %w", address, err)
	}
	if resp != nil && resp.Status >= 400 {
		return fmt.Errorf("opening %s: the server answered %d %s", address, resp.Status, resp.StatusText)
	}
	return nil
}

func (r *Browser) urlIs(expected string) error {
	expected = resource.ReplaceVariables(expected)
	ctx, cancel, err := r.stepContext(r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	current, err := chromedp.Run(ctx, chromedp.Location())
	if err != nil {
		return err
	}
	got := current
	if u, err := url.Parse(expected); err == nil && u.IsAbs() {
		expected = loopbackAsLocalhost(u).String()
	} else {
		u, err := url.Parse(current)
		if err != nil {
			return err
		}
		got = u.RequestURI()
		if u.Fragment != "" {
			got += "#" + u.Fragment
		}
	}
	if got != expected {
		return fmt.Errorf("%q is at %q, expected %q", r.name, got, expected)
	}
	return nil
}

// --- using the page -----------------------------------------------------------

func (r *Browser) click(target string) error {
	target = resource.ReplaceVariables(target)
	ctx, cancel, err := r.stepContext(r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	if _, err := r.clickOn(ctx, target, "click"); err != nil {
		return fmt.Errorf("%q cannot click %q: %w", r.name, target, err)
	}
	return nil
}

func (r *Browser) fill(field, value string) error {
	field = resource.ReplaceVariables(field)
	value = resource.ReplaceVariables(value)
	ctx, cancel, err := r.stepContext(r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	err = retry(ctx, r.timeout, func() error {
		id, err := locate(ctx, field, "field")
		if err != nil {
			return err
		}
		return errorIf(call[string](ctx, id, "focusAll"))
	})
	if err != nil {
		return fmt.Errorf("%q cannot fill %q: %w", r.name, field, err)
	}
	if value == "" {
		return chromedp.Do(ctx, chromedp.KeyEvent(kb.Backspace))
	}
	// Inserted as one edit, as a paste or an IME commit would be; "types"
	// sends key by key.
	_, err = chromedp.Call(ctx, input.InsertText, input.InsertTextParams{Text: value})
	return err
}

func (r *Browser) typeAtEnd(text, target string) error {
	text = resource.ReplaceVariables(text)
	target = resource.ReplaceVariables(target)
	ctx, cancel, err := r.stepContext(r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	err = retry(ctx, r.timeout, func() error {
		id, err := locate(ctx, target, "block")
		if err != nil {
			return err
		}
		return errorIf(call[string](ctx, id, "caretAtEnd"))
	})
	if err != nil {
		return fmt.Errorf("%q cannot type at the end of %q: %w", r.name, target, err)
	}
	return chromedp.Do(ctx, chromedp.KeyEvent(text))
}

func (r *Browser) typeText(text string) error {
	text = resource.ReplaceVariables(text)
	ctx, cancel, err := r.stepContext(r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	doc, err := document(ctx)
	if err != nil {
		return err
	}
	focused, err := call[string](ctx, doc, "focused")
	if err != nil {
		return err
	}
	if focused == "" {
		return fmt.Errorf("%q has nothing focused to type into: click a field first, or use `fills`", r.name)
	}
	return chromedp.Do(ctx, chromedp.KeyEvent(text))
}

func (r *Browser) press(keys string) error {
	key, mods, err := parseKeys(keys)
	if err != nil {
		return err
	}
	ctx, cancel, err := r.stepContext(r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	return chromedp.Do(ctx, chromedp.KeyEvent(string(key), chromedp.KeyModifiers(mods...)))
}

// keyNames are the names "presses" accepts besides those in chromedp's key
// table (Enter, Escape, ArrowDown, F5, …), which match case-insensitively.
var keyNames = map[string]rune{
	"esc": []rune(kb.Escape)[0], "return": []rune(kb.Enter)[0], "space": ' ',
	"del": []rune(kb.Delete)[0], "up": []rune(kb.ArrowUp)[0], "down": []rune(kb.ArrowDown)[0],
	"left": []rune(kb.ArrowLeft)[0], "right": []rune(kb.ArrowRight)[0],
}

var modifierNames = map[string]chromedp.Modifier{
	"ctrl":    chromedp.ModifierCtrl,
	"control": chromedp.ModifierCtrl,
	"shift":   chromedp.ModifierShift,
	"alt":     chromedp.ModifierAlt,
	"option":  chromedp.ModifierAlt,
	"meta":    chromedp.ModifierMeta,
	"cmd":     chromedp.ModifierMeta,
	"command": chromedp.ModifierMeta,
}

// namedKey finds a key by its name in chromedp's key table: Enter, Tab,
// Backspace, ArrowDown, F5 and the rest, in any case.
func namedKey(name string) (rune, bool) {
	found, ok := rune(0), false
	for r, k := range kb.Keys {
		// The lowest rune wins, so a name the table repeats always maps to
		// the same key.
		if strings.EqualFold(k.Key, name) && (!ok || r < found) {
			found, ok = r, true
		}
	}
	return found, ok
}

// parseKeys reads "ctrl+shift+z", "Enter" or "a".
func parseKeys(keys string) (rune, []chromedp.Modifier, error) {
	if keys == "+" {
		return '+', nil, nil
	}
	parts := strings.Split(keys, "+")
	// "ctrl++" presses ctrl and plus.
	if strings.HasSuffix(keys, "++") {
		parts = append(strings.Split(strings.TrimSuffix(keys, "++"), "+"), "+")
	}
	var mods []chromedp.Modifier
	for _, p := range parts[:len(parts)-1] {
		m, ok := modifierNames[strings.ToLower(strings.TrimSpace(p))]
		if !ok {
			return 0, nil, fmt.Errorf("unknown modifier %q in %q (use ctrl, shift, alt or meta)", p, keys)
		}
		mods = append(mods, m)
	}
	name := strings.TrimSpace(parts[len(parts)-1])
	if r := []rune(name); len(r) == 1 {
		return r[0], mods, nil
	}
	if r, ok := keyNames[strings.ToLower(name)]; ok {
		return r, mods, nil
	}
	if r, ok := namedKey(name); ok {
		return r, mods, nil
	}
	return 0, nil, fmt.Errorf("unknown key %q in %q (use a character, or a name such as Enter, Escape, Tab, Backspace, ArrowDown)", name, keys)
}

func (r *Browser) check(target string) error   { return r.setChecked(target, true) }
func (r *Browser) uncheck(target string) error { return r.setChecked(target, false) }

func (r *Browser) setChecked(target string, want bool) error {
	target = resource.ReplaceVariables(target)
	verb := "check"
	if !want {
		verb = "uncheck"
	}
	ctx, cancel, err := r.stepContext(3 * r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	var state bool
	err = retry(ctx, r.timeout, func() error {
		id, err := locate(ctx, target, "checkbox")
		if err != nil {
			return err
		}
		state, err = call[bool](ctx, id, "checked")
		return err
	})
	if err != nil {
		return fmt.Errorf("%q cannot %s %q: %w", r.name, verb, target, err)
	}
	if state == want {
		return nil
	}
	id, err := r.clickOn(ctx, target, "checkbox")
	if err != nil {
		return fmt.Errorf("%q cannot %s %q: %w", r.name, verb, target, err)
	}
	return r.waitChecked(ctx, id, target, want, verb)
}

// waitChecked waits for a click to take effect: a page can refuse it, and
// the step must not pass when it did.
func (r *Browser) waitChecked(ctx context.Context, id runtime.RemoteObjectID, target string, want bool, verb string) error {
	err := retry(ctx, r.timeout, func() error {
		state, err := call[bool](ctx, id, "checked")
		if err != nil {
			return err
		}
		if state != want {
			return fmt.Errorf("clicking %q did not %s it", target, verb)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%q cannot %s %q: %w", r.name, verb, target, err)
	}
	return nil
}

func (r *Browser) choose(target string) error {
	target = resource.ReplaceVariables(target)
	ctx, cancel, err := r.stepContext(3 * r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	var id runtime.RemoteObjectID
	var native bool
	err = retry(ctx, r.timeout, func() error {
		var err error
		if id, err = locate(ctx, target, "choice"); err != nil {
			return err
		}
		native, err = call[bool](ctx, id, "isNativeSelect")
		return err
	})
	if err != nil {
		return fmt.Errorf("%q cannot choose %q: %w", r.name, target, err)
	}
	// An option of a native <select> is not on the page to click: the
	// browser draws the list itself.
	if native {
		return errorIf(call[string](ctx, id, "selectOption", target))
	}
	id, err = r.clickOn(ctx, target, "choice")
	if err != nil {
		return fmt.Errorf("%q cannot choose %q: %w", r.name, target, err)
	}
	return r.waitChecked(ctx, id, target, true, "choose")
}

func (r *Browser) selectFrom(option, field string) error {
	option = resource.ReplaceVariables(option)
	field = resource.ReplaceVariables(field)
	ctx, cancel, err := r.stepContext(3 * r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	var id runtime.RemoteObjectID
	var native bool
	err = retry(ctx, r.timeout, func() error {
		var err error
		if id, err = locate(ctx, field, "select"); err != nil {
			return err
		}
		native, err = call[bool](ctx, id, "isNativeSelect")
		return err
	})
	if err != nil {
		return fmt.Errorf("%q cannot select from %q: %w", r.name, field, err)
	}
	if native {
		if err := errorIf(call[string](ctx, id, "selectOption", option)); err != nil {
			return fmt.Errorf("%q cannot select %q from %q: %w", r.name, option, field, err)
		}
		return nil
	}
	// A combobox a page draws itself opens on a click and lists its
	// options as elements.
	if _, err := r.clickOn(ctx, field, "select"); err != nil {
		return fmt.Errorf("%q cannot open %q: %w", r.name, field, err)
	}
	if _, err := r.clickOn(ctx, option, "choice"); err != nil {
		return fmt.Errorf("%q cannot select %q from %q: %w", r.name, option, field, err)
	}
	return nil
}

// --- what the page shows ------------------------------------------------------

// pageText is everything the page shows.
func pageText(ctx context.Context) (string, error) {
	doc, err := document(ctx)
	if err != nil {
		return "", err
	}
	return call[string](ctx, doc, "text")
}

// showing reports whether the page shows text, with what it shows instead.
func showing(ctx context.Context, text string) (bool, string, error) {
	shown, err := pageText(ctx)
	if err != nil {
		return false, "", err
	}
	return strings.Contains(shown, normalize(text)), shown, nil
}

func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// excerpt is what a failed assertion quotes of the page.
func excerpt(shown string) string {
	const max = 500
	if r := []rune(shown); len(r) > max {
		return string(r[:max]) + "…"
	}
	return shown
}

func (r *Browser) sees(text string) error {
	text = resource.ReplaceVariables(text)
	ctx, cancel, err := r.stepContext(r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	ok, shown, err := showing(ctx, text)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%q does not see %q; the page shows: %q", r.name, text, excerpt(shown))
	}
	return nil
}

func (r *Browser) seesWithin(text, within string) error {
	text = resource.ReplaceVariables(text)
	wait, err := parseWait(within)
	if err != nil {
		return err
	}
	ctx, cancel, err := r.stepContext(wait)
	if err != nil {
		return err
	}
	defer cancel()
	var shown string
	err = retry(ctx, wait, func() error {
		var ok bool
		var err error
		ok, shown, err = showing(ctx, text)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("not shown")
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%q did not see %q within %s; the page shows: %q", r.name, text, within, excerpt(shown))
	}
	return nil
}

func (r *Browser) doesNotSee(text string) error {
	text = resource.ReplaceVariables(text)
	ctx, cancel, err := r.stepContext(r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	ok, _, err := showing(ctx, text)
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("%q sees %q, expected the page not to show it", r.name, text)
	}
	return nil
}

func (r *Browser) noLongerSees(text, within string) error {
	text = resource.ReplaceVariables(text)
	wait, err := parseWait(within)
	if err != nil {
		return err
	}
	ctx, cancel, err := r.stepContext(wait)
	if err != nil {
		return err
	}
	defer cancel()
	err = retry(ctx, wait, func() error {
		ok, _, err := showing(ctx, text)
		if err != nil {
			return err
		}
		if ok {
			return errors.New("still shown")
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("%q still sees %q after %s", r.name, text, within)
	}
	return nil
}

func (r *Browser) seesDisabled(target string) error { return r.seesState(target, true) }
func (r *Browser) seesEnabled(target string) error  { return r.seesState(target, false) }

func (r *Browser) seesState(target string, wantDisabled bool) error {
	target = resource.ReplaceVariables(target)
	ctx, cancel, err := r.stepContext(r.timeout)
	if err != nil {
		return err
	}
	defer cancel()
	var id runtime.RemoteObjectID
	err = retry(ctx, r.timeout, func() error {
		var err error
		id, err = locate(ctx, target, "any")
		return err
	})
	if err != nil {
		return fmt.Errorf("%q does not see %q: %w", r.name, target, err)
	}
	disabled, err := call[bool](ctx, id, "disabled")
	if err != nil {
		return err
	}
	if disabled != wantDisabled {
		state, want := "enabled", "disabled"
		if disabled {
			state, want = want, state
		}
		desc, _ := call[string](ctx, id, "describe")
		return fmt.Errorf("%q sees %q is %s, expected %s (%s)", r.name, target, state, want, desc)
	}
	return nil
}

// --- screenshots --------------------------------------------------------------

func (r *Browser) saveScreenshot(name string) error {
	name = resource.ReplaceVariables(name)
	_, path, err := r.capture(name)
	if err != nil {
		return err
	}
	log.Info().Str("resource", r.name).Str("path", path).Msg("screenshot saved")
	return nil
}
