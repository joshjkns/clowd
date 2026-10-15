package check

import (
	"maps"
	"slices"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/resolve"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// Vocabulary is what a declaration of one kind may contain, for editor
// completion and hovers.
type Vocabulary struct {
	// Traits are the kind's trait groups; traits in one group exclude
	// each other.
	Traits []TraitGroup
	// Props are sorted by key.
	Props []Prop
}

// TraitGroup is a set of mutually exclusive traits, such as `public` and
// `private`.
type TraitGroup struct {
	Words []string
	// Versioned groups take an optional version (`postgres 16`).
	Versioned bool
}

// Prop is a property a declaration may set.
type Prop struct {
	// Key is the property's first word, lowercase.
	Key string
	// Name is the property as written, which may be a phrase
	// (`health check`).
	Name    string
	Example string
	// Repeat properties may be set more than once (`env`).
	Repeat bool
}

// VocabularyOf returns the traits and properties a declaration of kind k
// accepts. It is read from the checker itself, which checks an empty
// declaration while recording what it would accept instead of checking
// it, so it can't drift from what check accepts.
func VocabularyOf(k ast.Kind) Vocabulary {
	var v Vocabulary
	c := &checker{src: diag.NewFile("", ""), table: &resolve.Table{}, prog: &ir.Program{}, vocab: &v}
	d := &ast.Declare{Kind: k}
	if k.IsResource() {
		c.declare(d)
	} else {
		c.programWide(d)
	}
	return v
}

// recordTraits and recordProps store what a kind accepts in c.vocab.
func (c *checker) recordTraits(groups []traitGroup) {
	for _, g := range groups {
		c.vocab.Traits = append(c.vocab.Traits, TraitGroup{Words: g.words, Versioned: g.versioned})
	}
}

func (c *checker) recordProps(specs map[string]propSpec) {
	for _, key := range slices.Sorted(maps.Keys(specs)) {
		s := specs[key]
		name := key
		if s.name != "" {
			name = s.name
		}
		c.vocab.Props = append(c.vocab.Props, Prop{Key: key, Name: name, Example: s.example, Repeat: s.repeat})
	}
}

// CloudSettings returns the settings a top-level `on <cloud>` block
// accepts.
func CloudSettings(cl ir.Cloud) []string { return slices.Clone(cloudSettings[cl]) }

// NetworkSettings returns the settings of an existing network, in an
// `on <cloud>` `network` block.
func NetworkSettings() []string { return slices.Clone(networkSettings) }

// RelationTargets returns the kinds a service may relate to with v.
func RelationTargets(v ir.Verb) []ir.Kind { return slices.Clone(relations[v]) }
