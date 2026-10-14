package spell

import (
	"testing"

	_ "github.com/joshjkns/clowd/internal/golden" // defines -update
)

func TestDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"service", "service", 0},
		{"servcie", "service", 1}, // transposition
		{"servic", "service", 1},
		{"sevrcie", "service", 2},
		{"", "abc", 3},
		{"queue", "quuee", 1}, // adjacent swap
	}
	for _, tt := range tests {
		if got := Distance(tt.a, tt.b); got != tt.want {
			t.Errorf("Distance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestClosest(t *testing.T) {
	tests := []struct {
		word       string
		candidates []string
		want       string
		ok         bool
	}{
		{"dbb", []string{"db", "uploads"}, "db", true},
		{"uplods", []string{"db", "uploads"}, "uploads", true},
		{"UPLODS", []string{"db", "uploads"}, "uploads", true},
		{"cache", []string{"db", "uploads"}, "", false},
		{"db", []string{"db"}, "", false},                             // identical is not a typo
		{"servise", []string{"services", "service"}, "service", true}, // nearer wins
		{"ab", []string{"ac", "ad"}, "ac", true},                      // tie: first wins
	}
	for _, tt := range tests {
		got, ok := Closest(tt.word, tt.candidates)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Closest(%q, %v) = %q, %v; want %q, %v", tt.word, tt.candidates, got, ok, tt.want, tt.ok)
		}
	}
}
