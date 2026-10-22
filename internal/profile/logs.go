package profile

import (
	"regexp"
	"strings"
)

// logClassifier tells an app's error and warning lines from the rest, by the
// level the common log formats write: a bare ERROR or WARN, logfmt's
// level=error, JSON's "level":"error", and the short forms some loggers use.
type logClassifier struct {
	errorRe, warnRe *regexp.Regexp
}

func defaultLogClassifier() logClassifier {
	level := func(words string) *regexp.Regexp {
		return regexp.MustCompile(`(?i)` +
			`(?:^|[\s\[|:])(?:` + words + `)(?:[\s\]|:]|$)` + // ERROR  [error]  E|  error:
			`|level\s*[=:]\s*"?(?:` + words + `)\b` + // level=error  level: "error"
			`|"(?:level|severity|lvl)"\s*:\s*"(?:` + words + `)"`) // {"level":"error"}
	}
	return logClassifier{
		errorRe: level(`error|err|fatal|panic|critical|crit|severe`),
		warnRe:  level(`warn|warning`),
	}
}

func (c logClassifier) classify(line string) (isError, isWarning bool) {
	if c.errorRe.MatchString(line) {
		return true, false
	}
	return false, c.warnRe.MatchString(line)
}

// maxLogExamples is how many lines a scenario's summary quotes.
const maxLogExamples = 3

// ObserveLog counts an app log line toward the running scenario, if it is
// an error or a warning. Lines between scenarios count toward none.
func (p *Profiler) ObserveLog(line string) {
	isErr, isWarn := p.logLevel.classify(line)
	if !isErr && !isWarn {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	w := p.window
	if w == nil {
		return
	}
	if isErr {
		w.logs.Errors++
	} else {
		w.logs.Warnings++
	}
	if len(w.logs.Examples) < maxLogExamples {
		line = strings.TrimSpace(line)
		if len(line) > 300 {
			line = line[:300] + "…"
		}
		w.logs.Examples = append(w.logs.Examples, line)
	}
}
