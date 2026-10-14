package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/joshjkns/clowd/internal/capability"
	"github.com/joshjkns/clowd/internal/compile"
	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
)

// targetsFlag collects `--target aws --target gcp` or `--target aws,gcp`.
type targetsFlag []ir.Cloud

func (t *targetsFlag) String() string {
	var s []string
	for _, c := range *t {
		s = append(s, c.String())
	}
	return strings.Join(s, ",")
}

func (t *targetsFlag) Set(v string) error {
	for _, name := range strings.Split(v, ",") {
		c, ok := ir.ParseCloud(strings.TrimSpace(name))
		if !ok {
			return fmt.Errorf("unknown cloud %q (clouds are aws, gcp and azure)", name)
		}
		if !slices.Contains(*t, c) {
			*t = append(*t, c)
		}
	}
	return nil
}

// checked is one file's result.
type checked struct {
	file   *diag.File
	diags  diag.List
	report *capability.Report // nil when the file has errors
	// err is set when the file could not be read; file then has only a name.
	err error
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var targets targetsFlag
	fs.Var(&targets, "target", "check against these `cloud`s instead of the file's use lines (repeatable, or comma-separated)")
	asJSON := fs.Bool("json", false, "print machine-readable JSON")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: clowd check [--target cloud] [--json] [file.clowd ...]\n\n"+
			"With no files, checks every .clowd file in the current directory.\n\n")
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
			fmt.Fprintln(stderr, "clowd: no .clowd files in the current directory; pass a file to check")
			return exitBadInput
		}
	}

	var results []checked
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			// Keep going, so the other files are still checked and --json
			// still prints a result for every file.
			results = append(results, checked{file: diag.NewFile(path, ""), err: err})
			continue
		}
		results = append(results, checkFile(diag.NewFile(path, string(src)), targets))
	}

	if *asJSON {
		if err := writeJSON(stdout, results); err != nil {
			fmt.Fprintf(stderr, "clowd: %v\n", err)
			return exitBadInput
		}
	} else {
		for i, r := range results {
			if i > 0 {
				fmt.Fprintln(stdout)
			}
			writeHuman(stdout, stderr, r, len(results) > 1)
		}
	}

	code := exitOK
	for _, r := range results {
		switch {
		case r.report == nil:
			code = exitBadInput
		case !r.report.AllDeployable() && code == exitOK:
			code = exitFailure
		}
	}
	return code
}

// parseInterspersed parses flags that may come before or after file
// arguments (`clowd check app.clowd --json`), which flag.Parse alone stops
// at.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return positional, nil
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
}

func checkFile(f *diag.File, targets []ir.Cloud) checked {
	res := compile.Check(f)
	r := checked{file: f, diags: res.Diags}
	if res.Diags.HasErrors() {
		return r
	}
	clouds := targets
	if len(clouds) == 0 {
		for _, t := range res.Program.Targets {
			clouds = append(clouds, t.Cloud)
		}
	}
	r.report = capability.Evaluate(res.Program, clouds)
	r.diags.Add(r.report.Diagnostics()...)
	r.diags.Sort()
	return r
}

// ---- human output ----

func writeHuman(stdout, stderr io.Writer, r checked, header bool) {
	if r.err != nil {
		fmt.Fprintf(stderr, "clowd: %v\n", r.err)
		return
	}
	if len(r.diags) > 0 {
		_ = diag.RenderAll(stderr, r.file, r.diags)
		fmt.Fprintln(stderr)
	}
	if r.report == nil {
		fmt.Fprintf(stderr, "%s: %s; fix them to see the capability matrix\n", r.file.Name, count(r.diags))
		return
	}
	if header {
		fmt.Fprintf(stdout, "%s\n", r.file.Name)
	}
	writeTable(stdout, r.report)
}

// count summarises diagnostics: "2 errors, 1 warning".
func count(l diag.List) string {
	var errs, warns int
	for _, d := range l {
		switch d.Severity {
		case diag.Error:
			errs++
		case diag.Warning:
			warns++
		}
	}
	s := plural(errs, "error")
	if warns > 0 {
		s += ", " + plural(warns, "warning")
	}
	return s
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// writeTable prints the matrix (03-check.md):
//
//	                    aws  gcp
//	service api         ✓    ⚠    gcp: ...
//
//	Deployable: aws ✓  gcp ✓
func writeTable(w io.Writer, rep *capability.Report) {
	if len(rep.Clouds) == 0 {
		fmt.Fprintln(w, "No targets: add a `use` line or pass --target.")
		return
	}
	labelWidth := 0
	for _, row := range rep.Rows {
		labelWidth = max(labelWidth, utf8.RuneCountInString(row.Label))
	}
	colWidth := 0
	for _, c := range rep.Clouds {
		colWidth = max(colWidth, len(c.String()))
	}
	colWidth += 2

	var b strings.Builder
	b.WriteString(strings.Repeat(" ", labelWidth+2))
	for _, c := range rep.Clouds {
		b.WriteString(pad(c.String(), colWidth))
	}
	line(w, b.String())
	for _, row := range rep.Rows {
		b.Reset()
		b.WriteString(pad(row.Label, labelWidth+2))
		for _, cell := range row.Cells {
			b.WriteString(pad(cell.Level.Symbol(), colWidth))
		}
		b.WriteString(rowNote(rep, row))
		line(w, b.String())
	}

	b.Reset()
	b.WriteString("Deployable:")
	for i, c := range rep.Clouds {
		mark := "✓"
		if !rep.Deployable(i) {
			mark = "✗"
		}
		fmt.Fprintf(&b, " %s %s ", c, mark)
	}
	fmt.Fprintln(w)
	line(w, b.String())
}

// line writes s without trailing spaces.
func line(w io.Writer, s string) { fmt.Fprintln(w, strings.TrimRight(s, " ")) }

func pad(s string, width int) string {
	return s + strings.Repeat(" ", max(0, width-utf8.RuneCountInString(s)))
}

// rowNote is the short explanation after a row's marks: its first finding,
// with the clouds it applies to, and how many more there are. The full
// list is in the diagnostics above the table.
func rowNote(rep *capability.Report, row capability.Row) string {
	type note struct {
		text   string
		clouds []string
	}
	var notes []note
	for i, cell := range row.Cells {
		for _, f := range cell.Findings {
			text := f.Note
			if f.Level == capability.NotImplemented {
				text = fmt.Sprintf("%s not implemented yet", f.Feature)
			}
			j := slices.IndexFunc(notes, func(n note) bool { return n.text == text })
			if j < 0 {
				notes = append(notes, note{text: text})
				j = len(notes) - 1
			}
			notes[j].clouds = append(notes[j].clouds, rep.Clouds[i].String())
		}
	}
	if len(notes) == 0 {
		return ""
	}
	s := fmt.Sprintf("%s: %s", strings.Join(notes[0].clouds, ", "), notes[0].text)
	if len(notes) > 1 {
		s += fmt.Sprintf(" (+%d more)", len(notes)-1)
	}
	return s
}

// ---- JSON output ----

// The JSON shape is part of the CLI's interface for CI; add fields rather
// than changing existing ones.
type jsonOutput struct {
	Files []jsonFile `json:"files"`
}

type jsonFile struct {
	Path        string     `json:"path"`
	OK          bool       `json:"ok"`
	Clouds      []string   `json:"clouds"`
	Deployable  jsonClouds `json:"deployable"`
	Rows        []jsonRow  `json:"rows"`
	Diagnostics []jsonDiag `json:"diagnostics"`
	// Error is set when the file could not be read.
	Error string `json:"error,omitempty"`
}

// jsonClouds maps a cloud name to whether it is deployable.
type jsonClouds map[string]bool

type jsonRow struct {
	Label string              `json:"label"`
	Pos   jsonPos             `json:"pos"`
	Cells map[string]jsonCell `json:"cells"`
}

type jsonCell struct {
	Level    string        `json:"level"`
	Findings []jsonFinding `json:"findings"`
}

type jsonFinding struct {
	Feature string  `json:"feature"`
	Level   string  `json:"level"`
	Note    string  `json:"note,omitempty"`
	Pos     jsonPos `json:"pos"`
}

type jsonDiag struct {
	Severity   string  `json:"severity"`
	Message    string  `json:"message"`
	Suggestion string  `json:"suggestion,omitempty"`
	Pos        jsonPos `json:"pos"`
}

// jsonPos is a 1-based line and column range; End is exclusive.
type jsonPos struct {
	Line      int `json:"line"`
	Column    int `json:"column"`
	EndLine   int `json:"endLine"`
	EndColumn int `json:"endColumn"`
}

func toPos(f *diag.File, sp diag.Span) jsonPos {
	s, e := f.Position(sp.Start), f.Position(sp.End)
	return jsonPos{Line: s.Line, Column: s.Column, EndLine: e.Line, EndColumn: e.Column}
}

func writeJSON(w io.Writer, results []checked) error {
	out := jsonOutput{Files: []jsonFile{}}
	for _, r := range results {
		jf := jsonFile{
			Path:        r.file.Name,
			OK:          r.report != nil && r.report.AllDeployable(),
			Clouds:      []string{},
			Deployable:  jsonClouds{},
			Rows:        []jsonRow{},
			Diagnostics: []jsonDiag{},
		}
		if r.err != nil {
			jf.Error = r.err.Error()
		}
		for _, d := range r.diags {
			jf.Diagnostics = append(jf.Diagnostics, jsonDiag{
				Severity:   d.Severity.String(),
				Message:    d.Message,
				Suggestion: d.Suggestion,
				Pos:        toPos(r.file, d.Span),
			})
		}
		if rep := r.report; rep != nil {
			for i, c := range rep.Clouds {
				jf.Clouds = append(jf.Clouds, c.String())
				jf.Deployable[c.String()] = rep.Deployable(i)
			}
			for _, row := range rep.Rows {
				jr := jsonRow{Label: row.Label, Pos: toPos(r.file, row.Pos), Cells: map[string]jsonCell{}}
				for i, cell := range row.Cells {
					jc := jsonCell{Level: cell.Level.String(), Findings: []jsonFinding{}}
					for _, f := range cell.Findings {
						jc.Findings = append(jc.Findings, jsonFinding{
							Feature: string(f.Feature),
							Level:   f.Level.String(),
							Note:    f.Note,
							Pos:     toPos(r.file, f.Pos),
						})
					}
					jr.Cells[rep.Clouds[i].String()] = jc
				}
				jf.Rows = append(jf.Rows, jr)
			}
		}
		out.Files = append(out.Files, jf)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
