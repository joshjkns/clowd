// Command clowd compiles Clowd infrastructure files and deploys them with
// OpenTofu. See docs/plans/04-cli.md.
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strconv"

	"github.com/joshjkns/clowd/internal/check"
)

// Exit codes (04-cli.md).
const (
	exitOK       = 0
	exitFailure  = 1 // a check or deploy failed
	exitBadInput = 2 // usage or source errors
)

// colorUsage documents --color, which every command with human output takes.
const colorUsage = "colour output: `auto` (when writing to a terminal and NO_COLOR is unset), always or never"

// edition is the newest language edition this binary supports (06-versioning.md).
var edition = strconv.Itoa(check.LatestEdition)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `Clowd describes cloud infrastructure once and deploys it to several clouds.

Usage:
  clowd <command> [arguments]

Commands:
  init      create a project: clowd.toml and a starter app.clowd
  check     parse and check files, and show the per-cloud capability matrix
  fmt       format files in the canonical style
  build     compile to OpenTofu configuration in .clowd/build/<cloud>
  plan      build, then show what apply would change
  apply     build, then deploy
  destroy   destroy what apply deployed
  version   print the version and language edition
  help      show this help

Run "clowd <command> -h" for a command's flags.
`

// run is main without the process: it takes arguments and writers and
// returns the exit code, so the CLI can be tested end to end.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitBadInput
	}
	switch cmd, rest := args[0], args[1:]; cmd {
	case "check":
		return runCheck(rest, stdout, stderr)
	case "fmt":
		return runFmt(rest, stdout, stderr)
	case "init":
		return runInit(rest, stdout, stderr)
	case "build":
		return runBuild(rest, stdout, stderr)
	case "plan":
		return runPlan(rest, stdout, stderr)
	case "apply":
		return runApply(rest, stdout, stderr)
	case "destroy":
		return runDestroy(rest, stdout, stderr)
	case "version", "--version":
		fmt.Fprintf(stdout, "clowd %s (edition %s)\n", version(), edition)
		return exitOK
	case "help", "-h", "-help", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "clowd: unknown command %q\n\n%s", cmd, usage)
		return exitBadInput
	}
}

// version is the module version for `go install`ed builds, or "dev".
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}
