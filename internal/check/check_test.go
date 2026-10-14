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

func TestCIDR(t *testing.T) {
	tests := []struct {
		in, want, err string
	}{
		{"10.0.0.0/16", "10.0.0.0/16", ""},
		{"172.16.0.0/20", "172.16.0.0/20", ""},
		{"10.0.0.0/8", "", "between /16 and /20, not /8"},
		{"10.0.0.0/24", "", "between /16 and /20, not /24"},
		{"10.0.0.1/16", "", "has host bits set"},
		{"fd00::/16", "", "not an IPv4 CIDR block"},
		{"10.0.0.0", "", "not an IPv4 CIDR block"},
	}
	for _, tt := range tests {
		_, prog, _, diags := run("t.clowd", "use aws\n\nnetwork\n  cidr "+tt.in+"\n")
		if prog.Network.CIDR != tt.want {
			t.Errorf("cidr %s: got %q, want %q", tt.in, prog.Network.CIDR, tt.want)
		}
		if tt.err == "" && len(diags) > 0 {
			t.Errorf("cidr %s: unexpected %v", tt.in, diags)
		}
		if tt.err != "" && !strings.Contains(diags[0].Message, tt.err) {
			t.Errorf("cidr %s: got %q, want it to contain %q", tt.in, diags[0].Message, tt.err)
		}
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
