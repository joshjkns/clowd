package lsp

import (
	"unicode/utf16"
	"unicode/utf8"

	"github.com/joshjkns/clowd/internal/diag"
)

// encoding is how a position's character offset counts: LSP defaults to
// UTF-16 code units, and clients may offer UTF-8 bytes instead.
type encoding string

const (
	utf8Encoding  encoding = "utf-8"
	utf16Encoding encoding = "utf-16"
)

// units is how many units of e the rune r takes.
func (e encoding) units(r rune) int {
	if e == utf8Encoding {
		return utf8.RuneLen(r)
	}
	return utf16.RuneLen(r)
}

// lineEnd returns the offset of the end of the line starting at start,
// before its terminator.
func lineEnd(text string, start int) int {
	end := start
	for end < len(text) && text[end] != '\n' {
		end++
	}
	if end > start && text[end-1] == '\r' {
		end--
	}
	return end
}

// toPosition converts a byte offset in f to an LSP position.
func toPosition(f *diag.File, offset int, enc encoding) position {
	offset = max(0, min(offset, len(f.Text)))
	line := f.Position(offset).Line
	start := f.LineStart(line)
	char := 0
	for _, r := range f.Text[start:min(offset, lineEnd(f.Text, start))] {
		char += enc.units(r)
	}
	return position{Line: line - 1, Character: char}
}

func toRange(f *diag.File, sp diag.Span, enc encoding) lspRange {
	return lspRange{Start: toPosition(f, sp.Start, enc), End: toPosition(f, sp.End, enc)}
}

// toOffset converts an LSP position to a byte offset in f. Positions past
// the end of a line or of the file are clamped, as the protocol requires.
func toOffset(f *diag.File, p position, enc encoding) int {
	if p.Line < 0 {
		return 0
	}
	if p.Line+1 > f.Position(len(f.Text)).Line {
		return len(f.Text)
	}
	start := f.LineStart(p.Line + 1)
	end := lineEnd(f.Text, start)
	char := 0
	for i, r := range f.Text[start:end] {
		if char >= p.Character {
			return start + i
		}
		char += enc.units(r)
	}
	return end
}
