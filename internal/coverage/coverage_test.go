package coverage

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tomatool/tomato/internal/config"
)

func TestFilter(t *testing.T) {
	f := newFilter([]string{"com.example.*", "org.acme.Main"}, []string{"*.generated.*", "*$Proxy?"})
	tests := map[string]bool{
		"com.example.Orders":                  true,
		"com.example.orders.api.OrderService": true,
		"org.acme.Main":                       true,
		"org.acme.MainKt":                     false, // no wildcard: the exact name only
		"com.example.db.generated.Tables":     false,
		"com.example.Orders$Proxy1":           false,
		"com.example.Orders$Proxy12":          true, // ? is exactly one character
		"org.springframework.boot.Loader":     false,
		"com.examples.Other":                  false, // the dot is a dot, not any character
	}
	for class, want := range tests {
		if got := f.match(class); got != want {
			t.Errorf("match(%q) = %v, want %v", class, got, want)
		}
	}
}

func TestClassName(t *testing.T) {
	tests := []struct {
		entry string
		want  string
		ok    bool
	}{
		{"com/example/Orders.class", "com.example.Orders", true},
		{"BOOT-INF/classes/com/example/App.class", "com.example.App", true},
		{"WEB-INF/classes/com/example/App.class", "com.example.App", true},
		{"com/example/Orders$Line.class", "com.example.Orders$Line", true},
		{"META-INF/versions/21/com/example/Orders.class", "", false},
		{"module-info.class", "", false},
		{"com/example/application.yaml", "", false},
	}
	for _, tt := range tests {
		got, ok := className(tt.entry)
		if got != tt.want || ok != tt.ok {
			t.Errorf("className(%q) = %q, %v; want %q, %v", tt.entry, got, ok, tt.want, tt.ok)
		}
	}
}

// zipOf is a jar with the given entries.
func zipOf(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(entries[name])
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func classesIn(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out
}

func TestExtractClasses_FromASpringBootJar(t *testing.T) {
	// A Spring Boot jar: the app's own classes, a module of it as a nested
	// jar, and libraries that must not count.
	module := zipOf(t, map[string][]byte{
		"com/example/orders/OrderService.class":    []byte("orders"),
		"com/example/orders/generated/Tag.class":   []byte("generated"),
		"META-INF/versions/21/com/example/X.class": []byte("versioned"),
	})
	library := zipOf(t, map[string][]byte{"org/springframework/Loader.class": []byte("spring")})
	boot := zipOf(t, map[string][]byte{
		"BOOT-INF/classes/com/example/App.class":         []byte("app"),
		"BOOT-INF/lib/orders-1.0.jar":                    module,
		"BOOT-INF/lib/spring-core.jar":                   library,
		"org/springframework/boot/loader/Launcher.class": []byte("launcher"),
	})
	jar := filepath.Join(t.TempDir(), "app.jar")
	if err := os.WriteFile(jar, boot, 0o644); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()

	n, err := extractClasses([]string{jar}, out, newFilter([]string{"com.example.*"}, []string{"*.generated.*"}))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"com/example/App.class", "com/example/orders/OrderService.class"}
	if got := classesIn(t, out); strings.Join(got, ",") != strings.Join(want, ",") || n != len(want) {
		t.Errorf("extracted %d: %v, want %v", n, got, want)
	}
	if data, _ := os.ReadFile(filepath.Join(out, "com/example/orders/OrderService.class")); string(data) != "orders" {
		t.Errorf("the nested jar's class holds %q", data)
	}
}

func TestExtractClasses_FromDirectoriesFirstWins(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	for dir, content := range map[string]string{first: "first", second: "second"} {
		os.MkdirAll(filepath.Join(dir, "com/example"), 0o755)
		os.WriteFile(filepath.Join(dir, "com/example/App.class"), []byte(content), 0o644)
	}
	out := t.TempDir()
	n, err := extractClasses([]string{first, second}, out, newFilter([]string{"com.example.*"}, nil))
	if err != nil || n != 1 {
		t.Fatalf("extracted %d, %v; want 1 class", n, err)
	}
	if data, _ := os.ReadFile(filepath.Join(out, "com/example/App.class")); string(data) != "first" {
		t.Errorf("App.class holds %q, want the first directory's", data)
	}
}

func TestExtractClasses_MissingPath(t *testing.T) {
	_, err := extractClasses([]string{filepath.Join(t.TempDir(), "nope.jar")}, t.TempDir(), newFilter([]string{"*"}, nil))
	if err == nil || !strings.Contains(err.Error(), "app.coverage.classes") {
		t.Errorf("err = %v, want it to name app.coverage.classes", err)
	}
}

func TestParseCSV(t *testing.T) {
	csv := `GROUP,PACKAGE,CLASS,INSTRUCTION_MISSED,INSTRUCTION_COVERED,BRANCH_MISSED,BRANCH_COVERED,LINE_MISSED,LINE_COVERED,COMPLEXITY_MISSED,COMPLEXITY_COVERED,METHOD_MISSED,METHOD_COVERED
app,com.example,App,10,20,1,3,2,8,1,2,0,2
app,com.example.orders,OrderService,40,0,4,0,10,0,3,0,2,0
`
	sum, err := parseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if sum.Lines != (Counter{Covered: 8, Total: 20}) || sum.Branches != (Counter{Covered: 3, Total: 8}) || sum.Classes != 2 {
		t.Errorf("summary = %+v", sum)
	}
	if got := sum.Lines.String(); got != "40.0% (8/20)" {
		t.Errorf("Lines.String() = %q", got)
	}
	if (Counter{}).Percent() != 0 {
		t.Error("nothing to cover is 0%, not NaN")
	}
}

func TestParseCSV_NotJaCoCos(t *testing.T) {
	if _, err := parseCSV(strings.NewReader("a,b\n1,2\n")); err == nil {
		t.Error("a CSV without JaCoCo's columns was accepted")
	}
	if _, err := parseCSV(strings.NewReader("")); err == nil {
		t.Error("an empty CSV was accepted")
	}
}

func TestJavaAgent(t *testing.T) {
	dir := t.TempDir()
	c, err := New(&config.AppConfig{Command: "java -jar app.jar", Coverage: &config.AppCoverage{
		Type: "jacoco", Includes: []string{"com.example.*", "org.acme.*"}, Excludes: []string{"*.generated.*"},
	}}, dir)
	if err != nil {
		t.Fatal(err)
	}
	opt, err := c.JavaAgent()
	if err != nil {
		t.Fatal(err)
	}
	agent := filepath.Join(c.dir, "jacocoagent.jar")
	want := "-javaagent:" + agent + "=destfile=" + filepath.Join(c.dir, "jacoco.exec") +
		",includes=com.example.*:org.acme.*,excludes=*.generated.*"
	if opt != want {
		t.Errorf("JavaAgent() = %q\nwant         %q", opt, want)
	}
	if data, _ := os.ReadFile(agent); !bytes.Equal(data, agentJar) {
		t.Error("the agent written is not the embedded one")
	}
	if _, err := zip.NewReader(bytes.NewReader(agentJar), int64(len(agentJar))); err != nil {
		t.Errorf("the embedded agent is not a jar: %v", err)
	}
}

func TestNew_WithoutCoverage(t *testing.T) {
	c, err := New(&config.AppConfig{Command: "java -jar app.jar"}, t.TempDir())
	if c != nil || err != nil {
		t.Errorf("New() = %v, %v; want nothing for an app without coverage", c, err)
	}
}

func TestReport_NoData(t *testing.T) {
	c, _ := New(&config.AppConfig{Command: "java -jar app.jar", Coverage: &config.AppCoverage{
		Type: "jacoco", Includes: []string{"*"},
	}}, t.TempDir())
	if _, err := c.Report(context.Background(), "java"); !errors.Is(err, ErrNoData) {
		t.Errorf("err = %v, want ErrNoData", err)
	}
}

// TestReport_EndToEnd runs a class with the embedded agent the way tomato
// runs an app, and reports it: needs a JDK, so it is skipped without one.
func TestReport_EndToEnd(t *testing.T) {
	javac, err := exec.LookPath("javac")
	if err != nil {
		t.Skip("needs a JDK: no javac on the PATH")
	}
	java, err := exec.LookPath("java")
	if err != nil {
		t.Skip("needs a JDK: no java on the PATH")
	}
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "com/example"), 0o755)
	os.WriteFile(filepath.Join(src, "com/example/Main.java"), []byte(`package com.example;
public class Main {
  public static void main(String[] args) { System.out.println(sign(args.length)); }
  static String sign(int n) { if (n > 0) { return "positive"; } return "zero"; }
  static String unused() { return "never runs"; }
}
`), 0o644)
	classes := t.TempDir()
	if out, err := exec.Command(javac, "-d", classes, filepath.Join(src, "com/example/Main.java")).CombinedOutput(); err != nil {
		t.Fatalf("javac: %v\n%s", err, out)
	}

	c, err := New(&config.AppConfig{Command: "java com.example.Main", Coverage: &config.AppCoverage{
		Type: "jacoco", Includes: []string{"com.example.*"}, Classes: []string{classes},
	}}, filepath.Join(t.TempDir(), "coverage"))
	if err != nil {
		t.Fatal(err)
	}
	agent, err := c.JavaAgent()
	if err != nil {
		t.Fatal(err)
	}
	run := exec.Command(java, "-cp", classes, "com.example.Main")
	run.Env = append(os.Environ(), "JAVA_TOOL_OPTIONS="+agent)
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("running with the agent: %v\n%s", err, out)
	}

	// The report runs with the app's JAVA_TOOL_OPTIONS still set, as under
	// tomato: it must not attach the agent to itself.
	t.Setenv("JAVA_TOOL_OPTIONS", agent)
	sum, err := c.Report(context.Background(), java)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Classes != 1 {
		t.Errorf("classes = %d, want 1", sum.Classes)
	}
	// main ran and sign took one of its two branches; unused never ran.
	if sum.Branches != (Counter{Covered: 1, Total: 2}) {
		t.Errorf("branches = %+v, want 1 of 2", sum.Branches)
	}
	if sum.Lines.Covered == 0 || sum.Lines.Covered >= sum.Lines.Total {
		t.Errorf("lines = %+v, want some but not all", sum.Lines)
	}
	if _, err := os.Stat(sum.Report); err != nil {
		t.Errorf("no HTML report: %v", err)
	}
}
