package check

import (
	"net/netip"
	"regexp"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/spell"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// LatestEdition is the newest language edition this compiler supports
// (06-versioning.md).
const LatestEdition = 2026

// MaxProjectName keeps generated names inside AWS's shortest limits (32
// for a load balancer or target group) with room for the resource's own
// name.
const MaxProjectName = 20

var projectNameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ValidProjectName reports whether s can be a project name: up to
// MaxProjectName lowercase letters, digits and dashes, starting with a
// letter.
func ValidProjectName(s string) bool {
	return len(s) <= MaxProjectName && projectNameRE.MatchString(s)
}

// tagKeyRE is what both AWS tag keys and GCP label keys accept.
var tagKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,62}$`)

// programWide checks a `project` or `network` declaration. Each may
// appear once.
func (c *checker) programWide(d *ast.Declare) {
	var prev diag.Span
	switch d.Kind {
	case ast.Project:
		prev = c.prog.Project.Pos
	case ast.Network:
		prev = c.prog.Network.Pos
	}
	if prev != (diag.Span{}) {
		c.diags.Add(diag.Errorf(d.KindPos, "a program has one %s, and it is declared on line %d", d.Kind, c.line(prev)).
			Suggest("merge the two declarations"))
		return
	}
	switch d.Kind {
	case ast.Project:
		c.project(d)
	case ast.Network:
		c.network(d)
	}
}

func (c *checker) project(d *ast.Declare) {
	pr := &c.prog.Project
	pr.Header = ir.Header{Pos: d.KindPos, Spans: map[string]diag.Span{}}
	if !d.Name.Missing() {
		pr.Pos = d.Name.Pos
		name := d.Name.Name
		if ValidProjectName(name) {
			pr.Name = name
		} else {
			c.diags.Add(diag.Errorf(d.Name.Pos, "`%s` can't be a project name", name).
				Suggest("use up to %d lowercase letters, digits and dashes, starting with a letter", MaxProjectName))
		}
	}
	c.traits(d, &pr.Header, nil)
	tagAt := map[string]diag.Span{}
	specs := map[string]propSpec{
		"edition": {example: "edition 2026", set: func(p ast.Property) bool {
			return setFrom(&pr.Edition, c.oneThen(p, "edition 2026", c.edition))
		}},
		"tag": {repeat: true, example: "tag team payments", set: func(p ast.Property) bool {
			t, ok := c.tag(p)
			if !ok {
				return false
			}
			if prev, dup := tagAt[t.Key]; dup {
				c.diags.Add(diag.Errorf(p.Values[0].Pos, "tag `%s` is already set on line %d", t.Key, c.line(prev)).
					Suggest("remove one of them"))
				return false
			}
			tagAt[t.Key] = p.Pos
			pr.Tags = append(pr.Tags, t)
			return true
		}},
		"state": {example: "state local", set: func(p ast.Property) bool {
			v, ok := c.one(p, "state local")
			if !ok {
				return false
			}
			if !isWord(v, "local") {
				c.diags.Add(diag.Errorf(v.Pos, "the `%s` state backend is not implemented yet", v.Text).
					Suggest("state is kept in .clowd/ for now: `state local`"))
				return false
			}
			return true
		}},
	}
	c.props(d, &pr.Header, specs, nil)
}

func (c *checker) edition(v ast.Value) (int, bool) {
	n, ok := number(v.Text)
	switch {
	case !ok:
		c.diags.Add(diag.Errorf(v.Pos, "`%s` is not an edition", v.Text).Suggest("write `edition %d`", LatestEdition))
	case n > LatestEdition:
		c.diags.Add(diag.Errorf(v.Pos, "edition %d is newer than this clowd supports", n).
			Suggest("upgrade clowd, or write `edition %d`", LatestEdition))
	case n != LatestEdition:
		c.diags.Add(diag.Errorf(v.Pos, "there is no edition %d", n).Suggest("write `edition %d`", LatestEdition))
	default:
		return n, true
	}
	return 0, false
}

// tag parses `tag <key> <value>`.
func (c *checker) tag(p ast.Property) (ir.Tag, bool) {
	help := "write `tag <key> <value>`, such as `tag team payments`; quote values with spaces"
	if len(p.Values) != 2 {
		sp := p.Values[0].Pos.Join(p.Values[len(p.Values)-1].Pos)
		c.diags.Add(diag.Errorf(sp, "`tag` takes a key and a value").Suggest("%s", help))
		return ir.Tag{}, false
	}
	k, v := p.Values[0], p.Values[1]
	if k.Quoted || !tagKeyRE.MatchString(k.Text) {
		c.diags.Add(diag.Errorf(k.Pos, "`%s` can't be a tag key", k.Text).
			Suggest("tag keys are up to 63 lowercase letters, digits, `_` and `-`, starting with a letter, so every cloud accepts them"))
		return ir.Tag{}, false
	}
	return ir.Tag{Key: k.Text, Value: v.Text, Pos: p.Pos}, true
}

func (c *checker) network(d *ast.Declare) {
	n := &c.prog.Network
	n.Header = ir.Header{Pos: d.KindPos, Spans: map[string]diag.Span{}}
	groups := []traitGroup{
		{key: "services", words: []string{"public", "private"}, set: func(w, _ string) { n.Private = w == "private" }},
	}
	c.traits(d, &n.Header, groups)
	specs := map[string]propSpec{
		"cidr": {example: "cidr 10.0.0.0/16", set: func(p ast.Property) bool {
			v, ok := c.one(p, "cidr 10.0.0.0/16")
			if !ok {
				return false
			}
			cidr, ok := c.cidr(v)
			if ok {
				n.CIDR = cidr
			}
			return ok
		}},
	}
	c.props(d, &n.Header, specs, groups)
}

// cidr checks the block of a network Clowd creates. It carves /24 subnets
// out of it, so the block must leave room for them.
func (c *checker) cidr(v ast.Value) (string, bool) {
	prefix, err := netip.ParsePrefix(v.Text)
	switch {
	case err != nil || !prefix.Addr().Is4():
		c.diags.Add(diag.Errorf(v.Pos, "`%s` is not an IPv4 CIDR block", v.Text).Suggest("for example `cidr 10.0.0.0/16`"))
	case prefix.Bits() < 16 || prefix.Bits() > 20:
		c.diags.Add(diag.Errorf(v.Pos, "a network's block must be between /16 and /20, not /%d", prefix.Bits()).
			Suggest("for example `cidr %s/16`", prefix.Masked().Addr()))
	case prefix.Masked() != prefix:
		c.diags.Add(diag.Errorf(v.Pos, "`%s` has host bits set", v.Text).Suggest("write `cidr %s`", prefix.Masked()))
	default:
		return prefix.String(), true
	}
	return "", false
}

// cloudSettings are the program-wide settings each cloud's top-level `on`
// block accepts.
var cloudSettings = map[ir.Cloud][]string{
	ir.AWS: {"network"},
}

// cloudSetting checks one setting in a top-level `on <cloud>` block.
func (c *checker) cloudSetting(cl ir.Cloud, p ast.Property) {
	key := strings.ToLower(p.Key.Text)
	known := cloudSettings[cl]
	if !slices.Contains(known, key) {
		e := diag.Errorf(p.Key.Pos, "unknown `on %s` setting `%s`", cl, p.Key.Text)
		switch g, ok := spell.Closest(key, known); {
		case ok:
			e = e.Suggest("did you mean `%s`?", g)
		case len(known) > 0:
			e = e.Suggest("settings here are %s; settings for one resource go inside its declaration, under `on %s`", codeList(known, "and"), cl)
		default:
			e = e.Suggest("%s has no program-wide settings yet; settings for one resource go inside its declaration, under `on %s`", cl, cl)
		}
		c.diags.Add(e)
		return
	}
	switch key {
	case "network":
		c.existingNetwork(cl, p)
	}
}

// existingNetwork checks an `on aws` `network` block naming a VPC that
// already exists.
func (c *checker) existingNetwork(cl ir.Cloud, p ast.Property) {
	if len(p.Values) > 0 || !p.HasBlock() {
		c.diags.Add(diag.Errorf(p.Pos, "`network` takes an indented block").
			Suggest("list the VPC and its subnets below it: `vpc vpc-...`, `public_subnets subnet-..., subnet-...`, `private_subnets ...`"))
		return
	}
	if prev := c.prog.Network.ExistingOn(cl); prev != nil {
		c.diags.Add(diag.Errorf(p.Pos, "the %s network is already set on line %d", cl, c.line(prev.Pos)).Suggest("remove one of them"))
		return
	}
	e := ir.ExistingNetwork{Cloud: cl, Pos: p.Pos}
	known := []string{"vpc", "public_subnets", "private_subnets"}
	at := map[string]diag.Span{}
	for _, s := range p.Block {
		key := strings.ToLower(s.Key.Text)
		if !slices.Contains(known, key) {
			d := diag.Errorf(s.Key.Pos, "unknown network setting `%s`", s.Key.Text)
			if g, ok := spell.Closest(key, known); ok {
				d = d.Suggest("did you mean `%s`?", g)
			} else if key == "cidr" {
				d = d.Suggest("`cidr` is for the network Clowd creates, in the `network` declaration")
			} else {
				d = d.Suggest("settings here are %s", codeList(known, "and"))
			}
			c.diags.Add(d)
			continue
		}
		if prev, dup := at[key]; dup {
			c.diags.Add(diag.Errorf(s.Pos, "`%s` is already set on line %d", key, c.line(prev)).Suggest("remove one of them"))
			continue
		}
		if s.HasBlock() || len(s.Values) == 0 {
			c.diags.Add(diag.Errorf(s.Pos, "expected a value after `%s`", key).Suggest("write `%s` and its value on one line", key))
			continue
		}
		at[key] = s.Pos
		switch key {
		case "vpc":
			v, ok := c.one(s, "vpc vpc-0123456789abcdef0")
			if ok {
				e.VPC = v.Text
			}
		case "public_subnets", "private_subnets":
			ids := c.subnets(s)
			if ids == nil {
				continue
			}
			if key == "public_subnets" {
				e.PublicSubnets = ids
			} else {
				e.PrivateSubnets = ids
			}
		}
	}

	if _, ok := at["vpc"]; !ok {
		c.diags.Add(diag.Errorf(p.Key.Pos, "an existing network needs its `vpc`").
			Suggest("add `vpc vpc-...` to the block"))
		return
	}
	for _, need := range []struct {
		key string
		ids []string
	}{{"public_subnets", e.PublicSubnets}, {"private_subnets", e.PrivateSubnets}} {
		if sp, ok := at[need.key]; !ok {
			c.diags.Add(diag.Errorf(p.Key.Pos, "an existing network needs `%s` too", need.key).
				Suggest("list two or more subnets in different availability zones"))
		} else if need.ids != nil && len(need.ids) < 2 {
			c.diags.Add(diag.Errorf(sp, "`%s` needs at least two subnets", need.key).
				Suggest("load balancers and databases span two availability zones"))
		}
	}
	if c.prog.Network.Has("cidr") {
		c.diags.Add(diag.Errorf(c.prog.Network.At("cidr"), "`cidr` is for the network Clowd creates, but an existing %s network is set on line %d", cl, c.line(p.Pos)).
			Suggest("remove one of them"))
	}
	c.prog.Network.Existing = append(c.prog.Network.Existing, e)
}

// subnets reads a comma list of subnet IDs. IDs never contain commas, so
// `a,b` is split too.
func (c *checker) subnets(s ast.Property) []string {
	var ids []string
	if len(s.Commas) > 0 {
		items, ok := c.list(s)
		if !ok {
			return nil
		}
		ids = items
	} else {
		ids = []string{joinValues(s.Values)}
	}
	var out []string
	for _, item := range ids {
		for _, id := range strings.Split(item, ",") {
			if strings.ContainsAny(id, " ") || id == "" {
				c.diags.Add(diag.Errorf(s.Pos, "`%s` is not a subnet ID", item).
					Suggest("separate subnet IDs with commas: `%s subnet-a, subnet-b`", strings.ToLower(s.Key.Text)))
				return nil
			}
			out = append(out, id)
		}
	}
	return out
}
