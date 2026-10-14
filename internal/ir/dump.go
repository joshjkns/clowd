package ir

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
)

// Dump renders p as indented text with line:column positions, for golden
// tests and debugging. The format is not stable API.
func Dump(src *diag.File, p *Program) string {
	var b strings.Builder
	pos := func(sp diag.Span) string {
		s := src.Position(sp.Start)
		return fmt.Sprintf("%d:%d", s.Line, s.Column)
	}
	line := func(indent int, format string, args ...any) {
		b.WriteString(strings.Repeat("  ", indent))
		fmt.Fprintf(&b, format, args...)
		b.WriteByte('\n')
	}

	for _, t := range p.Targets {
		region := t.Region
		if region == "" {
			region = "<default>"
		}
		line(0, "target %s %s %s", t.Cloud, region, pos(t.Pos))
	}
	if pr := p.Project; pr.Name != "" || len(pr.Tags) > 0 || pr.Edition != 0 {
		line(0, "project %q edition %d", pr.Name, pr.Edition)
		for _, t := range pr.Tags {
			line(1, "tag %s=%q %s", t.Key, t.Value, pos(t.Pos))
		}
	}
	if n := p.Network; n.Pos != (diag.Span{}) || len(n.Existing) > 0 {
		line(0, "network private %t cidr %q", n.Private, n.CIDR)
		for _, e := range n.Existing {
			line(1, "existing %s %s public %q private %q %s", e.Cloud, e.VPC, e.PublicSubnets, e.PrivateSubnets, pos(e.Pos))
		}
	}
	for _, r := range p.Resources {
		h := r.Meta()
		line(0, "%s %s %s", r.Kind(), h.Name, pos(h.Pos))
		switch r := r.(type) {
		case *Service:
			line(1, "image %q", r.Image)
			line(1, "port %d", r.Port)
			line(1, "scale %d to %d", r.Scale.Min, r.Scale.Max)
			if r.CPU != 0 {
				line(1, "cpu %dm", r.CPU)
			}
			if r.Memory != 0 {
				line(1, "memory %dMiB", r.Memory)
			}
			for _, e := range r.Env {
				line(1, "env %s=%q", e.Name, e.Value)
			}
		case *Database:
			version := r.Version
			if version == "" {
				version = "<default>"
			}
			engine := "<none>"
			if r.Engine != 0 {
				engine = r.Engine.String()
			}
			line(1, "engine %s %s", engine, version)
			line(1, "size %s", r.Size)
		case *Bucket:
			line(1, "public %t", r.Public)
		case *Queue:
			line(1, "fifo %t", r.FIFO)
		case *LoadBalancer:
			line(1, "internal %t", r.Internal)
			for _, l := range r.Listeners {
				if l.TLS() {
					line(1, "listen %d tls %s", l.Port, l.Domain)
				} else {
					line(1, "listen %d", l.Port)
				}
			}
			for _, rt := range r.Routes {
				line(1, "route %s -> %s", rt.Path, rt.Service)
			}
		}
		for _, k := range slices.Sorted(maps.Keys(h.Spans)) {
			line(1, "at %s %s", k, pos(h.Spans[k]))
		}
	}
	for _, rel := range p.Relations {
		line(0, "relation %s %s %s %s", rel.From, rel.Verb, rel.To, pos(rel.Pos))
	}
	var settings func(indent int, ss []Setting)
	settings = func(indent int, ss []Setting) {
		for _, s := range ss {
			switch {
			case s.Block != nil:
				line(indent, "%s %s", s.Key, pos(s.Pos))
				settings(indent+1, s.Block)
			case s.List != nil:
				line(indent, "%s = %q %s", s.Key, s.List, pos(s.Pos))
			default:
				line(indent, "%s = %q %s", s.Key, s.Value, pos(s.Pos))
			}
		}
	}
	for _, o := range p.Overrides {
		line(0, "override %s %s %s %s", o.Cloud, o.Kind, o.Resource, pos(o.Pos))
		settings(1, o.Settings)
	}
	return b.String()
}
