package lsp

import (
	"fmt"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/capability"
	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/lower/aws"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// hover describes what is under offset: a resource and what it becomes
// on each target cloud, a keyword, trait or property and what it means,
// or a raw provider argument and the resource it sets.
func (s *server) hover(doc *document, offset int) *hover {
	text, sp := s.describe(doc, offset)
	if text == "" {
		return nil
	}
	r := toRange(doc.file, sp, s.enc)
	return &hover{Contents: markupContent{Kind: "markdown", Value: text}, Range: &r}
}

// in reports whether offset is in sp or at its end, so a hover just after
// a word, where the cursor sits after typing it, still finds it.
func in(sp diag.Span, offset int) bool {
	return sp.Start <= offset && offset <= sp.End && sp.Len() > 0
}

func (s *server) describe(doc *document, offset int) (string, diag.Span) {
	res := doc.analysis
	for _, ref := range res.Symbols.Refs {
		if ref.Decl != nil && in(ref.Pos, offset) {
			line := doc.file.Position(ref.Decl.Name.Pos.Start).Line
			return s.resource(doc, ref.Decl) + fmt.Sprintf("\n\nDeclared on line %d.", line), ref.Pos
		}
	}
	for _, st := range res.AST.Stmts {
		switch st := st.(type) {
		case *ast.Use:
			if in(st.Cloud.Pos, offset) {
				return cloudDoc(st.Cloud.Name), st.Cloud.Pos
			}
			if kw := (diag.Span{Start: st.Pos.Start, End: st.Pos.Start + len("use")}); in(kw, offset) {
				return keywordDocs["use"], kw
			}
		case *ast.Declare:
			if text, sp := s.describeDeclare(doc, st, offset); text != "" {
				return text, sp
			}
		case *ast.Relate:
			if in(st.VerbPos, offset) && st.Verb != ast.BadVerb {
				return s.relation(doc, st), st.VerbPos
			}
		case *ast.On:
			if in(st.Cloud.Pos, offset) {
				return cloudDoc(st.Cloud.Name), st.Cloud.Pos
			}
			if kw := (diag.Span{Start: st.Pos.Start, End: st.Pos.Start + len("on")}); in(kw, offset) {
				return keywordDocs["on"], kw
			}
			if text, sp := describeCloudSettings(st.Settings, nil, offset); text != "" {
				return text, sp
			}
		}
	}
	return "", diag.Span{}
}

func (s *server) describeDeclare(doc *document, d *ast.Declare, offset int) (string, diag.Span) {
	switch {
	case in(d.KindPos, offset):
		text := fmt.Sprintf("**%s**\n\n%s", d.Kind, kindDocs[d.Kind])
		if k := irKind(d.Kind); k != 0 {
			text += "\n\n" + s.products(doc, k)
		}
		return text, d.KindPos
	case in(d.Name.Pos, offset):
		if d.Kind.IsResource() {
			return s.resource(doc, d), d.Name.Pos
		}
		return fmt.Sprintf("**%s %s**\n\n%s", d.Kind, d.Name.Name, kindDocs[d.Kind]), d.Name.Pos
	}
	for _, t := range d.Traits {
		if in(t.Pos, offset) {
			if text := traitDoc(d.Kind, t); text != "" {
				return text, t.Pos
			}
		}
	}
	for _, p := range d.Props {
		if isWord(p.Key, "on") {
			if in(p.Key.Pos, offset) {
				return keywordDocs["on"], p.Key.Pos
			}
			if len(p.Values) > 0 && in(p.Values[0].Pos, offset) {
				return cloudDoc(p.Values[0].Text), p.Values[0].Pos
			}
			if len(p.Values) == 1 {
				if text, sp := describeOverrides(d, p.Values[0].Text, p.Block, nil, offset); text != "" {
					return text, sp
				}
			}
			continue
		}
		if in(p.Key.Pos, offset) {
			key := strings.ToLower(p.Key.Text)
			if text, ok := propDocs[d.Kind][key]; ok {
				return fmt.Sprintf("**%s** %s\n\n%s", d.Kind, propName(d.Kind, key), text), p.Key.Pos
			}
		}
	}
	return "", diag.Span{}
}

// propName is a property as written, which may be a phrase.
func propName(k ast.Kind, key string) string {
	for _, p := range vocabulary(k).Props {
		if p.Key == key {
			return fmt.Sprintf("`%s`", p.Name)
		}
	}
	return fmt.Sprintf("`%s`", key)
}

func traitDoc(k ast.Kind, t ast.Trait) string {
	var words []string
	for _, w := range t.Words {
		words = append(words, strings.ToLower(w.Text))
	}
	phrase := strings.Join(words, " ")
	if text, ok := traitDocs[k][phrase]; ok {
		return fmt.Sprintf("**%s** trait `%s`\n\n%s", k, phrase, text)
	}
	// A versioned trait: `postgres 16`.
	if text, ok := traitDocs[k][words[0]]; ok {
		return fmt.Sprintf("**%s** trait `%s`\n\n%s", k, phrase, text)
	}
	return ""
}

// resource describes a declared resource: what it becomes on each target
// cloud, and the matrix's notes about it.
func (s *server) resource(doc *document, d *ast.Declare) string {
	text := fmt.Sprintf("**%s %s**\n\n%s", d.Kind, d.Name.Name, s.products(doc, irKind(d.Kind)))
	return text + s.notes(doc, d.Name.Pos)
}

func (s *server) relation(doc *document, r *ast.Relate) string {
	text := fmt.Sprintf("**%s**\n\n%s", r.Verb, verbDocs[r.Verb])
	return text + s.notes(doc, r.Pos)
}

// clouds are the clouds hovers describe: the file's targets, or every
// cloud with a backend's products when it has none yet.
func clouds(doc *document) []ir.Cloud {
	var cs []ir.Cloud
	for _, t := range doc.analysis.Program.Targets {
		cs = append(cs, t.Cloud)
	}
	if len(cs) == 0 {
		cs = []ir.Cloud{ir.AWS, ir.GCP}
	}
	return cs
}

// products is what kind k becomes on each cloud:
// "AWS: ECS Fargate · GCP: Cloud Run".
func (s *server) products(doc *document, k ir.Kind) string {
	var parts []string
	for _, c := range clouds(doc) {
		p := capability.Product(k, c)
		if p == "" {
			p = "not implemented yet"
		}
		parts = append(parts, fmt.Sprintf("%s: %s", cloudTitle(c), p))
	}
	return strings.Join(parts, " · ")
}

// notes lists the capability matrix's findings for the row at pos. The
// matrix is evaluated even when the file has errors: the IR is always
// usable (D9), and the notes are still true of what was written.
func (s *server) notes(doc *document, pos diag.Span) string {
	cs := clouds(doc)
	rep := capability.Evaluate(doc.analysis.Program, cs)
	var lines []string
	for _, row := range rep.Rows {
		if row.Pos != pos {
			continue
		}
		for i, cell := range row.Cells {
			for _, f := range cell.Findings {
				note := f.Note
				if f.Level == capability.NotImplemented {
					note = fmt.Sprintf("%s is not implemented yet", f.Feature)
				}
				lines = append(lines, fmt.Sprintf("- %s **%s**: %s", f.Level.Symbol(), cs[i], note))
			}
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(lines, "\n")
}

// describeOverrides describes a raw provider argument in a resource's
// `on <cloud>` block. path is the nested blocks above settings.
func describeOverrides(d *ast.Declare, cloud string, settings []ast.Property, path []string, offset int) (string, diag.Span) {
	c, ok := ir.ParseCloud(cloud)
	if !ok {
		return "", diag.Span{}
	}
	for _, p := range settings {
		key := strings.ToLower(p.Key.Text)
		if in(p.Key.Pos, offset) {
			if c != ir.AWS {
				return fmt.Sprintf("`%s`: a %s provider argument, applied only when deploying to %s.", p.Key.Text, cloudTitle(c), cloudTitle(c)), p.Key.Pos
			}
			resource, _ := aws.Overridable(irKind(d.Kind))
			where := fmt.Sprintf("`%s`", resource)
			if len(path) > 0 {
				where = fmt.Sprintf("the `%s` block of `%s`", strings.Join(path, "."), resource)
			}
			return fmt.Sprintf("`%s`: an argument of %s, passed to the provider as written and applied only when deploying to AWS.\n\n[%s documentation](%s)",
				p.Key.Text, where, resource, registryURL(resource)), p.Key.Pos
		}
		if text, sp := describeOverrides(d, cloud, p.Block, append(slices.Clone(path), key), offset); text != "" {
			return text, sp
		}
	}
	return "", diag.Span{}
}

// describeCloudSettings describes a setting in a top-level `on` block.
func describeCloudSettings(settings []ast.Property, path []string, offset int) (string, diag.Span) {
	docs := map[string]string{
		"network":         "Uses a network that already exists instead of creating one. It replaces the `network` declaration's `cidr`.",
		"vpc":             "The ID of the existing VPC.",
		"public_subnets":  "Subnets for load balancers (and public services): two or more, in different availability zones, separated by `, `.",
		"private_subnets": "Subnets for databases (and private services): two or more, in different availability zones, separated by `, `.",
	}
	for _, p := range settings {
		key := strings.ToLower(p.Key.Text)
		if in(p.Key.Pos, offset) {
			if text, ok := docs[key]; ok {
				return fmt.Sprintf("`%s`\n\n%s", key, text), p.Key.Pos
			}
			return "", diag.Span{}
		}
		if text, sp := describeCloudSettings(p.Block, append(slices.Clone(path), key), offset); text != "" {
			return text, sp
		}
	}
	return "", diag.Span{}
}

// registryURL is the provider documentation page for an AWS resource.
func registryURL(resource string) string {
	return "https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/" + strings.TrimPrefix(resource, "aws_")
}

func cloudDoc(name string) string {
	c, ok := ir.ParseCloud(name)
	if !ok {
		return ""
	}
	full := map[ir.Cloud]string{
		ir.AWS:   "Amazon Web Services",
		ir.GCP:   "Google Cloud",
		ir.Azure: "Microsoft Azure. Known to the language, but Clowd can't deploy to it yet.",
	}[c]
	return fmt.Sprintf("**%s**: %s", c, full)
}

func cloudTitle(c ir.Cloud) string {
	switch c {
	case ir.AWS:
		return "AWS"
	case ir.GCP:
		return "GCP"
	case ir.Azure:
		return "Azure"
	}
	return c.String()
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

func isWord(v ast.Value, w string) bool { return !v.Quoted && strings.EqualFold(v.Text, w) }
