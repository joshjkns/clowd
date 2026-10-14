package diag

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// Span is a half-open byte range [Start, End) in a File's text.
// A zero-width span (Start == End) marks a position, e.g. "expected a name here".
type Span struct {
	Start, End int
}

// Len returns the number of bytes covered by the span.
func (s Span) Len() int { return s.End - s.Start }

// Join returns the smallest span covering both s and o.
func (s Span) Join(o Span) Span {
	return Span{Start: min(s.Start, o.Start), End: max(s.End, o.End)}
}

// Position is a human-facing location in a file.
// Line and Column are 1-based; Column counts runes, not bytes, so it matches
// what an editor shows for non-ASCII text.
type Position struct {
	Line, Column int
}

// File is a named source text with a line index for turning byte offsets
// into positions.
type File struct {
	Name string
	Text string

	// lineStarts[i] is the byte offset of the first byte of line i+1.
	lineStarts []int
}

// NewFile indexes text for position lookups.
func NewFile(name, text string) *File {
	starts := []int{0}
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &File{Name: name, Text: text, lineStarts: starts}
}

// Position converts a byte offset to a line and column. Offsets outside the
// text are clamped, so a span at EOF still renders.
func (f *File) Position(offset int) Position {
	offset = max(0, min(offset, len(f.Text)))
	// Index of the last line start <= offset.
	i := sort.Search(len(f.lineStarts), func(i int) bool { return f.lineStarts[i] > offset }) - 1
	col := utf8.RuneCountInString(f.Text[f.lineStarts[i]:offset]) + 1
	return Position{Line: i + 1, Column: col}
}

// LineStart returns the byte offset of the start of the 1-based line n,
// clamped to the lines in the file.
func (f *File) LineStart(n int) int {
	return f.lineStarts[max(0, min(n-1, len(f.lineStarts)-1))]
}

// Line returns the text of the 1-based line n without its line terminator,
// or "" if n is out of range.
func (f *File) Line(n int) string {
	if n < 1 || n > len(f.lineStarts) {
		return ""
	}
	start := f.lineStarts[n-1]
	end := len(f.Text)
	if n < len(f.lineStarts) {
		end = f.lineStarts[n] - 1
	}
	return strings.TrimSuffix(f.Text[start:end], "\r")
}
