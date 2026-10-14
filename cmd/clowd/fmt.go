package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/format"
)

// runFmt formats files in place, or with --check lists the files that are
// not formatted and changes nothing.
func runFmt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	checkOnly := fs.Bool("check", false, "list files that are not formatted, without changing them")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: clowd fmt [--check] [file.clowd ...]\n\n"+
			"With no files, formats every .clowd file in the current directory.\n"+
			"Files with syntax errors are left unchanged.\n\n")
		fs.PrintDefaults()
	}
	paths, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitBadInput
	}
	if len(paths) == 0 {
		paths, err = filepath.Glob("*.clowd")
		if err != nil || len(paths) == 0 {
			fmt.Fprintln(stderr, "clowd: no .clowd files in the current directory; pass a file to format")
			return exitBadInput
		}
	}

	code := exitOK
	for _, path := range paths {
		changed, ok := fmtFile(path, *checkOnly, stderr)
		switch {
		case !ok:
			code = exitBadInput
		case changed && *checkOnly:
			fmt.Fprintln(stdout, path)
			if code == exitOK {
				code = exitFailure
			}
		}
	}
	return code
}

// fmtFile formats one file, writing it back unless checkOnly. It reports
// whether the file was (or would be) changed, and ok is false when it could
// not be read, parsed or written; the problem has been printed.
func fmtFile(path string, checkOnly bool, stderr io.Writer) (changed, ok bool) {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "clowd: %v\n", err)
		return false, false
	}
	f := diag.NewFile(path, string(src))
	out, diags := format.File(f)
	if out == nil {
		_ = diag.RenderAll(stderr, f, diags)
		fmt.Fprintf(stderr, "\n%s: %s; not formatted\n", path, count(diags))
		return false, false
	}
	if bytes.Equal(out, src) {
		return false, true
	}
	if checkOnly {
		return true, true
	}
	info, err := os.Stat(path)
	if err == nil {
		err = os.WriteFile(path, out, info.Mode().Perm())
	}
	if err != nil {
		fmt.Fprintf(stderr, "clowd: %v\n", err)
		return true, false
	}
	return true, true
}
