package runner

import (
	"strings"
	"testing"

	messages "github.com/cucumber/messages/go/v21"
)

func TestToGodogTags(t *testing.T) {
	tests := []struct {
		expr, want string
	}{
		{"", ""},
		{"@smoke", "@smoke"},
		{"@smoke && ~@slow", "@smoke && ~@slow"}, // legacy syntax passes through
		{"not @live", "~@live"},
		{"@smoke and not @slow", "@smoke && ~@slow"},
		{"@smoke or @api", "@smoke,@api"},
		{"(@a or @b) and not @wip", "@a,@b && ~@wip"},
		{"@a or @b and @c", "@a,@b && @a,@c"},
		{"not (@a and @b)", "~@a,~@b"},
		{"not (@a or @b)", "~@a && ~@b"},
	}
	for _, tt := range tests {
		got, err := toGodogTags(tt.expr)
		if err != nil {
			t.Errorf("toGodogTags(%q) error: %v", tt.expr, err)
			continue
		}
		if got != tt.want {
			t.Errorf("toGodogTags(%q) = %q, want %q", tt.expr, got, tt.want)
		}
	}
}

func TestToGodogTagsInvalid(t *testing.T) {
	for _, expr := range []string{"@a and", "(@a or @b", "not", "@a and smoke", "@a )"} {
		if _, err := toGodogTags(expr); err == nil {
			t.Errorf("toGodogTags(%q) expected error", expr)
		}
	}
}

// TestToGodogTagsMatchesCucumberSemantics checks the translated filter
// against godog's own matcher for every combination of three tags.
func TestToGodogTagsMatchesCucumberSemantics(t *testing.T) {
	exprs := map[string]func(a, b, c bool) bool{
		"not @a":                  func(a, b, c bool) bool { return !a },
		"@a and not @b":           func(a, b, c bool) bool { return a && !b },
		"@a or @b and @c":         func(a, b, c bool) bool { return a || (b && c) },
		"(@a or @b) and not @c":   func(a, b, c bool) bool { return (a || b) && !c },
		"not (@a and (@b or @c))": func(a, b, c bool) bool { return !(a && (b || c)) },
		"@a and @b or not @c":     func(a, b, c bool) bool { return (a && b) || !c },
	}
	for expr, want := range exprs {
		filter, err := toGodogTags(expr)
		if err != nil {
			t.Fatalf("toGodogTags(%q): %v", expr, err)
		}
		for mask := 0; mask < 8; mask++ {
			a, b, c := mask&1 != 0, mask&2 != 0, mask&4 != 0
			var tags []*messages.PickleTag
			for name, on := range map[string]bool{"@a": a, "@b": b, "@c": c} {
				if on {
					tags = append(tags, &messages.PickleTag{Name: name})
				}
			}
			got := godogMatch(filter, tags)
			if got != want(a, b, c) {
				t.Errorf("%q (godog %q) with a=%v b=%v c=%v: got %v, want %v", expr, filter, a, b, c, got, want(a, b, c))
			}
		}
	}
}

// godogMatch mirrors godog v0.15.1 internal/tags.match (not importable):
// "&&" separates AND groups, "," separates OR terms, "~" negates.
func godogMatch(filter string, tags []*messages.PickleTag) bool {
	has := func(tag string) bool {
		for _, t := range tags {
			if strings.ReplaceAll(t.Name, "@", "") == tag {
				return true
			}
		}
		return false
	}
	ok := true
	for _, andTags := range strings.Split(filter, "&&") {
		var okComma bool
		for _, tag := range strings.Split(andTags, ",") {
			tag = strings.ReplaceAll(strings.TrimSpace(tag), "@", "")
			okComma = has(tag) || okComma
			if tag[0] == '~' {
				okComma = !has(tag[1:]) || okComma
			}
		}
		ok = ok && okComma
	}
	return ok
}
