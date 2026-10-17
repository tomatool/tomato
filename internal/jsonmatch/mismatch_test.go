package jsonmatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

// Each resource that compares a whole value spelled its failure by hand
// before Mismatch existed. The text is part of every console log, JUnit and
// Cucumber report, so Mismatch must reproduce it byte for byte.
func TestMismatchErrorIsTheTextTheSitesPrintedBefore(t *testing.T) {
	exp := `{"sku": "TMT-01", "qty": 2, "status": "created"}`
	act := `{"id":"1","sku":"TMT-01","qty":2,"status":"pending"}`
	cases := []struct {
		site string
		m    *Mismatch
		was  string
	}{
		// kafka.go, rabbitmq.go, websocket/client.go (shouldReceiveMessage, lastMessageShouldBe)
		{"message", &Mismatch{What: "message", Expected: exp, Actual: act, Mode: ModeText},
			fmt.Sprintf("message mismatch:\nexpected: %s\nactual: %s", exp, act)},
		// websocket/client.go (shouldReceiveJSONMatching, lastMessageShouldBeJSONMatching)
		{"websocket JSON", &Mismatch{What: "JSON", Expected: exp, Actual: act, Mode: ModeJSON},
			fmt.Sprintf("JSON mismatch:\nexpected: %s\nactual: %s", exp, act)},
		// httpclient/http_client.go (responseBodyShouldBe)
		{"http body", &Mismatch{What: "body", Expected: exp, Actual: act, Mode: ModeText},
			fmt.Sprintf("body mismatch:\nexpected: %s\nactual: %s", exp, act)},
		// shell/shell.go (stdoutShouldBe): no colon after the heading
		{"shell stdout", &Mismatch{What: "stdout", Expected: "a\nb", Actual: "a\nc", Mode: ModeText, NoColon: true},
			fmt.Sprintf("stdout mismatch\nexpected: %s\nactual: %s", "a\nb", "a\nc")},
		{"empty actual", &Mismatch{What: "message", Expected: "x", Actual: "", Mode: ModeText},
			fmt.Sprintf("message mismatch:\nexpected: %s\nactual: %s", "x", "")},
	}
	for _, c := range cases {
		if got := c.m.Error(); got != c.was {
			t.Errorf("%s: Error() = %q, want %q", c.site, got, c.was)
		}
	}
}

func TestMismatchIsFoundThroughWrapping(t *testing.T) {
	inner := &Mismatch{What: "message", Expected: "a", Actual: "b", Mode: ModeText}
	err := fmt.Errorf("no matching message: %w", fmt.Errorf("last: %w", inner))

	var m *Mismatch
	if !errors.As(err, &m) {
		t.Fatal("errors.As did not find the Mismatch through two %w layers")
	}
	if m != inner {
		t.Errorf("found %+v, want the wrapped one", m)
	}
	if want := "no matching message: last: message mismatch:\nexpected: a\nactual: b"; err.Error() != want {
		t.Errorf("wrapped text = %q, want %q", err.Error(), want)
	}
	if errors.As(errors.New("plain"), &m) {
		t.Error("errors.As found a Mismatch in a plain error")
	}
}

func decode(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// CompareDocs keeps Compare's text, so the JSON steps that use it print what
// they always did, and adds both documents and the mode.
func TestCompareDocs(t *testing.T) {
	exp := `{"user": {"status": "created"}}`
	act := `{"id": 1, "user": {"status": "pending", "email": "ada@example.com"}}`

	for _, partial := range []bool{true, false} {
		err := CompareDocs("response JSON", decode(t, exp), decode(t, act), exp, act, partial)
		want := Compare(decode(t, exp), decode(t, act), "", partial)
		if err == nil || want == nil {
			t.Fatalf("partial=%v: want a failure from both, got %v and %v", partial, err, want)
		}
		if err.Error() != want.Error() {
			t.Errorf("partial=%v: text = %q, Compare said %q", partial, err.Error(), want.Error())
		}
		var m *Mismatch
		if !errors.As(err, &m) {
			t.Fatalf("partial=%v: not a Mismatch: %T", partial, err)
		}
		wantMode := ModeJSON
		if partial {
			wantMode = ModeSubset
		}
		if m.Mode != wantMode || m.What != "response JSON" || m.Expected != exp || m.Actual != act {
			t.Errorf("partial=%v: got %+v", partial, m)
		}
		if errors.Unwrap(err) == nil {
			t.Errorf("partial=%v: Unwrap lost the cause", partial)
		}
	}

	if err := CompareDocs("x", decode(t, `{"a": 1}`), decode(t, `{"a": 1, "b": 2}`), "", "", true); err != nil {
		t.Errorf("a subset that matches failed: %v", err)
	}
}
