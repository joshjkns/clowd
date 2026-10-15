package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/joshjkns/clowd/internal/lsp"
)

// runLSP serves the language server on stdin and stdout. Editors start it;
// people don't run it by hand.
func runLSP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("lsp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	// --stdio is what VS Code's client passes; stdio is the only transport.
	fs.Bool("stdio", true, "talk to the editor over stdin and stdout (the default and only transport)")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: clowd lsp\n\nRuns the language server on stdin and stdout, for editors.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitOK
		}
		return exitBadInput
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "clowd lsp: unexpected argument %q\n", fs.Arg(0))
		return exitBadInput
	}
	if err := lsp.Serve(stdin, stdout, version()); err != nil {
		fmt.Fprintf(stderr, "clowd lsp: %v\n", err)
		return exitFailure
	}
	return exitOK
}

// stdin is the process's standard input; tests replace it.
var stdin io.Reader = os.Stdin
