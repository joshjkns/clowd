package check

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/spell"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// defaultPort is the container port when a service doesn't set one. It is
// part of the language (01-language.md), not of any backend.
const defaultPort = 8080

func (c *checker) declare(d *ast.Declare) {
	h := ir.Header{Name: d.Name.Name, Pos: d.Name.Pos, Spans: map[string]diag.Span{}}
	var r ir.Resource
	switch d.Kind {
	case ast.Service:
		r = c.service(d, h)
	case ast.Database:
		r = c.database(d, h)
	case ast.Bucket:
		r = c.bucket(d, h)
	case ast.LoadBalancer:
		r = c.loadBalancer(d, h)
	case ast.Queue:
		r = c.queue(d, h)
	default:
		return
	}
	c.prog.Resources = append(c.prog.Resources, r)
}

// ---- traits ----

// traitGroup is a set of mutually exclusive traits, such as `public` and
// `private`. key names the group in Header.Spans.
type traitGroup struct {
	key   string
	words []string
	// versioned groups take an optional second word (`postgres 16`).
	versioned bool
	set       func(word, version string)
}

// traitSynonyms are wrong guesses accepted with an error, keyed by kind.
var traitSynonyms = map[ast.Kind]map[string]string{
	ast.Database: {
		"postgresql": "postgres",
		"pg":         "postgres",
		"psql":       "postgres",
		"tiny":       "small",
		"xs":         "small",
		"big":        "large",
		"xl":         "large",
	},
	ast.Bucket: {
		"internal": "private",
		"open":     "public",
	},
	ast.LoadBalancer: {
		"private":  "internal",
		"external": "public",
	},
	ast.Queue: {
		"ordered": "fifo",
	},
}

// traits checks d's traits against groups. Missing groups keep the
// resource's defaults.
func (c *checker) traits(d *ast.Declare, h *ir.Header, groups []traitGroup) {
	if len(groups) == 0 {
		if len(d.Traits) > 0 {
			sp := d.Traits[0].Pos.Join(d.Traits[len(d.Traits)-1].Pos)
			e := diag.Errorf(sp, "a %s has no traits", d.Kind)
			if w := strings.ToLower(d.Traits[0].Words[0].Text); w == "public" || w == "internal" || w == "private" {
				e = e.Suggest("to expose a service, put a load balancer in front of it: `send / to %s`", d.Name.Name)
			} else {
				e = e.Suggest("settings go in an indented block below the declaration, such as `  port 8080`")
			}
			c.diags.Add(e)
		}
		return
	}
	chosen := map[string]string{} // group key → trait as written, e.g. "postgres 16"
	for _, t := range d.Traits {
		first := t.Words[0]
		word := strings.ToLower(first.Text)
		version := ""
		if first.Quoted {
			// Accepted, like a quoted name, so the rest still checks.
			c.diags.Add(diag.Errorf(first.Pos, "traits aren't quoted").Suggest("write `%s`", first.Text))
		}
		if canon, ok := traitSynonyms[d.Kind][word]; ok {
			c.diags.Add(diag.Errorf(first.Pos, "unknown %s trait `%s`", d.Kind, first.Text).Suggest("write `%s`", canon))
			word = canon
		} else if name, v, ok := splitVersion(word); ok && len(t.Words) == 1 {
			if canon, ok := traitSynonyms[d.Kind][name]; ok {
				// `pg16`
				c.diags.Add(diag.Errorf(first.Pos, "unknown %s trait `%s`", d.Kind, first.Text).Suggest("write `%s %s`", canon, v))
				word, version = canon, v
			}
		}
		gi := slices.IndexFunc(groups, func(g traitGroup) bool { return slices.Contains(g.words, word) })
		if gi < 0 {
			name, v, joined := joinedVersion(word, groups)
			c.diags.Add(c.unknownTrait(d.Kind, first, groups))
			if !joined || len(t.Words) > 1 {
				continue
			}
			// `postgres16`: reported above, but understood.
			word, version = name, v
			gi = slices.IndexFunc(groups, func(g traitGroup) bool { return slices.Contains(g.words, word) })
		}
		g := groups[gi]
		if len(t.Words) > 1 {
			if !g.versioned || len(t.Words) > 2 {
				extra := t.Words[1]
				if g.versioned {
					extra = t.Words[2]
				}
				c.diags.Add(diag.Errorf(extra.Pos, "unexpected `%s` after `%s`", extra.Text, word).
					Suggest("separate traits with `,` or `and`"))
				continue
			}
			version = t.Words[1].Text
			if !validVersion(version) {
				c.diags.Add(diag.Errorf(t.Words[1].Pos, "`%s` is not a version", version).
					Suggest("write a version number, such as `%s 16`", word))
				continue
			}
		}
		written := strings.TrimSpace(word + " " + version)
		if prev, ok := chosen[g.key]; ok {
			switch {
			case prev == written:
				c.diags.Add(diag.Warnf(t.Pos, "`%s` is repeated", written).Suggest("remove it"))
			case g.versioned && strings.Fields(prev)[0] == word:
				c.diags.Add(diag.Errorf(t.Pos, "`%s` contradicts `%s`", written, prev).
					Suggest("a %s has one version", d.Kind))
			default:
				c.diags.Add(diag.Errorf(t.Pos, "`%s` contradicts `%s`", written, prev).
					Suggest("a %s takes one of %s", d.Kind, codeList(g.words, "or")))
			}
			continue
		}
		chosen[g.key] = written
		h.Spans[g.key] = t.Pos
		g.set(word, version)
	}
}

// splitVersion splits `pg16` into `pg` and `16`.
func splitVersion(word string) (name, version string, ok bool) {
	i := strings.IndexAny(word, "0123456789")
	if i <= 0 || !validVersion(word[i:]) {
		return "", "", false
	}
	return word[:i], word[i:], true
}

// joinedVersion splits `postgres16` into `postgres` and `16`.
func joinedVersion(word string, groups []traitGroup) (name, version string, ok bool) {
	for _, g := range groups {
		if !g.versioned {
			continue
		}
		for _, w := range g.words {
			if v, found := strings.CutPrefix(word, w); found && validVersion(v) {
				return w, v, true
			}
		}
	}
	return "", "", false
}

func (c *checker) unknownTrait(k ast.Kind, v ast.Value, groups []traitGroup) diag.Diagnostic {
	d := diag.Errorf(v.Pos, "unknown %s trait `%s`", k, v.Text)
	if name, version, ok := joinedVersion(strings.ToLower(v.Text), groups); ok {
		return d.Suggest("put a space before the version: `%s %s`", name, version)
	}
	var all []string
	for _, g := range groups {
		all = append(all, g.words...)
	}
	if guess, ok := spell.Closest(v.Text, all); ok {
		return d.Suggest("did you mean `%s`?", guess)
	}
	return d.Suggest("a %s's traits are %s", k, codeList(all, "and"))
}

// ---- properties ----

type propSpec struct {
	repeat  bool
	example string
	// set interprets p and reports whether it was valid.
	set func(p ast.Property) bool
}

// propSynonyms are wrong guesses for property names, accepted with an error.
var propSynonyms = map[string]string{
	"replicas":    "scale",
	"instances":   "scale",
	"count":       "scale",
	"autoscale":   "scale",
	"ports":       "port",
	"environment": "env",
	"envs":        "env",
	"var":         "env",
	"mem":         "memory",
	"ram":         "memory",
	"vcpu":        "cpu",
	"cpus":        "cpu",
	"img":         "image",
	"container":   "image",
	"route":       "send",
	"forward":     "send",
	"listener":    "listen",
	"listens":     "listen",
}

// props checks d's properties against specs, keyed by lowercase name.
// groups are the kind's traits, for suggesting a trait written as a
// property.
func (c *checker) props(d *ast.Declare, h *ir.Header, specs map[string]propSpec, groups []traitGroup) {
	seen := map[string]diag.Span{}
	for _, p := range d.Props {
		key := strings.ToLower(p.Key.Text)
		spec, ok := specs[key]
		if !ok {
			if canon, isSyn := propSynonyms[key]; isSyn {
				if s, known := specs[canon]; known {
					c.diags.Add(diag.Errorf(p.Key.Pos, "unknown %s property `%s`", d.Kind, p.Key.Text).Suggest("write `%s`", canon))
					key, spec, ok = canon, s, true
				}
			}
		}
		if !ok {
			c.diags.Add(c.unknownProp(d, p, specs, groups))
			continue
		}
		if prev, dup := seen[key]; dup && !spec.repeat {
			c.diags.Add(diag.Errorf(p.Pos, "`%s` is already set on line %d", key, c.line(prev)).
				Suggest("remove one of them"))
			continue
		}
		seen[key] = p.Pos
		if len(p.Values) == 0 {
			c.diags.Add(diag.Errorf(diag.Span{Start: p.Key.Pos.End, End: p.Key.Pos.End}, "expected a value after `%s`", key).
				Suggest("for example `%s`", spec.example))
			continue
		}
		if spec.set(p) && !h.Has(key) {
			h.Spans[key] = p.Pos
		}
	}
}

func (c *checker) unknownProp(d *ast.Declare, p ast.Property, specs map[string]propSpec, groups []traitGroup) diag.Diagnostic {
	key := strings.ToLower(p.Key.Text)
	e := diag.Errorf(p.Key.Pos, "unknown %s property `%s`", d.Kind, p.Key.Text)
	is := func(trait string) diag.Diagnostic {
		return e.Suggest("`%s` is a trait: `%s %s is %s`", trait, d.Kind, d.Name.Name, trait)
	}
	for _, g := range groups {
		if slices.Contains(g.words, key) {
			return is(key) // `public` as a property
		}
		if g.key == key && len(p.Values) > 0 { // `size small`
			return is(strings.ToLower(joinValues(p.Values)))
		}
	}
	if len(specs) == 0 {
		return e.Suggest("a %s has no properties", d.Kind)
	}
	names := slices.Sorted(maps.Keys(specs))
	if guess, ok := spell.Closest(key, names); ok {
		return e.Suggest("did you mean `%s`?", guess)
	}
	return e.Suggest("a %s's properties are %s", d.Kind, codeList(names, "and"))
}

// setFrom stores the result of parse in dst if it is valid.
func setFrom[T any](dst *T, parse func() (T, bool)) bool {
	v, ok := parse()
	if ok {
		*dst = v
	}
	return ok
}

// oneThen returns a parser for a property with a single value.
func (c *checker) oneThen(p ast.Property, example string, parse func(ast.Value) (int, bool)) func() (int, bool) {
	return func() (int, bool) {
		v, ok := c.one(p, example)
		if !ok {
			return 0, false
		}
		return parse(v)
	}
}

// one returns the single value of p, reporting extra values.
func (c *checker) one(p ast.Property, example string) (ast.Value, bool) {
	if len(p.Values) > 1 {
		sp := p.Values[1].Pos.Join(p.Values[len(p.Values)-1].Pos)
		c.diags.Add(diag.Errorf(sp, "`%s` takes one value", strings.ToLower(p.Key.Text)).
			Suggest("for example `%s`; quote values that contain spaces", example))
		return ast.Value{}, false
	}
	return p.Values[0], true
}

// required reports a missing property.
func (c *checker) required(d *ast.Declare, h *ir.Header, key, example string) {
	if _, ok := h.Spans[key]; ok {
		return
	}
	for _, p := range d.Props {
		k := strings.ToLower(p.Key.Text)
		if k == key || propSynonyms[k] == key {
			return // present but invalid; already reported
		}
	}
	c.diags.Add(diag.Errorf(d.Name.Pos, "%s `%s` needs `%s`", d.Kind, d.Name.Name, key).
		Suggest("add it in an indented block: `  %s`", example))
}

// ---- kinds ----

func (c *checker) service(d *ast.Declare, h ir.Header) *ir.Service {
	s := &ir.Service{Header: h, Port: defaultPort, Scale: ir.Scale{Min: 1, Max: 1}}
	c.traits(d, &s.Header, nil)
	envAt := map[string]diag.Span{}
	specs := map[string]propSpec{
		"image": {example: "image ghcr.io/me/api:latest", set: func(p ast.Property) bool {
			v, ok := c.one(p, "image ghcr.io/me/api:latest")
			if ok {
				s.Image = v.Text
			}
			return ok
		}},
		"port": {example: "port 8080", set: func(p ast.Property) bool {
			return setFrom(&s.Port, c.oneThen(p, "port 8080", func(v ast.Value) (int, bool) { return c.port(v, "port") }))
		}},
		"scale": {example: "scale 2 to 10", set: func(p ast.Property) bool {
			return setFrom(&s.Scale, func() (ir.Scale, bool) { return c.scale(p) })
		}},
		"cpu": {example: "cpu 0.5", set: func(p ast.Property) bool {
			return setFrom(&s.CPU, c.oneThen(p, "cpu 0.5", c.cpu))
		}},
		"memory": {example: "memory 512mb", set: func(p ast.Property) bool {
			return setFrom(&s.Memory, c.oneThen(p, "memory 512mb", c.memory))
		}},
		"env": {repeat: true, example: "env LOG_LEVEL debug", set: func(p ast.Property) bool {
			e, ok := c.env(p)
			if !ok {
				return false
			}
			if prev, dup := envAt[e.Name]; dup {
				c.diags.Add(diag.Errorf(p.Values[0].Pos, "`%s` is already set on line %d", e.Name, c.line(prev)).
					Suggest("remove one of them"))
				return false
			}
			envAt[e.Name] = p.Pos
			s.Env = append(s.Env, e)
			return true
		}},
	}
	c.props(d, &s.Header, specs, nil)
	c.required(d, &s.Header, "image", "image ghcr.io/me/api:latest")
	return s
}

func (c *checker) database(d *ast.Declare, h ir.Header) *ir.Database {
	db := &ir.Database{Header: h, Size: ir.Small}
	groups := []traitGroup{
		{key: "engine", words: []string{"postgres", "mysql"}, versioned: true, set: func(w, v string) {
			db.Engine = map[string]ir.Engine{"postgres": ir.Postgres, "mysql": ir.MySQL}[w]
			db.Version = v
		}},
		{key: "size", words: []string{"small", "medium", "large"}, set: func(w, _ string) {
			db.Size = map[string]ir.Size{"small": ir.Small, "medium": ir.Medium, "large": ir.Large}[w]
		}},
	}
	c.traits(d, &db.Header, groups)
	c.props(d, &db.Header, nil, groups)
	if db.Engine == 0 && !c.hasTraitError(d, groups[0]) {
		c.diags.Add(diag.Errorf(d.Name.Pos, "database `%s` needs an engine", d.Name.Name).
			Suggest("for example `database %s is postgres 16`", d.Name.Name))
	}
	return db
}

// hasTraitError reports whether d has a trait that looks like an attempt at
// group g, so a "needs an engine" error does not repeat a "did you mean".
func (c *checker) hasTraitError(d *ast.Declare, g traitGroup) bool {
	for _, t := range d.Traits {
		w := strings.ToLower(t.Words[0].Text)
		if _, ok := traitSynonyms[d.Kind][w]; ok {
			continue
		}
		if name, _, ok := splitVersion(w); ok {
			if _, ok := traitSynonyms[d.Kind][name]; ok {
				continue // `pg16`: accepted as postgres
			}
		}
		if slices.Contains(g.words, w) {
			return true // a bad version
		}
		if _, ok := spell.Closest(w, g.words); ok {
			return true
		}
		if _, _, ok := joinedVersion(w, []traitGroup{g}); ok {
			return true
		}
	}
	return false
}

func (c *checker) bucket(d *ast.Declare, h ir.Header) *ir.Bucket {
	b := &ir.Bucket{Header: h}
	groups := []traitGroup{
		{key: "visibility", words: []string{"private", "public"}, set: func(w, _ string) { b.Public = w == "public" }},
	}
	c.traits(d, &b.Header, groups)
	c.props(d, &b.Header, nil, groups)
	return b
}

func (c *checker) queue(d *ast.Declare, h ir.Header) *ir.Queue {
	q := &ir.Queue{Header: h}
	groups := []traitGroup{
		{key: "fifo", words: []string{"fifo"}, set: func(string, string) { q.FIFO = true }},
	}
	c.traits(d, &q.Header, groups)
	c.props(d, &q.Header, nil, groups)
	return q
}

func (c *checker) loadBalancer(d *ast.Declare, h ir.Header) *ir.LoadBalancer {
	lb := &ir.LoadBalancer{Header: h}
	groups := []traitGroup{
		{key: "visibility", words: []string{"public", "internal"}, set: func(w, _ string) { lb.Internal = w == "internal" }},
	}
	c.traits(d, &lb.Header, groups)
	specs := map[string]propSpec{
		"listen": {repeat: true, example: "listen on 443 with tls for api.example.com", set: func(p ast.Property) bool {
			l, ok := c.listen(p)
			if !ok {
				return false
			}
			if i := slices.IndexFunc(lb.Listeners, func(o ir.Listener) bool { return o.Port == l.Port }); i >= 0 {
				c.diags.Add(diag.Errorf(p.Pos, "port %d is already used on line %d", l.Port, c.line(lb.Listeners[i].Pos)).
					Suggest("remove one of them"))
				return false
			}
			lb.Listeners = append(lb.Listeners, l)
			return true
		}},
		"send": {repeat: true, example: "send / to api", set: func(p ast.Property) bool {
			r, ok := c.route(p)
			if !ok {
				return false
			}
			if i := slices.IndexFunc(lb.Routes, func(o ir.Route) bool { return o.Path == r.Path }); i >= 0 {
				c.diags.Add(diag.Errorf(p.Values[0].Pos, "`%s` is already routed on line %d", r.Path, c.line(lb.Routes[i].Pos)).
					Suggest("send each path to one service"))
				return false
			}
			lb.Routes = append(lb.Routes, r)
			return true
		}},
	}
	c.props(d, &lb.Header, specs, groups)
	c.required(d, &lb.Header, "listen", "listen on 443 with tls for api.example.com")
	c.required(d, &lb.Header, "send", fmt.Sprintf("send / to %s", c.someService()))
	return lb
}

// someService returns a declared service name for examples, or `api`.
func (c *checker) someService() string {
	for _, d := range c.table.Decls {
		if d.Kind == ast.Service {
			return d.Name.Name
		}
	}
	return "api"
}
