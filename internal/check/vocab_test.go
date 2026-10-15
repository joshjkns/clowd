package check

import (
	"strings"
	"testing"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

func TestVocabularyOf(t *testing.T) {
	describe := func(v Vocabulary) string {
		var parts []string
		for _, g := range v.Traits {
			s := strings.Join(g.Words, "|")
			if g.Versioned {
				s += " <version>"
			}
			parts = append(parts, s)
		}
		for _, p := range v.Props {
			s := p.Name
			if p.Repeat {
				s += "*"
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, ", ")
	}
	tests := []struct {
		kind ast.Kind
		want string
	}{
		{ast.Service, "command, cpu, env*, health check, image, memory, port, scale"},
		{ast.Database, "postgres|mysql <version>, small|medium|large, highly available, protected, backups, storage"},
		{ast.Bucket, "private|public, versioned, protected, expire after"},
		{ast.LoadBalancer, "public|internal, listen*, send*"},
		{ast.Queue, "fifo, keep messages for, retry, timeout"},
		{ast.Project, "edition, state, tag*"},
		{ast.Network, "public|private, cidr"},
	}
	for _, tt := range tests {
		if got := describe(VocabularyOf(tt.kind)); got != tt.want {
			t.Errorf("VocabularyOf(%s) = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

// Completion inserts the examples, so each kind's examples, all set at
// once, must check cleanly.
func TestVocabularyExamplesCheck(t *testing.T) {
	heads := map[ast.Kind]string{
		ast.Service:      "service api",
		ast.Database:     "database db is postgres",
		ast.Bucket:       "bucket b",
		ast.LoadBalancer: "load balancer web",
		ast.Queue:        "queue q",
		ast.Project:      "project demo",
		ast.Network:      "network",
	}
	for kind, head := range heads {
		src := "use aws\n" + head + "\n"
		for _, p := range VocabularyOf(kind).Props {
			src += "  " + p.Example + "\n"
		}
		if kind != ast.Service {
			src += "service api\n  image a\n" // for `send / to api`
		}
		f, _, earlier, diags := run("t.clowd", src)
		if all := append(earlier, diags...); len(all) > 0 {
			var b strings.Builder
			_ = diag.RenderAll(&b, f, all)
			t.Errorf("%s examples don't check:\n%s", kind, b.String())
		}
	}
}

func TestCloudSettings(t *testing.T) {
	if got := strings.Join(CloudSettings(ir.AWS), ","); got != "network" {
		t.Errorf("CloudSettings(aws) = %q, want network", got)
	}
}
