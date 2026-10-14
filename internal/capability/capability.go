// Package capability says, per target cloud, whether a checked program can
// be deployed there.
//
// A program asks for features (a service, scale to zero, a FIFO queue, a
// service connecting to a database); the matrix records how well each cloud
// provides each feature. Features are extracted from the IR without
// reference to any cloud, and the matrix is plain data, so adding a cloud is
// adding entries rather than code. The same data drives `clowd check`, LSP
// hovers and, later, lowering, which refuses what the matrix marks
// unavailable.
package capability

import (
	"fmt"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
)

// Level is how well a cloud provides a feature.
type Level int

// Levels in increasing order of trouble, so the worst level in a cell is
// the maximum.
const (
	// NotApplicable marks a cell that doesn't apply, such as an `on aws`
	// block in the gcp column.
	NotApplicable Level = iota
	// Supported maps directly to the cloud.
	Supported
	// Emulated works, with a caveat shown to the user.
	Emulated
	// NotImplemented means the cloud could do it but Clowd doesn't yet.
	NotImplemented
	// Unsupported means the cloud cannot do it.
	Unsupported
)

func (l Level) String() string {
	switch l {
	case NotApplicable:
		return "not applicable"
	case Supported:
		return "supported"
	case Emulated:
		return "emulated"
	case NotImplemented:
		return "not implemented"
	case Unsupported:
		return "unsupported"
	}
	return fmt.Sprintf("Level(%d)", int(l))
}

// Symbol is the level's mark in the matrix table.
func (l Level) Symbol() string {
	switch l {
	case Supported:
		return "✓"
	case Emulated:
		return "⚠"
	case NotImplemented, Unsupported:
		return "✗"
	}
	return "–"
}

// Deployable reports whether a cell at this level can be deployed.
func (l Level) Deployable() bool { return l <= Emulated }

// Entry is one cell of the matrix: how one cloud provides one feature.
type Entry struct {
	Level Level
	// Note is the caveat for Emulated, or the reason for Unsupported. It
	// reads after "on <cloud>: ". NotImplemented needs none: the feature's
	// name says what is missing.
	Note string
}

// Lookup returns how cloud provides f. Features a cloud has no entry for
// are not implemented: that is the state of every feature on a new cloud.
func Lookup(f Feature, cloud ir.Cloud) Entry {
	if e, ok := matrix[f][cloud]; ok {
		return e
	}
	return Entry{Level: NotImplemented}
}

// Product returns what a resource kind becomes on cloud ("ECS Fargate"), or
// "" if the kind is not implemented there.
func Product(k ir.Kind, cloud ir.Cloud) string { return products[k][cloud] }

// Finding is a feature a row uses that a cloud doesn't fully support.
type Finding struct {
	Feature Feature
	Entry
	// Pos is where the feature was asked for.
	Pos diag.Span
}

// Cell is one row's result for one cloud.
type Cell struct {
	// Level is the worst level among the row's features.
	Level Level
	// Findings are the features below Supported, in the order the row
	// asked for them.
	Findings []Finding
}

// Section is the part of the matrix a row belongs to.
type Section int

// Sections, in the order the matrix lists them.
const (
	SectionResources Section = iota
	SectionRelationships
	SectionOverrides
)

func (s Section) String() string {
	switch s {
	case SectionResources:
		return "resources"
	case SectionRelationships:
		return "relationships"
	case SectionOverrides:
		return "overrides"
	}
	return fmt.Sprintf("Section(%d)", int(s))
}

// Row is one line of the matrix: a resource, a relation or an `on` block.
type Row struct {
	Section Section
	// Label is how the row is shown: "service api", "api connects to db",
	// "on aws".
	Label string
	Pos   diag.Span
	// Cells holds one cell per evaluated cloud, in Report.Clouds order.
	Cells []Cell
}

// Report is a program evaluated against some clouds.
type Report struct {
	Clouds []ir.Cloud
	Rows   []Row
}

// Deployable reports whether every row can be deployed to the cloud in
// column i.
func (r *Report) Deployable(i int) bool {
	for _, row := range r.Rows {
		if !row.Cells[i].Level.Deployable() {
			return false
		}
	}
	return true
}

// AllDeployable reports whether every evaluated cloud is deployable.
func (r *Report) AllDeployable() bool {
	for i := range r.Clouds {
		if !r.Deployable(i) {
			return false
		}
	}
	return true
}

// Diagnostics returns a diagnostic per finding: warnings for what blocks a
// deploy and infos for caveats. The same finding on several clouds is one
// diagnostic. Capability problems are never errors, so a file that is
// valid but not deployable everywhere is distinguishable (exit code 1) from
// one with mistakes in it (exit code 2).
func (r *Report) Diagnostics() diag.List {
	type key struct {
		pos     diag.Span
		feature Feature
		entry   Entry
	}
	var order []key
	clouds := map[key][]string{}
	for _, row := range r.Rows {
		for i, cell := range row.Cells {
			for _, f := range cell.Findings {
				k := key{f.Pos, f.Feature, f.Entry}
				if _, seen := clouds[k]; !seen {
					order = append(order, k)
				}
				clouds[k] = append(clouds[k], r.Clouds[i].String())
			}
		}
	}
	var l diag.List
	for _, k := range order {
		on := list(clouds[k])
		switch k.entry.Level {
		case Emulated:
			l.Add(diag.Infof(k.pos, "on %s: %s", on, k.entry.Note))
		case NotImplemented:
			l.Add(diag.Warnf(k.pos, "%s is not implemented for %s yet", k.feature, on))
		case Unsupported:
			l.Add(diag.Warnf(k.pos, "not supported on %s: %s", on, k.entry.Note))
		}
	}
	l.Sort()
	return l
}

// Evaluate checks p against clouds, which are usually p's targets but may
// be any clouds (`clowd check --target`).
func Evaluate(p *ir.Program, clouds []ir.Cloud) *Report {
	rep := &Report{Clouds: clouds}
	add := func(sec Section, label string, pos diag.Span, uses []use) {
		row := Row{Section: sec, Label: label, Pos: pos}
		for _, c := range clouds {
			row.Cells = append(row.Cells, evaluate(uses, c))
		}
		rep.Rows = append(rep.Rows, row)
	}
	for _, r := range p.Resources {
		h := r.Meta()
		add(SectionResources, fmt.Sprintf("%s %s", r.Kind(), h.Name), h.Pos, resourceFeatures(r))
	}
	for _, rel := range p.Relations {
		if f, ok := relationFeature(p, rel); ok {
			add(SectionRelationships, fmt.Sprintf("%s %s %s", rel.From, rel.Verb, rel.To), rel.Pos, []use{{feature: f, pos: rel.Pos}})
		}
	}
	for _, o := range p.Overrides {
		row := Row{Section: SectionOverrides, Label: fmt.Sprintf("%s %s on %s", o.Kind, o.Resource, o.Cloud), Pos: o.Pos}
		for _, c := range clouds {
			if c != o.Cloud {
				row.Cells = append(row.Cells, Cell{Level: NotApplicable})
				continue
			}
			row.Cells = append(row.Cells, evaluate([]use{{feature: Overrides, pos: o.Pos}}, c))
		}
		rep.Rows = append(rep.Rows, row)
	}
	return rep
}

// evaluate combines the entries for uses on one cloud. When the resource
// kind itself is unavailable, its features are not listed separately: "a
// service is not implemented for azure" says everything.
func evaluate(uses []use, cloud ir.Cloud) Cell {
	var cell Cell
	for i, u := range uses {
		e := u.entry(cloud)
		if e.Level > cell.Level {
			cell.Level = e.Level
		}
		if e.Level > Supported {
			cell.Findings = append(cell.Findings, Finding{Feature: u.feature, Entry: e, Pos: u.pos})
		}
		if i == 0 && !e.Level.Deployable() && u.feature.isKind() {
			break
		}
	}
	return cell
}
