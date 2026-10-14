package compile

import (
	"testing"

	"github.com/joshjkns/clowd/internal/diag"
	_ "github.com/joshjkns/clowd/internal/golden" // defines -update
)

func TestCheckCollectsEveryStage(t *testing.T) {
	src := "use aws\nservice api\n  image a\napi connects to dbb\nservice\nbucket b is shiny\n"
	r := Check(diag.NewFile("t.clowd", src))
	want := []string{
		"no resource named `dbb`",         // resolve
		"expected a name after `service`", // parser
		"unknown bucket trait `shiny`",    // check
	}
	if len(r.Diags) != len(want) {
		t.Fatalf("got %d diagnostics, want %d: %v", len(r.Diags), len(want), r.Diags)
	}
	for i, d := range r.Diags {
		if d.Message != want[i] {
			t.Errorf("diagnostic %d = %q, want %q", i, d.Message, want[i])
		}
	}
	if r.AST == nil || r.Symbols == nil || r.Program == nil {
		t.Fatal("Result has nil fields")
	}
	if got := len(r.Program.Resources); got != 2 {
		t.Errorf("got %d resources, want 2", got)
	}
}
