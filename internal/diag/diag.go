// Package diag defines the diagnostics shared by every compiler stage, the
// CLI and the language server.
//
// User-facing problems (syntax errors, unknown names, unsupported features)
// are Diagnostics, never Go errors; Go errors are reserved for internal and
// I/O failures.
package diag

import (
	"cmp"
	"fmt"
	"slices"
)

// Severity is how serious a diagnostic is.
type Severity int

// Severities in decreasing order of seriousness, so sorting puts errors first.
const (
	Error Severity = iota
	Warning
	Info
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	case Info:
		return "info"
	}
	return fmt.Sprintf("Severity(%d)", int(s))
}

// Diagnostic is one problem found in a source file.
type Diagnostic struct {
	Severity Severity
	Span     Span
	Message  string
	// Suggestion is an optional fix shown as help, e.g. "did you mean `connects to`?".
	Suggestion string
}

// Errorf returns an error diagnostic at span.
func Errorf(span Span, format string, args ...any) Diagnostic {
	return Diagnostic{Severity: Error, Span: span, Message: fmt.Sprintf(format, args...)}
}

// Warnf returns a warning diagnostic at span.
func Warnf(span Span, format string, args ...any) Diagnostic {
	return Diagnostic{Severity: Warning, Span: span, Message: fmt.Sprintf(format, args...)}
}

// Infof returns an informational diagnostic at span.
func Infof(span Span, format string, args ...any) Diagnostic {
	return Diagnostic{Severity: Info, Span: span, Message: fmt.Sprintf(format, args...)}
}

// Suggest returns a copy of d with a suggestion attached.
func (d Diagnostic) Suggest(format string, args ...any) Diagnostic {
	d.Suggestion = fmt.Sprintf(format, args...)
	return d
}

// List collects diagnostics from one or more stages.
type List []Diagnostic

// Add appends diagnostics to the list.
func (l *List) Add(ds ...Diagnostic) {
	*l = append(*l, ds...)
}

// HasErrors reports whether any diagnostic is an error.
func (l List) HasErrors() bool {
	return slices.ContainsFunc(l, func(d Diagnostic) bool { return d.Severity == Error })
}

// Sort orders diagnostics by position, then severity. It is stable so that
// diagnostics at the same place keep the order they were reported in.
func (l List) Sort() {
	slices.SortStableFunc(l, func(a, b Diagnostic) int {
		return cmp.Or(
			cmp.Compare(a.Span.Start, b.Span.Start),
			cmp.Compare(a.Severity, b.Severity),
		)
	})
}
