package compare

import (
	"encoding/json"

	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"
)

// JSON compares two json strings, processes them to handle wild cards,
func JSON(a []byte, b []byte) (string, error) {
	differ := gojsondiff.New()
	d, err := differ.Compare(a, b)
	if err != nil {
		return "", err
	}

	// filter the fields we do not want
	filteredDiffer := jsondiff{
		deltas: filter(d.Deltas(), filterWildcards),
	}

	if filteredDiffer.Modified() {
		var aJSON map[string]interface{}
		json.Unmarshal(a, &aJSON)

		formatter := formatter.NewAsciiFormatter(aJSON, formatter.AsciiFormatterConfig{
			ShowArrayIndex: false,
			Coloring:       false,
		})
		return formatter.Format(filteredDiffer)
	}
	return "", nil
}

// Implement to gojsondiff Differ interface
type jsondiff struct {
	deltas []gojsondiff.Delta
}

func (j jsondiff) Deltas() []gojsondiff.Delta {
	return j.deltas
}

func (j jsondiff) Modified() bool {
	return len(j.deltas) > 0
}

// filter each delta using the passed filterfunc
func filter(deltas []gojsondiff.Delta, f func(gojsondiff.Delta) bool) []gojsondiff.Delta {
	filtered := make([]gojsondiff.Delta, 0)
	for _, delta := range deltas {
		switch d := delta.(type) {
		case *gojsondiff.Object:
			d.Deltas = filter(d.Deltas, f)
			filtered = append(filtered, d)
		case *gojsondiff.Array:
			d.Deltas = filter(d.Deltas, f)
			filtered = append(filtered, d)
		default:
			if f(d) {
				filtered = append(filtered, d)
			}
		}
	}
	return filtered
}

const (
	wildcard = "*"
)

// filterWildcards removes and "modified" fields where the new value is a wildcard
func filterWildcards(delta gojsondiff.Delta) bool {
	switch delta.(type) {
	case *gojsondiff.Modified:
		d := delta.(*gojsondiff.Modified)
		if v, ok := d.NewValue.(string); ok && v == wildcard {
			return false
		}
	}
	return true
}
