// Package runner drives OpenTofu (or Terraform) in a build directory.
//
// Clowd hands state, planning and applying to OpenTofu (D1). The runner
// keeps that boundary thin: it runs the tool with the user's terminal for
// the commands that need it (plan, apply, destroy, which ask for
// confirmation) and captures output for the ones Clowd reads (init, show,
// output), so the CLI can hide OpenTofu jargon unless asked for it.
package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Tools Clowd can run, in order of preference.
var Tools = []string{"tofu", "terraform"}

// Runner runs one tool in one directory.
type Runner struct {
	// Tool is "tofu" or "terraform", for messages.
	Tool string
	// Path is the executable.
	Path string
	// Dir is the root module's directory.
	Dir string
	// Stdin, Stdout and Stderr are given to commands that talk to the
	// user. Nil means the process's own.
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

// ErrNotFound is returned by Find when no tool is installed.
var ErrNotFound = errors.New("neither tofu nor terraform is installed")

// Find locates the tool to run: preferred if set, otherwise tofu if it is
// installed, otherwise terraform.
func Find(preferred string) (tool, path string, err error) {
	tools := Tools
	if preferred != "" {
		tools = []string{preferred}
	}
	for _, t := range tools {
		if p, err := exec.LookPath(t); err == nil {
			return t, p, nil
		}
	}
	if preferred != "" {
		return "", "", fmt.Errorf("%s is not installed", preferred)
	}
	return "", "", ErrNotFound
}

// Error is a command that ran and failed. The tool has already explained
// why on the output it was given (or in Output, for captured commands).
type Error struct {
	Tool    string
	Command string
	Output  []byte
	Err     error
}

func (e *Error) Error() string { return fmt.Sprintf("%s %s failed: %v", e.Tool, e.Command, e.Err) }

func (e *Error) Unwrap() error { return e.Err }

func (r *Runner) cmd(ctx context.Context, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, r.Path, args...)
	c.Dir = r.Dir
	// TF_IN_AUTOMATION stops the tool suggesting its own commands ("run
	// tofu apply"); the user runs clowd.
	c.Env = append(os.Environ(), "TF_IN_AUTOMATION=1")
	return c
}

// interactive runs a command on the user's terminal.
func (r *Runner) interactive(ctx context.Context, args ...string) error {
	c := r.cmd(ctx, args...)
	c.Stdin, c.Stdout, c.Stderr = or(r.Stdin, os.Stdin), orW(r.Stdout, os.Stdout), orW(r.Stderr, os.Stderr)
	if err := c.Run(); err != nil {
		return &Error{Tool: r.Tool, Command: args[0], Err: err}
	}
	return nil
}

// captured runs a command and returns its stdout. On failure the error
// holds everything the command printed.
func (r *Runner) captured(ctx context.Context, args ...string) ([]byte, error) {
	c := r.cmd(ctx, args...)
	var stdout, both bytes.Buffer
	c.Stdout = io.MultiWriter(&stdout, &both)
	c.Stderr = &both
	if err := c.Run(); err != nil {
		return nil, &Error{Tool: r.Tool, Command: args[0], Output: both.Bytes(), Err: err}
	}
	return stdout.Bytes(), nil
}

func or(r, def io.Reader) io.Reader {
	if r == nil {
		return def
	}
	return r
}

func orW(w, def io.Writer) io.Writer {
	if w == nil {
		return def
	}
	return w
}

// Init installs providers. Its output is returned rather than shown;
// it is noise unless something fails.
func (r *Runner) Init(ctx context.Context) ([]byte, error) {
	return r.captured(ctx, "init", "-input=false", "-no-color")
}

// Plan shows what apply would change.
func (r *Runner) Plan(ctx context.Context) error {
	return r.interactive(ctx, "plan")
}

// Apply applies the configuration, asking for confirmation unless
// autoApprove. With targets it applies only those resources and what
// they depend on.
func (r *Runner) Apply(ctx context.Context, autoApprove bool, targets ...string) error {
	args := []string{"apply"}
	if autoApprove {
		args = append(args, "-auto-approve")
	}
	for _, t := range targets {
		args = append(args, "-target="+t)
	}
	return r.interactive(ctx, args...)
}

// Destroy destroys everything in the state, asking for confirmation
// unless autoApprove.
func (r *Runner) Destroy(ctx context.Context, autoApprove bool) error {
	args := []string{"destroy"}
	if autoApprove {
		args = append(args, "-auto-approve")
	}
	return r.interactive(ctx, args...)
}

// Outputs returns the root module's outputs as raw JSON values.
func (r *Runner) Outputs(ctx context.Context) (map[string]json.RawMessage, error) {
	out, err := r.captured(ctx, "output", "-json", "-no-color")
	if err != nil {
		return nil, err
	}
	var raw map[string]struct {
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("reading %s output: %w", r.Tool, err)
	}
	values := make(map[string]json.RawMessage, len(raw))
	for k, v := range raw {
		values[k] = v.Value
	}
	return values, nil
}

// Resources returns the attribute values of every resource in the state,
// by address. An empty state gives an empty map.
func (r *Runner) Resources(ctx context.Context) (map[string]json.RawMessage, error) {
	out, err := r.captured(ctx, "show", "-json", "-no-color")
	if err != nil {
		return nil, err
	}
	var state struct {
		Values struct {
			RootModule struct {
				Resources []struct {
					Address string          `json:"address"`
					Values  json.RawMessage `json:"values"`
				} `json:"resources"`
			} `json:"root_module"`
		} `json:"values"`
	}
	if len(bytes.TrimSpace(out)) > 0 {
		if err := json.Unmarshal(out, &state); err != nil {
			return nil, fmt.Errorf("reading %s state: %w", r.Tool, err)
		}
	}
	res := map[string]json.RawMessage{}
	for _, rs := range state.Values.RootModule.Resources {
		res[rs.Address] = rs.Values
	}
	return res, nil
}
