package lsp

import (
	"regexp"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// Semantic tokens colour a file from the parse, so every editor that
// speaks LSP highlights Clowd the same way, and as the compiler reads it:
// `queue connects to db` is a relationship, not a queue named `connects`.
// The types are standard ones, so themes already colour them; they match
// the VS Code grammar's scopes and editors/vim/syntax/clowd.vim.

// tokenTypes is the legend; a token's type is its index.
var tokenTypes = []string{
	"keyword",    // use, in, is, and, on, and words inside values (to, with tls for)
	"struct",     // kinds: service, load balancer
	"variable",   // resource names
	"operator",   // verbs: connects to, reads and writes
	"enumMember", // clouds
	"type",       // traits: postgres, private, fifo
	"property",   // property keys: image, health check
	"decorator",  // raw provider arguments under `on <cloud>`
	"string",     // values
	"number",     // numbers, sizes and durations
	"comment",
}

const (
	tokKeyword = iota
	tokKind
	tokName
	tokVerb
	tokCloud
	tokTrait
	tokProperty
	tokRaw
	tokString
	tokNumber
	tokComment
)

// tokenModifiers is the modifier legend; `declaration` marks the name in
// a resource's own declaration, as against a reference to it.
var tokenModifiers = []string{"declaration"}

const modDeclaration = 1 << 0

// numberRE matches what the checker reads as a number: integers and
// decimals, optionally with a size or duration unit (`512mb`, `14d`).
var numberRE = regexp.MustCompile(`^\d+(\.\d+)?(s|m|h|d|mb|gb)?$`)

// valueKeywords are the keyword words that can appear among a property's
// values: `scale 2 to 10`, `listen on 443 with tls for`, `retry 3 times`.
var valueKeywords = map[string]bool{"to": true, "on": true, "with": true, "tls": true, "for": true, "times": true, "days": true, "off": true}

type semToken struct {
	span diag.Span
	typ  int
	mods int
}

// semanticTokens returns doc's tokens in the protocol's relative encoding.
func (s *server) semanticTokens(doc *document) semanticTokensResult {
	b := tokenBuilder{text: doc.file.Text}
	for _, st := range doc.analysis.AST.Stmts {
		b.stmt(st)
	}
	for _, c := range doc.analysis.AST.Comments {
		b.add(c.Span, tokComment, 0)
	}
	return semanticTokensResult{Data: b.encode(doc.file, s.enc)}
}

type tokenBuilder struct {
	text string
	toks []semToken
}

func (b *tokenBuilder) add(sp diag.Span, typ, mods int) {
	if sp.Len() > 0 {
		b.toks = append(b.toks, semToken{span: sp, typ: typ, mods: mods})
	}
}

// gap colours the words of the source between from and to that are one
// of words. The AST doesn't keep these joining words (`in`, `is`, `and`).
func (b *tokenBuilder) gap(from, to int, words ...string) {
	if from < 0 || to > len(b.text) || from >= to {
		return
	}
	sep := func(c byte) bool { return c == ' ' || c == '\t' || c == ',' || c == '\r' || c == '\n' }
	for i := from; i < to; {
		if sep(b.text[i]) {
			i++
			continue
		}
		j := i
		for j < to && !sep(b.text[j]) {
			j++
		}
		if slices.Contains(words, strings.ToLower(b.text[i:j])) {
			b.add(diag.Span{Start: i, End: j}, tokKeyword, 0)
		}
		i = j
	}
}

func (b *tokenBuilder) stmt(st ast.Stmt) {
	switch st := st.(type) {
	case *ast.Use:
		b.add(diag.Span{Start: st.Pos.Start, End: st.Pos.Start + len("use")}, tokKeyword, 0)
		b.add(st.Cloud.Pos, tokCloud, 0)
		if st.Region != nil {
			b.gap(st.Cloud.Pos.End, st.Region.Pos.Start, "in")
			b.add(st.Region.Pos, tokString, 0)
		}
	case *ast.Declare:
		b.add(st.KindPos, tokKind, 0)
		end := st.KindPos.End
		if !st.Name.Missing() {
			b.add(st.Name.Pos, tokName, modDeclaration)
			end = st.Name.Pos.End
		}
		for _, t := range st.Traits {
			b.gap(end, t.Pos.Start, "is", "and")
			for _, w := range t.Words {
				if !w.Quoted && numberRE.MatchString(w.Text) {
					b.add(w.Pos, tokNumber, 0)
				} else {
					b.add(w.Pos, tokTrait, 0)
				}
			}
			end = t.Pos.End
		}
		b.props(st.Kind, st.Props)
	case *ast.Relate:
		b.add(st.From.Pos, tokName, 0)
		if st.Verb != ast.BadVerb {
			b.add(st.VerbPos, tokVerb, 0)
		}
		b.add(st.To.Pos, tokName, 0)
	case *ast.On:
		b.add(diag.Span{Start: st.Pos.Start, End: st.Pos.Start + len("on")}, tokKeyword, 0)
		b.add(st.Cloud.Pos, tokCloud, 0)
		b.raw(st.Settings)
	}
}

// props colours a declaration's properties. A property's key may be a
// phrase (`health check`), whose later words the parser keeps as values.
func (b *tokenBuilder) props(k ast.Kind, ps []ast.Property) {
	for _, p := range ps {
		if isWord(p.Key, "on") {
			b.add(p.Key.Pos, tokKeyword, 0)
			if len(p.Values) > 0 {
				b.add(p.Values[0].Pos, tokCloud, 0)
			}
			b.raw(p.Block)
			continue
		}
		b.add(p.Key.Pos, tokProperty, 0)
		vs := p.Values
		for _, words := range phraseRests(k, strings.ToLower(p.Key.Text)) {
			if len(vs) >= len(words) && slices.EqualFunc(vs[:len(words)], words, isWord) {
				for _, v := range vs[:len(words)] {
					b.add(v.Pos, tokProperty, 0)
				}
				vs = vs[len(words):]
				break
			}
		}
		for _, v := range vs {
			b.value(v)
		}
	}
}

// phraseRests returns the words after the first of k's phrase properties
// keyed by key: `check` for `health check`.
func phraseRests(k ast.Kind, key string) [][]string {
	var out [][]string
	for _, p := range vocabulary(k).Props {
		if p.Key == key && strings.Contains(p.Name, " ") {
			out = append(out, strings.Fields(p.Name)[1:])
		}
	}
	return out
}

func (b *tokenBuilder) value(v ast.Value) {
	switch {
	case v.Quoted:
		b.add(v.Pos, tokString, 0)
	case valueKeywords[strings.ToLower(v.Text)]:
		b.add(v.Pos, tokKeyword, 0)
	case numberRE.MatchString(v.Text):
		b.add(v.Pos, tokNumber, 0)
	default:
		b.add(v.Pos, tokString, 0)
	}
}

// raw colours provider settings: argument names as written, values as
// strings or numbers, nested blocks alike.
func (b *tokenBuilder) raw(ps []ast.Property) {
	for _, p := range ps {
		b.add(p.Key.Pos, tokRaw, 0)
		for _, v := range p.Values {
			if !v.Quoted && numberRE.MatchString(v.Text) {
				b.add(v.Pos, tokNumber, 0)
			} else {
				b.add(v.Pos, tokString, 0)
			}
		}
		b.raw(p.Block)
	}
}

// encode sorts the tokens, drops any that overlap an earlier one, and
// writes each as five integers relative to the previous token: line delta,
// start delta, length, type and modifiers.
func (b *tokenBuilder) encode(f *diag.File, enc encoding) []uint32 {
	slices.SortStableFunc(b.toks, func(x, y semToken) int { return x.span.Start - y.span.Start })
	data := []uint32{}
	var prevLine, prevChar, lastEnd int
	for _, t := range b.toks {
		if t.span.Start < lastEnd {
			continue
		}
		start, end := toPosition(f, t.span.Start, enc), toPosition(f, t.span.End, enc)
		if end.Line != start.Line {
			continue // tokens may not span lines
		}
		deltaChar := start.Character
		if start.Line == prevLine {
			deltaChar -= prevChar
		}
		data = append(data, uint32(start.Line-prevLine), uint32(deltaChar), uint32(end.Character-start.Character), uint32(t.typ), uint32(t.mods))
		prevLine, prevChar, lastEnd = start.Line, start.Character, t.span.End
	}
	return data
}
