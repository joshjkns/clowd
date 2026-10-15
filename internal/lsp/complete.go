package lsp

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/joshjkns/clowd/internal/check"
	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/lower/aws"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// Completion works from the text rather than the AST: the line being
// typed is usually incomplete, and what it can continue with depends only
// on its words so far and on the lines it is indented under.

var kinds = []ast.Kind{ast.Service, ast.Database, ast.Bucket, ast.LoadBalancer, ast.Queue, ast.Project, ast.Network}

var verbs = []ast.Verb{ast.ConnectsTo, ast.Reads, ast.Writes, ast.ReadsAndWrites, ast.SendsTo}

var vocabularies = sync.OnceValue(func() map[ast.Kind]check.Vocabulary {
	m := map[ast.Kind]check.Vocabulary{}
	for _, k := range kinds {
		m[k] = check.VocabularyOf(k)
	}
	return m
})

func vocabulary(k ast.Kind) check.Vocabulary { return vocabularies()[k] }

// valueWords are the keywords inside property values, by property and by
// how many words of the line come before them.
var valueWords = map[string]map[int][]string{
	"scale":   {2: {"to"}},
	"send":    {2: {"to"}},
	"listen":  {1: {"on"}, 3: {"with tls for"}},
	"backups": {1: {"off"}, 2: {"days"}},
	"retry":   {2: {"times"}},
}

// completer collects the items for one request.
type completer struct {
	s       *server
	doc     *document
	replace lspRange // the partly typed word that items replace
	items   []completionItem
}

func (c *completer) add(label string, kind int, detail, docs string) {
	item := completionItem{
		Label:    label,
		Kind:     kind,
		Detail:   detail,
		TextEdit: &textEdit{Range: c.replace, NewText: label},
		SortText: fmt.Sprintf("%04d", len(c.items)),
	}
	if docs != "" {
		item.Documentation = &markupContent{Kind: "markdown", Value: docs}
	}
	c.items = append(c.items, item)
}

func (s *server) complete(doc *document, offset int) *completionList {
	f := doc.file
	line := f.Position(offset).Line
	start := f.LineStart(line)
	before := f.Text[start:offset]
	if inComment(before) {
		return &completionList{Items: []completionItem{}}
	}
	// The word being typed is replaced by the item.
	partialStart := offset - len(lastWord(before))
	c := &completer{
		s:       s,
		doc:     doc,
		replace: toRange(f, diag.Span{Start: partialStart, End: offset}, s.enc),
		items:   []completionItem{},
	}
	words := tokens(f.Text[start:partialStart])
	indent := indentOf(before)
	if strings.TrimSpace(before) == "" {
		indent = len(before)
	}
	parents := parentLines(f, line, indent)
	if len(parents) == 0 {
		c.topLevel(words)
	} else {
		c.nested(parents, words)
	}
	return &completionList{Items: c.items}
}

// lastWord is the partly typed word at the end of s.
func lastWord(s string) string {
	return s[strings.LastIndexAny(s, " \t,")+1:]
}

// inComment reports whether the end of line text is inside a comment: a
// `#` outside quotes.
func inComment(text string) bool {
	quoted := false
	for i := 0; i < len(text); i++ {
		switch {
		case text[i] == '\\' && quoted:
			i++
		case text[i] == '"':
			quoted = !quoted
		case text[i] == '#' && !quoted:
			return true
		}
	}
	return false
}

// tokens splits text into lowercase words, with each comma its own token.
func tokens(text string) []string {
	var out []string
	for _, w := range strings.Fields(strings.ReplaceAll(text, ",", " , ")) {
		out = append(out, strings.ToLower(w))
	}
	return out
}

func indentOf(s string) int { return len(s) - len(strings.TrimLeft(s, " ")) }

// parentLines returns the words of the lines that the line numbered n,
// indented by indent, is nested under, innermost first. Blank and comment
// lines are skipped.
func parentLines(f *diag.File, n, indent int) [][]string {
	var out [][]string
	for i := n - 1; i >= 1 && indent > 0; i-- {
		text := f.Line(i)
		trimmed := strings.TrimSpace(text)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if ind := indentOf(text); ind < indent {
			out = append(out, tokens(text))
			indent = ind
		}
	}
	return out
}

// kindAt matches a kind at the start of words, returning it and how many
// words it takes.
func kindAt(words []string) (ast.Kind, int) {
	if len(words) >= 2 && words[0] == "load" && words[1] == "balancer" {
		return ast.LoadBalancer, 2
	}
	if len(words) >= 1 {
		if words[0] == "lb" {
			return ast.LoadBalancer, 1
		}
		for _, k := range kinds {
			if words[0] == k.String() {
				return k, 1
			}
		}
	}
	return ast.BadKind, 0
}

func (c *completer) topLevel(words []string) {
	if len(words) == 0 {
		for _, k := range kinds {
			c.add(k.String(), itemClass, "", kindDocs[k])
		}
		c.add("use", itemKeyword, "use <cloud> in <region>", keywordDocs["use"])
		c.add("on", itemKeyword, "on <cloud>", keywordDocs["on"])
		for _, d := range c.doc.analysis.Symbols.Decls {
			if d.Kind == ast.Service {
				c.add(d.Name.Name, itemVariable, d.Kind.String(), "")
			}
		}
		return
	}
	switch words[0] {
	case "use":
		switch len(words) {
		case 1:
			c.clouds()
		case 2:
			c.add("in", itemKeyword, "in <region>", "")
		}
		return
	case "on":
		if len(words) == 1 {
			c.clouds()
		}
		return
	}
	if k, n := kindAt(words); k != ast.BadKind {
		c.declaration(k, words[n:])
		return
	}
	if d := c.doc.analysis.Symbols.Lookup(words[0]); d != nil {
		c.relation(d, words[1:])
	}
}

func (c *completer) clouds() {
	for _, cl := range ir.Clouds {
		c.add(cl.String(), itemModule, "", cloudDoc(cl.String()))
	}
}

// declaration completes a declaration's header after its kind.
func (c *completer) declaration(k ast.Kind, rest []string) {
	if k.Named() {
		if len(rest) == 0 {
			return // the name is the user's to choose
		}
		rest = rest[1:]
	}
	if len(vocabulary(k).Traits) == 0 {
		return
	}
	if len(rest) == 0 {
		c.add("is", itemKeyword, "", keywordDocs["is"])
		return
	}
	if rest[0] != "is" {
		return
	}
	if last := rest[len(rest)-1]; last != "is" && last != "," && last != "and" {
		return
	}
	written := strings.Join(rest[1:], " ")
	for _, g := range vocabulary(k).Traits {
		// Traits in a group exclude each other, so a group already used
		// has nothing more to offer.
		if slices.ContainsFunc(g.Words, func(w string) bool { return hasPhrase(written, w) }) {
			continue
		}
		for _, w := range g.Words {
			detail := ""
			if g.Versioned {
				detail = w + " <version>"
			}
			c.add(w, itemEnumMember, detail, traitDocs[k][w])
		}
	}
}

// hasPhrase reports whether the words of phrase appear in text as words.
func hasPhrase(text, phrase string) bool {
	return strings.Contains(" "+text+" ", " "+phrase+" ")
}

// relation completes a relationship after its first name.
func (c *completer) relation(from *ast.Declare, rest []string) {
	if from.Kind != ast.Service {
		return // only services start relationships
	}
	typed := strings.Join(rest, " ")
	for _, v := range verbs {
		phrase := v.String()
		switch {
		case typed == "":
			c.add(phrase, itemKeyword, "", verbDocs[v])
		case strings.HasPrefix(phrase, typed+" "):
			c.add(strings.TrimPrefix(phrase, typed+" "), itemKeyword, phrase, verbDocs[v])
		case phrase == typed:
			targets := check.RelationTargets(irVerb(v))
			for _, d := range c.doc.analysis.Symbols.Decls {
				if d != from && slices.Contains(targets, irKind(d.Kind)) {
					c.add(d.Name.Name, itemVariable, d.Kind.String(), "")
				}
			}
		}
	}
}

// nested completes a line inside a block. parents are the lines above it,
// innermost first; the last is the top-level statement.
func (c *completer) nested(parents [][]string, words []string) {
	root := parents[len(parents)-1]
	if len(root) == 0 {
		return
	}
	// The keys of the lines between the root and this one, outermost
	// first: `on`, `capacity_provider_strategy`, ...
	var path []string
	for i := len(parents) - 2; i >= 0; i-- {
		if len(parents[i]) == 0 {
			return
		}
		path = append(path, parents[i][0])
	}
	if root[0] == "on" {
		if len(root) < 2 || len(words) > 0 {
			return
		}
		cl, ok := ir.ParseCloud(root[1])
		if !ok {
			return
		}
		switch {
		case len(path) == 0:
			for _, s := range check.CloudSettings(cl) {
				c.add(s, itemField, "", "")
			}
		case len(path) == 1 && path[0] == "network":
			for _, s := range check.NetworkSettings() {
				c.add(s, itemField, "", "")
			}
		}
		return
	}
	k, _ := kindAt(root)
	if k == ast.BadKind {
		return
	}
	if len(path) == 0 {
		c.property(k, words)
		return
	}
	// Inside an `on <cloud>` block in a resource.
	on := parents[len(parents)-2]
	if on[0] != "on" || len(on) < 2 || len(words) > 0 || !k.IsResource() {
		return
	}
	if cl, ok := ir.ParseCloud(on[1]); !ok || cl != ir.AWS {
		return
	}
	resource, keys := aws.Overridable(irKind(k), path[1:]...)
	for _, key := range keys {
		c.add(key, itemField, resource, "")
	}
}

// property completes a property line of a declaration of kind k.
func (c *completer) property(k ast.Kind, words []string) {
	if len(words) == 0 {
		for _, p := range vocabulary(k).Props {
			c.add(p.Name, itemProperty, p.Example, propDocs[k][p.Key])
		}
		if k.IsResource() {
			c.add("on", itemKeyword, "on <cloud>", keywordDocs["on"])
		}
		return
	}
	key := words[0]
	switch {
	case key == "on" && len(words) == 1 && k.IsResource():
		c.clouds()
	case key == "send" && k == ast.LoadBalancer && len(words) == 3 && words[2] == "to":
		for _, d := range c.doc.analysis.Symbols.Decls {
			if d.Kind == ast.Service {
				c.add(d.Name.Name, itemVariable, d.Kind.String(), "")
			}
		}
	default:
		for _, w := range valueWords[key][len(words)] {
			c.add(w, itemKeyword, "", "")
		}
	}
}

func irVerb(v ast.Verb) ir.Verb {
	switch v {
	case ast.ConnectsTo:
		return ir.ConnectsTo
	case ast.Reads:
		return ir.Reads
	case ast.Writes:
		return ir.Writes
	case ast.ReadsAndWrites:
		return ir.ReadsAndWrites
	case ast.SendsTo:
		return ir.SendsTo
	}
	return 0
}
