package feature

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "login.feature", `Feature: Login

  Scenario: Test user login
    Given a user

  Scenario: User can login successfully
    Given a user

  Scenario: Rejecting bad orders (400)
    Given a user
`)
	write(t, dir, "accounts.feature", `Feature: Accounts

  Scenario: User creates account
    Given a user

  Scenario Outline: User sees <role> page
    Given a <role>

    Examples:
      | role  |
      | admin |
      | guest |
`)
	return dir
}

func TestSelect_Patterns(t *testing.T) {
	dir := fixture(t)
	tests := []struct {
		name    string
		pattern string
		want    []string
	}{
		{"prefix match", "^Test.*", []string{"Test user login"}},
		{"contains match", ".*login.*", []string{"Test user login", "User can login successfully"}},
		{"exact match", "^User creates account$", []string{"User creates account"}},
		{"case sensitive", "^test.*", nil},
		{"outline matches by its own name", "^User sees", []string{"User sees <role> page"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Select([]string{dir}, regexp.MustCompile(tt.pattern))
			if tt.want == nil {
				if err == nil {
					t.Fatalf("expected no-match error, got %d matches", len(got))
				}
				if !strings.Contains(err.Error(), "no scenario matches") {
					t.Errorf("error should say nothing matched, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Select: %v", err)
			}
			var names []string
			for _, m := range got {
				names = append(names, m.Name)
			}
			if strings.Join(names, "|") != strings.Join(tt.want, "|") {
				t.Errorf("matched %v, want %v", names, tt.want)
			}
		})
	}
}

// A name with regex metacharacters must be reachable. The UI quotes the name
// it sends for exactly this reason: unquoted, `Rejecting bad orders (400)` is
// a valid regex that does not match itself, so Run on that card ran nothing.
func TestSelect_QuotedNameWithMetacharacters(t *testing.T) {
	dir := fixture(t)
	name := "Rejecting bad orders (400)"

	if _, err := Select([]string{dir}, regexp.MustCompile("^"+name+"$")); err == nil {
		t.Error("unquoted, the name should not match itself — the premise of quoting")
	}

	got, err := Select([]string{dir}, regexp.MustCompile("^"+regexp.QuoteMeta(name)+"$"))
	if err != nil {
		t.Fatalf("quoted name should match: %v", err)
	}
	if len(got) != 1 || got[0].Name != name {
		t.Errorf("got %v, want exactly %q", got, name)
	}
}

// godog resolves "file.feature:N" against the scenario declared on line N.
func TestSelect_PathCarriesTheScenarioLine(t *testing.T) {
	dir := fixture(t)
	got, err := Select([]string{dir}, regexp.MustCompile("^User creates account$"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d matches, want 1", len(got))
	}
	if !strings.HasSuffix(got[0].Path, "accounts.feature:3") {
		t.Errorf("path %q should point at the scenario's own line", got[0].Path)
	}
}

func TestFiles_MissingRootIsNotAnError(t *testing.T) {
	files, err := Files(filepath.Join(t.TempDir(), "nope"))
	if err != nil || files != nil {
		t.Errorf("missing root: got (%v, %v), want (nil, nil)", files, err)
	}
}
