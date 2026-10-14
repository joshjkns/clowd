// Package parser turns Clowd source into an AST.
//
// The parser is hand-written recursive descent (decision D6) and never stops
// at the first error: each statement is one line plus an optional indented
// block, so after an error it skips to the end of the line (and past any
// block that cannot belong there) and carries on. Keywords are recognised by
// position, case-insensitively (decision D8).
package parser

import (
	"fmt"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/syntax/ast"
	"github.com/joshjkns/clowd/internal/syntax/lexer"
	"github.com/joshjkns/clowd/internal/syntax/token"
)

// Parse lexes and parses f. It always returns a File, however broken the
// input; the diagnostics (lexer and parser, sorted by position) say what is
// wrong.
func Parse(f *diag.File) (*ast.File, diag.List) {
	lx := lexer.Lex(f)
	p := &parser{toks: lx.Tokens, diags: lx.Diags}
	file := &ast.File{Comments: lx.Comments}
	for p.tok().Kind != token.EOF {
		switch t := p.tok(); t.Kind {
		case token.Newline, token.Dedent:
			// A DEDENT here closes a stray top-level block reported below.
			p.next()
		case token.Indent:
			// Only possible on the first statement: every other statement
			// consumes or rejects the block that follows it. Parse the
			// contents as top-level statements.
			p.report(diag.Errorf(p.lineSpan(p.i+1), "unexpected indentation").
				Suggest("top-level statements start at the beginning of the line"))
			p.next()
		default:
			file.Stmts = append(file.Stmts, p.statement())
		}
	}
	p.diags.Sort()
	return file, p.diags
}

type parser struct {
	toks  []token.Token
	i     int
	diags diag.List
	// prevEnd is the end of the last word, string or comma consumed, used to
	// end statement spans before the NEWLINE and to place "expected X"
	// diagnostics right after what the user typed.
	prevEnd int
}

func (p *parser) tok() token.Token { return p.peek(0) }

func (p *parser) peek(n int) token.Token {
	return p.toks[min(p.i+n, len(p.toks)-1)] // the last token is EOF
}

func (p *parser) next() token.Token {
	t := p.toks[p.i]
	if t.Kind == token.EOF {
		return t
	}
	p.i++
	switch t.Kind {
	case token.Word, token.String, token.Comma:
		p.prevEnd = t.Span.End
	}
	return t
}

func (p *parser) report(d diag.Diagnostic) { p.diags.Add(d) }

func (p *parser) atEOL() bool {
	k := p.tok().Kind
	return k == token.Newline || k == token.EOF
}

// here is a zero-width span just after the last thing consumed.
func (p *parser) here() diag.Span { return diag.Span{Start: p.prevEnd, End: p.prevEnd} }

func isValue(t token.Token) bool { return t.Kind == token.Word || t.Kind == token.String }

// lineSpan returns the span from token i to the end of its line.
func (p *parser) lineSpan(i int) diag.Span {
	i = min(i, len(p.toks)-1)
	sp := p.toks[i].Span
	for ; i < len(p.toks); i++ {
		k := p.toks[i].Kind
		if k == token.Newline || k == token.EOF {
			break
		}
		sp = sp.Join(p.toks[i].Span)
	}
	return sp
}

// quote describes a token for an error message.
func quote(t token.Token) string {
	switch t.Kind {
	case token.Word:
		return "`" + t.Text + "`"
	case token.String:
		return fmt.Sprintf("`%q`", t.Text)
	}
	return t.Kind.String()
}

// skipLine consumes the rest of the line including its NEWLINE.
func (p *parser) skipLine() {
	for !p.atEOL() {
		p.next()
	}
	if p.tok().Kind == token.Newline {
		p.next()
	}
}

// finishLine reports anything left on the line as unexpected, then consumes
// the NEWLINE.
func (p *parser) finishLine() {
	if !p.atEOL() {
		p.report(diag.Errorf(p.lineSpan(p.i), "unexpected %s", quote(p.tok())))
	}
	p.skipLine()
}

// skipBlock consumes an indented block, including nested blocks.
func (p *parser) skipBlock() {
	depth := 0
	for {
		switch p.next().Kind {
		case token.Indent:
			depth++
		case token.Dedent:
			depth--
			if depth == 0 {
				return
			}
		case token.EOF:
			return
		}
	}
}

// noBlock rejects an indented block after a statement that cannot have one.
func (p *parser) noBlock(why string) {
	if p.tok().Kind != token.Indent {
		return
	}
	p.report(diag.Errorf(p.lineSpan(p.i+1), "unexpected indented block").Suggest("%s", why))
	p.skipBlock()
}

// matchWords reports whether the tokens starting n ahead are exactly words.
func (p *parser) matchWords(n int, words []string) bool {
	for j, w := range words {
		if !p.peek(n + j).Is(w) {
			return false
		}
	}
	return true
}

// match returns the longest phrase in ps starting n tokens ahead.
func match[T any](p *parser, n int, ps []phrase[T]) (value T, length int) {
	for _, ph := range ps {
		if len(ph.words) > length && p.matchWords(n, ph.words) {
			value, length = ph.value, len(ph.words)
		}
	}
	return value, length
}

// verbAt finds a relationship verb starting n tokens ahead. canonical is
// false when only a synonym matched; a longer synonym wins over a shorter
// verb, so `reads from` is caught rather than read as `reads` + `from`.
func (p *parser) verbAt(n int) (v ast.Verb, length int, canonical bool) {
	v, length = match(p, n, verbs)
	if sv, sl := match(p, n, verbSynonyms); sl > length {
		return sv, sl, false
	}
	return v, length, length > 0
}

// consume consumes n tokens and returns their combined span.
func (p *parser) consume(n int) diag.Span {
	sp := p.tok().Span
	for range n {
		sp = sp.Join(p.next().Span)
	}
	return sp
}

// endStmt returns a statement span from start to the last thing consumed.
func (p *parser) endStmt(start diag.Span) diag.Span {
	return diag.Span{Start: start.Start, End: max(start.End, p.prevEnd)}
}

func (p *parser) statement() ast.Stmt {
	first := p.tok()
	if first.Kind != token.Word {
		p.report(diag.Errorf(first.Span, "unexpected %s at the start of a statement", quote(first)))
		return p.bad()
	}

	// Relationships come first so that a resource may share a name with a
	// statement keyword (`queue connects to db`). A canonical verb is enough,
	// so a half-typed `api connects to` still parses; a synonym needs a target
	// after it before we assume the line is a relationship.
	if _, n, canonical := p.verbAt(1); n > 0 && (canonical || isValue(p.peek(1+n))) {
		return p.relate()
	}

	switch {
	case first.Is("use"):
		return p.use()
	case first.Is("on"):
		return p.on()
	case first.Is("create"):
		p.report(diag.Errorf(first.Span, "statements don't start with `create`").
			Suggest("remove `create`: a Clowd file describes what exists"))
		p.next()
		if p.atEOL() {
			return p.bad()
		}
		return p.statement()
	}

	if k, n := match(p, 0, kinds); n > 0 {
		return p.declare(k, n)
	}
	if k, ok := guessKind(first.Text); ok {
		p.report(diag.Errorf(first.Span, "unknown resource kind %s", quote(first)).
			Suggest("did you mean `%s`?", k))
		return p.declare(k, 1)
	}

	// Three or more words looks like a relationship with an unknown verb.
	if second := p.peek(1); second.Kind == token.Word && isValue(p.peek(2)) {
		d := diag.Errorf(second.Span, "unknown relationship %s", quote(second))
		if v, ok := guessVerb(second.Text); ok {
			d = d.Suggest("did you mean `%s`?", v)
		} else {
			d = d.Suggest("relationships are %s", verbList())
		}
		p.report(d)
		return p.bad()
	}

	p.report(diag.Errorf(first.Span, "unknown statement %s", quote(first)).
		Suggest("statements start with `use`, `on`, `project`, `network` or a resource kind (%s), or relate two names: `api connects to db`", kindList()))
	return p.bad()
}

func verbList() string {
	var s []string
	for _, v := range verbs {
		s = append(s, "`"+strings.Join(v.words, " ")+"`")
	}
	return strings.Join(s, ", ")
}

func kindList() string {
	var s []string
	for _, k := range kinds {
		if k.value == ast.LoadBalancer && len(k.words) == 1 {
			continue // `lb` is an alias; don't advertise both
		}
		if !k.value.IsResource() {
			continue
		}
		s = append(s, "`"+strings.Join(k.words, " ")+"`")
	}
	return strings.Join(s, ", ")
}

// bad skips the current line and any block after it.
func (p *parser) bad() ast.Stmt {
	sp := p.lineSpan(p.i)
	p.skipLine()
	if p.tok().Kind == token.Indent {
		p.skipBlock()
	}
	return &ast.Bad{Pos: sp}
}

// name parses a resource or cloud name. what describes the name for the
// error message ("a name after `service`"); example, if not empty, is shown
// as help.
func (p *parser) name(what, example string) ast.Ident {
	t := p.tok()
	switch t.Kind {
	case token.Word:
		p.next()
		return ast.Ident{Name: t.Text, Pos: t.Span}
	case token.String:
		p.next()
		p.report(diag.Errorf(t.Span, "names can't be quoted").Suggest("write `%s` without quotes", t.Text))
		return ast.Ident{Name: t.Text, Pos: t.Span}
	}
	d := diag.Errorf(p.here(), "expected %s", what)
	if example != "" {
		d = d.Suggest("for example `%s`", example)
	}
	p.report(d)
	return ast.Ident{Pos: p.here()}
}

func value(t token.Token) ast.Value {
	return ast.Value{Text: t.Text, Quoted: t.Kind == token.String, Pos: t.Span}
}

// use parses `use <cloud> [in <region>]`.
func (p *parser) use() ast.Stmt {
	start := p.next().Span
	s := &ast.Use{}
	s.Cloud = p.name("a cloud after `use`", "use aws in eu-west-2")
	switch {
	case p.tok().Is("in"):
		p.next()
		if isValue(p.tok()) {
			r := value(p.next())
			s.Region = &r
		} else {
			p.report(diag.Errorf(p.here(), "expected a region after `in`"))
		}
	case isValue(p.tok()) && p.peek(1).Kind != token.Word && p.peek(1).Kind != token.String:
		// `use aws eu-west-2`: almost certainly a region missing its `in`.
		r := value(p.next())
		p.report(diag.Errorf(r.Pos, "expected `in` before the region").
			Suggest("write `use %s in %s`", s.Cloud.Name, r.Text))
		s.Region = &r
	}
	s.Pos = p.endStmt(start)
	p.finishLine()
	p.noBlock("`use` lines don't take a block")
	return s
}

// declare parses `<kind> <name> [is <traits>]` and its property block. The
// kind is the next n tokens.
func (p *parser) declare(kind ast.Kind, n int) ast.Stmt {
	d := &ast.Declare{Kind: kind}
	d.KindPos = p.consume(n)

	if !kind.Named() {
		// A word after `network` is a trait missing its `is`, reported
		// below.
		d.Name = ast.Ident{Pos: p.here()}
	} else if p.tok().Is("is") && isValue(p.peek(1)) {
		// `service is public`: the name is missing, not called `is`.
		p.report(diag.Errorf(p.here(), "expected a name after `%s`", kind).
			Suggest("for example `%s api is ...`", kind))
		d.Name = ast.Ident{Pos: p.here()}
	} else {
		d.Name = p.name(fmt.Sprintf("a name after `%s`", kind), fmt.Sprintf("%s api", kind))
	}

	switch {
	case p.tok().Is("is"):
		d.Traits = p.traits(p.next())
	case !p.atEOL():
		// `bucket uploads private`: parse the words as traits so later
		// stages see what was meant.
		head, after := fmt.Sprintf("%s %s", kind, d.Name.Name), "the name"
		if !kind.Named() {
			head, after = kind.String(), "`"+kind.String()+"`"
		}
		p.report(diag.Errorf(p.lineSpan(p.i), "unexpected %s after %s", quote(p.tok()), after).
			Suggest("traits go after `is`: `%s is %s`", head, p.tok().Text))
		d.Traits = p.traits(token.Token{})
	}
	d.Pos = p.endStmt(d.KindPos)
	p.finishLine()
	d.Props = p.block(false)
	return d
}

// block parses the indented block of properties that follows a line, if
// there is one. raw is set inside `on` blocks, whose settings may be comma
// lists; it carries down into nested blocks.
func (p *parser) block(raw bool) []ast.Property {
	if p.tok().Kind != token.Indent {
		return nil
	}
	p.next()
	var props []ast.Property
	for {
		switch p.tok().Kind {
		case token.Dedent:
			p.next()
			return props
		case token.EOF:
			return props
		case token.Indent:
			// property consumes the block after its line, even a broken
			// one, so this is only reachable on malformed layout.
			p.report(diag.Errorf(p.lineSpan(p.i+1), "unexpected indentation"))
			p.skipBlock()
		default:
			if prop, ok := p.property(raw); ok {
				props = append(props, prop)
			}
		}
	}
}

// traits parses a trait list: words separated by `,` or `and`. after is the
// `is` token, or the zero Token if `is` was missing.
func (p *parser) traits(after token.Token) []ast.Trait {
	var (
		traits []ast.Trait
		cur    ast.Trait
		prev   = "`is`"
	)
	if after.Kind != token.Word {
		prev = "the name"
	}
	for !p.atEOL() {
		t := p.tok()
		if t.Kind == token.Comma || t.Is("and") {
			if len(cur.Words) == 0 {
				p.report(diag.Errorf(t.Span, "expected a trait before %s", quote(t)))
			} else {
				traits = append(traits, cur)
				cur = ast.Trait{}
			}
			prev = quote(t)
			p.next()
			continue
		}
		p.next()
		if len(cur.Words) == 0 {
			cur.Pos = t.Span
		}
		cur.Words = append(cur.Words, value(t))
		cur.Pos = cur.Pos.Join(t.Span)
	}
	if len(cur.Words) == 0 {
		p.report(diag.Errorf(p.here(), "expected a trait after %s", prev))
	} else {
		traits = append(traits, cur)
	}
	return traits
}

// property parses one `<key> <value...>` line and the block under it.
// Within raw (`on`) blocks commas separate list items; elsewhere they are
// an error. An `on` property starts a raw block.
func (p *parser) property(raw bool) (ast.Property, bool) {
	t := p.tok()
	switch t.Kind {
	case token.Word:
	case token.String:
		p.report(diag.Errorf(t.Span, "property names can't be quoted"))
	default:
		p.report(diag.Errorf(t.Span, "expected a property name, got %s", quote(t)))
		p.bad()
		return ast.Property{}, false
	}
	p.next()
	prop := ast.Property{Key: value(t), Pos: t.Span}
	for !p.atEOL() {
		v := p.next()
		if v.Kind == token.Comma {
			if raw {
				prop.Commas = append(prop.Commas, v.Span)
				continue
			}
			p.report(diag.Errorf(v.Span, "unexpected `,` in the value of `%s`", t.Text).
				Suggest("quote values that contain commas: `\"a, b\"`"))
			continue
		}
		prop.Values = append(prop.Values, value(v))
	}
	prop.Pos = p.endStmt(prop.Pos)
	p.skipLine()
	prop.Block = p.block(raw || (t.Kind == token.Word && t.Is("on")))
	return prop, true
}

// relate parses `<name> <verb> <name>`; statement has already checked that a
// verb follows the first word.
func (p *parser) relate() ast.Stmt {
	from := p.next()
	s := &ast.Relate{From: ast.Ident{Name: from.Text, Pos: from.Span}}

	var n int
	var canonical bool
	s.Verb, n, canonical = p.verbAt(0)
	var written []string
	for _, t := range p.toks[p.i : p.i+n] {
		written = append(written, t.Text)
	}
	s.VerbPos = p.consume(n)
	if !canonical {
		p.report(diag.Errorf(s.VerbPos, "unknown relationship `%s`", strings.Join(written, " ")).
			Suggest("did you mean `%s`?", s.Verb))
	}

	s.To = p.name(fmt.Sprintf("a name after `%s`", s.Verb), "")

	if (p.tok().Kind == token.Comma || p.tok().Is("and")) && p.peek(1).Kind == token.Word {
		next := p.peek(1).Text
		p.report(diag.Errorf(p.lineSpan(p.i), "a relationship has one target").
			Suggest("put each target on its own line: `%s %s %s`", s.From.Name, s.Verb, next))
		p.skipLine()
	} else {
		p.finishLine()
	}
	s.Pos = diag.Span{Start: from.Span.Start, End: max(s.VerbPos.End, s.To.Pos.End)}
	p.noBlock("relationships don't take a block")
	return s
}

// on parses a top-level `on <cloud>` block of program-wide settings. Its
// settings are raw: they may be comma lists. check reports the old
// `<kind> <name> sets <key> to <value>` form, which parses as a setting.
func (p *parser) on() ast.Stmt {
	start := p.next().Span
	s := &ast.On{}
	s.Cloud = p.name("a cloud after `on`", "on aws")
	s.Pos = p.endStmt(start)
	p.finishLine()

	if p.tok().Kind != token.Indent {
		p.report(diag.Errorf(s.Pos, "expected an indented block of settings after `on %s`", s.Cloud.Name).
			Suggest("settings for one resource go inside its declaration, under `  on %s`", strings.ToLower(s.Cloud.Name)))
		return s
	}
	s.Settings = p.block(true)
	return s
}
