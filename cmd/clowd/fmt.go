package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/format"
	"github.com/joshjkns/clowd/internal/term"
)

// runFmt formats files in place, or with --check lists the files that are
// not formatted and changes nothing.
func runFmt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	checkOnly := fs.Bool("check", false, "list files that are not formatted, without changing them")
	var color term.Mode
	fs.Var(&color, "color", colorUsage)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: clowd fmt [--check] [--color mode] [file.clowd ...]\n\n"+
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

	st := term.For(stderr, color)
	code := exitOK
	var changed, failed int
	for _, path := range paths {
		c, ok := fmtFile(path, *checkOnly, stderr, st)
		switch {
		case !ok:
			failed++
			code = exitBadInput
		case c:
			changed++
			// Paths go to stdout, one per line, so scripts can use them.
			fmt.Fprintln(stdout, path)
			if *checkOnly && code == exitOK {
				code = exitFailure
			}
		}
	}
	fmt.Fprintln(stderr, fmtSummary(st, len(paths), changed, failed, *checkOnly))
	return code
}

// fmtSummary is the closing line on stderr: "✓ 3 files already formatted",
// "✓ formatted 1 of 3 files", "✗ 2 of 3 files need formatting; run `clowd fmt`".
func fmtSummary(st term.Style, total, changed, failed int, checkOnly bool) string {
	var parts []string
	switch {
	case changed == 0 && failed == 0:
		parts = append(parts, st.Good("✓ "+filesPhrase(total, "already formatted")))
	case changed == 0:
	case checkOnly:
		verb := "need"
		if changed == 1 {
			verb = "needs"
		}
		parts = append(parts, st.Warning(fmt.Sprintf("✗ %d of %s %s formatting", changed, plural(total, "file"), verb))+"; run `clowd fmt`")
	default:
		parts = append(parts, st.Good(fmt.Sprintf("✓ formatted %d of %s", changed, plural(total, "file"))))
	}
	if failed > 0 {
		parts = append(parts, st.Error(fmt.Sprintf("✗ %s could not be formatted", plural(failed, "file"))))
	}
	return strings.Join(parts, st.Dim(" · "))
}

func filesPhrase(n int, what string) string {
	if n == 1 {
		return "1 file " + what
	}
	return fmt.Sprintf("%d files %s", n, what)
}

// fmtFile formats one file, writing it back unless checkOnly. It reports
// whether the file was (or would be) changed, and ok is false when it could
// not be read, parsed or written; the problem has been printed.
func fmtFile(path string, checkOnly bool, stderr io.Writer, st term.Style) (changed, ok bool) {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "%s %v\n", st.Error("clowd:"), err)
		return false, false
	}
	f := diag.NewFile(path, string(src))
	out, diags := format.File(f)
	if out == nil {
		_ = diag.RenderAllStyled(stderr, f, diags, st)
		fmt.Fprintf(stderr, "\n%s: %s; not formatted\n\n", st.Bold(path), st.Error(count(diags)))
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
		fmt.Fprintf(stderr, "%s %v\n", st.Error("clowd:"), err)
		return true, false
	}
	return true, true
}
