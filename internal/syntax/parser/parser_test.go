package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/golden"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

func render(f *diag.File, file *ast.File, diags diag.List) string {
	var b strings.Builder
	b.WriteString(ast.Dump(f, file))
	if len(diags) > 0 {
		b.WriteString("\ndiagnostics:\n")
		if err := diag.RenderAll(&b, f, diags); err != nil {
			panic(err) // strings.Builder never fails
		}
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
			f := diag.NewFile(filepath.Base(in), string(src))
			file, diags := Parse(f)
			checkInvariants(t, f, file, diags)
			if strings.HasPrefix(filepath.Base(in), "all_forms") && diags.HasErrors() {
				t.Errorf("all_forms must parse without errors")
			}
			golden.Check(t, strings.TrimSuffix(in, ".clowd")+".golden", render(f, file, diags))
		})
	}
}

// TestExamples is the Phase 1 exit criterion: every example parses cleanly.
func TestExamples(t *testing.T) {
	examples, err := filepath.Glob("../../../examples/*.clowd")
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
			f := diag.NewFile(filepath.Base(in), string(src))
			file, diags := Parse(f)
			if len(diags) > 0 {
				var b strings.Builder
				_ = diag.RenderAll(&b, f, diags)
				t.Errorf("examples must parse without diagnostics:\n%s", b.String())
			}
			checkInvariants(t, f, file, diags)
			golden.Check(t, filepath.Join("testdata", "examples", name+".golden"), render(f, file, diags))
		})
	}
}

// checkInvariants verifies properties that must hold for any input: spans are
// well-formed and inside the file, and statements appear in source order.
func checkInvariants(t *testing.T, f *diag.File, file *ast.File, diags diag.List) {
	t.Helper()
	n := len(f.Text)
	bad := func(sp diag.Span) bool { return sp.Start < 0 || sp.End < sp.Start || sp.End > n }
	for _, d := range diags {
		if bad(d.Span) {
			t.Errorf("diagnostic %q has bad span %+v (file length %d)", d.Message, d.Span, n)
		}
		if d.Message == "" {
			t.Errorf("diagnostic with empty message at %+v", d.Span)
		}
	}
	prev := -1
	for _, s := range file.Stmts {
		sp := s.Span()
		if bad(sp) {
			t.Errorf("%T has bad span %+v (file length %d)", s, sp, n)
		}
		if sp.Start < prev {
			t.Errorf("%T at %d starts before the previous statement at %d", s, sp.Start, prev)
		}
		prev = sp.Start
	}
}

func TestStatementKinds(t *testing.T) {
	tests := []struct {
		src  string
		want string // %T of the single statement
	}{
		{"use aws\n", "*ast.Use"},
		{"service api\n", "*ast.Declare"},
		{"lb web\n", "*ast.Declare"},
		{"load balancer web\n", "*ast.Declare"},
		{"api connects to db\n", "*ast.Relate"},
		{"use connects to db\n", "*ast.Relate"},    // name shadows `use`
		{"service reads uploads\n", "*ast.Relate"}, // name shadows a kind
		{"on aws\n  network\n    vpc v\n", "*ast.On"},
		{"nonsense\n", "*ast.Bad"},
	}
	for _, tt := range tests {
		file, _ := Parse(diag.NewFile("t.clowd", tt.src))
		if len(file.Stmts) != 1 {
			t.Errorf("%q: got %d statements, want 1", tt.src, len(file.Stmts))
			continue
		}
		if got := typeName(file.Stmts[0]); got != tt.want {
			t.Errorf("%q: got %s, want %s", tt.src, got, tt.want)
		}
	}
}

func typeName(s ast.Stmt) string {
	switch s.(type) {
	case *ast.Use:
		return "*ast.Use"
	case *ast.Declare:
		return "*ast.Declare"
	case *ast.Relate:
		return "*ast.Relate"
	case *ast.On:
		return "*ast.On"
	case *ast.Bad:
		return "*ast.Bad"
	}
	return "?"
}

// FuzzParse checks that no input makes the parser panic, loop forever or
// produce malformed spans.
func FuzzParse(f *testing.F) {
	seeds, _ := filepath.Glob("testdata/*.clowd")
	for _, s := range seeds {
		if b, err := os.ReadFile(s); err == nil {
			f.Add(string(b))
		}
	}
	f.Add("on aws\n\tservice\n  \"")
	f.Fuzz(func(t *testing.T, src string) {
		file := diag.NewFile("fuzz.clowd", src)
		ast, diags := Parse(file)
		checkInvariants(t, file, ast, diags)
	})
}
