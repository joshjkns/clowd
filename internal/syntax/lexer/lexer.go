// Package lexer turns Clowd source into tokens, including the NEWLINE, INDENT
// and DEDENT tokens that give the language its indentation-based blocks.
//
// The lexer never fails: problems are reported as diagnostics and lexing
// continues, so the parser (and the LSP) always get a usable token stream.
package lexer

import (
	"strings"
	"unicode/utf8"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/syntax/token"
)

// Result is the output of Lex.
type Result struct {
	// Tokens always ends with a single EOF token, preceded by enough DEDENTs
	// to close every open block.
	Tokens []token.Token
	// Comments are kept out of Tokens so the parser never has to skip them;
	// the formatter uses them to put comments back.
	Comments []token.Token
	Diags    diag.List
}

// Lex tokenises f.
func Lex(f *diag.File) Result {
	l := &lexer{src: f.Text, indents: []int{0}}
	if strings.HasPrefix(l.src, bom) {
		l.pos = len(bom)
	}
	for l.pos < len(l.src) {
		l.line()
	}
	end := diag.Span{Start: len(l.src), End: len(l.src)}
	for len(l.indents) > 1 {
		l.indents = l.indents[:len(l.indents)-1]
		l.emit(token.Dedent, end, "")
	}
	l.emit(token.EOF, end, "")
	return l.res
}

// bom is the UTF-8 byte order mark, which some Windows editors prepend.
const bom = "\xef\xbb\xbf"

type lexer struct {
	src string
	pos int
	// indents is the stack of open block indentation widths; it always
	// starts with 0 for the top level.
	indents []int
	res     Result
}

func (l *lexer) emit(k token.Kind, sp diag.Span, text string) {
	l.res.Tokens = append(l.res.Tokens, token.Token{Kind: k, Span: sp, Text: text})
}

func (l *lexer) report(d diag.Diagnostic) {
	l.res.Diags.Add(d)
}

// eolLen returns the length of the line terminator at pos: 1 for "\n",
// 2 for "\r\n", 0 if pos is not at one. EOF counts as a zero-length end of line.
func (l *lexer) eolLen() (n int, ok bool) {
	switch {
	case l.pos >= len(l.src):
		return 0, true
	case l.src[l.pos] == '\n':
		return 1, true
	case strings.HasPrefix(l.src[l.pos:], "\r\n"):
		return 2, true
	}
	return 0, false
}

func (l *lexer) atEOL() bool {
	_, ok := l.eolLen()
	return ok
}

// line lexes one physical line, including its terminator.
func (l *lexer) line() {
	start := l.pos
	width := 0
	tabReported := false
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		if l.src[l.pos] == '\t' && !tabReported {
			l.report(diag.Errorf(diag.Span{Start: l.pos, End: l.pos + 1}, "tabs are not allowed in indentation").
				Suggest("indent with spaces"))
			tabReported = true
		}
		// A tab counts as one column, so a file indented consistently with
		// tabs still gets the right block structure after the error.
		width++
		l.pos++
	}

	// Blank and comment-only lines don't affect layout.
	if l.atEOL() || l.src[l.pos] == '#' {
		if !l.atEOL() {
			l.comment()
		}
		l.skipEOL()
		return
	}

	l.indent(width, diag.Span{Start: start, End: l.pos})

	for !l.atEOL() {
		switch c := l.src[l.pos]; c {
		case ' ', '\t', '\r':
			l.pos++
		case '#':
			// Words swallow a `#` that isn't preceded by whitespace, so any `#`
			// seen here starts a comment.
			l.comment()
		case '"':
			l.string()
		case ',':
			l.emit(token.Comma, diag.Span{Start: l.pos, End: l.pos + 1}, ",")
			l.pos++
		default:
			l.word()
		}
	}

	n, _ := l.eolLen()
	l.emit(token.Newline, diag.Span{Start: l.pos, End: l.pos + n}, "")
	l.pos += n
}

func (l *lexer) skipEOL() {
	n, _ := l.eolLen()
	l.pos += n
}

// indent emits INDENT/DEDENT tokens for a line whose indentation is width
// columns, spanning sp.
func (l *lexer) indent(width int, sp diag.Span) {
	top := l.indents[len(l.indents)-1]
	if width > top {
		l.indents = append(l.indents, width)
		l.emit(token.Indent, sp, "")
		return
	}
	at := diag.Span{Start: sp.End, End: sp.End}
	for width < top {
		outer := l.indents[len(l.indents)-2]
		if width > outer {
			// Between two levels. Stay in the inner block: the line most
			// likely belongs to it (a mistyped property), and treating it as
			// top-level would cascade into confusing errors.
			l.report(diag.Errorf(sp, "indentation of %d spaces doesn't match the block (%d) or the level outside it (%d)", width, top, outer).
				Suggest("use %d spaces to stay in the block, or %d to leave it", top, outer))
			return
		}
		l.indents = l.indents[:len(l.indents)-1]
		l.emit(token.Dedent, at, "")
		top = outer
	}
}

func (l *lexer) comment() {
	start := l.pos
	for !l.atEOL() {
		l.pos++
	}
	l.res.Comments = append(l.res.Comments, token.Token{
		Kind: token.Comment,
		Span: diag.Span{Start: start, End: l.pos},
		Text: l.src[start:l.pos],
	})
}

// word lexes a bare run of non-whitespace. Trailing commas are split off so
// `16, small` and `16,` both give a separate COMMA, while `a,b` stays one
// word (a value may contain commas, it just can't end with one).
func (l *lexer) word() {
	start := l.pos
	for l.pos < len(l.src) && !isSpace(l.src[l.pos]) {
		l.pos++
	}
	text := strings.TrimRight(l.src[start:l.pos], ",")
	end := start + len(text)
	l.emit(token.Word, diag.Span{Start: start, End: end}, text)
	for i := end; i < l.pos; i++ {
		l.emit(token.Comma, diag.Span{Start: i, End: i + 1}, ",")
	}
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func (l *lexer) string() {
	start := l.pos
	l.pos++ // opening quote
	var b strings.Builder
	for {
		if l.atEOL() {
			l.report(diag.Errorf(diag.Span{Start: start, End: l.pos}, "unterminated string").
				Suggest("add a closing `\"` before the end of the line"))
			break
		}
		c := l.src[l.pos]
		if c == '"' {
			l.pos++
			break
		}
		if c != '\\' {
			b.WriteByte(c)
			l.pos++
			continue
		}
		// Escape sequence.
		esc := l.pos
		l.pos++
		if l.atEOL() {
			continue // reported as unterminated on the next iteration
		}
		r, size := utf8.DecodeRuneInString(l.src[l.pos:])
		l.pos += size
		switch r {
		case '"', '\\':
			b.WriteRune(r)
		default:
			l.report(diag.Errorf(diag.Span{Start: esc, End: l.pos}, "unknown escape `\\%c`", r).
				Suggest("the only escapes are `\\\"` and `\\\\`; write `\\\\` for a backslash"))
			b.WriteByte('\\')
			b.WriteRune(r)
		}
	}
	l.emit(token.String, diag.Span{Start: start, End: l.pos}, b.String())

	if !l.atEOL() && !isSpace(l.src[l.pos]) && l.src[l.pos] != ',' {
		l.report(diag.Errorf(diag.Span{Start: l.pos, End: l.pos}, "expected a space after the closing quote"))
	}
}
