// Package spell finds the word a user probably meant, for "did you mean"
// suggestions in every compiler stage.
package spell

import "strings"

// IsTypo reports whether got is a plausible misspelling of want: within one
// edit for short words and two for longer ones, but not identical.
func IsTypo(got, want string) bool {
	limit := 1
	if len(want) > 4 {
		limit = 2
	}
	d := Distance(got, want)
	return d > 0 && d <= limit
}

// Closest returns the candidate that word is most plausibly a misspelling
// of, comparing case-insensitively. Ties go to the earliest candidate, so
// callers control preference through order.
func Closest(word string, candidates []string) (string, bool) {
	w := strings.ToLower(word)
	best, bestDist := "", -1
	for _, c := range candidates {
		lc := strings.ToLower(c)
		if !IsTypo(w, lc) {
			continue
		}
		if d := Distance(w, lc); bestDist < 0 || d < bestDist {
			best, bestDist = c, d
		}
	}
	return best, bestDist >= 0
}

// Distance is the optimal string alignment distance between a and b:
// insertions, deletions, substitutions and adjacent transpositions each
// cost 1, so "servcie" is one edit from "service".
func Distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	// d[i][j] is the distance between ra[:i] and rb[:j].
	d := make([][]int, len(ra)+1)
	for i := range d {
		d[i] = make([]int, len(rb)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(ra); i++ {
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
			if i > 1 && j > 1 && ra[i-1] == rb[j-2] && ra[i-2] == rb[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(ra)][len(rb)]
}
