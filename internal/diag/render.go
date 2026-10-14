package diag

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Render writes d in a compiler-style format with the offending source line
// and a caret underline:
//
//	error: unknown verb `links`
//	  --> app.clowd:17:5
//	   |
//	17 | api links to db
//	   |     ^^^^^
//	   = help: did you mean `connects to`?
//
// Spans that run past the end of their first line are underlined to the end
// of that line.
func Render(w io.Writer, f *File, d Diagnostic) error {
	pos := f.Position(d.Span.Start)
	line := f.Line(pos.Line)
	gutter := strings.Repeat(" ", len(strconv.Itoa(pos.Line)))

	// Slice the line in bytes, then count runes so the underline matches what
	// the terminal shows.
	lineStart := f.LineStart(pos.Line)
	startByte := min(max(d.Span.Start-lineStart, 0), len(line))
	endByte := min(max(d.Span.End-lineStart, startByte), len(line))
	carets := max(1, utf8.RuneCountInString(line[startByte:endByte]))

	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\n", d.Severity, d.Message)
	fmt.Fprintf(&b, "%s--> %s:%d:%d\n", gutter, f.Name, pos.Line, pos.Column)
	fmt.Fprintf(&b, "%s |\n", gutter)
	fmt.Fprintf(&b, "%d | %s\n", pos.Line, line)
	fmt.Fprintf(&b, "%s | %s%s\n", gutter, padding(line[:startByte]), strings.Repeat("^", carets))
	if d.Suggestion != "" {
		fmt.Fprintf(&b, "%s = help: %s\n", gutter, d.Suggestion)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// RenderAll renders each diagnostic in l, separated by blank lines.
func RenderAll(w io.Writer, f *File, l List) error {
	for i, d := range l {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if err := Render(w, f, d); err != nil {
			return err
		}
	}
	return nil
}

// padding turns the text before the caret into whitespace of the same visual
// width, keeping tabs as tabs so the caret lines up however the terminal
// expands them.
func padding(prefix string) string {
	var b strings.Builder
	for _, r := range prefix {
		if r == '\t' {
			b.WriteRune('\t')
		} else {
			b.WriteRune(' ')
		}
	}
	return b.String()
}
