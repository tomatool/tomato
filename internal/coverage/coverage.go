// Package coverage measures which of the app's code a suite ran, with the
// JaCoCo agent for an app on the JVM: tomato attaches the agent when it starts
// the app, the agent writes what ran when the app exits on tomato's SIGTERM,
// and tomato reports it against the app's own classes.
package coverage

import (
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/tomatool/tomato/internal/config"
)

// The JaCoCo 0.8.16 agent and command line, from Maven Central
// (org.jacoco.agent runtime, org.jacoco.cli nodeps). 0.8.16 is the first
// release that reads Java 25 class files. Embedded, so a run downloads nothing.
var (
	//go:embed jacoco/jacoco-agent.jar
	agentJar []byte
	//go:embed jacoco/jacoco-cli.jar
	cliJar []byte
)

// Counter is how much of one kind of thing ran.
type Counter struct {
	Covered int `json:"covered"`
	Total   int `json:"total"`
}

// Percent is the covered share, 0 when there is nothing to cover.
func (c Counter) Percent() float64 {
	if c.Total == 0 {
		return 0
	}
	return 100 * float64(c.Covered) / float64(c.Total)
}

func (c Counter) String() string {
	return fmt.Sprintf("%.1f%% (%d/%d)", c.Percent(), c.Covered, c.Total)
}

// Summary is the coverage of a run.
type Summary struct {
	Lines    Counter `json:"lines"`
	Branches Counter `json:"branches"`
	// Classes is how many of the app's classes the report counts.
	Classes int `json:"classes"`
	// Report is the HTML report's index page.
	Report string `json:"report,omitempty"`
}

// Coverage is a run's coverage directory, under its run directory.
type Coverage struct {
	cfg *config.AppCoverage
	// classes are the jars and directories to read the app's classes from.
	classes []string
	dir     string
}

// New prepares coverage for the app in dir, which it creates.
func New(app *config.AppConfig, dir string) (*Coverage, error) {
	if app.Coverage == nil {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &Coverage{cfg: app.Coverage, classes: app.CoverageClasses(), dir: abs}, nil
}

func (c *Coverage) execFile() string { return filepath.Join(c.dir, "jacoco.exec") }

// JavaAgent writes the agent next to the coverage data and returns the
// option that attaches it. The agent appends to its file, so an app tomato
// stops and starts again mid-run adds to the same data.
func (c *Coverage) JavaAgent() (string, error) {
	agent := filepath.Join(c.dir, "jacocoagent.jar")
	if err := os.WriteFile(agent, agentJar, 0o644); err != nil {
		return "", err
	}
	opts := "destfile=" + c.execFile() + ",includes=" + strings.Join(c.cfg.Includes, ":")
	if len(c.cfg.Excludes) > 0 {
		opts += ",excludes=" + strings.Join(c.cfg.Excludes, ":")
	}
	return "-javaagent:" + agent + "=" + opts, nil
}

// ErrNoData is a run whose app wrote no coverage data: it never started,
// or did not exit on SIGTERM before tomato killed it.
var ErrNoData = errors.New("the app wrote no coverage data: the JaCoCo agent writes it when the app exits, and the app did not exit on SIGTERM before tomato killed it 5s later")

// Report reports the coverage data the app wrote against its classes, as
// HTML, XML and CSV in the coverage directory. java is the JVM to run
// JaCoCo's command line with.
func (c *Coverage) Report(ctx context.Context, java string) (*Summary, error) {
	if _, err := os.Stat(c.execFile()); err != nil {
		return nil, ErrNoData
	}
	classesDir := filepath.Join(c.dir, "classes")
	if err := os.RemoveAll(classesDir); err != nil {
		return nil, err
	}
	filter := newFilter(c.cfg.Includes, c.cfg.Excludes)
	n, err := extractClasses(c.classes, classesDir, filter)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, fmt.Errorf("no class in %s matches app.coverage.includes %s", strings.Join(c.classes, ", "), strings.Join(c.cfg.Includes, ", "))
	}

	cli := filepath.Join(c.dir, "jacococli.jar")
	if err := os.WriteFile(cli, cliJar, 0o644); err != nil {
		return nil, err
	}
	args := []string{"-jar", cli, "report", c.execFile(), "--classfiles", classesDir,
		"--html", filepath.Join(c.dir, "html"),
		"--xml", filepath.Join(c.dir, "jacoco.xml"),
		"--csv", filepath.Join(c.dir, "jacoco.csv"),
		"--quiet"}
	for _, src := range c.cfg.Sources {
		args = append(args, "--sourcefiles", src)
	}
	cmd := exec.CommandContext(ctx, java, args...)
	// JAVA_TOOL_OPTIONS is the app's, and would attach the agent to the report too.
	cmd.Env = withoutEnv(os.Environ(), "JAVA_TOOL_OPTIONS")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("JaCoCo could not report: %w\n%s", err, out)
	}

	f, err := os.Open(filepath.Join(c.dir, "jacoco.csv"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sum, err := parseCSV(f)
	if err != nil {
		return nil, err
	}
	sum.Report = filepath.Join(c.dir, "html", "index.html")
	return sum, nil
}

func withoutEnv(env []string, key string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if k, _, _ := strings.Cut(kv, "="); k != key {
			out = append(out, kv)
		}
	}
	return out
}

// parseCSV totals JaCoCo's CSV report, one row per class.
func parseCSV(r io.Reader) (*Summary, error) {
	rows, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading JaCoCo's CSV report: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("JaCoCo's CSV report is empty")
	}
	col := map[string]int{}
	for i, name := range rows[0] {
		col[name] = i
	}
	for _, name := range []string{"LINE_MISSED", "LINE_COVERED", "BRANCH_MISSED", "BRANCH_COVERED"} {
		if _, ok := col[name]; !ok {
			return nil, fmt.Errorf("JaCoCo's CSV report has no %s column", name)
		}
	}
	sum := &Summary{}
	for _, row := range rows[1:] {
		num := func(name string) (int, error) { return strconv.Atoi(row[col[name]]) }
		lm, err1 := num("LINE_MISSED")
		lc, err2 := num("LINE_COVERED")
		bm, err3 := num("BRANCH_MISSED")
		bc, err4 := num("BRANCH_COVERED")
		if err := errors.Join(err1, err2, err3, err4); err != nil {
			return nil, fmt.Errorf("JaCoCo's CSV report: %w", err)
		}
		sum.Lines.Covered += lc
		sum.Lines.Total += lc + lm
		sum.Branches.Covered += bc
		sum.Branches.Total += bc + bm
		sum.Classes++
	}
	return sum, nil
}

// filter matches class names against JaCoCo's patterns, where * is any
// run of characters and ? any one.
type filter struct{ includes, excludes []*regexp.Regexp }

func newFilter(includes, excludes []string) filter {
	compile := func(patterns []string) []*regexp.Regexp {
		var out []*regexp.Regexp
		for _, p := range patterns {
			q := regexp.QuoteMeta(p)
			q = strings.ReplaceAll(q, `\*`, ".*")
			q = strings.ReplaceAll(q, `\?`, ".")
			out = append(out, regexp.MustCompile("^"+q+"$"))
		}
		return out
	}
	return filter{includes: compile(includes), excludes: compile(excludes)}
}

func (f filter) match(class string) bool {
	matches := func(res []*regexp.Regexp) bool {
		for _, re := range res {
			if re.MatchString(class) {
				return true
			}
		}
		return false
	}
	return matches(f.includes) && !matches(f.excludes)
}

// classRoots are where packaging puts a class's package path inside a jar.
var classRoots = []string{"BOOT-INF/classes/", "WEB-INF/classes/"}

// className is the class an archive or directory entry holds, and false for
// an entry that is no class of the app: not a class file, module-info, or a
// release-specific copy under META-INF/versions that JaCoCo would count twice.
func className(entry string) (string, bool) {
	entry = filepath.ToSlash(entry)
	if !strings.HasSuffix(entry, ".class") || strings.HasPrefix(entry, "META-INF/") || strings.HasSuffix(entry, "module-info.class") {
		return "", false
	}
	for _, root := range classRoots {
		entry = strings.TrimPrefix(entry, root)
	}
	return strings.ReplaceAll(strings.TrimSuffix(entry, ".class"), "/", "."), true
}

// extractClasses copies the classes the filter matches, from jars, the jars
// nested in them (a Spring Boot jar's BOOT-INF/lib) and directories, into
// dir by package path. A class found twice is copied once, the first time.
func extractClasses(paths []string, dir string, f filter) (int, error) {
	seen := map[string]bool{}
	write := func(class string, open func() ([]byte, error)) error {
		if seen[class] || !f.match(class) {
			return nil
		}
		data, err := open()
		if err != nil {
			return err
		}
		seen[class] = true
		target := filepath.Join(dir, filepath.FromSlash(strings.ReplaceAll(class, ".", "/"))+".class")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	}

	var fromZip func(r *zip.Reader) error
	fromZip = func(r *zip.Reader) error {
		for _, entry := range r.File {
			read := func() ([]byte, error) {
				rc, err := entry.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
			if strings.HasSuffix(entry.Name, ".jar") {
				data, err := read()
				if err != nil {
					return err
				}
				nested, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
				if err != nil {
					return fmt.Errorf("%s: %w", entry.Name, err)
				}
				if err := fromZip(nested); err != nil {
					return err
				}
				continue
			}
			if class, ok := className(entry.Name); ok {
				if err := write(class, read); err != nil {
					return err
				}
			}
		}
		return nil
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return 0, fmt.Errorf("app.coverage.classes: %w", err)
		}
		if info.IsDir() {
			err = filepath.WalkDir(path, func(file string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(path, file)
				if class, ok := className(rel); ok {
					return write(class, func() ([]byte, error) { return os.ReadFile(file) })
				}
				return nil
			})
		} else {
			var r *zip.ReadCloser
			if r, err = zip.OpenReader(path); err == nil {
				err = fromZip(&r.Reader)
				r.Close()
			}
		}
		if err != nil {
			return 0, fmt.Errorf("reading the classes in %s: %w", path, err)
		}
	}
	return len(seen), nil
}
