package lexer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/golden"
	"github.com/joshjkns/clowd/internal/syntax/token"
)

// dump renders a lex result in a stable, reviewable text form.
func dump(f *diag.File, r Result) string {
	var b strings.Builder
	span := func(sp diag.Span) string {
		s, e := f.Position(sp.Start), f.Position(sp.End)
		return fmt.Sprintf("%d:%d-%d:%d", s.Line, s.Column, e.Line, e.Column)
	}
	b.WriteString("tokens:\n")
	for _, t := range r.Tokens {
		fmt.Fprintf(&b, "  %-12s %s\n", span(t.Span), t)
	}
	if len(r.Comments) > 0 {
		b.WriteString("comments:\n")
		for _, c := range r.Comments {
			fmt.Fprintf(&b, "  %-12s %s\n", span(c.Span), c)
		}
	}
	if len(r.Diags) > 0 {
		b.WriteString("diagnostics:\n")
		if err := diag.RenderAll(&b, f, r.Diags); err != nil {
			panic(err) // strings.Builder never fails
		}
	}
	return b.String()
}

func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.clowd")
	if err != nil {
		t.Fatal(err)
	}
	examples, err := filepath.Glob("../../../examples/*.clowd")
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) == 0 {
		t.Fatal("no examples found")
	}
	for _, in := range append(inputs, examples...) {
		want := strings.TrimSuffix(in, ".clowd") + ".golden"
		isExample := strings.Contains(in, "examples")
		if isExample {
			want = filepath.Join("testdata", "examples", strings.TrimSuffix(filepath.Base(in), ".clowd")+".golden")
		}
		t.Run(filepath.Base(in), func(t *testing.T) {
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			f := diag.NewFile(filepath.Base(in), string(src))
			r := Lex(f)
			if isExample && len(r.Diags) > 0 {
				t.Errorf("examples must lex cleanly, got %d diagnostics", len(r.Diags))
			}
			golden.Check(t, want, dump(f, r))
		})
	}
}

// kinds returns the token kinds of r as a compact string, e.g. "word word newline eof".
func kinds(r Result) string {
	var ks []string
	for _, t := range r.Tokens {
		switch t.Kind {
		case token.Word, token.String:
			ks = append(ks, t.Text)
		default:
			ks = append(ks, strings.ToUpper(strings.Trim(t.Kind.String(), "`")))
		}
	}
	return strings.Join(ks, " ")
}

func TestLex(t *testing.T) {
	tests := []struct {
		name, src, want string
		diags           int
	}{
		{"empty", "", "END OF FILE", 0},
		{"only comments and blanks", "# a\n\n   # b\n", "END OF FILE", 0},
		{"no trailing newline", "use aws", "use aws END OF LINE END OF FILE", 0},
		{"crlf", "service api\r\n  port 1\r\n", "service api END OF LINE INDENT port 1 END OF LINE DEDENT END OF FILE", 0},
		{"byte order mark", "\xef\xbb\xbfuse aws\n", "use aws END OF LINE END OF FILE", 0},
		{"lone carriage return is whitespace", "a\rb\n", "a b END OF LINE END OF FILE", 0},
		{"mid-line tab is whitespace", "a\tb\n", "a b END OF LINE END OF FILE", 0},
		{"multi-level dedent", "a\n  b\n    c\nd\n", "a END OF LINE INDENT b END OF LINE INDENT c END OF LINE DEDENT DEDENT d END OF LINE END OF FILE", 0},
		{"leading comma", ",x\n", ", x END OF LINE END OF FILE", 0},
		{"escape at end of line", "\"a\\\n", "a END OF LINE END OF FILE", 1},
		{"escaped non-ascii", "\"\\é\"\n", "\\é END OF LINE END OF FILE", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Lex(diag.NewFile("t.clowd", tt.src))
			if got := kinds(r); got != tt.want {
				t.Errorf("tokens:\n got %s\nwant %s", got, tt.want)
			}
			if len(r.Diags) != tt.diags {
				t.Errorf("got %d diagnostics, want %d: %v", len(r.Diags), tt.diags, r.Diags)
			}
		})
	}
}

func TestCRLFSpans(t *testing.T) {
	src := "a # c\r\nb\r\n"
	r := Lex(diag.NewFile("t.clowd", src))
	if got := r.Comments[0].Text; got != "# c" {
		t.Errorf("comment text = %q, want %q", got, "# c")
	}
	nl := r.Tokens[1]
	if nl.Kind != token.Newline || src[nl.Span.Start:nl.Span.End] != "\r\n" {
		t.Errorf("newline token = %v spanning %q, want newline spanning \"\\r\\n\"", nl, src[nl.Span.Start:nl.Span.End])
	}
}
