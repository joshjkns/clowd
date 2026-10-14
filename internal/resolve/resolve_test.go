package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/golden"
	"github.com/joshjkns/clowd/internal/syntax/parser"
)

// run parses and resolves src, returning the table and the resolver's
// diagnostics only, so goldens show what this stage adds.
func run(name, src string) (*diag.File, *Table, diag.List) {
	f := diag.NewFile(name, src)
	file, _ := parser.Parse(f)
	t, diags := Resolve(f, file)
	return f, t, diags
}

func dump(f *diag.File, t *Table, diags diag.List) string {
	var b strings.Builder
	pos := func(sp diag.Span) string {
		p := f.Position(sp.Start)
		return fmt.Sprintf("%d:%d", p.Line, p.Column)
	}
	b.WriteString("decls:\n")
	for _, d := range t.Decls {
		fmt.Fprintf(&b, "  %s %s %s\n", pos(d.Name.Pos), d.Kind, d.Name.Name)
	}
	b.WriteString("refs:\n")
	for _, r := range t.Refs {
		to := "<unresolved>"
		if r.Decl != nil {
			to = fmt.Sprintf("%s %s at %s", r.Decl.Kind, r.Decl.Name.Name, pos(r.Decl.Name.Pos))
		}
		fmt.Fprintf(&b, "  %s %s -> %s\n", pos(r.Pos), r.Name, to)
	}
	if len(diags) > 0 {
		b.WriteString("\ndiagnostics:\n")
		_ = diag.RenderAll(&b, f, diags) // strings.Builder never fails
	}
	return b.String()
}

func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.clowd")
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range inputs {
		t.Run(filepath.Base(in), func(t *testing.T) {
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			f, tab, diags := run(filepath.Base(in), string(src))
			golden.Check(t, strings.TrimSuffix(in, ".clowd")+".golden", dump(f, tab, diags))
		})
	}
}

func TestExamples(t *testing.T) {
	examples, err := filepath.Glob("../../examples/*.clowd")
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) == 0 {
		t.Fatal("no examples found")
	}
	for _, in := range examples {
		name := strings.TrimSuffix(filepath.Base(in), ".clowd")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			f, tab, diags := run(filepath.Base(in), string(src))
			if len(diags) > 0 {
				var b strings.Builder
				_ = diag.RenderAll(&b, f, diags)
				t.Errorf("examples must resolve without diagnostics:\n%s", b.String())
			}
			golden.Check(t, filepath.Join("testdata", "examples", name+".golden"), dump(f, tab, diags))
		})
	}
}

func TestLookup(t *testing.T) {
	_, tab, _ := run("t.clowd", "service api\ndatabase DB\n")
	tests := []struct {
		name string
		want string
	}{
		{"api", "api"},
		{"API", "api"},
		{"db", "DB"},
		{"nope", ""},
	}
	for _, tt := range tests {
		got := ""
		if d := tab.Lookup(tt.name); d != nil {
			got = d.Name.Name
		}
		if got != tt.want {
			t.Errorf("Lookup(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestValidName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"api", true},
		{"API", true},
		{"api-2_b", true},
		{"a", true},
		{"2api", false},
		{"-api", false},
		{"my.api", false},
		{"", false},
		{"café", false},
	}
	for _, tt := range tests {
		if got := validName(tt.name); got != tt.want {
			t.Errorf("validName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
