package gcp

import (
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshjkns/clowd/internal/capability"
	"github.com/joshjkns/clowd/internal/compile"
	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/golden"
	"github.com/joshjkns/clowd/internal/ir"
)

// options gives fixtures non-default deploy settings by name. Everything
// else is written in the fixture itself.
var options = map[string]func(*Options){
	"network_pinned": func(o *Options) { o.ProviderVersions = map[string]string{"google": "8.7.0", "random": "3.9.1"} },
}

// lower compiles the file at path and lowers it, failing the test if it
// has errors or isn't deployable to gcp.
func lower(t *testing.T, path string, opts Options) (*diag.File, *Result) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f := diag.NewFile(filepath.Base(path), string(src))
	res := compile.Check(f)
	if res.Diags.HasErrors() {
		var b strings.Builder
		_ = diag.RenderAll(&b, f, res.Diags)
		t.Fatalf("%s has errors:\n%s", path, b.String())
	}
	if rep := capability.Evaluate(res.Program, []ir.Cloud{ir.GCP}); !rep.AllDeployable() {
		t.Fatalf("%s isn't deployable to gcp", path)
	}
	opts.Source = f.Name
	return f, Lower(res.Program, opts)
}

func render(t *testing.T, f *diag.File, r *Result) string {
	t.Helper()
	out, err := tfjson.Marshal(r.Config)
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if len(r.Diags) > 0 {
		_ = diag.RenderAll(&b, f, r.Diags)
		b.WriteString("\n")
	}
	b.Write(out)
	return b.String()
}

// TestGolden lowers each testdata/*.clowd. A fixture may be lowered with
// several settings: network.clowd also gives network_private.golden and
// so on, per the options table.
func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.clowd")
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no inputs: %v", err)
	}
	for _, in := range inputs {
		base := strings.TrimSuffix(filepath.Base(in), ".clowd")
		variants := []string{base}
		for name := range options {
			if strings.HasPrefix(name, base+"_") {
				variants = append(variants, name)
			}
		}
		for _, name := range variants {
			t.Run(name, func(t *testing.T) {
				opts := Options{Project: "shop"}
				if set, ok := options[name]; ok {
					set(&opts)
				}
				f, r := lower(t, in, opts)
				golden.Check(t, filepath.Join("testdata", name+".golden"), render(t, f, r))
			})
		}
	}
}

// TestExamples lowers every example, so the files users start from
// always produce a configuration.
func TestExamples(t *testing.T) {
	examples, err := filepath.Glob("../../../examples/*.clowd")
	if err != nil || len(examples) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	for _, ex := range examples {
		name := strings.TrimSuffix(filepath.Base(ex), ".clowd")
		t.Run(name, func(t *testing.T) {
			f, r := lower(t, ex, Options{Project: "shop"})
			if r.Diags.HasErrors() {
				t.Errorf("lowering %s reported errors", ex)
			}
			golden.Check(t, filepath.Join("testdata", "examples", name+".golden"), render(t, f, r))
		})
	}
}

func TestOverridable(t *testing.T) {
	tests := []struct {
		kind ir.Kind
		path []string
		res  string
		want string
	}{
		{ir.QueueKind, nil, "google_pubsub_subscription", "filter retain_acked_messages retry_policy"},
		{ir.DatabaseKind, []string{"Settings", "maintenance_window"}, "google_sql_database_instance", "day hour update_track"},
		{ir.ServiceKind, []string{"launch_stage"}, "google_cloud_run_v2_service", ""},
	}
	for _, tt := range tests {
		res, keys := Overridable(tt.kind, tt.path...)
		if got := strings.Join(keys, " "); res != tt.res || got != tt.want {
			t.Errorf("Overridable(%s, %q) = %s, %q; want %s, %q", tt.kind, tt.path, res, got, tt.res, tt.want)
		}
	}
}

func TestSixteenth(t *testing.T) {
	tests := []struct {
		cidr string
		i    int
		want string
	}{
		{"10.0.0.0/16", 0, "10.0.0.0/20"},
		{"10.0.0.0/16", 2, "10.0.32.0/20"},
		{"10.20.16.0/20", 1, "10.20.17.0/24"},
	}
	for _, tt := range tests {
		if got := sixteenth(netip.MustParsePrefix(tt.cidr), tt.i).String(); got != tt.want {
			t.Errorf("sixteenth(%s, %d) = %s, want %s", tt.cidr, tt.i, got, tt.want)
		}
	}
}

func TestPhysical(t *testing.T) {
	l := &lowerer{opts: Options{Project: "shop"}}
	tests := []struct {
		max   int
		parts []string
		want  string
	}{
		{32, []string{"web"}, "shop-web"},
		{32, []string{"user_files", "api"}, "shop-user-files-api"},
		{20, []string{"a-load-balancer-with-a-long-name"}, "shop-a-load-b-e05df1"},
	}
	for _, tt := range tests {
		got := l.physical(tt.max, tt.parts...)
		if got != tt.want || len(got) > tt.max {
			t.Errorf("physical(%d, %q) = %q, want %q", tt.max, tt.parts, got, tt.want)
		}
	}
}

// TestValidate runs `tofu validate` (or terraform) on every golden output.
// It downloads the Google provider, so it only runs when CLOWD_TF_VALIDATE
// is set: CLOWD_TF_VALIDATE=1 go test ./internal/lower/gcp -run Validate
func TestValidate(t *testing.T) {
	if os.Getenv("CLOWD_TF_VALIDATE") == "" {
		t.Skip("set CLOWD_TF_VALIDATE=1 to validate with tofu or terraform")
	}
	bin, err := exec.LookPath("tofu")
	if err != nil {
		if bin, err = exec.LookPath("terraform"); err != nil {
			t.Skip("neither tofu nor terraform is installed")
		}
	}
	goldens, _ := filepath.Glob("testdata/*.golden")
	examples, _ := filepath.Glob("testdata/examples/*.golden")
	// One working directory, so the provider is downloaded once.
	dir := t.TempDir()
	for _, g := range append(goldens, examples...) {
		data, err := os.ReadFile(g)
		if err != nil {
			t.Fatal(err)
		}
		// Goldens may start with notes and diagnostics before the JSON.
		text := string(data)
		text = text[strings.Index(text, "\n{")+1:]
		// The working directory's lock holds one version; pins are covered
		// by the goldens themselves.
		text = strings.NewReplacer(`"= 8.7.0"`, `"`+ProviderConstraint+`"`, `"= 3.9.1"`, `"`+RandomConstraint+`"`).Replace(text)
		if err := os.WriteFile(filepath.Join(dir, "main.tf.json"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"init", "-backend=false", "-input=false"}, {"validate", "-no-color"}} {
			cmd := exec.Command(bin, args...)
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("%s: %s %s failed: %v\n%s", g, bin, args[0], err, out)
				break
			}
		}
	}
}
