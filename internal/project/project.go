// Package project loads a Clowd project: its clowd.toml settings and its
// clowd.lock provider pins (06-versioning.md).
//
// A project is the directory holding clowd.toml, found by walking up from
// the working directory. Without one, the working directory is the project
// and every setting takes its default, so a lone .clowd file can still be
// built and deployed.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/spell"
)

// File names.
const (
	ConfigFile = "clowd.toml"
	LockFile   = "clowd.lock"
	// WorkDir holds build output and OpenTofu working directories; it is
	// gitignored.
	WorkDir = ".clowd"
)

// LatestEdition is the newest language edition this binary supports.
const LatestEdition = 2026

// Project is a loaded project.
type Project struct {
	// Root is the directory holding clowd.toml, or the working directory
	// when there is none.
	Root string
	// HasConfig reports whether clowd.toml exists.
	HasConfig bool
	Config    Config
	Lock      Lock
}

// BuildDir is where the root module for cloud is written.
func (p *Project) BuildDir(cloud string) string {
	return filepath.Join(p.Root, WorkDir, "build", cloud)
}

// Config is clowd.toml.
type Config struct {
	// Name prefixes the names of everything deployed, so two projects can
	// share an account. Defaults to the project directory's name.
	Name    string
	Edition int
	// Runtime is "tofu" or "terraform"; empty picks tofu when it is
	// installed and terraform otherwise.
	Runtime string
	State   State
	AWS     AWS
}

// State says where OpenTofu keeps state.
type State struct {
	// Backend is "local", the only backend so far.
	Backend string
}

// AWS holds settings for the aws target.
type AWS struct {
	Network Network
}

// Network is where AWS resources are placed. By default Clowd creates a
// VPC with public and private subnets in two availability zones and runs
// services in the public ones, which needs no NAT gateway.
type Network struct {
	// Services is "public" (default) or "private". Private services run
	// in private subnets and reach the internet through a NAT gateway.
	Services string
	// CIDR is the block for the VPC Clowd creates.
	CIDR string
	// VPC, when set, is an existing VPC to use instead of creating one;
	// PublicSubnets and PrivateSubnets then name its subnets.
	VPC            string
	PublicSubnets  []string
	PrivateSubnets []string
}

// Defaults.
const (
	defaultCIDR     = "10.0.0.0/16"
	defaultServices = "public"
	defaultBackend  = "local"
)

// Load finds the project containing dir and loads it. User mistakes in
// clowd.toml or clowd.lock are returned as diagnostics against src, which
// holds whichever file they are in; err is for I/O failures.
func Load(dir string) (p *Project, src *diag.File, diags diag.List, err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	p = &Project{Root: dir}
	root, found, err := find(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	if found {
		p.Root, p.HasConfig = root, true
	}
	p.Config = Config{Name: DefaultName(p.Root), Edition: LatestEdition, State: State{Backend: defaultBackend}}
	p.Config.AWS.Network = Network{Services: defaultServices, CIDR: defaultCIDR}

	if p.HasConfig {
		path := filepath.Join(p.Root, ConfigFile)
		text, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, nil, err
		}
		src = diag.NewFile(relTo(dir, path), string(text))
		diags = p.Config.decode(src)
		if diags.HasErrors() {
			return p, src, diags, nil
		}
	}

	path := filepath.Join(p.Root, LockFile)
	text, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, nil, nil, err
	default:
		lockSrc := diag.NewFile(relTo(dir, path), string(text))
		if ld := p.Lock.decode(lockSrc); len(ld) > 0 {
			return p, lockSrc, ld, nil
		}
	}
	return p, src, diags, nil
}

// find walks up from dir to the nearest directory with clowd.toml.
func find(dir string) (string, bool, error) {
	for d := dir; ; {
		_, err := os.Stat(filepath.Join(d, ConfigFile))
		if err == nil {
			return d, true, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", false, err
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false, nil
		}
		d = parent
	}
}

// relTo returns path relative to dir when that is shorter, for messages.
func relTo(dir, path string) string {
	if rel, err := filepath.Rel(dir, path); err == nil && len(rel) < len(path) {
		return rel
	}
	return path
}

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// maxNameLen keeps generated names inside AWS's shortest limits (32 for a
// load balancer or target group) with room for the resource's own name.
const maxNameLen = 20

// DefaultName turns a directory name into a project name: lowercase
// letters, digits and dashes, starting with a letter.
func DefaultName(dir string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(filepath.Base(dir)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		name = "app-" + name
	}
	return strings.TrimRight(name[:min(len(name), maxNameLen)], "-")
}

// decode reads clowd.toml into c, keeping defaults for what it leaves out.
func (c *Config) decode(f *diag.File) diag.List {
	doc, diags := parseTOML(f)
	if diags.HasErrors() {
		return diags
	}
	d := decoder{doc: doc}

	top := d.keys("", "name", "edition", "runtime")
	if e, ok := top["name"]; ok && d.str(e, &c.Name) {
		if !nameRE.MatchString(c.Name) || len(c.Name) > maxNameLen {
			d.add(diag.Errorf(e.value.pos, "`%s` can't be a project name", c.Name).
				Suggest("use up to %d lowercase letters, digits and dashes, starting with a letter", maxNameLen))
		}
	}
	if e, ok := top["edition"]; ok {
		var n int64
		if d.int(e, &n) {
			switch {
			case n > LatestEdition:
				d.add(diag.Errorf(e.value.pos, "edition %d is newer than this clowd supports", n).
					Suggest("upgrade clowd, or set `edition = %d`", LatestEdition))
			case n != LatestEdition:
				d.add(diag.Errorf(e.value.pos, "there is no edition %d", n).Suggest("set `edition = %d`", LatestEdition))
			default:
				c.Edition = int(n)
			}
		}
	}
	if e, ok := top["runtime"]; ok {
		d.oneOf(e, &c.Runtime, "tofu", "terraform")
	}

	state := d.keys("state", "backend")
	if e, ok := state["backend"]; ok && d.str(e, &c.State.Backend) && c.State.Backend != defaultBackend {
		d.add(diag.Errorf(e.value.pos, "the `%s` state backend is not implemented yet", c.State.Backend).
			Suggest("use `backend = \"local\"`"))
	}

	d.keys("aws")
	n := &c.AWS.Network
	net := d.keys("aws.network", "services", "cidr", "vpc", "public_subnets", "private_subnets")
	if e, ok := net["services"]; ok {
		d.oneOf(e, &n.Services, "public", "private")
	}
	if e, ok := net["cidr"]; ok {
		d.str(e, &n.CIDR)
	}
	if e, ok := net["vpc"]; ok {
		d.str(e, &n.VPC)
	}
	pub, hasPub := net["public_subnets"]
	priv, hasPriv := net["private_subnets"]
	if hasPub {
		d.strs(pub, &n.PublicSubnets)
	}
	if hasPriv {
		d.strs(priv, &n.PrivateSubnets)
	}
	if n.VPC != "" {
		if e, ok := net["cidr"]; ok {
			d.add(diag.Errorf(e.keyPos, "`cidr` is for the VPC Clowd creates, and `vpc` is set").Suggest("remove one of them"))
		}
		for _, need := range []struct {
			key    string
			e      entry
			has    bool
			values []string
		}{{"public_subnets", pub, hasPub, n.PublicSubnets}, {"private_subnets", priv, hasPriv, n.PrivateSubnets}} {
			switch {
			case !need.has:
				d.add(diag.Errorf(net["vpc"].keyPos, "an existing `vpc` needs `%s` too", need.key).
					Suggest("list two or more subnets in different availability zones"))
			case len(need.values) < 2:
				d.add(diag.Errorf(need.e.value.pos, "`%s` needs at least two subnets", need.key).
					Suggest("load balancers and databases span two availability zones"))
			}
		}
	} else {
		for _, e := range []entry{pub, priv} {
			if e.path != "" {
				_, key := splitPath(e.path)
				d.add(diag.Errorf(e.keyPos, "`%s` names subnets in an existing VPC, but `vpc` isn't set", key).
					Suggest("add `vpc = \"vpc-...\"`, or remove `%s` to let Clowd create the network", key))
			}
		}
	}
	return append(diags, d.diags...)
}

// decoder turns document entries into typed settings, reporting what
// doesn't fit.
type decoder struct {
	doc   *document
	diags diag.List
}

func (d *decoder) add(ds ...diag.Diagnostic) { d.diags.Add(ds...) }

// keys returns the entries in table path, reporting any key not in known
// and any sub-table that isn't a known one.
func (d *decoder) keys(path string, known ...string) map[string]entry {
	entries := d.doc.table(path)
	for _, e := range d.doc.entries {
		parent, key := splitPath(e.path)
		if parent != path || slices.Contains(known, key) {
			continue
		}
		where := "clowd.toml"
		if path != "" {
			where = "[" + path + "]"
		}
		problem := diag.Errorf(e.keyPos, "unknown setting `%s` in %s", key, where)
		if s, ok := spell.Closest(key, known); ok {
			problem = problem.Suggest("did you mean `%s`?", s)
		} else if strings.EqualFold(key, "region") || spell.IsTypo(strings.ToLower(key), "region") {
			problem = problem.Suggest("the region is set in the .clowd file: `use aws in <region>`")
		} else if len(known) > 0 {
			problem = problem.Suggest("settings here are %s", codeList(known))
		}
		d.add(problem)
		delete(entries, key)
	}
	for name, sp := range d.doc.tables {
		if parent, _ := splitPath(name); parent == path && !knownTable(name) {
			d.add(diag.Errorf(sp, "unknown table `[%s]`", name))
		}
	}
	return entries
}

func knownTable(name string) bool {
	switch name {
	case "state", "aws", "aws.network":
		return true
	}
	return false
}

func (d *decoder) want(e entry, k valueKind) bool {
	if e.value.kind == k {
		return true
	}
	_, key := splitPath(e.path)
	d.add(diag.Errorf(e.value.pos, "`%s` should be %s, not %s", key, k, e.value.kind))
	return false
}

func (d *decoder) str(e entry, dst *string) bool {
	if !d.want(e, stringValue) {
		return false
	}
	*dst = e.value.str
	return true
}

func (d *decoder) int(e entry, dst *int64) bool {
	if !d.want(e, intValue) {
		return false
	}
	*dst = e.value.num
	return true
}

func (d *decoder) strs(e entry, dst *[]string) bool {
	if !d.want(e, arrayValue) {
		return false
	}
	var out []string
	for _, item := range e.value.items {
		if item.kind != stringValue {
			d.add(diag.Errorf(item.pos, "expected a string, not %s", item.kind))
			return false
		}
		out = append(out, item.str)
	}
	*dst = out
	return true
}

func (d *decoder) oneOf(e entry, dst *string, allowed ...string) {
	var s string
	if !d.str(e, &s) {
		return
	}
	if slices.Contains(allowed, s) {
		*dst = s
		return
	}
	_, key := splitPath(e.path)
	problem := diag.Errorf(e.value.pos, "`%s` can't be %s", key, quote(s))
	if c, ok := spell.Closest(s, allowed); ok {
		problem = problem.Suggest("did you mean %s?", quote(c))
	} else {
		problem = problem.Suggest("use %s", quotedList(allowed))
	}
	d.add(problem)
}

func codeList(words []string) string {
	q := make([]string, len(words))
	for i, w := range words {
		q[i] = "`" + w + "`"
	}
	return joinList(q, "and")
}

func quotedList(words []string) string {
	q := make([]string, len(words))
	for i, w := range words {
		q[i] = quote(w)
	}
	return joinList(q, "or")
}

func joinList(q []string, conj string) string {
	if len(q) <= 1 {
		return strings.Join(q, "")
	}
	return strings.Join(q[:len(q)-1], ", ") + " " + conj + " " + q[len(q)-1]
}

// InitConfig is the clowd.toml that `clowd init` writes.
func InitConfig(name string) string {
	return fmt.Sprintf(`# Clowd project settings.

# Prefixes the names of everything deployed.
name = %s
edition = %d

# [aws.network]
# services = "private"   # run services behind a NAT gateway (default "public")
# cidr = "10.0.0.0/16"   # the VPC Clowd creates
# Or use an existing VPC:
# vpc = "vpc-0123456789abcdef0"
# public_subnets = ["subnet-...", "subnet-..."]
# private_subnets = ["subnet-...", "subnet-..."]
`, quote(name), LatestEdition)
}
