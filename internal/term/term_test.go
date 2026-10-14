package term

import (
	"bytes"
	"testing"

	_ "github.com/joshjkns/clowd/internal/golden" // defines -update
)

func TestParseMode(t *testing.T) {
	for _, s := range []string{"auto", "always", "never"} {
		m, err := ParseMode(s)
		if err != nil || m.String() != s {
			t.Errorf("ParseMode(%q) = %v, %v", s, m, err)
		}
	}
	if _, err := ParseMode("yes"); err == nil {
		t.Error("ParseMode(yes) succeeded")
	}
}

func TestFor(t *testing.T) {
	var buf bytes.Buffer
	tests := []struct {
		name    string
		mode    Mode
		noColor string
		want    bool
	}{
		{"auto, not a terminal", Auto, "", false},
		{"always wins over NO_COLOR", Always, "1", true},
		{"never", Never, "", false},
		{"auto with NO_COLOR", Auto, "1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", tt.noColor)
			if got := For(&buf, tt.mode).Color; got != tt.want {
				t.Errorf("Color = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPaint(t *testing.T) {
	if got := (Style{}).Error("x"); got != "x" {
		t.Errorf("plain style painted: %q", got)
	}
	if got := (Style{Color: true}).Error("x"); got != "\x1b[1;31mx\x1b[0m" {
		t.Errorf("Error = %q", got)
	}
	if got := (Style{Color: true}).Bold(""); got != "" {
		t.Errorf("empty text painted: %q", got)
	}
}
