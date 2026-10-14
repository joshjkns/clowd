// Package term styles CLI output with ANSI colours when it goes to a
// terminal, and leaves it plain otherwise.
//
// The rules follow common practice: colour is on for a terminal unless
// NO_COLOR is set (https://no-color.org) or TERM is "dumb", and a
// --color=always|never flag overrides detection. Piped output, files and
// test buffers are plain, so golden files never contain escapes.
package term

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Style paints text when enabled. The zero Style is plain.
type Style struct {
	Color bool
}

// Mode is the --color setting.
type Mode int

// Colour modes.
const (
	Auto Mode = iota
	Always
	Never
)

// ParseMode parses a --color value.
func ParseMode(s string) (Mode, error) {
	switch s {
	case "auto":
		return Auto, nil
	case "always":
		return Always, nil
	case "never":
		return Never, nil
	}
	return Auto, fmt.Errorf("invalid colour mode %q (want auto, always or never)", s)
}

// Set implements flag.Value, so a Mode can be a --color flag.
func (m *Mode) Set(s string) error {
	v, err := ParseMode(s)
	if err != nil {
		return err
	}
	*m = v
	return nil
}

func (m Mode) String() string {
	switch m {
	case Always:
		return "always"
	case Never:
		return "never"
	}
	return "auto"
}

// For returns the style for output written to w in mode m.
func For(w io.Writer, m Mode) Style {
	switch m {
	case Always:
		return Style{Color: true}
	case Never:
		return Style{}
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return Style{}
	}
	f, ok := w.(*os.File)
	if !ok {
		return Style{}
	}
	info, err := f.Stat()
	return Style{Color: err == nil && info.Mode()&os.ModeCharDevice != 0}
}

// SGR codes used by Clowd. Only the basic 16 colours, so the user's
// terminal theme decides what they look like.
const (
	bold   = "1"
	dim    = "2"
	red    = "31"
	green  = "32"
	yellow = "33"
	blue   = "34"
	cyan   = "36"
)

func (s Style) paint(text string, codes ...string) string {
	if !s.Color || text == "" {
		return text
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + text + "\x1b[0m"
}

// Bold is for headings and messages.
func (s Style) Bold(text string) string { return s.paint(text, bold) }

// Dim is for secondary text: notes, legends, not-applicable marks.
func (s Style) Dim(text string) string { return s.paint(text, dim) }

// Error is for errors and anything that blocks a deploy.
func (s Style) Error(text string) string { return s.paint(text, bold, red) }

// Warning is for warnings and caveats.
func (s Style) Warning(text string) string { return s.paint(text, bold, yellow) }

// Info is for informational diagnostics.
func (s Style) Info(text string) string { return s.paint(text, bold, blue) }

// Good is for success.
func (s Style) Good(text string) string { return s.paint(text, bold, green) }

// Help is for suggestions.
func (s Style) Help(text string) string { return s.paint(text, bold, cyan) }

// Frame is for the structure around source snippets: gutters and arrows.
func (s Style) Frame(text string) string { return s.paint(text, bold, blue) }

// Red colours text red, without bold, for marks in tables.
func (s Style) Red(text string) string { return s.paint(text, red) }

// Green colours text green.
func (s Style) Green(text string) string { return s.paint(text, green) }

// Yellow colours text yellow.
func (s Style) Yellow(text string) string { return s.paint(text, yellow) }
