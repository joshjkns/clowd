// Package golden compares test output against files in testdata/ and
// regenerates them with `go test ./... -update`.
//
// Every package with tests must import this package (blank import if it has
// no golden tests) so that the -update flag is defined in every test binary;
// otherwise `go test ./... -update` fails with "flag provided but not defined".
package golden

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files in testdata/")

// Check compares got with the golden file at path, or rewrites the file when
// -update is set.
func Check(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create it): %v", err)
	}
	if got != string(want) {
		t.Errorf("output differs from %s (run with -update to accept):\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}
