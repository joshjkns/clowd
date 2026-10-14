package runner

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	_ "github.com/joshjkns/clowd/internal/golden" // defines -update
)

// fakeTool writes a shell script standing in for tofu. It logs its
// arguments and environment to calls.log and answers `output` and `show`
// with canned JSON; `fail` as the first argument exits 1.
func fakeTool(t *testing.T) (*Runner, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake tool is a shell script")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	script := `#!/bin/sh
echo "$* TF_IN_AUTOMATION=$TF_IN_AUTOMATION" >> ` + log + `
case "$1" in
  output) echo '{"web_url":{"value":"http://x","type":"string","sensitive":false}}' ;;
  show) echo '{"values":{"root_module":{"resources":[{"address":"aws_acm_certificate.web","values":{"domain_name":"a.example.com"}}]}}}' ;;
  apply) echo "applied"; read answer; echo "got $answer" ;;
  init) echo "Initializing..."; [ -f fail-init ] && { echo "Error: no registry" >&2; exit 1; } ;;
esac
exit 0
`
	path := filepath.Join(dir, "tofu")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Runner{Tool: "tofu", Path: path, Dir: dir}, log
}

func calls(t *testing.T, log string) string {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCommands(t *testing.T) {
	r, log := fakeTool(t)
	var out bytes.Buffer
	r.Stdout, r.Stderr = &out, &out
	ctx := context.Background()

	if _, err := r.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.Plan(ctx); err != nil {
		t.Fatal(err)
	}
	// exec drains a non-file stdin eagerly, so give apply its own.
	r.Stdin = strings.NewReader("yes\n")
	if err := r.Apply(ctx, false, "aws_acm_certificate.web"); err != nil {
		t.Fatal(err)
	}
	if err := r.Destroy(ctx, true); err != nil {
		t.Fatal(err)
	}
	want := `init -input=false -no-color TF_IN_AUTOMATION=1
plan TF_IN_AUTOMATION=1
apply -target=aws_acm_certificate.web TF_IN_AUTOMATION=1
destroy -auto-approve TF_IN_AUTOMATION=1
`
	if got := calls(t, log); got != want {
		t.Errorf("calls:\n%s\nwant:\n%s", got, want)
	}
	if out.String() != "applied\ngot yes\n" {
		t.Errorf("interactive output = %q; init output should be captured, apply's shown", out.String())
	}
}

func TestOutputsAndResources(t *testing.T) {
	r, _ := fakeTool(t)
	ctx := context.Background()
	outs, err := r.Outputs(ctx)
	if err != nil || string(outs["web_url"]) != `"http://x"` {
		t.Errorf("Outputs = %v, %v", outs, err)
	}
	res, err := r.Resources(ctx)
	if err != nil || !strings.Contains(string(res["aws_acm_certificate.web"]), "a.example.com") {
		t.Errorf("Resources = %v, %v", res, err)
	}
}

func TestCapturedFailure(t *testing.T) {
	r, _ := fakeTool(t)
	if err := os.WriteFile(filepath.Join(r.Dir, "fail-init"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := r.Init(context.Background())
	var re *Error
	if !errors.As(err, &re) || !strings.Contains(string(re.Output), "Error: no registry") || re.Command != "init" {
		t.Errorf("Init error = %#v", err)
	}
}

func TestFind(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "terraform"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	tool, path, err := Find("")
	if err != nil || tool != "terraform" || path != filepath.Join(dir, "terraform") {
		t.Errorf("Find falls back to terraform: %s %s %v", tool, path, err)
	}
	if _, _, err := Find("tofu"); err == nil || err.Error() != "tofu is not installed" {
		t.Errorf("Find(tofu) = %v", err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, _, err := Find(""); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find with nothing installed = %v", err)
	}
}
