package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/joshjkns/clowd/internal/capability"
	"github.com/joshjkns/clowd/internal/compile"
	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/emit/tfjson"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/lower/aws"
	"github.com/joshjkns/clowd/internal/project"
	"github.com/joshjkns/clowd/internal/runner"
	"github.com/joshjkns/clowd/internal/term"
)

// hasBackend reports whether Clowd can deploy to c.
func hasBackend(c ir.Cloud) bool { return c == ir.AWS }

// awsOptions maps project settings to the AWS backend's.
func awsOptions(p *project.Project, source string) aws.Options {
	n := p.Config.AWS.Network
	return aws.Options{
		Project:         p.Config.Name,
		Source:          source,
		ProviderVersion: p.Lock.Version("aws"),
		Network: aws.Network{
			PrivateServices: n.Services == "private",
			CIDR:            n.CIDR,
			VPC:             n.VPC,
			PublicSubnets:   n.PublicSubnets,
			PrivateSubnets:  n.PrivateSubnets,
		},
	}
}

// deployFlags are the flags shared by build, plan, apply and destroy.
type deployFlags struct {
	fs      *flag.FlagSet
	targets targetsFlag
	color   term.Mode
	verbose bool
	json    bool // build only
	yes     bool // apply and destroy only
}

func newDeployFlags(cmd, usage string, stderr io.Writer) *deployFlags {
	d := &deployFlags{fs: flag.NewFlagSet(cmd, flag.ContinueOnError)}
	d.fs.SetOutput(stderr)
	d.fs.Var(&d.targets, "target", "the `cloud` to deploy to; needed when the file uses more than one")
	d.fs.Var(&d.color, "color", colorUsage)
	d.fs.BoolVar(&d.verbose, "verbose", false, "show the output of tofu init")
	switch cmd {
	case "build":
		d.fs.BoolVar(&d.json, "json", false, "print machine-readable JSON")
	case "apply", "destroy":
		d.fs.BoolVar(&d.yes, "yes", false, "don't ask for confirmation")
	}
	d.fs.Usage = func() {
		fmt.Fprint(stderr, usage)
		d.fs.PrintDefaults()
	}
	return d
}

// session is one deploy command's state: the project, the compiled file
// and where output goes.
type session struct {
	stdout, stderr io.Writer
	st             styles
	proj           *project.Project
	file           *diag.File
	prog           *ir.Program
	diags          diag.List
}

// open loads the project and compiles its source file, rendering any
// problems. It returns an exit code when the command can't go on.
func open(args []string, d *deployFlags, stdout, stderr io.Writer) (*session, int) {
	paths, err := parseInterspersed(d.fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, exitOK
		}
		return nil, exitBadInput
	}
	s := &session{stdout: stdout, stderr: stderr, st: styles{out: term.For(stdout, d.color), err: term.For(stderr, d.color)}}
	proj, src, diags, err := project.Load(".")
	if err != nil {
		s.fail("%v", err)
		return nil, exitBadInput
	}
	if len(diags) > 0 {
		_ = diag.RenderAllStyled(stderr, src, diags, s.st.err)
		if diags.HasErrors() {
			fmt.Fprintf(stderr, "\n%s: %s\n", s.st.err.Bold(src.Name), s.st.err.Error(count(diags)))
			return nil, exitBadInput
		}
	}
	s.proj = proj

	switch len(paths) {
	case 0:
		found := sourceFiles(proj.Root)
		switch len(found) {
		case 0:
			s.fail("no .clowd file in this project; run `clowd init` or pass a file")
			return nil, exitBadInput
		case 1:
			paths = []string{rel(found[0])}
		default:
			var names []string
			for _, f := range found {
				names = append(names, filepath.Base(f))
			}
			s.fail("found %s; pass the one to deploy", strings.Join(names, ", "))
			return nil, exitBadInput
		}
	case 1:
	default:
		s.fail("pass one .clowd file: a project deploys one program")
		return nil, exitBadInput
	}
	text, err := os.ReadFile(paths[0])
	if err != nil {
		s.fail("%v", err)
		return nil, exitBadInput
	}
	s.file = diag.NewFile(paths[0], string(text))
	res := compile.Check(s.file)
	s.prog, s.diags = res.Program, res.Diags
	if res.Diags.HasErrors() {
		s.render(res.Diags)
		fmt.Fprintf(stderr, "%s: %s\n", s.st.err.Bold(s.file.Name), s.st.err.Error(count(res.Diags)))
		return nil, exitBadInput
	}
	return s, exitOK
}

// rel shortens path relative to the working directory for messages.
func rel(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	if r, err := filepath.Rel(wd, path); err == nil && !strings.HasPrefix(r, "..") {
		return r
	}
	return path
}

func (s *session) fail(format string, args ...any) {
	fmt.Fprintf(s.stderr, "%s %s\n", s.st.err.Error("clowd:"), fmt.Sprintf(format, args...))
}

func (s *session) render(l diag.List) {
	if len(l) == 0 {
		return
	}
	l.Sort()
	_ = diag.RenderAllStyled(s.stderr, s.file, l, s.st.err)
	fmt.Fprintln(s.stderr)
}

// clouds returns the clouds to work on: --target, or the file's use lines.
// When one is required, more than one is an error.
func (s *session) clouds(targets []ir.Cloud, one bool) ([]ir.Cloud, int) {
	var used []ir.Cloud
	for _, t := range s.prog.Targets {
		used = append(used, t.Cloud)
	}
	for _, c := range targets {
		if !slices.Contains(used, c) {
			s.fail("%s doesn't `use %s`, so it can't be deployed there; add `use %s in <region>`", s.file.Name, c, c)
			return nil, exitBadInput
		}
	}
	clouds := targets
	if len(clouds) == 0 {
		clouds = used
	}
	switch {
	case len(clouds) == 0:
		s.fail("%s has no `use` line, so it deploys nowhere; add one, for example `use aws in eu-west-2`", s.file.Name)
		return nil, exitBadInput
	case one && len(clouds) > 1:
		var opts []string
		for _, c := range clouds {
			opts = append(opts, "--target "+c.String())
		}
		s.fail("%s deploys to %s; choose one with %s", s.file.Name, joinClouds(clouds), strings.Join(opts, " or "))
		return nil, exitBadInput
	}
	return clouds, exitOK
}

func joinClouds(cs []ir.Cloud) string {
	var names []string
	for _, c := range cs {
		names = append(names, c.String())
	}
	if len(names) <= 1 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// built is one target's build.
type built struct {
	cloud ir.Cloud
	dir   string
	path  string
	certs []string
}

// build lowers the program for cloud and writes the root module. Only
// main.tf.json is written: the directory also holds OpenTofu's state.
func (s *session) build(cloud ir.Cloud) (*built, int) {
	if !hasBackend(cloud) {
		s.fail("deploying to %s isn't implemented yet", cloud)
		return nil, exitFailure
	}
	rep := capability.Evaluate(s.prog, []ir.Cloud{cloud})
	if !rep.Deployable(0) {
		s.render(append(slices.Clone(s.diags), rep.Diagnostics()...))
		s.fail("%s isn't deployable to %s; run `clowd check` for details", s.file.Name, cloud)
		return nil, exitFailure
	}
	res := aws.Lower(s.prog, awsOptions(s.proj, filepath.Base(s.file.Name)))
	s.render(append(append(slices.Clone(s.diags), rep.Diagnostics()...), res.Diags...))
	if res.Diags.HasErrors() {
		fmt.Fprintf(s.stderr, "%s: %s\n", s.st.err.Bold(s.file.Name), s.st.err.Error(count(res.Diags)))
		return nil, exitBadInput
	}
	data, err := tfjson.Marshal(res.Config)
	if err != nil {
		s.fail("generating %s configuration: %v", cloud, err)
		return nil, exitFailure
	}
	b := &built{cloud: cloud, dir: s.proj.BuildDir(cloud.String()), certs: res.Certificates}
	b.path = filepath.Join(b.dir, "main.tf.json")
	if err := os.MkdirAll(b.dir, 0o755); err != nil {
		s.fail("%v", err)
		return nil, exitFailure
	}
	if err := os.WriteFile(b.path, data, 0o644); err != nil {
		s.fail("%v", err)
		return nil, exitFailure
	}
	return b, exitOK
}

// ---- build ----

const buildUsage = `Usage: clowd build [--target cloud] [--json] [file.clowd]

Compiles the project's .clowd file to .clowd/build/<cloud>/main.tf.json for
each cloud it uses, or only --target.

`

func runBuild(args []string, stdout, stderr io.Writer) int {
	d := newDeployFlags("build", buildUsage, stderr)
	s, code := open(args, d, stdout, stderr)
	if s == nil {
		return code
	}
	clouds, code := s.clouds(d.targets, false)
	if clouds == nil {
		return code
	}
	var done []*built
	code = exitOK
	for _, c := range clouds {
		b, c2 := s.build(c)
		if b == nil {
			code = max(code, c2)
			continue
		}
		done = append(done, b)
	}
	if d.json {
		type jsonTarget struct {
			Cloud string `json:"cloud"`
			Path  string `json:"path"`
		}
		out := struct {
			OK      bool         `json:"ok"`
			Targets []jsonTarget `json:"targets"`
		}{OK: code == exitOK, Targets: []jsonTarget{}}
		for _, b := range done {
			out.Targets = append(out.Targets, jsonTarget{Cloud: b.cloud.String(), Path: rel(b.path)})
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return code
	}
	for _, b := range done {
		fmt.Fprintf(stderr, "%s %s\n", s.st.err.Good("✓ built "+b.cloud.String()), s.st.err.Dim("→ "+rel(b.path)))
	}
	return code
}

// ---- plan, apply, destroy ----

const planUsage = `Usage: clowd plan [--target cloud] [file.clowd]

Builds the project for one cloud and shows what apply would change.

`

const applyUsage = `Usage: clowd apply [--target cloud] [--yes] [file.clowd]

Builds the project for one cloud and deploys it.

`

const destroyUsage = `Usage: clowd destroy [--target cloud] [--yes] [file.clowd]

Destroys everything the last apply to a cloud created, including data in
databases and buckets.

`

func runPlan(args []string, stdout, stderr io.Writer) int {
	return deploy("plan", planUsage, args, stdout, stderr)
}

func runApply(args []string, stdout, stderr io.Writer) int {
	return deploy("apply", applyUsage, args, stdout, stderr)
}

func runDestroy(args []string, stdout, stderr io.Writer) int {
	return deploy("destroy", destroyUsage, args, stdout, stderr)
}

func deploy(cmd, usage string, args []string, stdout, stderr io.Writer) int {
	d := newDeployFlags(cmd, usage, stderr)
	s, code := open(args, d, stdout, stderr)
	if s == nil {
		return code
	}
	clouds, code := s.clouds(d.targets, true)
	if clouds == nil {
		return code
	}
	cloud := clouds[0]

	var b *built
	if cmd == "destroy" {
		// Destroy with the configuration that was last built, which is
		// what the state was made from, even if the file has changed.
		b = &built{cloud: cloud, dir: s.proj.BuildDir(cloud.String())}
		b.path = filepath.Join(b.dir, "main.tf.json")
		if _, err := os.Stat(b.path); err != nil {
			fmt.Fprintf(stderr, "%s\n", s.st.err.Good(fmt.Sprintf("✓ nothing to destroy on %s: it was never built here", cloud)))
			return exitOK
		}
	} else if b, code = s.build(cloud); b == nil {
		return code
	}

	tool, path, err := runner.Find(s.proj.Config.Runtime)
	if err != nil {
		s.fail("%v; install OpenTofu from https://opentofu.org/docs/intro/install/", err)
		return exitBadInput
	}
	r := &runner.Runner{Tool: tool, Path: path, Dir: b.dir, Stdout: stdout, Stderr: stderr, NoColor: !s.st.out.Color}

	// Ctrl-C reaches tofu directly, which stops cleanly and releases its
	// state lock; clowd must outlive it rather than die first.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	ctx := context.Background()

	if code := s.init(ctx, r, d.verbose); code != exitOK {
		return code
	}
	switch cmd {
	case "plan":
		if err := r.Plan(ctx); err != nil {
			return s.toolFailed(err)
		}
		return exitOK
	case "destroy":
		if err := r.Destroy(ctx, d.yes); err != nil {
			return s.toolFailed(err)
		}
		fmt.Fprintln(stderr, s.st.err.Good("✓ destroyed "+cloud.String()))
		return exitOK
	}

	if len(b.certs) > 0 {
		if code := s.certificates(ctx, r, b, d.yes); code != exitOK {
			return code
		}
	}
	if err := r.Apply(ctx, d.yes); err != nil {
		return s.toolFailed(err)
	}
	s.summary(ctx, r, cloud)
	return exitOK
}

// init runs tofu init and records the provider versions it chose in
// clowd.lock, so the next build pins them.
func (s *session) init(ctx context.Context, r *runner.Runner, verbose bool) int {
	// A pin from clowd.lock that differs from what this directory last
	// installed (after a `git pull`, say) means the old selection must go,
	// or init refuses the new constraint.
	want := s.proj.Lock.Version("aws")
	if have, err := project.ReadTofuLock(r.Dir); err == nil && want != "" && have[aws.ProviderSource] != want {
		_ = os.Remove(filepath.Join(r.Dir, ".terraform.lock.hcl"))
	}
	fmt.Fprintln(s.stderr, s.st.err.Dim(fmt.Sprintf("Preparing %s (%s init)…", rel(r.Dir), r.Tool)))
	out, err := r.Init(ctx)
	if verbose || err != nil {
		var re *runner.Error
		if errors.As(err, &re) {
			out = re.Output
		}
		_, _ = s.stderr.Write(out)
	}
	if err != nil {
		return s.toolFailed(err)
	}
	have, err := project.ReadTofuLock(r.Dir)
	if err != nil {
		s.fail("reading the provider versions %s chose: %v", r.Tool, err)
		return exitFailure
	}
	version := have[aws.ProviderSource]
	if version == "" || version == want {
		return exitOK
	}
	lock := project.Lock{Providers: map[string]project.Provider{}}
	for k, v := range s.proj.Lock.Providers {
		lock.Providers[k] = v
	}
	lock.Providers["aws"] = project.Provider{Source: aws.ProviderSource, Version: version}
	if _, err := s.proj.WriteLock(lock); err != nil {
		s.fail("writing %s: %v", project.LockFile, err)
		return exitFailure
	}
	fmt.Fprintln(s.stderr, s.st.err.Dim(fmt.Sprintf("Pinned %s %s in %s; commit it.", aws.ProviderSource, version, project.LockFile)))
	return exitOK
}

func (s *session) toolFailed(err error) int {
	s.fail("%v", err)
	return exitFailure
}

// certificates creates the program's TLS certificates on their own first,
// then shows the DNS records that validate them. The full apply waits for
// validation, so without this step the user would be left waiting with no
// way to know which records to create.
func (s *session) certificates(ctx context.Context, r *runner.Runner, b *built, yes bool) int {
	state, err := r.Resources(ctx)
	if err != nil {
		return s.toolFailed(err)
	}
	if slices.ContainsFunc(b.certs, func(c string) bool { _, ok := state[c]; return !ok }) {
		fmt.Fprintln(s.stderr, s.st.err.Bold("Step 1 of 2: request the TLS certificates, which need DNS records to be issued."))
		if err := r.Apply(ctx, yes, b.certs...); err != nil {
			return s.toolFailed(err)
		}
		if state, err = r.Resources(ctx); err != nil {
			return s.toolFailed(err)
		}
		fmt.Fprintln(s.stderr, s.st.err.Bold("\nStep 2 of 2: deploy everything else."))
	}
	var records []string
	for _, addr := range b.certs {
		var cert struct {
			Status  string `json:"status"`
			Options []struct {
				Name  string `json:"resource_record_name"`
				Type  string `json:"resource_record_type"`
				Value string `json:"resource_record_value"`
			} `json:"domain_validation_options"`
		}
		if err := json.Unmarshal(state[addr], &cert); err != nil || cert.Status == "ISSUED" {
			continue
		}
		for _, o := range cert.Options {
			records = append(records, fmt.Sprintf("  %s  %s  →  %s", o.Type, o.Name, o.Value))
		}
	}
	if len(records) > 0 {
		fmt.Fprintln(s.stderr, s.st.err.Warning("Add these DNS records at your DNS provider.")+
			" The apply waits until AWS sees them (usually minutes, up to 72 hours for new records):")
		fmt.Fprintln(s.stderr, strings.Join(records, "\n"))
	}
	return exitOK
}

// summary prints where things ended up, from the configuration's outputs.
func (s *session) summary(ctx context.Context, r *runner.Runner, cloud ir.Cloud) {
	fmt.Fprintln(s.stderr, s.st.err.Good("\n✓ deployed to "+cloud.String()))
	outs, err := r.Outputs(ctx)
	if err != nil {
		return
	}
	values := map[string]string{}
	var names []string
	for name, raw := range outs {
		var v string
		if json.Unmarshal(raw, &v) == nil && !strings.HasSuffix(name, "_dns_name") {
			values[name] = v
			names = append(names, name)
		}
	}
	sort.Strings(names)
	width := 0
	for _, n := range names {
		width = max(width, len(n))
	}
	for _, n := range names {
		fmt.Fprintf(s.stdout, "  %s%s  %s\n", n, spaces(width-len(n)), values[n])
		// A TLS URL is on the user's domain, which must point at the
		// load balancer before it works.
		lb, isURL := strings.CutSuffix(n, "_url")
		var dns string
		if isURL && json.Unmarshal(outs[lb+"_dns_name"], &dns) == nil && !strings.Contains(values[n], dns) {
			host := strings.TrimPrefix(values[n], "https://")
			host, _, _ = strings.Cut(host, ":")
			fmt.Fprintf(s.stdout, "  %s  %s\n", spaces(width), s.st.out.Dim(fmt.Sprintf("point %s at %s with a CNAME record", host, dns)))
		}
	}
}

// ---- init ----

const initUsage = `Usage: clowd init [dir]

Creates clowd.toml, a starter app.clowd (unless the directory has .clowd
files already), and ignores .clowd/ in .gitignore.
`

const starter = `# Describe your infrastructure, then run ` + "`clowd check`" + ` and ` + "`clowd apply`" + `.

use aws in eu-west-2

service app
  image nginx:latest
  port 80

load balancer web
  listen on 80
  send / to app
`

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var color term.Mode
	fs.Var(&color, "color", colorUsage)
	fs.Usage = func() {
		fmt.Fprint(stderr, initUsage+"\n")
		fs.PrintDefaults()
	}
	paths, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitBadInput
	}
	st := term.For(stderr, color)
	failf := func(format string, args ...any) int {
		fmt.Fprintf(stderr, "%s %s\n", st.Error("clowd:"), fmt.Sprintf(format, args...))
		return exitBadInput
	}
	dir := "."
	switch len(paths) {
	case 0:
	case 1:
		dir = paths[0]
	default:
		return failf("pass at most one directory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return failf("%v", err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return failf("%v", err)
	}
	config := filepath.Join(dir, project.ConfigFile)
	if _, err := os.Stat(config); err == nil {
		return failf("%s already exists; this is a Clowd project", config)
	}

	var created []string
	write := func(name, text string) error {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			return err
		}
		created = append(created, path)
		return nil
	}
	if err := write(project.ConfigFile, project.InitConfig(project.DefaultName(abs))); err != nil {
		return failf("%v", err)
	}
	if len(sourceFiles(dir)) == 0 {
		if err := write("app.clowd", starter); err != nil {
			return failf("%v", err)
		}
	}
	if changed, err := ignoreWorkDir(filepath.Join(dir, ".gitignore")); err != nil {
		return failf("%v", err)
	} else if changed {
		created = append(created, filepath.Join(dir, ".gitignore"))
	}
	for _, c := range created {
		fmt.Fprintln(stdout, c)
	}
	fmt.Fprintln(stderr, st.Good(fmt.Sprintf("✓ created a Clowd project named %s", project.DefaultName(abs)))+
		st.Dim(" · next: `clowd check`, then `clowd apply`"))
	return exitOK
}

// ignoreWorkDir adds .clowd/ to a .gitignore, creating it if needed, and
// reports whether it changed anything.
func ignoreWorkDir(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		switch strings.TrimSpace(line) {
		case ".clowd", ".clowd/", "/.clowd", "/.clowd/":
			return false, nil
		}
	}
	text := string(data)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += "# Clowd build output and local OpenTofu state\n.clowd/\n"
	return true, os.WriteFile(path, []byte(text), 0o644)
}
