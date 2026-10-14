package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/joshjkns/clowd/internal/golden"
)

func runCLI(args ...string) (stdout, stderr string, code int) {
	var out, errOut bytes.Buffer
	code = run(args, &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestCLI runs the CLI end to end. Each golden records the exit code,
// stdout and stderr.
func TestCLI(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code int
	}{
		{"example", []string{"check", "../../examples/app.clowd"}, exitOK},
		{"example_json", []string{"check", "--json", "../../examples/app.clowd"}, exitOK},
		{"flags_after_file", []string{"check", "../../examples/app.clowd", "--target", "aws,azure"}, exitFailure},
		{"undeployable", []string{"check", "testdata/undeployable.clowd"}, exitFailure},
		{"undeployable_json", []string{"check", "--json", "testdata/undeployable.clowd"}, exitFailure},
		{"errors", []string{"check", "testdata/errors.clowd"}, exitBadInput},
		{"errors_json", []string{"check", "--json", "testdata/errors.clowd"}, exitBadInput},
		{"no_targets", []string{"check", "testdata/no_targets.clowd"}, exitOK},
		{"several_files", []string{"check", "testdata/no_targets.clowd", "testdata/undeployable.clowd"}, exitFailure},
		{"missing_file", []string{"check", "testdata/nope.clowd"}, exitBadInput},
		{"bad_target", []string{"check", "--target", "oracle", "../../examples/app.clowd"}, exitBadInput},
		{"unknown_command", []string{"deploy"}, exitBadInput},
		{"no_args", nil, exitBadInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, code := runCLI(tt.args...)
			if code != tt.code {
				t.Errorf("exit code %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tt.code, stdout, stderr)
			}
			got := fmt.Sprintf("exit %d\n--- stdout\n%s--- stderr\n%s", code, stdout, stderr)
			golden.Check(t, filepath.Join("testdata", tt.name+".golden"), got)
		})
	}
}

// TestJSONIsValid guards the --json contract: it must always parse, even
// for files with errors.
func TestJSONIsValid(t *testing.T) {
	for _, file := range []string{"../../examples/app.clowd", "testdata/errors.clowd", "testdata/undeployable.clowd"} {
		stdout, _, _ := runCLI("check", "--json", file)
		var v jsonOutput
		if err := json.Unmarshal([]byte(stdout), &v); err != nil {
			t.Errorf("%s: invalid JSON: %v\n%s", file, err, stdout)
		}
		if len(v.Files) != 1 || v.Files[0].Path != file {
			t.Errorf("%s: unexpected files in output: %+v", file, v.Files)
		}
	}
}

// TestExamplesPass is the CLAUDE.md rule: every example passes `clowd check`.
func TestExamplesPass(t *testing.T) {
	examples, err := filepath.Glob("../../examples/*.clowd")
	if err != nil || len(examples) == 0 {
		t.Fatalf("no examples found: %v", err)
	}
	for _, ex := range examples {
		if _, stderr, code := runCLI("check", ex); code != exitOK {
			t.Errorf("clowd check %s exited %d:\n%s", ex, code, stderr)
		}
	}
}

func TestVersion(t *testing.T) {
	stdout, _, code := runCLI("version")
	if code != exitOK || stdout != "clowd dev (edition 2026)\n" {
		t.Errorf("version: code %d, output %q", code, stdout)
	}
}
