package parser

import (
	"strings"

	"github.com/joshjkns/clowd/internal/spell"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// phrase is a keyword written as one or more words.
type phrase[T any] struct {
	words []string
	value T
}

// verbs are the canonical relationship verbs, in the order the spec lists
// them. Matching picks the longest phrase, so order does not affect parsing.
var verbs = []phrase[ast.Verb]{
	{[]string{"connects", "to"}, ast.ConnectsTo},
	{[]string{"reads"}, ast.Reads},
	{[]string{"writes"}, ast.Writes},
	{[]string{"reads", "and", "writes"}, ast.ReadsAndWrites},
	{[]string{"sends", "to"}, ast.SendsTo},
}

// verbSynonyms are plausible wrong guesses. They are accepted with an error
// that suggests the canonical verb (design rule 2: one canonical phrasing),
// so the rest of the file still resolves.
var verbSynonyms = []phrase[ast.Verb]{
	{[]string{"connects"}, ast.ConnectsTo},
	{[]string{"connect", "to"}, ast.ConnectsTo},
	{[]string{"connects", "with"}, ast.ConnectsTo},
	{[]string{"links", "to"}, ast.ConnectsTo},
	{[]string{"links"}, ast.ConnectsTo},
	{[]string{"talks", "to"}, ast.ConnectsTo},
	{[]string{"depends", "on"}, ast.ConnectsTo},
	{[]string{"uses"}, ast.ConnectsTo},
	{[]string{"calls"}, ast.ConnectsTo},
	{[]string{"reads", "from"}, ast.Reads},
	{[]string{"read"}, ast.Reads},
	{[]string{"writes", "to"}, ast.Writes},
	{[]string{"write"}, ast.Writes},
	{[]string{"reads", "writes"}, ast.ReadsAndWrites},
	{[]string{"reads/writes"}, ast.ReadsAndWrites},
	{[]string{"read", "and", "write"}, ast.ReadsAndWrites},
	{[]string{"writes", "and", "reads"}, ast.ReadsAndWrites},
	{[]string{"sends"}, ast.SendsTo},
	{[]string{"send", "to"}, ast.SendsTo},
	{[]string{"publishes", "to"}, ast.SendsTo},
}

// kinds are the canonical resource kinds.
var kinds = []phrase[ast.Kind]{
	{[]string{"service"}, ast.Service},
	{[]string{"database"}, ast.Database},
	{[]string{"bucket"}, ast.Bucket},
	{[]string{"load", "balancer"}, ast.LoadBalancer},
	{[]string{"lb"}, ast.LoadBalancer},
	{[]string{"queue"}, ast.Queue},
}

// kindSynonyms map product names and near-misses to a kind.
var kindSynonyms = map[string]ast.Kind{
	"server":        ast.Service,
	"app":           ast.Service,
	"container":     ast.Service,
	"svc":           ast.Service,
	"db":            ast.Database,
	"postgres":      ast.Database,
	"mysql":         ast.Database,
	"rds":           ast.Database,
	"sql":           ast.Database,
	"storage":       ast.Bucket,
	"s3":            ast.Bucket,
	"gcs":           ast.Bucket,
	"load":          ast.LoadBalancer,
	"loadbalancer":  ast.LoadBalancer,
	"load-balancer": ast.LoadBalancer,
	"load_balancer": ast.LoadBalancer,
	"alb":           ast.LoadBalancer,
	"elb":           ast.LoadBalancer,
	"sqs":           ast.Queue,
	"pubsub":        ast.Queue,
	"topic":         ast.Queue,
}

// guessKind returns the kind the user probably meant by word: a known
// synonym, or a close misspelling of a kind.
func guessKind(word string) (ast.Kind, bool) {
	w := strings.ToLower(word)
	if k, ok := kindSynonyms[w]; ok {
		return k, true
	}
	for _, k := range kinds {
		if len(k.words) == 1 && len(k.words[0]) > 2 && spell.IsTypo(w, k.words[0]) {
			return k.value, true
		}
	}
	return ast.BadKind, false
}

// guessVerb returns the verb whose first word is a close misspelling of word.
func guessVerb(word string) (ast.Verb, bool) {
	w := strings.ToLower(word)
	for _, v := range verbs {
		if spell.IsTypo(w, v.words[0]) {
			return v.value, true
		}
	}
	return ast.BadVerb, false
}
