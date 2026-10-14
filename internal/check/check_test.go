package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/golden"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/resolve"
	"github.com/joshjkns/clowd/internal/syntax/parser"
)

// run takes src through the pipeline up to check. It returns earlier
// stages' diagnostics separately so goldens show only what check adds.
func run(name, src string) (f *diag.File, prog *ir.Program, earlier, diags diag.List) {
	f = diag.NewFile(name, src)
	file, pd := parser.Parse(f)
	tab, rd := resolve.Resolve(f, file)
	earlier = append(pd, rd...)
	prog, diags = Check(f, file, tab)
	return f, prog, earlier, diags
}

func dump(f *diag.File, prog *ir.Program, diags diag.List) string {
	var b strings.Builder
	b.WriteString(ir.Dump(f, prog))
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
			f, prog, earlier, diags := run(filepath.Base(in), string(src))
			if earlier.HasErrors() {
				var b strings.Builder
				_ = diag.RenderAll(&b, f, earlier)
				t.Fatalf("check fixtures must parse and resolve cleanly:\n%s", b.String())
			}
			golden.Check(t, strings.TrimSuffix(in, ".clowd")+".golden", dump(f, prog, diags))
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
			f, prog, earlier, diags := run(filepath.Base(in), string(src))
			all := append(earlier, diags...)
			if len(all) > 0 {
				var b strings.Builder
				_ = diag.RenderAll(&b, f, all)
				t.Errorf("examples must check without diagnostics:\n%s", b.String())
			}
			golden.Check(t, filepath.Join("testdata", "examples", name+".golden"), dump(f, prog, diags))
		})
	}
}

// FuzzCheck checks that no input makes resolve or check panic.
func FuzzCheck(f *testing.F) {
	seeds, _ := filepath.Glob("testdata/*.clowd")
	for _, s := range seeds {
		if b, err := os.ReadFile(s); err == nil {
			f.Add(string(b))
		}
	}
	f.Fuzz(func(t *testing.T, src string) {
		_, prog, _, _ := run("fuzz.clowd", src)
		if prog == nil {
			t.Fatal("Check returned a nil program")
		}
	})
}
