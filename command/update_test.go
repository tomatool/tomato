package command

import "testing"

func TestHomebrewUpgrade(t *testing.T) {
	cask := "brew upgrade --cask tomatool/tap/tomato"
	formula := "brew uninstall tomato && brew install --cask tomatool/tap/tomato"
	cases := map[string]string{
		"/opt/homebrew/Caskroom/tomato/2.2.0/tomato":              cask,
		"/usr/local/Caskroom/tomato/2.2.0/tomato":                 cask,
		"/home/linuxbrew/.linuxbrew/Caskroom/tomato/2.2.0/tomato": cask,
		"/opt/homebrew/Cellar/tomato/2.0.3/bin/tomato":            formula,
		"/usr/local/bin/tomato":                                   "",
		"/Users/someone/go/bin/tomato":                            "",
		"/Users/someone/Cellar-notes/tomato":                      "",
	}
	for path, want := range cases {
		if got := homebrewUpgrade(path); got != want {
			t.Errorf("homebrewUpgrade(%q) = %q, want %q", path, got, want)
		}
	}
}
