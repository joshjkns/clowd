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
	// Project and Network are program-wide declarations, not resources:
	// nothing can refer to them, and each appears at most once.
	Project
	Network
)

var kindNames = [...]string{
	BadKind:      "<bad kind>",
	Service:      "service",
	Database:     "database",
	Bucket:       "bucket",
	LoadBalancer: "load balancer",
	Queue:        "queue",
	Project:      "project",
	Network:      "network",
}

// String returns the canonical spelling of the kind.
func (k Kind) String() string { return kindNames[k] }

// IsResource reports whether declarations of k are resources, which have
// names that relationships and routes refer to.
func (k Kind) IsResource() bool { return k >= Service && k <= Queue }

// Named reports whether a declaration of k has a name. A program has one
// network, so `network` takes none.
func (k Kind) Named() bool { return k != Network }

// Declare declares a resource, the project or the network:
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

// Property is an indented `<key> <value...>` line, optionally with an
// indented block of properties under it:
//
//	service api
//	  port 8080
//	  on aws
//	    platform_version LATEST
//
// Values may be empty; check reports properties that need one, and
// blocks under properties that don't take one.
type Property struct {
	Key    Value
	Values []Value
	// Commas are the commas between values. Only settings inside an `on`
	// block may have them, to write a list (`public_subnets a, b`).
	Commas []diag.Span
	Block  []Property
	// Pos covers the line, not the block.
	Pos diag.Span
}

// HasBlock reports whether an indented block follows the property line.
func (p Property) HasBlock() bool { return len(p.Block) > 0 }

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

// On is a top-level block of program-wide settings for one cloud:
//
//	on aws
//	  network
//	    vpc vpc-0123
//
// Settings for one resource go in an `on` property inside its declaration
// instead. Settings may contain commas and nested blocks.
type On struct {
	Pos      diag.Span
	Cloud    Ident
	Settings []Property
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
