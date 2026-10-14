package project

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/joshjkns/clowd/internal/diag"
)

// This file is a parser for the subset of TOML that clowd.toml and
// clowd.lock use: comments, `[table]` headers, and `key = value` where a
// value is a string, an integer, a boolean or an array of those. Anything
// else in TOML (floats, dates, inline tables, multi-line strings, arrays of
// tables, dotted keys) is reported as unsupported rather than misread, so
// a valid file never means something different here than in a real TOML
// parser. A dependency for the full language isn't worth it for a handful
// of settings.

// valueKind is the type of a TOML value.
type valueKind int

const (
	stringValue valueKind = iota + 1
	intValue
	boolValue
	arrayValue
)

func (k valueKind) String() string {
	switch k {
	case stringValue:
		return "a string"
	case intValue:
		return "an integer"
	case boolValue:
		return "a boolean"
	case arrayValue:
		return "an array"
	}
	return fmt.Sprintf("valueKind(%d)", int(k))
}

// value is a parsed TOML value.
type value struct {
	kind  valueKind
	str   string
	num   int64
	bool  bool
	items []value
	pos   diag.Span
}

// entry is one `key = value` line.
type entry struct {
	// path is the table and key joined by dots: "aws.network.cidr".
	path   string
	keyPos diag.Span
	value  value
}

// document is a parsed TOML file.
type document struct {
	// entries are in source order.
	entries []entry
	// tables maps each `[table]` header to its span.
	tables map[string]diag.Span
}

// table returns the entries directly in the table at path ("" for the
// top level), keyed by their own name.
func (d *document) table(path string) map[string]entry {
	out := map[string]entry{}
	for _, e := range d.entries {
		parent, key := splitPath(e.path)
		if parent == path {
			out[key] = e
		}
	}
	return out
}

func splitPath(path string) (parent, key string) {
	if i := strings.LastIndexByte(path, '.'); i >= 0 {
		return path[:i], path[i+1:]
	}
	return "", path
}

type tomlParser struct {
	f     *diag.File
	src   string
	i     int
	table string
	doc   *document
	diags diag.List
	// seen maps each key path and table to where it was defined, to
	// report duplicates.
	seen map[string]diag.Span
}

// parseTOML parses f. It recovers at the next line after an error, so
// every mistake in the file is reported at once.
func parseTOML(f *diag.File) (*document, diag.List) {
	p := &tomlParser{f: f, src: f.Text, doc: &document{tables: map[string]diag.Span{}}, seen: map[string]diag.Span{}}
	for {
		p.skipBlank()
		if p.i >= len(p.src) {
			break
		}
		start := len(p.diags)
		if p.src[p.i] == '[' {
			p.header()
		} else {
			p.keyValue()
		}
		if len(p.diags) > start {
			p.skipLine()
			continue
		}
		p.endLine()
	}
	return p.doc, p.diags
}

func (p *tomlParser) errorf(sp diag.Span, format string, args ...any) {
	p.diags.Add(diag.Errorf(sp, format, args...))
}

func (p *tomlParser) at(n int) diag.Span { return diag.Span{Start: p.i, End: min(p.i+n, len(p.src))} }

// skipSpace skips spaces and tabs.
func (p *tomlParser) skipSpace() {
	for p.i < len(p.src) && (p.src[p.i] == ' ' || p.src[p.i] == '\t') {
		p.i++
	}
}

// skipBlank skips whitespace, newlines and comments.
func (p *tomlParser) skipBlank() {
	for p.i < len(p.src) {
		switch p.src[p.i] {
		case ' ', '\t', '\r', '\n':
			p.i++
		case '#':
			p.skipLine()
		default:
			return
		}
	}
}

func (p *tomlParser) skipLine() {
	for p.i < len(p.src) && p.src[p.i] != '\n' {
		p.i++
	}
}

// endLine expects nothing but a comment before the end of the line.
func (p *tomlParser) endLine() {
	p.skipSpace()
	if p.i >= len(p.src) || p.src[p.i] == '\n' || p.src[p.i] == '#' || strings.HasPrefix(p.src[p.i:], "\r\n") {
		p.skipLine()
		return
	}
	end := p.i
	for end < len(p.src) && p.src[end] != '\n' && p.src[end] != '#' {
		end++
	}
	p.errorf(diag.Span{Start: p.i, End: end}, "unexpected text after the value")
	p.skipLine()
}

// header parses `[a.b]`.
func (p *tomlParser) header() {
	start := p.i
	p.i++ // [
	if p.i < len(p.src) && p.src[p.i] == '[' {
		p.errorf(p.at(2), "arrays of tables (`[[...]]`) aren't supported in %s", p.f.Name)
		return
	}
	var parts []string
	for {
		p.skipSpace()
		k, ok := p.key()
		if !ok {
			return
		}
		parts = append(parts, k)
		p.skipSpace()
		if p.i < len(p.src) && p.src[p.i] == '.' {
			p.i++
			continue
		}
		break
	}
	if p.i >= len(p.src) || p.src[p.i] != ']' {
		p.errorf(p.at(1), "expected `]` to close the table name")
		return
	}
	p.i++
	sp := diag.Span{Start: start, End: p.i}
	name := strings.Join(parts, ".")
	if prev, dup := p.seen[name]; dup {
		p.errorf(sp, "`%s` is already defined on line %d", name, p.f.Position(prev.Start).Line)
		return
	}
	p.seen[name] = sp
	p.doc.tables[name] = sp
	p.table = name
}

// keyValue parses `key = value`.
func (p *tomlParser) keyValue() {
	start := p.i
	k, ok := p.key()
	if !ok {
		return
	}
	keyPos := diag.Span{Start: start, End: p.i}
	p.skipSpace()
	if p.i < len(p.src) && p.src[p.i] == '.' {
		p.errorf(p.at(1), "dotted keys aren't supported in %s; use a `[table]` header", p.f.Name)
		return
	}
	if p.i >= len(p.src) || p.src[p.i] != '=' {
		p.errorf(p.at(1), "expected `=` after `%s`", k)
		return
	}
	p.i++
	p.skipSpace()
	v, ok := p.value()
	if !ok {
		return
	}
	path := k
	if p.table != "" {
		path = p.table + "." + k
	}
	if prev, dup := p.seen[path]; dup {
		p.errorf(keyPos, "`%s` is already set on line %d", k, p.f.Position(prev.Start).Line)
		return
	}
	p.seen[path] = keyPos
	p.doc.entries = append(p.doc.entries, entry{path: path, keyPos: keyPos, value: v})
}

func isBareKey(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// key parses a bare or quoted key.
func (p *tomlParser) key() (string, bool) {
	if p.i < len(p.src) && (p.src[p.i] == '"' || p.src[p.i] == '\'') {
		v, ok := p.value()
		return v.str, ok
	}
	start := p.i
	for p.i < len(p.src) && isBareKey(p.src[p.i]) {
		p.i++
	}
	if p.i == start {
		p.errorf(p.at(1), "expected a key")
		return "", false
	}
	return p.src[start:p.i], true
}

// value parses a string, integer, boolean or array.
func (p *tomlParser) value() (value, bool) {
	start := p.i
	if p.i >= len(p.src) || p.src[p.i] == '\n' || p.src[p.i] == '\r' || p.src[p.i] == '#' {
		p.errorf(p.at(0), "expected a value")
		return value{}, false
	}
	switch c := p.src[p.i]; {
	case strings.HasPrefix(p.src[p.i:], `"""`) || strings.HasPrefix(p.src[p.i:], "'''"):
		p.errorf(p.at(3), "multi-line strings aren't supported in %s", p.f.Name)
		return value{}, false
	case c == '"':
		return p.basicString()
	case c == '\'':
		end := strings.IndexAny(p.src[p.i+1:], "'\n")
		if end < 0 || p.src[p.i+1+end] != '\'' {
			p.errorf(p.at(1), "unterminated string")
			return value{}, false
		}
		s := p.src[p.i+1 : p.i+1+end]
		p.i += end + 2
		return value{kind: stringValue, str: s, pos: diag.Span{Start: start, End: p.i}}, true
	case c == '[':
		return p.array()
	case c == '{':
		p.errorf(p.at(1), "inline tables aren't supported in %s; use a `[table]` header", p.f.Name)
		return value{}, false
	}
	end := p.i
	for end < len(p.src) && !strings.ContainsRune(" \t\r\n#,]", rune(p.src[end])) {
		end++
	}
	word := p.src[p.i:end]
	sp := diag.Span{Start: p.i, End: end}
	p.i = end
	switch word {
	case "true", "false":
		return value{kind: boolValue, bool: word == "true", pos: sp}, true
	}
	n, err := strconv.ParseInt(strings.ReplaceAll(word, "_", ""), 10, 64)
	if err == nil && !strings.HasPrefix(word, "_") && !strings.HasSuffix(word, "_") && !strings.Contains(word, "__") {
		return value{kind: intValue, num: n, pos: sp}, true
	}
	if word[0] >= '0' && word[0] <= '9' || word[0] == '+' || word[0] == '-' {
		p.errorf(sp, "only whole numbers are supported in %s", p.f.Name)
	} else {
		p.diags.Add(diag.Errorf(sp, "expected a value, found `%s`", word).Suggest(`quote strings: "%s"`, word))
	}
	return value{}, false
}

// basicString parses a "..." string with TOML escapes.
func (p *tomlParser) basicString() (value, bool) {
	start := p.i
	p.i++
	var b strings.Builder
	for {
		if p.i >= len(p.src) || p.src[p.i] == '\n' {
			p.errorf(diag.Span{Start: start, End: p.i}, "unterminated string")
			return value{}, false
		}
		c := p.src[p.i]
		if c == '"' {
			p.i++
			return value{kind: stringValue, str: b.String(), pos: diag.Span{Start: start, End: p.i}}, true
		}
		if c != '\\' {
			b.WriteByte(c)
			p.i++
			continue
		}
		if p.i+1 >= len(p.src) {
			p.errorf(p.at(1), "unterminated string")
			return value{}, false
		}
		esc := p.src[p.i+1]
		if s, ok := simpleEscapes[esc]; ok {
			b.WriteString(s)
			p.i += 2
			continue
		}
		n := map[byte]int{'u': 4, 'U': 8}[esc]
		if n == 0 || p.i+2+n > len(p.src) {
			p.errorf(p.at(2), "unknown escape `\\%c`", esc)
			return value{}, false
		}
		r, err := strconv.ParseUint(p.src[p.i+2:p.i+2+n], 16, 32)
		if err != nil || !utf8.ValidRune(rune(r)) {
			p.errorf(p.at(2+n), "invalid unicode escape")
			return value{}, false
		}
		b.WriteRune(rune(r))
		p.i += 2 + n
	}
}

var simpleEscapes = map[byte]string{'"': `"`, '\\': `\`, 'n': "\n", 't': "\t", 'r': "\r", 'b': "\b", 'f': "\f"}

// array parses `[a, b, ...]`, which may span lines and end with a comma.
func (p *tomlParser) array() (value, bool) {
	start := p.i
	p.i++
	v := value{kind: arrayValue}
	for {
		p.skipBlank()
		if p.i >= len(p.src) {
			p.errorf(diag.Span{Start: start, End: start + 1}, "unterminated array")
			return value{}, false
		}
		if p.src[p.i] == ']' {
			p.i++
			v.pos = diag.Span{Start: start, End: p.i}
			return v, true
		}
		item, ok := p.value()
		if !ok {
			return value{}, false
		}
		if item.kind == arrayValue {
			p.errorf(item.pos, "nested arrays aren't supported in %s", p.f.Name)
			return value{}, false
		}
		v.items = append(v.items, item)
		p.skipBlank()
		if p.i < len(p.src) && p.src[p.i] == ',' {
			p.i++
			continue
		}
		if p.i < len(p.src) && p.src[p.i] == ']' {
			continue
		}
		p.errorf(p.at(1), "expected `,` or `]` in the array")
		return value{}, false
	}
}

// quote returns s as a TOML basic string.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
