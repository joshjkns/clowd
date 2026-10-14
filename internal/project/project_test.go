package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/golden"
)

// TestConfigGolden decodes each testdata/*.toml and records the settings
// and the rendered diagnostics.
func TestConfigGolden(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.toml")
	if err != nil || len(inputs) == 0 {
		t.Fatalf("no inputs: %v", err)
	}
	for _, in := range inputs {
		name := strings.TrimSuffix(filepath.Base(in), ".toml")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			f := diag.NewFile(filepath.Base(in), string(src))
			c := Config{Name: "default", Edition: LatestEdition, State: State{Backend: defaultBackend}}
			c.AWS.Network = Network{Services: defaultServices, CIDR: defaultCIDR}
			diags := c.decode(f)

			var b strings.Builder
			n := c.AWS.Network
			fmt.Fprintf(&b, "name %q\nedition %d\nruntime %q\nstate.backend %q\n", c.Name, c.Edition, c.Runtime, c.State.Backend)
			fmt.Fprintf(&b, "aws.network.services %q\naws.network.cidr %q\naws.network.vpc %q\n", n.Services, n.CIDR, n.VPC)
			fmt.Fprintf(&b, "aws.network.public_subnets %q\naws.network.private_subnets %q\n", n.PublicSubnets, n.PrivateSubnets)
			if len(diags) > 0 {
				b.WriteString("\ndiagnostics:\n")
				diags.Sort()
				_ = diag.RenderAll(&b, f, diags)
			}
			golden.Check(t, strings.TrimSuffix(in, ".toml")+".golden", b.String())
		})
	}
}

func TestTOMLValues(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{`a = "x\ty\"z\u00e9"`, "a string \"x\\ty\\\"zé\""},
		{`a = 'C:\path'`, `a string "C:\\path"`},
		{`a = 1_000`, "a int 1000"},
		{`a = -5`, "a int -5"},
		{`a = true`, "a bool true"},
		{`a = []`, "a array []"},
		{"a = [\n  1,\n  2, # two\n]", "a array [1 2]"},
		{`"quoted key" = 1`, "quoted key int 1"},
		{"[t]\na = 1\n[t.u]\nb = 2", "t.a int 1\nt.u.b int 2"},
	}
	for _, tt := range tests {
		doc, diags := parseTOML(diag.NewFile("t.toml", tt.src))
		if len(diags) > 0 {
			t.Errorf("%q: unexpected diagnostics: %v", tt.src, diags)
			continue
		}
		var got []string
		for _, e := range doc.entries {
			got = append(got, e.path+" "+show(e.value))
		}
		if g := strings.Join(got, "\n"); g != tt.want {
			t.Errorf("%q:\ngot  %s\nwant %s", tt.src, g, tt.want)
		}
	}
}

func show(v value) string {
	switch v.kind {
	case stringValue:
		return fmt.Sprintf("string %q", v.str)
	case intValue:
		return fmt.Sprintf("int %d", v.num)
	case boolValue:
		return fmt.Sprintf("bool %t", v.bool)
	}
	var items []string
	for _, it := range v.items {
		items = append(items, strings.TrimPrefix(show(it), "int "))
	}
	return "array [" + strings.Join(items, " ") + "]"
}

func TestQuoteRoundTrips(t *testing.T) {
	for _, s := range []string{"", "plain", `a"b\c`, "tab\tnew\nline", "\x01"} {
		doc, diags := parseTOML(diag.NewFile("t.toml", "a = "+quote(s)))
		if len(diags) > 0 || len(doc.entries) != 1 || doc.entries[0].value.str != s {
			t.Errorf("quote(%q) = %s did not round-trip: %v", s, quote(s), diags)
		}
	}
}

func TestDefaultName(t *testing.T) {
	tests := map[string]string{
		"/home/me/shop":                     "shop",
		"/home/me/My Shop":                  "my-shop",
		"/home/me/2024-site":                "app-2024-site",
		"/home/me/___":                      "app-",
		"/home/me/a-very-long-project-name": "a-very-long-project",
	}
	for dir, want := range tests {
		got := DefaultName(dir)
		if got != strings.TrimRight(want, "-") {
			t.Errorf("DefaultName(%q) = %q, want %q", dir, got, want)
		}
		if !nameRE.MatchString(got) || len(got) > maxNameLen {
			t.Errorf("DefaultName(%q) = %q is not a valid project name", dir, got)
		}
	}
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "infra", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, ConfigFile), "name = \"shop\"\n[aws.network]\nservices = \"private\"\n")

	p, _, diags, err := Load(sub)
	if err != nil || len(diags) > 0 {
		t.Fatalf("Load: %v %v", err, diags)
	}
	if !p.HasConfig || p.Root != root || p.Config.Name != "shop" || p.Config.AWS.Network.Services != "private" {
		t.Errorf("Load from a subdirectory: %+v", p)
	}
	if p.Config.AWS.Network.CIDR != defaultCIDR || p.Config.State.Backend != defaultBackend {
		t.Errorf("defaults not kept: %+v", p.Config)
	}
	if got, want := p.BuildDir("aws"), filepath.Join(root, ".clowd", "build", "aws"); got != want {
		t.Errorf("BuildDir = %s, want %s", got, want)
	}

	// The lock round-trips through WriteLock and Load.
	lock := Lock{Providers: map[string]Provider{"aws": {Source: "hashicorp/aws", Version: "6.14.1"}}}
	if wrote, err := p.WriteLock(lock); err != nil || !wrote {
		t.Fatalf("WriteLock: %v %v", wrote, err)
	}
	if wrote, err := p.WriteLock(lock); err != nil || wrote {
		t.Errorf("WriteLock rewrote an unchanged lock: %v %v", wrote, err)
	}
	p, _, diags, err = Load(root)
	if err != nil || len(diags) > 0 {
		t.Fatalf("Load after lock: %v %v", err, diags)
	}
	if p.Lock.Version("aws") != "6.14.1" {
		t.Errorf("lock not read back: %+v", p.Lock)
	}
}

func TestLoadWithoutConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "My App")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p, _, diags, err := Load(dir)
	if err != nil || len(diags) > 0 {
		t.Fatalf("Load: %v %v", err, diags)
	}
	if p.HasConfig || p.Root != dir || p.Config.Name != "my-app" || p.Config.Edition != LatestEdition {
		t.Errorf("defaults: %+v", p)
	}
}

func TestLoadReportsBadFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ConfigFile), "edition = 2030\n")
	_, src, diags, err := Load(dir)
	if err != nil || !diags.HasErrors() || src == nil || src.Name != ConfigFile {
		t.Errorf("bad config: %v %v %v", src, diags, err)
	}

	write(t, filepath.Join(dir, ConfigFile), "")
	write(t, filepath.Join(dir, LockFile), "[providers.aws]\nsource = \"hashicorp/aws\"\n")
	_, src, diags, err = Load(dir)
	if err != nil || !diags.HasErrors() || src == nil || src.Name != LockFile {
		t.Errorf("bad lock: %v %v %v", src, diags, err)
	}
}

func TestInitConfigLoads(t *testing.T) {
	var c Config
	if diags := c.decode(diag.NewFile(ConfigFile, InitConfig("shop"))); len(diags) > 0 {
		t.Errorf("InitConfig doesn't load: %v", diags)
	}
	if c.Name != "shop" || c.Edition != LatestEdition {
		t.Errorf("InitConfig decoded to %+v", c)
	}
}

func TestReadTofuLock(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".terraform.lock.hcl"), `# This file is maintained automatically by "tofu init".

provider "registry.opentofu.org/hashicorp/aws" {
  version     = "6.14.1"
  constraints = "~> 6.0"
  hashes = [
    "h1:abc=",
  ]
}

provider "registry.terraform.io/hashicorp/random" {
  version = "3.7.2"
}
`)
	got, err := ReadTofuLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got["hashicorp/aws"] != "6.14.1" || got["hashicorp/random"] != "3.7.2" || len(got) != 2 {
		t.Errorf("ReadTofuLock = %v", got)
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}
