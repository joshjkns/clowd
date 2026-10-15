// Package override applies `on <cloud>` blocks to the resources a
// backend generated (D13, D16).
//
// Each backend describes, per kind, the one resource an `on` block sets
// arguments on and an allowlist of those arguments. This package checks
// settings against that allowlist and merges them into the generated
// body, so every backend reports mistakes the same way. It knows nothing
// about any one cloud.
package override

import (
	"maps"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/spell"
)

// Arg is an argument an `on` block may set: a single value, a list, or a
// nested block with arguments of its own.
type Arg struct {
	// Block, when set, makes the argument a nested block.
	Block map[string]Arg
	// Repeat allows the block more than once, as a list of blocks.
	Repeat bool
	// List makes the argument take a comma list.
	List bool
}

// Scalars returns single-valued arguments named keys.
func Scalars(keys ...string) map[string]Arg {
	m := make(map[string]Arg, len(keys))
	for _, k := range keys {
		m[k] = Arg{}
	}
	return m
}

// With adds a to m as key and returns m.
func With(m map[string]Arg, key string, a Arg) map[string]Arg {
	m[key] = a
	return m
}

// Spec is the resource an `on` block in one kind of resource sets.
type Spec struct {
	// Resource is the provider resource type, such as aws_ecs_service.
	Resource string
	Args     map[string]Arg
	// Wiring are arguments the backend sets to connect what it
	// generates, which an `on` block can't change.
	Wiring []string
	// Portable maps arguments to the portable form that sets them (D18),
	// such as `the versioned trait`.
	Portable map[string]string
}

// Keys returns the arguments at path in s (nested block names, any
// case), for completion and hovers; a path that isn't a block has none.
func (s Spec) Keys(path ...string) []string {
	args := s.Args
	for _, name := range path {
		args = args[strings.ToLower(name)].Block
	}
	return slices.Sorted(maps.Keys(args))
}

// Apply sets settings on body, which holds the arguments of a resource
// of s.Resource, and reports settings the spec doesn't allow. Values are
// written as strings; the provider converts "14" and "true" to the
// argument's type and reports a value that doesn't fit at plan time.
func Apply(body tfjson.Body, settings []ir.Setting, s Spec, cloud ir.Cloud) diag.List {
	a := applier{spec: s, cloud: cloud}
	a.apply(body, settings, s.Args, s.Resource)
	return a.diags
}

type applier struct {
	spec  Spec
	cloud ir.Cloud
	diags diag.List
}

// apply sets settings on body, a block of resource whose arguments are
// args.
func (a *applier) apply(body tfjson.Body, settings []ir.Setting, args map[string]Arg, resource string) {
	replaced := map[string]bool{}
	for _, s := range settings {
		key := strings.ToLower(s.Key)
		arg, ok := args[key]
		if !ok {
			a.diags.Add(a.unknownKey(s, resource, args))
			continue
		}
		if arg.Block == nil {
			switch {
			case s.Block != nil:
				a.diags.Add(diag.Errorf(s.KeyPos, "`%s` takes a value, not a block", s.Key).
					Suggest("write `%s <value>` on one line", key))
			case arg.List && s.List != nil:
				body[key] = s.List
			case arg.List:
				body[key] = []string{s.Value}
			case s.List != nil:
				a.diags.Add(diag.Errorf(s.Pos, "`%s` takes one value, not a list", s.Key))
			default:
				body[key] = s.Value
			}
			continue
		}
		if s.Block == nil {
			a.diags.Add(diag.Errorf(s.Pos, "`%s` is a block", s.Key).
				Suggest("indent its settings under it, such as `%s`", firstKey(arg.Block)))
			continue
		}
		inner := tfjson.Body{}
		if !arg.Repeat {
			// Merge into what Clowd set, so one setting can change one
			// field of a block such as deployment_circuit_breaker.
			if old, ok := body[key].(tfjson.Body); ok {
				inner = old
			}
		}
		a.apply(inner, s.Block, arg.Block, resource+" "+key)
		switch {
		case !arg.Repeat:
			body[key] = inner
		case !replaced[key]:
			// The first block replaces whatever Clowd generated.
			body[key] = []tfjson.Body{inner}
			replaced[key] = true
		default:
			body[key] = append(body[key].([]tfjson.Body), inner)
		}
	}
}

func firstKey(args map[string]Arg) string {
	return slices.Sorted(maps.Keys(args))[0] + " <value>"
}

func (a *applier) unknownKey(s ir.Setting, resource string, args map[string]Arg) diag.Diagnostic {
	key := strings.ToLower(s.Key)
	top := a.spec.Resource
	d := diag.Errorf(s.KeyPos, "%s has no setting `%s` that `on %s` can change", resource, s.Key, a.cloud)
	nested := resource != top
	if p, ok := a.spec.Portable[key]; ok && !nested {
		return d.Suggest("use %s instead, which works on every cloud", p)
	}
	if slices.Contains(a.spec.Wiring, key) && !nested {
		return d.Suggest("Clowd sets `%s` to connect %s to the rest of the program", key, top)
	}
	names := slices.Sorted(maps.Keys(args))
	if g, ok := spell.Closest(s.Key, names); ok {
		return d.Suggest("did you mean `%s`?", g)
	}
	// The commonest miss is a field of a nested block, such as
	// capacity_provider in capacity_provider_strategy.
	for _, name := range names {
		if arg := args[name]; arg.Block != nil {
			if _, ok := arg.Block[key]; ok {
				return d.Suggest("`%s` goes in a `%s` block", key, name)
			}
		}
	}
	return d.Suggest("%s settings that can be changed: %s", resource, strings.Join(names, ", "))
}
