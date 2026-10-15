// Package analyze runs everything `clowd check` reports on a file (the
// front end, the capability matrix and the backends' own checks), so the
// CLI and the language server report the same diagnostics.
//
// It is separate from compile because it imports the backends, whose
// tests import compile.
package analyze

import (
	"github.com/joshjkns/clowd/internal/capability"
	"github.com/joshjkns/clowd/internal/compile"
	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/lower/aws"
	"github.com/joshjkns/clowd/internal/lower/gcp"
)

// Analysis is a checked file evaluated against its clouds.
type Analysis struct {
	*compile.Result
	// Report is the capability matrix, or nil when the file has errors.
	Report *capability.Report
	// All are the front end's diagnostics plus the capability findings
	// and what lowering reports, sorted by position.
	All diag.List
}

// HasBackend reports whether Clowd can lower and deploy to c.
func HasBackend(c ir.Cloud) bool { return c == ir.AWS || c == ir.GCP }

// lowerDiags lowers p for c and returns what lowering reports.
func lowerDiags(p *ir.Program, c ir.Cloud, source string) diag.List {
	switch c {
	case ir.AWS:
		return aws.Lower(p, aws.Options{Project: "check", Source: source}).Diags
	case ir.GCP:
		return gcp.Lower(p, gcp.Options{Project: "check", Source: source}).Diags
	}
	return nil
}

// File checks f and evaluates it against targets, or against the file's
// own `use` lines when targets is empty.
func File(f *diag.File, targets []ir.Cloud) *Analysis {
	res := compile.Check(f)
	a := &Analysis{Result: res}
	a.All.Add(res.Diags...)
	if res.Diags.HasErrors() {
		return a
	}
	clouds := targets
	if len(clouds) == 0 {
		for _, t := range res.Program.Targets {
			clouds = append(clouds, t.Cloud)
		}
	}
	a.Report = capability.Evaluate(res.Program, clouds)
	a.All.Add(a.Report.Diagnostics()...)
	// Lowering knows what the matrix can't, such as whether an override
	// names a real provider argument, so run it for every cloud that has
	// a backend and that the program can be deployed to.
	for i, c := range clouds {
		if HasBackend(c) && a.Report.Deployable(i) {
			a.All.Add(lowerDiags(res.Program, c, f.Name)...)
		}
	}
	a.All.Sort()
	if a.All.HasErrors() {
		a.Report = nil
	}
	return a
}
