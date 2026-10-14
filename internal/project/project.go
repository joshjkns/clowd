// Package project loads a Clowd project: the directory holding its .clowd
// file, and the clowd.lock provider pins the CLI keeps there
// (06-versioning.md).
//
// Everything about a project is written in its .clowd file (D17): its
// name, edition and tags in a `project` declaration, and its network in a
// `network` declaration. A lone .clowd file with neither is a complete
// project, named after its directory.
package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/joshjkns/clowd/internal/check"
	"github.com/joshjkns/clowd/internal/diag"
)

// File names.
const (
	LockFile = "clowd.lock"
	// WorkDir holds build output and OpenTofu working directories; it is
	// gitignored.
	WorkDir = ".clowd"
	// oldConfig is the settings file Clowd used to read. Finding one is an
	// error, so its settings aren't silently ignored.
	oldConfig = "clowd.toml"
)

// Project is a loaded project.
type Project struct {
	// Root is the directory holding the project's .clowd file.
	Root string
	Lock Lock
}

// BuildDir is where the root module for cloud is written.
func (p *Project) BuildDir(cloud string) string {
	return filepath.Join(p.Root, WorkDir, "build", cloud)
}

// Name returns the project's name: declared, if the program has a
// `project` declaration with a name, and otherwise the directory's.
func (p *Project) Name(declared string) string {
	if declared != "" {
		return declared
	}
	return DefaultName(p.Root)
}

// Load loads the project in dir. User mistakes in clowd.lock, or a
// leftover clowd.toml, are returned as diagnostics against src, the file
// they are in; err is for I/O failures.
func Load(dir string) (p *Project, src *diag.File, diags diag.List, err error) {
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	p = &Project{Root: dir}

	path := filepath.Join(dir, oldConfig)
	switch text, err := os.ReadFile(path); {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, nil, nil, err
	default:
		src = diag.NewFile(relTo(path), string(text))
		diags.Add(diag.Errorf(diag.Span{}, "%s is no longer read; project settings are written in the .clowd file", oldConfig).
			Suggest("`name = \"x\"` is now `project x`, with `edition` in its block; `services = \"private\"` is `network is private`, with `cidr` in its block; `vpc` and the subnets go in an `on aws` block under `network`; `runtime` is `--runtime` or CLOWD_RUNTIME. Then delete %s", oldConfig))
		return p, src, diags, nil
	}

	path = filepath.Join(dir, LockFile)
	text, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return nil, nil, nil, err
	default:
		src = diag.NewFile(relTo(path), string(text))
		if ld := p.Lock.decode(src); len(ld) > 0 {
			return p, src, ld, nil
		}
	}
	return p, nil, nil, nil
}

// relTo returns path relative to the working directory when that is
// shorter, for messages.
func relTo(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	if rel, err := filepath.Rel(wd, path); err == nil && len(rel) < len(path) {
		return rel
	}
	return path
}

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
	return strings.TrimRight(name[:min(len(name), check.MaxProjectName)], "-")
}
