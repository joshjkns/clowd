// Package compile runs the front of the pipeline (parse, resolve, check) as
// one call, so the CLI and the language server process a file identically.
package compile

import (
	"github.com/joshjkns/clowd/internal/check"
	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/resolve"
	"github.com/joshjkns/clowd/internal/syntax/ast"
	"github.com/joshjkns/clowd/internal/syntax/parser"
)

// Result holds every stage's output. All fields are set however broken the
// source is.
type Result struct {
	AST     *ast.File
	Symbols *resolve.Table
	Program *ir.Program
	// Diags are every stage's diagnostics, sorted by position.
	Diags diag.List
}

// Check parses, resolves and checks f.
func Check(f *diag.File) *Result {
	file, pd := parser.Parse(f)
	tab, rd := resolve.Resolve(f, file)
	prog, cd := check.Check(f, file, tab)
	r := &Result{AST: file, Symbols: tab, Program: prog}
	r.Diags.Add(pd...)
	r.Diags.Add(rd...)
	r.Diags.Add(cd...)
	r.Diags.Sort()
	return r
}
