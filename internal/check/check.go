// Package check validates a resolved file and builds its IR.
//
// Checking and building the IR are one pass (decision D9): every value has
// to be interpreted to be validated (`2 to 10`, `512mb`, `on 443 with tls
// for ...`), and doing it once means the IR cannot disagree with what check
// accepted. Problems are reported as diagnostics and the offending piece is
// left out of the IR (or given its default), so the IR is always usable for
// the LSP even when the file has errors.
package check

import (
	"fmt"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/resolve"
	"github.com/joshjkns/clowd/internal/spell"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// Check validates f, whose names have been resolved into t, and returns its
// IR. The Program is never nil.
func Check(src *diag.File, f *ast.File, t *resolve.Table) (*ir.Program, diag.List) {
	c := &checker{src: src, table: t, prog: &ir.Program{}}
	for _, s := range f.Stmts {
		if u, ok := s.(*ast.Use); ok {
			c.use(u)
		}
	}
	for _, s := range f.Stmts {
		if d, ok := s.(*ast.Declare); ok && !d.Kind.IsResource() {
			c.programWide(d)
		}
	}
	for _, d := range t.Decls {
		c.declare(d)
	}
	for _, s := range f.Stmts {
		switch s := s.(type) {
		case *ast.Relate:
			c.relate(s)
		case *ast.On:
			c.on(s)
		}
	}
	if len(c.prog.Targets) == 0 && len(f.Stmts) > 0 && !c.badUse {
		c.diags.Add(diag.Warnf(diag.Span{}, "this file has no `use` line, so it deploys nowhere").
			Suggest("add a target, for example `use aws in eu-west-2`"))
	}
	c.diags.Sort()
	return c.prog, c.diags
}

type checker struct {
	src   *diag.File
	table *resolve.Table
	prog  *ir.Program
	diags diag.List
	// badUse is set when a `use` line was rejected, so the missing-target
	// warning does not pile onto that error.
	badUse bool
}

func (c *checker) errorf(sp diag.Span, format string, args ...any) diag.Diagnostic {
	return diag.Errorf(sp, format, args...)
}

func (c *checker) line(sp diag.Span) int { return c.src.Position(sp.Start).Line }

// cloudSynonyms are product and company names people write for a cloud.
var cloudSynonyms = map[string]ir.Cloud{
	"amazon":    ir.AWS,
	"ec2":       ir.AWS,
	"google":    ir.GCP,
	"gcloud":    ir.GCP,
	"gce":       ir.GCP,
	"microsoft": ir.Azure,
	"az":        ir.Azure,
}

// cloud interprets a cloud name, reporting unknown ones. A synonym is
// reported but accepted, so the rest of the file still checks.
func (c *checker) cloud(id ast.Ident) (ir.Cloud, bool) {
	if id.Missing() {
		return 0, false // the parser has reported it
	}
	if cl, ok := ir.ParseCloud(id.Name); ok {
		return cl, true
	}
	d := diag.Errorf(id.Pos, "unknown cloud `%s`", id.Name)
	var names []string
	for _, cl := range ir.Clouds {
		names = append(names, cl.String())
	}
	if cl, ok := cloudSynonyms[strings.ToLower(id.Name)]; ok {
		c.diags.Add(d.Suggest("write `%s`", cl))
		return cl, true
	}
	if guess, ok := spell.Closest(id.Name, names); ok {
		c.diags.Add(d.Suggest("did you mean `%s`?", guess))
		cl, _ := ir.ParseCloud(guess)
		return cl, true
	}
	c.diags.Add(d.Suggest("clouds are %s", codeList(names, "and")))
	return 0, false
}

func (c *checker) use(u *ast.Use) {
	cl, ok := c.cloud(u.Cloud)
	if !ok {
		c.badUse = true
		return
	}
	for _, t := range c.prog.Targets {
		if t.Cloud == cl {
			c.diags.Add(diag.Errorf(u.Pos, "`%s` is already a target on line %d", cl, c.line(t.Pos)).
				Suggest("deploying one cloud to several regions is not supported yet; remove one of the `use %s` lines", cl))
			return
		}
	}
	t := ir.Target{Cloud: cl, Pos: u.Pos}
	if u.Region != nil {
		t.Region = u.Region.Text
	}
	c.prog.Targets = append(c.prog.Targets, t)
}

func irKind(k ast.Kind) ir.Kind {
	switch k {
	case ast.Service:
		return ir.ServiceKind
	case ast.Database:
		return ir.DatabaseKind
	case ast.Bucket:
		return ir.BucketKind
	case ast.LoadBalancer:
		return ir.LoadBalancerKind
	case ast.Queue:
		return ir.QueueKind
	}
	return 0
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

// relations lists, for each verb, the kinds a service may relate to with
// it. Only services start relationships: load balancers route with `send`,
// and data stores are passive.
var relations = map[ir.Verb][]ir.Kind{
	ir.ConnectsTo:     {ir.DatabaseKind, ir.ServiceKind},
	ir.Reads:          {ir.BucketKind, ir.QueueKind},
	ir.Writes:         {ir.BucketKind},
	ir.ReadsAndWrites: {ir.BucketKind},
	ir.SendsTo:        {ir.QueueKind},
}

func validRelation(from ir.Kind, v ir.Verb, to ir.Kind) bool {
	return from == ir.ServiceKind && slices.Contains(relations[v], to)
}

// relationHint suggests the right phrasing when a service uses a verb with a
// kind it doesn't apply to.
func relationHint(from string, v ir.Verb, to string, toKind ir.Kind) string {
	switch {
	case toKind == ir.QueueKind && v == ir.Writes:
		return fmt.Sprintf("to publish to a queue, write `%s sends to %s`", from, to)
	case toKind == ir.QueueKind && v == ir.ReadsAndWrites:
		return fmt.Sprintf("write `%s reads %s` and `%s sends to %s` on separate lines", from, to, from, to)
	case toKind == ir.QueueKind && v == ir.ConnectsTo:
		return fmt.Sprintf("write `%s reads %s` to consume or `%s sends to %s` to publish", from, to, from, to)
	case toKind == ir.BucketKind && v == ir.SendsTo:
		return fmt.Sprintf("write `%s writes %s`", from, to)
	case toKind == ir.BucketKind && v == ir.ConnectsTo:
		return fmt.Sprintf("say what access it needs: `%s reads %s`, `%s writes %s` or `%s reads and writes %s`", from, to, from, to, from, to)
	case toKind == ir.DatabaseKind:
		return fmt.Sprintf("write `%s connects to %s`", from, to)
	case toKind == ir.LoadBalancerKind:
		return "load balancers send traffic to services with `send <path> to <service>`, not the other way round"
	}
	return ""
}

func (c *checker) relate(s *ast.Relate) {
	if s.Verb == ast.BadVerb || s.From.Missing() || s.To.Missing() {
		return
	}
	from, to := c.table.Lookup(s.From.Name), c.table.Lookup(s.To.Name)
	if from == nil || to == nil {
		return // resolve has reported it
	}
	v := irVerb(s.Verb)
	fk, tk := irKind(from.Kind), irKind(to.Kind)
	fn, tn := from.Name.Name, to.Name.Name

	switch {
	case from == to:
		c.diags.Add(diag.Errorf(s.Pos, "`%s` can't relate to itself", fn))
		return
	case fk != ir.ServiceKind:
		d := diag.Errorf(s.From.Pos, "only a service can start a relationship; `%s` is a %s", fn, fk)
		if validRelation(tk, v, fk) {
			d = d.Suggest("relationships go from the service: `%s %s %s`", tn, v, fn)
		} else if fk == ir.LoadBalancerKind && tk == ir.ServiceKind {
			d = d.Suggest("route to it with `send / to %s` in the load balancer's block", tn)
		}
		c.diags.Add(d)
		return
	case !validRelation(fk, v, tk):
		d := diag.Errorf(s.VerbPos, "`%s` doesn't apply to a %s", v, tk)
		if hint := relationHint(fn, v, tn, tk); hint != "" {
			d = d.Suggest("%s", hint)
		}
		c.diags.Add(d)
		return
	}

	for _, r := range c.prog.Relations {
		if r.From == fn && r.Verb == v && r.To == tn {
			c.diags.Add(diag.Warnf(s.Pos, "`%s %s %s` is already declared on line %d", fn, v, tn, c.line(r.Pos)).
				Suggest("remove one of them"))
			return
		}
	}
	c.prog.Relations = append(c.prog.Relations, ir.Relation{From: fn, Verb: v, To: tn, Pos: s.Pos})
}

// codeList formats words as "`a`, `b` and `c`", joining the last two with
// conj.
func codeList(words []string, conj string) string {
	q := make([]string, len(words))
	for i, w := range words {
		q[i] = "`" + w + "`"
	}
	if len(q) <= 1 {
		return strings.Join(q, "")
	}
	return strings.Join(q[:len(q)-1], ", ") + " " + conj + " " + q[len(q)-1]
}
