// Package ast defines the syntax tree produced by the parser.
//
// The tree is deliberately close to the source: names are kept as written
// (case and all), cloud names and property keys are not validated, and
// missing pieces are represented rather than causing a parse failure, so
// later stages and the LSP can work with half-typed files.
//
// Fields named Pos (and KindPos, VerbPos) are source spans; the name avoids
// clashing with the Span method.
package ast

import (
	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/syntax/token"
)

// File is a parsed .clowd file.
type File struct {
	Stmts []Stmt
	// Comments in source order; the parser ignores them but the formatter
	// needs them.
	Comments []token.Token
}

// Stmt is a top-level statement: *Use, *Declare, *Relate, *On or *Bad.
type Stmt interface {
	Span() diag.Span
	stmt()
}

// Ident is a resource or cloud name as written. A missing name (for example
// `service` on its own) has an empty Name and a zero-width span where the
// name was expected.
type Ident struct {
	Name string
	Pos  diag.Span
}

// Missing reports whether the name was absent from the source.
func (id Ident) Missing() bool { return id.Name == "" }

// Value is a bare word or quoted string used as a value.
type Value struct {
	// Text is the word as written, or the unescaped contents of a string.
	Text   string
	Quoted bool
	Pos    diag.Span
}

// Use declares a target cloud: `use aws in eu-west-2`.
type Use struct {
	Pos   diag.Span
	Cloud Ident
	// Region is nil when there is no `in` clause.
	Region *Value
}

// Kind is a resource kind.
type Kind int

// Resource kinds. BadKind is never produced for a successfully parsed
// declaration; it exists so the zero value is not a real kind.
const (
	BadKind Kind = iota
	Service
	Database
	Bucket
	LoadBalancer
	Queue
)

var kindNames = [...]string{
	BadKind:      "<bad kind>",
	Service:      "service",
	Database:     "database",
	Bucket:       "bucket",
	LoadBalancer: "load balancer",
	Queue:        "queue",
}

// String returns the canonical spelling of the kind.
func (k Kind) String() string { return kindNames[k] }

// Declare declares a resource:
//
//	service api is public
//	  port 8080
type Declare struct {
	Pos     diag.Span
	Kind    Kind
	KindPos diag.Span
	Name    Ident
	Traits  []Trait
	Props   []Property
}

// Trait is one adjective after `is`; it may be several words (`postgres 16`).
type Trait struct {
	Words []Value
	Pos   diag.Span
}

// Property is an indented `<key> <value...>` line in a declaration.
// Values may be empty; check reports properties that need one.
type Property struct {
	Key    Value
	Values []Value
	Pos    diag.Span
}

// Verb is a relationship verb.
type Verb int

// Relationship verbs.
const (
	BadVerb Verb = iota
	ConnectsTo
	Reads
	Writes
	ReadsAndWrites
	SendsTo
)

var verbNames = [...]string{
	BadVerb:        "<bad verb>",
	ConnectsTo:     "connects to",
	Reads:          "reads",
	Writes:         "writes",
	ReadsAndWrites: "reads and writes",
	SendsTo:        "sends to",
}

// String returns the canonical spelling of the verb.
func (v Verb) String() string { return verbNames[v] }

// Relate is a relationship: `api connects to db`.
type Relate struct {
	Pos     diag.Span
	From    Ident
	Verb    Verb
	VerbPos diag.Span
	To      Ident
}

// On is an escape-hatch block applied only when lowering to Cloud:
//
//	on aws
//	  service api sets capacity_provider to FARGATE_SPOT
type On struct {
	Pos       diag.Span
	Cloud     Ident
	Overrides []Override
}

// Override sets a raw provider property on a generated resource.
type Override struct {
	Pos     diag.Span
	Kind    Kind
	KindPos diag.Span
	Name    Ident
	Key     Value
	Value   []Value
}

// Bad is a statement the parser could not make sense of. A diagnostic has
// already been reported for it.
type Bad struct {
	Pos diag.Span
}

func (s *Use) Span() diag.Span     { return s.Pos }
func (s *Declare) Span() diag.Span { return s.Pos }
func (s *Relate) Span() diag.Span  { return s.Pos }
func (s *On) Span() diag.Span      { return s.Pos }
func (s *Bad) Span() diag.Span     { return s.Pos }

func (*Use) stmt()     {}
func (*Declare) stmt() {}
func (*Relate) stmt()  {}
func (*On) stmt()      {}
func (*Bad) stmt()     {}
