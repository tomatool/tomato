package feature

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/cucumber/gherkin-go"
	"github.com/tomatool/tomato/dictionary"
)

func TestParse(t *testing.T) {
	f, err := os.Open("./fixtures/http.feature")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	feature, err := gherkin.ParseGherkinDocument(f)
	if err != nil {
		t.Fatal(err)
	}

	dict, err := dictionary.Retrieve("../dictionary.yml")
	if err != nil {
		t.Fatal(err)
	}

	p := Parser{
		Dictionary: dict,
	}

	r, err := p.Parse(feature)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	t.Logf("%s", b)
	t.Errorf("")
}
