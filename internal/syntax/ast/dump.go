package ast

import (
	"fmt"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
)

// Dump renders the tree as indented text with line:column spans, for golden
// tests and debugging. The format is not stable API.
func Dump(src *diag.File, f *File) string {
	d := dumper{src: src}
	for _, s := range f.Stmts {
		d.stmt(s)
	}
	return d.b.String()
}

type dumper struct {
	src *diag.File
	b   strings.Builder
}

func (d *dumper) span(sp diag.Span) string {
	s, e := d.src.Position(sp.Start), d.src.Position(sp.End)
	return fmt.Sprintf("%d:%d-%d:%d", s.Line, s.Column, e.Line, e.Column)
}

func (d *dumper) line(indent int, format string, args ...any) {
	d.b.WriteString(strings.Repeat("  ", indent))
	fmt.Fprintf(&d.b, format, args...)
	d.b.WriteByte('\n')
}

func (d *dumper) ident(id Ident) string {
	if id.Missing() {
		return fmt.Sprintf("<missing> %s", d.span(id.Pos))
	}
	return fmt.Sprintf("%q %s", id.Name, d.span(id.Pos))
}

func (d *dumper) value(v Value) string {
	if v.Quoted {
		return fmt.Sprintf("string %q %s", v.Text, d.span(v.Pos))
	}
	return fmt.Sprintf("%q %s", v.Text, d.span(v.Pos))
}

func (d *dumper) values(indent int, label string, vs []Value) {
	for _, v := range vs {
		d.line(indent, "%s %s", label, d.value(v))
	}
}

func (d *dumper) stmt(s Stmt) {
	switch s := s.(type) {
	case *Use:
		d.line(0, "use %s", d.span(s.Pos))
		d.line(1, "cloud %s", d.ident(s.Cloud))
		if s.Region != nil {
			d.line(1, "region %s", d.value(*s.Region))
		}
	case *Declare:
		d.line(0, "declare %s %s", d.span(s.Pos), s.Kind)
		d.line(1, "kind %s", d.span(s.KindPos))
		d.line(1, "name %s", d.ident(s.Name))
		for _, t := range s.Traits {
			d.line(1, "trait %s", d.span(t.Pos))
			d.values(2, "word", t.Words)
		}
		for _, p := range s.Props {
			d.line(1, "property %s", d.span(p.Pos))
			d.line(2, "key %s", d.value(p.Key))
			d.values(2, "value", p.Values)
		}
	case *Relate:
		d.line(0, "relate %s", d.span(s.Pos))
		d.line(1, "from %s", d.ident(s.From))
		d.line(1, "verb %q %s", s.Verb, d.span(s.VerbPos))
		d.line(1, "to %s", d.ident(s.To))
	case *On:
		d.line(0, "on %s", d.span(s.Pos))
		d.line(1, "cloud %s", d.ident(s.Cloud))
		for _, o := range s.Overrides {
			d.line(1, "override %s %s", d.span(o.Pos), o.Kind)
			d.line(2, "name %s", d.ident(o.Name))
			d.line(2, "key %s", d.value(o.Key))
			d.values(2, "value", o.Value)
		}
	case *Bad:
		d.line(0, "bad %s", d.span(s.Pos))
	default:
		panic(fmt.Sprintf("ast.Dump: unexpected statement %T", s))
	}
}
