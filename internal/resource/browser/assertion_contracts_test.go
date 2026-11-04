package browser

import (
	"github.com/chromedp/chromedp"
	"testing"
)

func TestPressHonorsPreventDefaultWhenFocusChanges(t *testing.T) {
	for _, key := range []string{"Enter", "Space"} {
		t.Run(key, func(t *testing.T) {
			srv := site(t, map[string]string{"/": `<div role="button" tabindex="0" aria-label="Open review" onkeydown="if(event.key==='Enter'||event.key===' '){event.preventDefault();panel.hidden=false;closeReview.focus()}">Open review</div>
 <div id="panel" role="dialog" aria-label="Review" hidden><p>Review stays open</p><button id="closeReview" onclick="panel.hidden=true">Close review</button></div>`})
			r := person(t, srv, nil)
			must(t, r.open("/"))
			must(t, r.click("Open review"))
			must(t, r.press(key))
			must(t, r.sees("Review stays open"))
			focused, err := chromedp.Run(r.tab, chromedp.Evaluate[string](`document.activeElement.id`))
			must(t, err)
			if focused != "closeReview" {
				t.Fatalf("focus = %q, want closeReview", focused)
			}
		})
	}
}

func TestControlStateWaitsForTheConditionAndReacquiresReplacedControls(t *testing.T) {
	srv := site(t, map[string]string{"/": `<button id="apply" disabled>Apply</button><script>setTimeout(()=>{apply.outerHTML='<button>Apply</button>'},350)</script>`})
	r := person(t, srv, map[string]any{"timeout": "2s"})
	must(t, r.open("/"))
	must(t, r.seesEnabled("Apply"))
}

func TestControlStateWaitsForDisabling(t *testing.T) {
	srv := site(t, map[string]string{"/": `<button id="apply">Apply</button><script>setTimeout(()=>{apply.disabled=true},350)</script>`})
	r := person(t, srv, map[string]any{"timeout": "2s"})
	must(t, r.open("/"))
	must(t, r.seesDisabled("Apply"))
}

func TestStateAssertionFollowsAnUppercaseLabelToItsControl(t *testing.T) {
	srv := site(t, map[string]string{"/": `<label for="written" style="text-transform:uppercase">Written form</label><input id="written" disabled>`})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.seesDisabled("Written form"))
}

func TestPressTypesOnceAndHonorsPreventedPrintableKeys(t *testing.T) {
	srv := site(t, map[string]string{"/": `<label>Notes <input onkeydown="if(event.key==='x')event.preventDefault()"></label>`})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.click("Notes"))
	must(t, r.press("a"))
	must(t, r.press("x"))
	value, err := chromedp.Run(r.tab, chromedp.Evaluate[string](`document.querySelector('input').value`))
	must(t, err)
	if value != "a" {
		t.Fatalf("value = %q, want exactly one a and no prevented x", value)
	}
}

func TestNamedRolesScopeContentWithoutMatchingTheBackground(t *testing.T) {
	srv := site(t, map[string]string{"/": `<p>Ready from the background</p><h2>Review</h2><input value="outside value">
 <section role="dialog" aria-label="Review"><p>Selected question</p><label>Answer <input value="inside value"></label><input type="password" value="secret"><p hidden>hidden copy</p></section>
 <section role="dialog" aria-label="Review" hidden><p>Hidden duplicate</p></section>`})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.seesRoleWithin("dialog", "Review", "200ms"))
	must(t, r.seesInRoleWithin("Selected question", "dialog", "Review", "200ms"))
	must(t, r.seesInRoleWithin("inside value", "dialog", "Review", "200ms"))
	must(t, r.doesNotSeeInRole("Ready from the background", "dialog", "Review"))
	must(t, r.doesNotSeeInRole("outside value", "dialog", "Review"))
	must(t, r.doesNotSeeInRole("hidden copy", "dialog", "Review"))
	must(t, r.doesNotSeeInRole("secret", "dialog", "Review"))
	mustFail(t, r.seesInRoleWithin("Ready from the background", "dialog", "Review", "50ms"), "region shows")
	mustFail(t, r.doesNotSeeInRole("Selected question", "dialog", "Review"), "expected the region not to show")
	mustFail(t, r.doesNotSeeInRole("anything", "dialog", "Absent"), "no visible dialog")
	mustFail(t, r.seesRoleWithin("button", "Review", "50ms"), "no visible button")
}

func TestNamedRolesRejectAmbiguityAndUseImplicitSemantics(t *testing.T) {
	srv := site(t, map[string]string{"/": `<button aria-label="Close review">x</button><section role="dialog" aria-label="Review">One</section><section role="dialog" aria-label="Review">Two</section>`})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.seesRoleWithin("button", "Close review", "200ms"))
	mustFail(t, r.seesRoleWithin("dialog", "Review", "50ms"), "2 visible dialog elements")
	mustFail(t, r.noLongerSeesRole("dialog", "Review", "50ms"), "2 visible dialog elements")
	mustFail(t, r.seesRoleWithin("dialog", "Review", "soon"), "invalid duration")
	mustFail(t, r.seesInRoleWithin("One", "dialog", "Review", "soon"), "invalid duration")
	mustFail(t, r.noLongerSeesRole("dialog", "Review", "soon"), "invalid duration")
}

func TestNamedRolesWaitAcrossReplacementAndDisappearance(t *testing.T) {
	srv := site(t, map[string]string{"/": `<section role="dialog" aria-label="Review" id="panel">Loading</section><button onclick="panel.hidden=true">Close review</button>
 <script>setTimeout(()=>{panel.outerHTML='<section role="dialog" aria-label="Review" id="panel">Ready</section>'},200)</script>`})
	r := person(t, srv, nil)
	must(t, r.open("/"))
	must(t, r.seesInRoleWithin("Ready", "dialog", "Review", "2s"))
	mustFail(t, r.noLongerSeesRole("dialog", "Review", "50ms"), "still visible")
	must(t, r.click("Close review"))
	must(t, r.noLongerSeesRole("dialog", "Review", "2s"))
}
