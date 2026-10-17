package resource

import "sync"

// Screenshot is an image a step left behind: what the browser resource
// writes for `saves screenshot "name"`, and the page when a step failed.
type Screenshot struct {
	// Resource is the resource that took it, the person whose page it shows.
	Resource string `json:"resource"`
	// Path is the file, as the resource wrote it: relative to the working
	// directory unless the screenshots option is absolute.
	Path string `json:"path"`
}

var (
	screenshotsMu sync.Mutex
	// screenshots holds, per scenario, what its running step has left so far.
	// The formatter takes them when the step ends, so each one is reported on
	// the step that produced it rather than guessed from a file name later.
	screenshots = map[string][]Screenshot{}
)

// RecordScreenshot notes a screenshot taken during the running step of the
// scenario, for the `tomato` formatter to report on that step's end. A
// failure's screenshot is taken in an after-step hook, which godog runs
// before it reports the step, so it lands on the step that failed.
func RecordScreenshot(scenario string, s Screenshot) {
	screenshotsMu.Lock()
	defer screenshotsMu.Unlock()
	screenshots[scenario] = append(screenshots[scenario], s)
}

// TakeScreenshots returns the screenshots recorded for the scenario since
// the last call, and forgets them. nil when there are none.
func TakeScreenshots(scenario string) []Screenshot {
	screenshotsMu.Lock()
	defer screenshotsMu.Unlock()
	out := screenshots[scenario]
	delete(screenshots, scenario)
	return out
}
