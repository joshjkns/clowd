package diag

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/joshjkns/clowd/internal/term"
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
	return RenderStyled(w, f, d, term.Style{})
}

// RenderStyled is Render with colour: the severity and carets in the
// severity's colour, the message bold, the gutter and help highlighted.
func RenderStyled(w io.Writer, f *File, d Diagnostic, st term.Style) error {
	pos := f.Position(d.Span.Start)
	line := f.Line(pos.Line)
	gutter := strings.Repeat(" ", len(strconv.Itoa(pos.Line)))

	// Slice the line in bytes, then count runes so the underline matches what
	// the terminal shows.
	lineStart := f.LineStart(pos.Line)
	startByte := min(max(d.Span.Start-lineStart, 0), len(line))
	endByte := min(max(d.Span.End-lineStart, startByte), len(line))
	carets := max(1, utf8.RuneCountInString(line[startByte:endByte]))

	severity := st.Error
	switch d.Severity {
	case Warning:
		severity = st.Warning
	case Info:
		severity = st.Info
	}
	bar := st.Frame("|")

	var b strings.Builder
	fmt.Fprintf(&b, "%s%s %s\n", severity(d.Severity.String()), st.Bold(":"), st.Bold(d.Message))
	fmt.Fprintf(&b, "%s%s %s:%d:%d\n", gutter, st.Frame("-->"), f.Name, pos.Line, pos.Column)
	fmt.Fprintf(&b, "%s %s\n", gutter, bar)
	fmt.Fprintf(&b, "%s %s %s\n", st.Frame(strconv.Itoa(pos.Line)), bar, line)
	fmt.Fprintf(&b, "%s %s %s%s\n", gutter, bar, padding(line[:startByte]), severity(strings.Repeat("^", carets)))
	if d.Suggestion != "" {
		fmt.Fprintf(&b, "%s %s %s %s\n", gutter, st.Frame("="), st.Help("help:"), d.Suggestion)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// RenderAll renders each diagnostic in l, separated by blank lines.
func RenderAll(w io.Writer, f *File, l List) error {
	return RenderAllStyled(w, f, l, term.Style{})
}

// RenderAllStyled is RenderAll with colour.
func RenderAllStyled(w io.Writer, f *File, l List, st term.Style) error {
	for i, d := range l {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if err := RenderStyled(w, f, d, st); err != nil {
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
