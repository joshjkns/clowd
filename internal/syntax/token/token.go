// Package token defines the lexical tokens of Clowd.
//
// There are no keyword tokens: keywords are words that the parser recognises
// by position (see decision D8).
package token

import (
	"fmt"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
)

// Kind is the type of a token.
type Kind int

const (
	// EOF ends every token stream.
	EOF Kind = iota
	// Word is a run of non-whitespace characters: a keyword, name or bare value.
	Word
	// String is a quoted string; Token.Text holds the unescaped value.
	String
	// Comma separates traits.
	Comma
	// Newline ends a non-blank line.
	Newline
	// Indent opens a block: the line is indented more than the previous one.
	Indent
	// Dedent closes a block.
	Dedent
	// Comment runs from `#` to the end of the line. Comments are not part of
	// the parser's token stream; the lexer returns them separately.
	Comment
)

var kindNames = [...]string{
	EOF:     "end of file",
	Word:    "word",
	String:  "string",
	Comma:   "`,`",
	Newline: "end of line",
	Indent:  "indent",
	Dedent:  "dedent",
	Comment: "comment",
}

// String returns a description suitable for error messages ("expected a word").
func (k Kind) String() string {
	if k >= 0 && int(k) < len(kindNames) {
		return kindNames[k]
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// Token is one lexical token.
type Token struct {
	Kind Kind
	Span diag.Span
	// Text is the word as written, the unescaped value of a string, or the
	// comment including its `#`. Layout tokens have no text.
	Text string
}

// Is reports whether t is a word equal to w, ignoring case.
func (t Token) Is(w string) bool {
	return t.Kind == Word && strings.EqualFold(t.Text, w)
}

func (t Token) String() string {
	switch t.Kind {
	case Word, String, Comment:
		return fmt.Sprintf("%s %q", t.Kind, t.Text)
	}
	return t.Kind.String()
}
