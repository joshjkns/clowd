package diag

import (
	"strings"
	"testing"
)

func TestPosition(t *testing.T) {
	f := NewFile("t.clowd", "use aws\n  é x\n\nlast")
	tests := []struct {
		offset int
		want   Position
	}{
		{0, Position{1, 1}},
		{4, Position{1, 5}},
		{7, Position{1, 8}},  // the newline itself
		{8, Position{2, 1}},  // start of line 2
		{10, Position{2, 3}}, // é
		{13, Position{2, 5}}, // x: é is 2 bytes but 1 column
		{15, Position{3, 1}}, // empty line
		{16, Position{4, 1}}, // "last"
		{20, Position{4, 5}}, // EOF
		{99, Position{4, 5}}, // clamped past EOF
		{-3, Position{1, 1}}, // clamped before start
	}
	for _, tt := range tests {
		if got := f.Position(tt.offset); got != tt.want {
			t.Errorf("Position(%d) = %+v, want %+v", tt.offset, got, tt.want)
		}
	}
}

func TestLine(t *testing.T) {
	f := NewFile("t.clowd", "one\r\ntwo\n\nfour\n")
	tests := []struct {
		n    int
		want string
	}{
		{0, ""},
		{1, "one"}, // CR stripped
		{2, "two"},
		{3, ""},
		{4, "four"},
		{5, ""}, // after the trailing newline
		{6, ""},
	}
	for _, tt := range tests {
		if got := f.Line(tt.n); got != tt.want {
			t.Errorf("Line(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestSpanJoin(t *testing.T) {
	got := Span{5, 8}.Join(Span{2, 6})
	if want := (Span{2, 8}); got != want {
		t.Errorf("Join = %+v, want %+v", got, want)
	}
}

func TestListSortAndHasErrors(t *testing.T) {
	var l List
	l.Add(
		Warnf(Span{10, 11}, "w at 10"),
		Infof(Span{0, 1}, "info at 0"),
		Errorf(Span{10, 12}, "e at 10"),
		Warnf(Span{0, 1}, "warn at 0"),
	)
	l.Sort()
	var got []string
	for _, d := range l {
		got = append(got, d.Message)
	}
	want := "warn at 0, info at 0, e at 10, w at 10"
	if strings.Join(got, ", ") != want {
		t.Errorf("sorted = %q, want %q", strings.Join(got, ", "), want)
	}
	if !l.HasErrors() {
		t.Error("HasErrors = false, want true")
	}
	if (List{Warnf(Span{}, "w")}).HasErrors() {
		t.Error("HasErrors on warnings only = true, want false")
	}
}

func TestRender(t *testing.T) {
	src := "use aws\n\napi links to db\n\tbad\nnamé ok\nend"
	f := NewFile("app.clowd", src)
	at := func(sub string) Span {
		i := strings.Index(src, sub)
		return Span{i, i + len(sub)}
	}
	tests := []struct {
		name string
		d    Diagnostic
		want string
	}{
		{
			name: "with suggestion",
			d:    Errorf(at("links"), "unknown verb `links`").Suggest("did you mean `connects to`?"),
			want: `error: unknown verb ` + "`links`" + `
 --> app.clowd:3:5
  |
3 | api links to db
  |     ^^^^^
  = help: did you mean ` + "`connects to`?" + `
`,
		},
		{
			name: "zero width",
			d:    Errorf(Span{3, 3}, "expected a cloud"),
			want: `error: expected a cloud
 --> app.clowd:1:4
  |
1 | use aws
  |    ^
`,
		},
		{
			name: "multi-line span stops at end of first line",
			d:    Warnf(Span{at("to db").Start, at("bad").End}, "too long"),
			want: `warning: too long
 --> app.clowd:3:11
  |
3 | api links to db
  |           ^^^^^
`,
		},
		{
			name: "tab before caret is preserved",
			d:    Errorf(at("bad"), "tabs are not allowed"),
			want: "error: tabs are not allowed\n --> app.clowd:4:2\n  |\n4 | \tbad\n  | \t^^^\n",
		},
		{
			name: "non-ascii counts runes",
			d:    Infof(at("ok"), "here"),
			want: `info: here
 --> app.clowd:5:6
  |
5 | namé ok
  |      ^^
`,
		},
		{
			name: "underline over non-ascii counts runes",
			d:    Warnf(at("namé"), "names are ascii"),
			want: `warning: names are ascii
 --> app.clowd:5:1
  |
5 | namé ok
  | ^^^^
`,
		},
		{
			name: "at EOF",
			d:    Errorf(Span{len(src), len(src)}, "unexpected end of file"),
			want: `error: unexpected end of file
 --> app.clowd:6:4
  |
6 | end
  |    ^
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b strings.Builder
			if err := Render(&b, f, tt.d); err != nil {
				t.Fatal(err)
			}
			if b.String() != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", b.String(), tt.want)
			}
		})
	}
}

func TestRenderWideGutter(t *testing.T) {
	f := NewFile("t.clowd", strings.Repeat("\n", 11)+"x")
	var b strings.Builder
	if err := Render(&b, f, Errorf(Span{11, 12}, "m")); err != nil {
		t.Fatal(err)
	}
	want := "error: m\n  --> t.clowd:12:1\n   |\n12 | x\n   | ^\n"
	if b.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestRenderAll(t *testing.T) {
	f := NewFile("t.clowd", "a\nb")
	var b strings.Builder
	l := List{Errorf(Span{0, 1}, "one"), Errorf(Span{2, 3}, "two")}
	if err := RenderAll(&b, f, l); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(b.String(), "\n\n"); got != 1 {
		t.Errorf("want exactly one blank separator line, got %d in:\n%s", got, b.String())
	}
}
