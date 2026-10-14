// Package resolve binds resource names to their declarations.
//
// Names are case-insensitive: `API` refers to `api`, with a warning, and two
// declarations that differ only by case are an error. Declarations may come
// after their uses, so resolution is two passes: collect every declaration,
// then bind every reference.
package resolve

import (
	"fmt"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/spell"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// Table is the result of resolving a file.
type Table struct {
	// Decls are the declarations that own their name, in source order.
	// Duplicates and declarations with a missing name are not included.
	Decls []*ast.Declare
	// Refs are every use of a resource name outside its declaration, in
	// source order. The LSP uses them for go-to-definition and rename.
	Refs []Ref

	byKey map[string]*ast.Declare
}

// Ref is one use of a resource name.
type Ref struct {
	Name string
	Pos  diag.Span
	// Decl is nil when the name is not declared; a diagnostic has already
	// been reported.
	Decl *ast.Declare
}

// Lookup returns the declaration for name, or nil. It reports nothing:
// Resolve has already diagnosed every reference in the file.
func (t *Table) Lookup(name string) *ast.Declare {
	return t.byKey[key(name)]
}

func key(name string) string { return strings.ToLower(name) }

// Resolve builds the symbol table for f and reports duplicate, malformed and
// unknown names.
func Resolve(src *diag.File, f *ast.File) (*Table, diag.List) {
	r := &resolver{src: src, t: &Table{byKey: map[string]*ast.Declare{}}}
	for _, s := range f.Stmts {
		if d, ok := s.(*ast.Declare); ok {
			r.declare(d)
		}
	}
	for _, s := range f.Stmts {
		switch s := s.(type) {
		case *ast.Relate:
			r.relate(s)
		case *ast.Declare:
			r.sends(s)
		case *ast.On:
			for _, o := range s.Overrides {
				r.override(o)
			}
		}
	}
	r.diags.Sort()
	return r.t, r.diags
}

type resolver struct {
	src   *diag.File
	t     *Table
	diags diag.List
}

func (r *resolver) line(sp diag.Span) int { return r.src.Position(sp.Start).Line }

func (r *resolver) declare(d *ast.Declare) {
	if d.Name.Missing() {
		return // the parser has reported it
	}
	name := d.Name.Name
	if !validName(name) {
		r.diags.Add(diag.Errorf(d.Name.Pos, "`%s` is not a valid name", name).
			Suggest("names start with a letter and contain only letters, digits, `-` and `_`%s", sanitizedHint(name)))
	}
	if prev, ok := r.t.byKey[key(name)]; ok {
		at := r.line(prev.Name.Pos)
		if prev.Name.Name == name {
			r.diags.Add(diag.Errorf(d.Name.Pos, "`%s` is already declared as a %s on line %d", name, prev.Kind, at).
				Suggest("give each resource its own name"))
		} else {
			r.diags.Add(diag.Errorf(d.Name.Pos, "`%s` differs only by case from `%s` on line %d", name, prev.Name.Name, at).
				Suggest("names are case-insensitive, so these are the same name; rename one of them"))
		}
		return
	}
	r.t.byKey[key(name)] = d
	r.t.Decls = append(r.t.Decls, d)
}

func (r *resolver) relate(s *ast.Relate) {
	// `api connects to db` reads as "a service connects to a database", so
	// those are the kinds suggested when a name is not declared.
	r.ref(s.From.Name, s.From.Pos, ast.Service)
	r.ref(s.To.Name, s.To.Pos, verbTargetKind(s.Verb))
}

func verbTargetKind(v ast.Verb) ast.Kind {
	switch v {
	case ast.Reads, ast.Writes, ast.ReadsAndWrites:
		return ast.Bucket
	case ast.SendsTo:
		return ast.Queue
	}
	return ast.Database
}

// sends resolves the target of `send <path> to <name>` in a load balancer.
// Only the well-formed shape is resolved; check reports malformed ones.
func (r *resolver) sends(d *ast.Declare) {
	if d.Kind != ast.LoadBalancer {
		return
	}
	for _, p := range d.Props {
		if !strings.EqualFold(p.Key.Text, "send") || len(p.Values) != 3 || !isWord(p.Values[1], "to") {
			continue
		}
		target := p.Values[2]
		r.ref(target.Text, target.Pos, ast.Service)
	}
}

func (r *resolver) override(o ast.Override) {
	if o.Name.Missing() {
		return
	}
	d := r.ref(o.Name.Name, o.Name.Pos, o.Kind)
	if d != nil && d.Kind != o.Kind {
		r.diags.Add(diag.Errorf(o.KindPos, "`%s` is a %s, not a %s", d.Name.Name, d.Kind, o.Kind).
			Suggest("write `%s %s sets ...`", d.Kind, d.Name.Name))
	}
}

// ref binds one use of a name. kind is the kind to suggest declaring if the
// name is unknown.
func (r *resolver) ref(name string, pos diag.Span, kind ast.Kind) *ast.Declare {
	if name == "" {
		return nil
	}
	d := r.t.byKey[key(name)]
	r.t.Refs = append(r.t.Refs, Ref{Name: name, Pos: pos, Decl: d})
	switch {
	case d == nil:
		e := diag.Errorf(pos, "no resource named `%s`", name)
		if guess, ok := spell.Closest(name, r.names()); ok {
			e = e.Suggest("did you mean `%s`?", guess)
		} else {
			e = e.Suggest("declare it, for example `%s %s`", kind, strings.ToLower(name))
		}
		r.diags.Add(e)
	case d.Name.Name != name:
		r.diags.Add(diag.Warnf(pos, "`%s` is declared as `%s`", name, d.Name.Name).
			Suggest("write `%s`", d.Name.Name))
	}
	return d
}

func (r *resolver) names() []string {
	names := make([]string, len(r.t.Decls))
	for i, d := range r.t.Decls {
		names[i] = d.Name.Name
	}
	return names
}

func isWord(v ast.Value, w string) bool { return !v.Quoted && strings.EqualFold(v.Text, w) }

// validName reports whether name matches [a-z][a-z0-9_-]*, ignoring case.
func validName(name string) bool {
	for i, c := range name {
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z':
		case i > 0 && ('0' <= c && c <= '9' || c == '-' || c == '_'):
		default:
			return false
		}
	}
	return name != ""
}

// sanitizedHint suggests a valid spelling of name, if a simple one exists.
func sanitizedHint(name string) string {
	var b strings.Builder
	for _, c := range name {
		switch {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9', c == '-', c == '_':
			b.WriteRune(c)
		case c == '.' || c == ' ' || c == '/' || c == ':':
			b.WriteRune('-')
		}
	}
	s := strings.Trim(b.String(), "-_")
	if s == "" || !validName(s) {
		return ""
	}
	return fmt.Sprintf(", e.g. `%s`", s)
}
