package check

import (
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// `on <cloud>` blocks hold raw provider settings (D17). check only checks
// their shape: one cloud, a block, well-formed values and lists, no key
// set twice. Whether a key exists on the provider's resource is for the
// backend to say, since only it knows the resource.

// onBlock checks an `on <cloud>` property in the declaration of resource
// h and records its settings as an override.
func (c *checker) onBlock(d *ast.Declare, h *ir.Header, p ast.Property) {
	if len(p.Values) == 0 {
		c.diags.Add(diag.Errorf(diag.Span{Start: p.Key.Pos.End, End: p.Key.Pos.End}, "expected a cloud after `on`").
			Suggest("for example `on aws`"))
		return
	}
	if len(p.Values) > 1 {
		sp := p.Values[1].Pos.Join(p.Values[len(p.Values)-1].Pos)
		c.diags.Add(diag.Errorf(sp, "`on` takes one cloud").
			Suggest("put the settings in an indented block under `on %s`", strings.ToLower(p.Values[0].Text)))
		return
	}
	v := p.Values[0]
	cl, ok := c.cloud(ast.Ident{Name: v.Text, Pos: v.Pos})
	if !ok {
		return
	}
	c.untargeted(cl, p.Pos)
	if !p.HasBlock() {
		c.diags.Add(diag.Errorf(p.Pos, "expected an indented block of %s settings under `on %s`", cl, cl).
			Suggest("indent provider settings below it, such as `    <argument> <value>`"))
		return
	}
	if i := slices.IndexFunc(c.prog.Overrides, func(o ir.Override) bool { return o.Cloud == cl && o.Resource == h.Name }); i >= 0 {
		c.diags.Add(diag.Errorf(p.Pos, "`%s` already has an `on %s` block on line %d", h.Name, cl, c.line(c.prog.Overrides[i].Pos)).
			Suggest("merge them into one block"))
		return
	}
	c.prog.Overrides = append(c.prog.Overrides, ir.Override{
		Cloud:    cl,
		Resource: h.Name,
		Kind:     irKind(d.Kind),
		Settings: c.settings(p.Block),
		Pos:      p.Pos,
	})
}

// untargeted warns about an `on` block for a cloud the file doesn't use.
func (c *checker) untargeted(cl ir.Cloud, sp diag.Span) {
	if !slices.ContainsFunc(c.prog.Targets, func(t ir.Target) bool { return t.Cloud == cl }) {
		c.diags.Add(diag.Warnf(sp, "`on %s` has no effect: this file doesn't `use %s`", cl, cl).
			Suggest("add `use %s in <region>`, or remove the block", cl))
	}
}

// settings converts raw settings, reporting malformed ones and leaving
// them out. A key with a block may repeat (a provider's repeated nested
// block); a key with a value may not.
func (c *checker) settings(ps []ast.Property) []ir.Setting {
	out := []ir.Setting{}
	seen := map[string]diag.Span{}
	for _, p := range ps {
		s := ir.Setting{Key: p.Key.Text, KeyPos: p.Key.Pos, Pos: p.Pos}
		key := strings.ToLower(p.Key.Text)
		if p.HasBlock() {
			if len(p.Values) > 0 {
				sp := p.Values[0].Pos.Join(p.Values[len(p.Values)-1].Pos)
				c.diags.Add(diag.Errorf(sp, "`%s` has a block, so it takes no value on its line", p.Key.Text).
					Suggest("put each setting of `%s` on its own line in the block", p.Key.Text))
				continue
			}
			s.Block = c.settings(p.Block)
			out = append(out, s)
			continue
		}
		if len(p.Values) == 0 {
			c.diags.Add(diag.Errorf(diag.Span{Start: p.Key.Pos.End, End: p.Key.Pos.End}, "expected a value after `%s`", p.Key.Text).
				Suggest("write `%s <value>`, or indent a block of settings under it", p.Key.Text))
			continue
		}
		if prev, dup := seen[key]; dup {
			c.diags.Add(diag.Errorf(p.Pos, "`%s` is already set on line %d", p.Key.Text, c.line(prev)).
				Suggest("remove one of them"))
			continue
		}
		seen[key] = p.Pos
		if len(p.Commas) > 0 {
			list, ok := c.list(p)
			if !ok {
				continue
			}
			s.List = list
		} else {
			s.Value = joinValues(p.Values)
		}
		out = append(out, s)
	}
	return out
}

// list splits p's values at its commas into items, each one or more
// words joined by spaces.
func (c *checker) list(p ast.Property) ([]string, bool) {
	var items []string
	var cur []string
	vi := 0
	for _, comma := range p.Commas {
		for vi < len(p.Values) && p.Values[vi].Pos.Start < comma.Start {
			cur = append(cur, p.Values[vi].Text)
			vi++
		}
		if len(cur) == 0 {
			c.diags.Add(diag.Errorf(comma, "expected a value before `,`"))
			return nil, false
		}
		items = append(items, strings.Join(cur, " "))
		cur = nil
	}
	for ; vi < len(p.Values); vi++ {
		cur = append(cur, p.Values[vi].Text)
	}
	if len(cur) == 0 {
		last := p.Commas[len(p.Commas)-1]
		c.diags.Add(diag.Errorf(last, "expected a value after `,`").Suggest("remove the trailing comma"))
		return nil, false
	}
	return append(items, strings.Join(cur, " ")), true
}

// on checks a top-level `on <cloud>` block of program-wide settings.
func (c *checker) on(s *ast.On) {
	cl, ok := c.cloud(s.Cloud)
	if !ok {
		return
	}
	c.untargeted(cl, s.Pos)
	for _, p := range s.Settings {
		if c.oldOverride(cl, p) {
			continue
		}
		c.cloudSetting(cl, p)
	}
}

// oldOverride reports the pre-D17 form `<kind> <name> sets <key> to
// <value>`, which parses as a setting keyed by the kind, and shows where
// it goes now.
func (c *checker) oldOverride(cl ir.Cloud, p ast.Property) bool {
	words := append([]ast.Value{p.Key}, p.Values...)
	i := slices.IndexFunc(words, func(v ast.Value) bool { return isWord(v, "sets") })
	if i < 2 {
		return false
	}
	kind := strings.ToLower(joinValues(words[:i-1]))
	if !slices.Contains([]string{"service", "database", "bucket", "queue", "load balancer", "lb"}, kind) {
		return false
	}
	name := words[i-1].Text
	rest := words[i+1:]
	e := diag.Errorf(p.Pos, "settings for one resource go inside its declaration")
	if len(rest) >= 3 && isWord(rest[1], "to") {
		c.diags.Add(e.Suggest("under `%s %s`, write `  on %s` and below it `    %s %s`",
			kind, name, cl, rest[0].Text, joinValues(rest[2:])))
	} else {
		c.diags.Add(e.Suggest("under `%s %s`, write `  on %s` and below it `    <argument> <value>`", kind, name, cl))
	}
	return true
}
