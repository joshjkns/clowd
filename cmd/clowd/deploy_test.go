package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/joshjkns/clowd/internal/golden"
)

// fakeTofu stands in for OpenTofu. It appends each call to calls.log in
// the project, writes a lock file on init, remembers that certificates
// were created by a targeted apply, and answers show and output with
// canned JSON.
const fakeTofu = `#!/bin/sh
echo "tofu $*" >> "$CLOWD_FAKE_DIR/calls.log"
case "$1" in
  init)
    echo "Initializing the backend..."
    cat > .terraform.lock.hcl <<'EOF'
provider "registry.opentofu.org/hashicorp/aws" {
  version     = "6.14.1"
  constraints = "~> 6.0"
}
EOF
    ;;
  show)
    if [ -f "$CLOWD_FAKE_DIR/certs" ]; then
      echo '{"values":{"root_module":{"resources":[{"address":"aws_acm_certificate.web_api_example_com","values":{"status":"PENDING_VALIDATION","domain_validation_options":[{"domain_name":"api.example.com","resource_record_name":"_abc.api.example.com.","resource_record_type":"CNAME","resource_record_value":"_xyz.acm-validations.aws."}]}}]}}}'
    else
      echo '{"format_version":"1.0"}'
    fi
    ;;
  apply)
    case "$*" in *-target=*) touch "$CLOWD_FAKE_DIR/certs" ;; esac
    echo "Apply complete!"
    ;;
  output)
    echo '{"web_url":{"value":"https://api.example.com"},"web_dns_name":{"value":"shop-web-1.eu-west-2.elb.amazonaws.com"},"uploads_bucket":{"value":"shop-uploads-123"}}'
    ;;
  plan) echo "Plan: 3 to add, 0 to change, 0 to destroy." ;;
  destroy) echo "Destroy complete!" ;;
esac
`

// project creates a project directory with the given files, puts the fake
// tofu first on PATH and makes the directory the working directory.
func setupProject(t *testing.T, files map[string]string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake tofu is a shell script")
	}
	dir := t.TempDir()
	bin := filepath.Join(t.TempDir(), "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "tofu"), []byte(fakeTofu), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CLOWD_FAKE_DIR", dir)
	for name, text := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	return dir
}

const tlsApp = `use aws in eu-west-2

service api
  image nginx
  port 80

bucket uploads

load balancer web
  listen on 443 with tls for api.example.com
  send / to api

api reads uploads
`

// transcript runs commands in order and records each one's exit code and
// output, then the fake tofu's calls and the named files.
func transcript(t *testing.T, dir string, commands [][]string, files ...string) string {
	t.Helper()
	var b strings.Builder
	for _, args := range commands {
		stdout, stderr, code := runCLI(args...)
		fmt.Fprintf(&b, "$ clowd %s\nexit %d\n--- stdout\n%s--- stderr\n%s\n", strings.Join(args, " "), code, stdout, stderr)
	}
	if calls, err := os.ReadFile(filepath.Join(dir, "calls.log")); err == nil {
		fmt.Fprintf(&b, "--- tofu calls\n%s\n", calls)
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			fmt.Fprintf(&b, "--- %s\n(missing: %v)\n\n", f, err)
			continue
		}
		fmt.Fprintf(&b, "--- %s\n%s\n", f, data)
	}
	return b.String()
}

func TestDeploy(t *testing.T) {
	tests := []struct {
		name     string
		files    map[string]string
		commands [][]string
		show     []string
	}{
		{
			name:     "init",
			files:    map[string]string{".gitignore": "node_modules/"},
			commands: [][]string{{"init"}, {"init"}, {"check"}},
			show:     []string{"clowd.toml", "app.clowd", ".gitignore"},
		},
		{
			name:     "init_keeps_existing_source",
			files:    map[string]string{"infra/site.clowd": "use aws\n"},
			commands: [][]string{{"init", "infra"}},
			show:     []string{"infra/clowd.toml", "infra/.gitignore"},
		},
		{
			name:     "apply_tls",
			files:    map[string]string{"clowd.toml": "name = \"shop\"\n", "app.clowd": tlsApp},
			commands: [][]string{{"apply", "--yes"}, {"apply", "--yes"}},
			show:     []string{"clowd.lock"},
		},
		{
			name:     "plan_with_lock",
			files:    map[string]string{"clowd.toml": "name = \"shop\"\n", "app.clowd": "use aws\n\nqueue jobs\n", "clowd.lock": "[providers.aws]\nsource = \"hashicorp/aws\"\nversion = \"6.14.1\"\n"},
			commands: [][]string{{"plan"}},
		},
		{
			name:     "destroy",
			files:    map[string]string{"app.clowd": "use aws\n\nqueue jobs\n"},
			commands: [][]string{{"destroy"}, {"build"}, {"destroy", "--yes"}},
		},
		{
			name:     "build_several_clouds",
			files:    map[string]string{"clowd.toml": "name = \"shop\"\n", "app.clowd": "use aws in eu-west-2\nuse gcp in europe-west2\n\nqueue jobs\n"},
			commands: [][]string{{"build"}, {"build", "--json", "--target", "aws"}, {"apply"}, {"apply", "--target", "azure"}, {"plan", "--target", "gcp"}},
		},
		{
			name:     "build_undeployable",
			files:    map[string]string{"app.clowd": "use aws\n\nservice api\n  image a\n  scale 0 to 3\n"},
			commands: [][]string{{"build"}},
		},
		{
			name: "bad_override",
			files: map[string]string{"app.clowd": `use aws

service api
  image a
  on aws
    capacity_provider FARGATE_SPOT
`},
			commands: [][]string{{"check"}, {"build"}},
		},
		{
			name:     "bad_config",
			files:    map[string]string{"clowd.toml": "name = \"shop\"\nregion = \"eu-west-2\"\n", "app.clowd": "use aws\n"},
			commands: [][]string{{"build"}},
		},
		{
			name:     "file_errors",
			files:    map[string]string{"app.clowd": "use aws\n\nservice api\n  prot 80\n"},
			commands: [][]string{{"plan"}},
		},
		{
			name:     "which_file",
			files:    map[string]string{"a.clowd": "use aws\n", "b.clowd": "use aws\n"},
			commands: [][]string{{"build"}, {"build", "a.clowd", "b.clowd"}, {"build", "a.clowd"}},
		},
		{
			// The .clowd work directory matches *.clowd but isn't a source.
			name:     "work_dir_is_not_a_source",
			files:    map[string]string{"app.clowd": "use aws\n\nqueue jobs\n", ".clowd/build/aws/main.tf.json": "{}"},
			commands: [][]string{{"check"}, {"fmt", "--check"}, {"build"}},
		},
		{
			name:     "no_file",
			commands: [][]string{{"build"}, {"check"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupProject(t, tt.files)
			got := transcript(t, dir, tt.commands, tt.show...)
			golden.Check(t, filepath.Join(testdataDir, "deploy_"+tt.name+".golden"), got)
		})
	}
}

// testdataDir is cmd/clowd/testdata, found before tests change directory.
var testdataDir = func() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(wd, "testdata")
}()

// TestBuildWritesLoweredConfig checks build output is the lowered program
// and that a rebuild keeps the rest of the directory (the state).
func TestBuildWritesLoweredConfig(t *testing.T) {
	dir := setupProject(t, map[string]string{"clowd.toml": "name = \"shop\"\n", "app.clowd": tlsApp})
	state := filepath.Join(dir, ".clowd", "build", "aws", "terraform.tfstate")
	if _, _, code := runCLI("build"); code != exitOK {
		t.Fatalf("build exited %d", code)
	}
	if err := os.WriteFile(state, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := runCLI("build"); code != exitOK {
		t.Fatalf("rebuild exited %d", code)
	}
	if _, err := os.Stat(state); err != nil {
		t.Errorf("rebuild removed the state: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".clowd", "build", "aws", "main.tf.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Resource map[string]map[string]any `json:"resource"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("main.tf.json: %v", err)
	}
	if cfg.Resource["aws_lb"]["web"] == nil || cfg.Resource["aws_ecs_service"]["api"] == nil {
		t.Errorf("main.tf.json is missing the load balancer or service")
	}
}
