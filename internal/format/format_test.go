package format

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/joshjkns/clowd/internal/compile"
	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/golden"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/syntax/parser"
)

func inputs(t *testing.T) []string {
	t.Helper()
	var paths []string
	for _, pattern := range []string{"testdata/*.clowd", "../../examples/*.clowd"} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, m...)
	}
	return paths
}

func read(t *testing.T, path string) *diag.File {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return diag.NewFile(filepath.Base(path), string(src))
}

// TestGolden records the formatted output, or the diagnostics for files
// that can't be formatted.
func TestGolden(t *testing.T) {
	paths, err := filepath.Glob("testdata/*.clowd")
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range paths {
		t.Run(filepath.Base(in), func(t *testing.T) {
			f := read(t, in)
			out, diags := File(f)
			var b strings.Builder
			if out != nil {
				b.Write(out)
			} else {
				b.WriteString("not formatted; diagnostics:\n")
				if err := diag.RenderAll(&b, f, diags); err != nil {
					t.Fatal(err)
				}
			}
			golden.Check(t, strings.TrimSuffix(in, ".clowd")+".golden", b.String())
		})
	}
}

// TestIdempotent is the Phase 3 exit criterion: formatting formatted
// output changes nothing.
func TestIdempotent(t *testing.T) {
	for _, in := range inputs(t) {
		t.Run(filepath.Base(in), func(t *testing.T) {
			once, _ := File(read(t, in))
			if once == nil {
				t.Skip("has syntax errors")
			}
			assertIdempotent(t, once)
		})
	}
}

func assertIdempotent(t *testing.T, once []byte) {
	t.Helper()
	twice, diags := File(diag.NewFile("once.clowd", string(once)))
	if twice == nil {
		t.Fatalf("formatted output has syntax errors:\n%s\n%v", once, diags)
	}
	if string(twice) != string(once) {
		t.Errorf("not idempotent\n--- once\n%s--- twice\n%s", once, twice)
	}
}

// TestExamplesFormatted keeps the examples in canonical style, since they
// are what people copy.
func TestExamplesFormatted(t *testing.T) {
	examples, err := filepath.Glob("../../examples/*.clowd")
	if err != nil || len(examples) == 0 {
		t.Fatalf("no examples found: %v", err)
	}
	for _, in := range examples {
		f := read(t, in)
		if out, _ := File(f); string(out) != f.Text {
			t.Errorf("%s is not formatted; run `clowd fmt %s`\n--- formatted\n%s", in, in, out)
		}
	}
}

var (
	positions = regexp.MustCompile(`\d+:\d+`)
	lineRefs  = regexp.MustCompile(`line \d+`)
)

// TestPreservesMeaning checks that formatting changes neither what the
// file compiles to nor its comments.
func TestPreservesMeaning(t *testing.T) {
	for _, in := range inputs(t) {
		t.Run(filepath.Base(in), func(t *testing.T) {
			f := read(t, in)
			out, _ := File(f)
			if out == nil {
				t.Skip("has syntax errors")
			}
			g := diag.NewFile(f.Name, string(out))
			assertSameMeaning(t, f, g)
		})
	}
}

func assertSameMeaning(t *testing.T, f, g *diag.File) {
	t.Helper()
	before, after := compile.Check(f), compile.Check(g)
	if b, a := dumpIR(f, before.Program), dumpIR(g, after.Program); b != a {
		t.Errorf("IR changed\n--- before\n%s--- after\n%s", b, a)
	}
	if b, a := messages(before.Diags), messages(after.Diags); b != a {
		t.Errorf("diagnostics changed\n--- before\n%s--- after\n%s", b, a)
	}
	fa, _ := parser.Parse(f)
	ga, _ := parser.Parse(g)
	if len(fa.Comments) != len(ga.Comments) {
		t.Fatalf("%d comments became %d", len(fa.Comments), len(ga.Comments))
	}
	for i := range fa.Comments {
		if b, a := strings.TrimRight(fa.Comments[i].Text, " \t\r"), ga.Comments[i].Text; b != a {
			t.Errorf("comment %d changed from %q to %q", i, b, a)
		}
	}
}

func dumpIR(f *diag.File, p *ir.Program) string {
	return positions.ReplaceAllString(ir.Dump(f, p), "")
}

// messages lists diagnostics without positions or line references, sorted, since moving lines
// reorders them. Messages that quote a keyword as written (`API` instead of
// `api`) are expected to change, so they are compared case-insensitively.
func messages(l diag.List) string {
	var lines []string
	for _, d := range l {
		msg := lineRefs.ReplaceAllString(d.Message, "line N")
		lines = append(lines, strings.ToLower(d.Severity.String()+": "+msg+"\n"))
	}
	slices.Sort(lines)
	return strings.Join(lines, "")
}

func TestCases(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"empty", "", ""},
		{"only blank lines", "\n\n  \n", ""},
		{"only comments", "\n# a\n\n\n# b\n", "# a\n\n# b\n"},
		{"crlf", "use aws\r\n\r\nservice api\r\n  image x # c\r\n", "use aws\n\nservice api\n  image x # c\n"},
		{"bom", "\ufeff# header\nuse aws\n", "# header\nuse aws\n"},
		{"no final newline", "use aws", "use aws\n"},
		{"lb alias kept", "LB web\n  listen on 80\n  send / to api\n", "lb web\n  listen on 80\n  send / to api\n"},
		{"load balancer spaces", "load    balancer web\n", "load balancer web\n"},
		{"trait separators", "database db is postgres 16 , small AND large", "database db is postgres 16, small and large\n"},
		{"quoted trait kept", `bucket b is "Private"`, "bucket b is \"Private\"\n"},
		{"names keep case", "service API\nAPI connects to DB\n", "service API\n\nAPI connects to DB\n"},
		{"keyword-like values kept", "service api\n  env TO To\n  scale 3\n", "service api\n  env TO To\n  scale 3\n"},
		{"region keeps case", "use aws in EU-West-2", "use aws in EU-West-2\n"},
		{"override raw case kept", "on aws\n  service api sets Tags to \"A\"\n", "on aws\n  service api sets Tags to \"A\"\n"},
		{"blocks separated", "service a\n  image x\nservice b\n  image y\n", "service a\n  image x\n\nservice b\n  image y\n"},
		{"shapes separated", "use aws\nbucket b\napi reads b\n", "use aws\n\nbucket b\n\napi reads b\n"},
		{"comment leaves block", "service a\n  image x\n# out\n  # still out\n", "service a\n  image x\n# out\n# still out\n"},
		{"groups collapse", "bucket a\n\n\nbucket b\napi reads a\n\napi reads b\n", "bucket a\nbucket b\n\napi reads a\napi reads b\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := diag.NewFile("case.clowd", tt.in)
			got, diags := File(f)
			if got == nil {
				t.Fatalf("not formatted: %v", diags)
			}
			if string(got) != tt.want {
				t.Errorf("got\n%q\nwant\n%q", got, tt.want)
			}
			assertIdempotent(t, got)
			assertSameMeaning(t, f, diag.NewFile("case.clowd", string(got)))
		})
	}
}

func TestSyntaxErrorsNotFormatted(t *testing.T) {
	for _, src := range []string{"servce api\n", "use aws\n\tservice api\n", "api links to db\n", `service "api"`} {
		out, diags := File(diag.NewFile("bad.clowd", src))
		if out != nil || !diags.HasErrors() {
			t.Errorf("%q: got output %q and diagnostics %v, want no output and errors", src, out, diags)
		}
	}
}

// FuzzFormat checks that any file that parses formats to a file that
// parses, means the same and is already formatted.
func FuzzFormat(f *testing.F) {
	seeds, _ := filepath.Glob("testdata/*.clowd")
	for _, s := range seeds {
		if b, err := os.ReadFile(s); err == nil {
			f.Add(string(b))
		}
	}
	f.Add("\n0 reAds 00") // diagnostics reorder when lines move
	f.Fuzz(func(t *testing.T, src string) {
		in := diag.NewFile("fuzz.clowd", src)
		out, _ := File(in)
		if out == nil {
			return
		}
		assertIdempotent(t, out)
		assertSameMeaning(t, in, diag.NewFile("fuzz.clowd", string(out)))
	})
}
