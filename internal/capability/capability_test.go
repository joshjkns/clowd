package capability

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joshjkns/clowd/internal/check"
	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/golden"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/resolve"
	"github.com/joshjkns/clowd/internal/syntax/parser"
)

// pipeline runs src through check and evaluates it against its targets,
// failing the test if earlier stages report errors.
func pipeline(t *testing.T, name, src string) (*diag.File, *Report) {
	t.Helper()
	f := diag.NewFile(name, src)
	file, pd := parser.Parse(f)
	tab, rd := resolve.Resolve(f, file)
	prog, cd := check.Check(f, file, tab)
	var all diag.List
	all.Add(pd...)
	all.Add(rd...)
	all.Add(cd...)
	if all.HasErrors() {
		var b strings.Builder
		_ = diag.RenderAll(&b, f, all)
		t.Fatalf("capability fixtures must check cleanly:\n%s", b.String())
	}
	var clouds []ir.Cloud
	for _, tg := range prog.Targets {
		clouds = append(clouds, tg.Cloud)
	}
	return f, Evaluate(prog, clouds)
}

func dump(f *diag.File, r *Report) string {
	var b strings.Builder
	for _, row := range r.Rows {
		var marks []string
		for i, c := range row.Cells {
			marks = append(marks, fmt.Sprintf("%s %s", r.Clouds[i], c.Level.Symbol()))
		}
		fmt.Fprintf(&b, "%s: %s\n", row.Label, strings.Join(marks, "  "))
		for i, c := range row.Cells {
			for _, fd := range c.Findings {
				p := f.Position(fd.Pos.Start)
				fmt.Fprintf(&b, "  %s %s (%s) at %d:%d: %s\n", r.Clouds[i], fd.Level, fd.Feature, p.Line, p.Column, fd.Note)
			}
		}
	}
	var dep []string
	for i, c := range r.Clouds {
		dep = append(dep, fmt.Sprintf("%s %t", c, r.Deployable(i)))
	}
	fmt.Fprintf(&b, "deployable: %s\n", strings.Join(dep, "  "))
	if diags := r.Diagnostics(); len(diags) > 0 {
		b.WriteString("\ndiagnostics:\n")
		_ = diag.RenderAll(&b, f, diags)
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
			f, rep := pipeline(t, filepath.Base(in), string(src))
			golden.Check(t, strings.TrimSuffix(in, ".clowd")+".golden", dump(f, rep))
		})
	}
}

func TestExamples(t *testing.T) {
	examples, err := filepath.Glob("../../examples/*.clowd")
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range examples {
		name := strings.TrimSuffix(filepath.Base(in), ".clowd")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			f, rep := pipeline(t, filepath.Base(in), string(src))
			golden.Check(t, filepath.Join("testdata", "examples", name+".golden"), dump(f, rep))
		})
	}
}

// TestMatrix renders the whole matrix as the support table, so a change to
// the data shows up as a reviewable diff.
func TestMatrix(t *testing.T) {
	var b strings.Builder
	b.WriteString("| Feature | aws | gcp | azure |\n|---|---|---|---|\n")
	features := make([]Feature, 0, len(matrix))
	for f := range matrix {
		features = append(features, f)
	}
	slices.Sort(features)
	for _, f := range features {
		var cells []string
		for _, c := range ir.Clouds {
			e := Lookup(f, c)
			cell := e.Level.Symbol()
			if e.Level != Supported && e.Level != NotImplemented {
				cell += " " + e.Note
			}
			cells = append(cells, cell)
		}
		fmt.Fprintf(&b, "| %s | %s |\n", f, strings.Join(cells, " | "))
	}
	golden.Check(t, "testdata/matrix.golden", b.String())
}

// TestEveryFeatureHasEntries catches a feature added to features.go but not
// to the matrix, which would silently show as not implemented everywhere.
func TestEveryFeatureHasEntries(t *testing.T) {
	all := []Feature{
		Service, Database, Bucket, LoadBalancer, Queue,
		ServiceAutoscale, ServiceScaleToZero, ServiceCPU, ServiceMemory, ServiceEnv,
		Postgres, MySQL, PublicBucket, FIFOQueue,
		InternalLB, LBTLS, LBPathRouting, LBOddPort,
		ConnectsToDatabase, ConnectsToService, BucketAccess, ReadsQueue, SendsToQueue,
		Overrides,
	}
	for _, f := range all {
		if _, ok := matrix[f]; !ok {
			t.Errorf("feature %q has no matrix entry", f)
		}
	}
	for _, k := range []ir.Kind{ir.ServiceKind, ir.DatabaseKind, ir.BucketKind, ir.LoadBalancerKind, ir.QueueKind} {
		for _, c := range []ir.Cloud{ir.AWS, ir.GCP} {
			if Product(k, c) == "" {
				t.Errorf("no product for %s on %s", k, c)
			}
		}
	}
}

func TestLevel(t *testing.T) {
	for _, tt := range []struct {
		l          Level
		symbol     string
		deployable bool
	}{
		{NotApplicable, "–", true},
		{Supported, "✓", true},
		{Emulated, "⚠", true},
		{NotImplemented, "✗", false},
		{Unsupported, "✗", false},
	} {
		if got := tt.l.Symbol(); got != tt.symbol {
			t.Errorf("%s.Symbol() = %q, want %q", tt.l, got, tt.symbol)
		}
		if got := tt.l.Deployable(); got != tt.deployable {
			t.Errorf("%s.Deployable() = %v, want %v", tt.l, got, tt.deployable)
		}
	}
}
