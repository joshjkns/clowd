// Package format prints Clowd source in the one canonical style used by
// `clowd fmt` and the language server (05-tooling.md).
//
// The formatter works from the AST rather than rewriting tokens, so the
// output is canonical by construction: keywords lowercase, two-space
// indentation, single spaces between words and fixed blank-line rules. The
// few spellings the AST does not record (`lb` versus `load balancer`, `,`
// versus `and` between traits) are read back from the source, because both
// are valid and the formatter should not choose for the user.
//
// A file with syntax errors is not formatted: the AST of a broken file is a
// guess, and printing a guess would silently rewrite what the user typed.
package format

import (
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/syntax/ast"
	"github.com/joshjkns/clowd/internal/syntax/parser"
	"github.com/joshjkns/clowd/internal/syntax/token"
)

// File formats f. It returns the parser's diagnostics, and nil output when
// any of them is an error.
func File(f *diag.File) ([]byte, diag.List) {
	file, diags := parser.Parse(f)
	if diags.HasErrors() {
		return nil, diags
	}
	p := &printer{src: f, file: file}
	return p.print(), diags
}

// line is one output line, either code or an own-line comment.
type line struct {
	indent int
	text   string
	// start is the source offset the line came from, used to put comments
	// back in order; srcLine is its 1-based source line.
	start, srcLine int
	// stmt is the index of the statement the line belongs to;
	// len(file.Stmts) for comments after the last statement.
	stmt    int
	comment bool
	// trailing is a comment at the end of a code line, including its `#`.
	trailing string
}

type printer struct {
	src  *diag.File
	file *ast.File
}

func (p *printer) print() []byte {
	lines := p.code()
	lines = p.placeComments(lines)

	var b strings.Builder
	for i, l := range lines {
		if i > 0 && p.blankBefore(lines[i-1], l) {
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat("  ", l.indent))
		b.WriteString(l.text)
		if l.trailing != "" {
			b.WriteString(" ")
			b.WriteString(l.trailing)
		}
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// blankBefore decides whether a blank line separates prev and l.
//
// Between statements there is one blank line, except that consecutive
// one-line statements of the same shape (use lines, relate lines,
// declarations without a block) are grouped. Inside a group the user may
// still start a section with a comment after a blank line. Leading comments
// keep a blank line the source had before them or before their statement,
// so a file header stays separate from the first statement. Blocks never
// contain blank lines.
func (p *printer) blankBefore(prev, l line) bool {
	srcBlank := l.srcLine-prev.srcLine > 1
	if l.stmt != prev.stmt {
		if l.stmt == len(p.file.Stmts) {
			return srcBlank // comments at the end of the file
		}
		if !grouped(p.file.Stmts[prev.stmt], p.file.Stmts[l.stmt]) {
			return true
		}
		return l.comment && srcBlank
	}
	return l.indent == 0 && prev.indent == 0 && prev.comment && srcBlank
}

// grouped reports whether a and b are one-line statements of the same shape.
func grouped(a, b ast.Stmt) bool {
	switch a := a.(type) {
	case *ast.Use:
		_, ok := b.(*ast.Use)
		return ok
	case *ast.Relate:
		_, ok := b.(*ast.Relate)
		return ok
	case *ast.Declare:
		bd, ok := b.(*ast.Declare)
		return ok && len(a.Props) == 0 && len(bd.Props) == 0
	}
	return false
}

// code returns the code lines of every statement, in source order.
func (p *printer) code() []line {
	var lines []line
	add := func(stmt, indent int, sp diag.Span, text string) {
		lines = append(lines, line{
			indent:  indent,
			text:    text,
			start:   sp.Start,
			srcLine: p.src.Position(sp.Start).Line,
			stmt:    stmt,
		})
	}
	var props func(stmt, indent int, ps []ast.Property, raw bool)
	props = func(stmt, indent int, ps []ast.Property, raw bool) {
		for _, prop := range ps {
			add(stmt, indent, prop.Pos, property(prop, raw))
			props(stmt, indent+1, prop.Block, raw || isOn(prop))
		}
	}
	for i, s := range p.file.Stmts {
		switch s := s.(type) {
		case *ast.Use:
			text := "use " + strings.ToLower(s.Cloud.Name)
			if s.Region != nil {
				text += " in " + value(*s.Region)
			}
			add(i, 0, s.Pos, text)
		case *ast.Declare:
			add(i, 0, s.Pos, p.declare(s))
			props(i, 1, s.Props, false)
		case *ast.Relate:
			add(i, 0, s.Pos, s.From.Name+" "+s.Verb.String()+" "+s.To.Name)
		case *ast.On:
			// Program-wide settings are Clowd's own, so their keys are
			// keywords; only `on` blocks in resources hold provider data.
			add(i, 0, s.Pos, "on "+strings.ToLower(s.Cloud.Name))
			props(i, 1, s.Settings, false)
		case *ast.Bad:
			// Unreachable: Bad statements always come with an error, and
			// files with errors are not formatted.
		}
	}
	return lines
}

func (p *printer) declare(d *ast.Declare) string {
	var b strings.Builder
	b.WriteString(p.kind(d.Kind, d.KindPos))
	if d.Kind.Named() {
		b.WriteString(" ")
		b.WriteString(d.Name.Name)
	}
	for i, t := range d.Traits {
		switch {
		case i == 0:
			b.WriteString(" is ")
		case strings.TrimSpace(p.src.Text[d.Traits[i-1].Pos.End:t.Pos.Start]) == ",":
			b.WriteString(", ")
		default:
			b.WriteString(" and ")
		}
		for j, w := range t.Words {
			if j > 0 {
				b.WriteString(" ")
			}
			// Traits are adjectives from a fixed vocabulary, so they are
			// keywords and lowercase like any other.
			b.WriteString(lowerWord(w))
		}
	}
	return b.String()
}

// kind spells a kind as the user chose: `lb` is a documented alias of
// `load balancer`, not a mistake.
func (p *printer) kind(k ast.Kind, sp diag.Span) string {
	if k == ast.LoadBalancer && strings.EqualFold(p.src.Text[sp.Start:sp.End], "lb") {
		return "lb"
	}
	return k.String()
}

// valueKeywords are the keyword words inside property values, by property
// key and value index (01-language.md, kind reference). Only these are
// lowercased: other values, such as env names and images, are data.
var valueKeywords = map[string]map[int]string{
	"scale":  {1: "to"},
	"send":   {1: "to"},
	"listen": {0: "on", 2: "with", 3: "tls", 4: "for"},
}

// property prints a property line. Inside raw (`on`) blocks the keys and
// values are provider data, so none are keywords and their case is kept;
// the commas of a list are kept too.
func property(prop ast.Property, raw bool) string {
	key := lowerWord(prop.Key)
	if raw {
		key = value(prop.Key)
	}
	var b strings.Builder
	b.WriteString(key)
	ci := 0
	for i, v := range prop.Values {
		for ci < len(prop.Commas) && prop.Commas[ci].Start < v.Pos.Start {
			b.WriteString(",")
			ci++
		}
		b.WriteString(" ")
		switch kw, ok := valueKeywords[key][i]; {
		case !raw && isOn(prop):
			b.WriteString(lowerWord(v)) // the cloud
		case !raw && ok && !v.Quoted && strings.EqualFold(v.Text, kw):
			b.WriteString(kw)
		default:
			b.WriteString(value(v))
		}
	}
	for ; ci < len(prop.Commas); ci++ {
		b.WriteString(",")
	}
	return b.String()
}

// isOn reports whether prop starts an `on <cloud>` block.
func isOn(prop ast.Property) bool { return !prop.Key.Quoted && strings.EqualFold(prop.Key.Text, "on") }

// placeComments attaches trailing comments to their code line and merges
// own-line comments into lines in source order.
func (p *printer) placeComments(code []line) []line {
	var own []line
	byLine := make(map[int]int, len(code)) // source line → index in code
	for i, l := range code {
		byLine[l.srcLine] = i
	}
	for _, c := range p.file.Comments {
		text := strings.TrimRight(c.Text, " \t\r")
		pos := p.src.Position(c.Span.Start)
		if i, ok := byLine[pos.Line]; ok && !p.ownLine(c) {
			code[i].trailing = text
			continue
		}
		own = append(own, line{
			text:    text,
			start:   c.Span.Start,
			srcLine: pos.Line,
			comment: true,
			// indent and stmt are set below, once the neighbours are known.
			indent: pos.Column - 1,
		})
	}

	merged := make([]line, 0, len(code)+len(own))
	ci := 0
	for _, c := range own {
		for ci < len(code) && code[ci].start < c.start {
			merged = append(merged, code[ci])
			ci++
		}
		// prev is the line before, comment or code: once a comment has left
		// a block, a later indented comment must not rejoin it.
		var prev, next *line
		if len(merged) > 0 {
			prev = &merged[len(merged)-1]
		}
		if ci < len(code) {
			next = &code[ci]
		}
		indented := c.indent > 0
		switch {
		case next != nil && next.indent > 0:
			// Between a block's header and a line in it: part of the block.
			c.indent, c.stmt = next.indent, next.stmt
		case prev != nil && prev.indent > 0 && indented:
			// After a block's last line and still indented: the end of it.
			c.indent, c.stmt = prev.indent, prev.stmt
		case next != nil:
			c.indent, c.stmt = 0, next.stmt
		default:
			c.indent, c.stmt = 0, len(p.file.Stmts)
		}
		merged = append(merged, c)
	}
	return append(merged, code[ci:]...)
}

// ownLine reports whether only whitespace precedes c on its line.
func (p *printer) ownLine(c token.Token) bool {
	start := p.src.LineStart(p.src.Position(c.Span.Start).Line)
	return strings.TrimLeft(p.src.Text[start:c.Span.Start], " \t\ufeff") == ""
}

// value prints a word as written, or a string with its escapes restored.
func value(v ast.Value) string {
	if !v.Quoted {
		return v.Text
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(v.Text) + `"`
}

func values(vs []ast.Value) string {
	s := make([]string, len(vs))
	for i, v := range vs {
		s[i] = value(v)
	}
	return strings.Join(s, " ")
}

// lowerWord lowercases a bare keyword; quoted strings are data and kept.
func lowerWord(v ast.Value) string {
	if v.Quoted {
		return value(v)
	}
	return strings.ToLower(v.Text)
}
