package check

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// Portable properties (D18): settings every cloud can express, with plain
// names. Where one cloud's limits are narrower than the language's, the
// capability matrix says so rather than the syntax.

const (
	minute = 60
	hour   = 60 * minute
	day    = 24 * hour
)

var durationRE = regexp.MustCompile(`^([0-9]+)(s|m|h|d)$`)

// duration parses `30s`, `5m`, `1h` or `14d` into seconds, between min
// and max.
func (c *checker) duration(v ast.Value, what string, min, max int) (int, bool) {
	m := durationRE.FindStringSubmatch(strings.ToLower(v.Text))
	if m == nil {
		d := diag.Errorf(v.Pos, "`%s` is not a duration", v.Text)
		if n, unit := splitUnit(strings.ToLower(v.Text)); n != "" && unit == "" {
			d = d.Suggest("add a unit: `%ss`, `%sm`, `%sh` or `%sd`", n, n, n, n)
		} else {
			d = d.Suggest("write a whole number of seconds, minutes, hours or days, such as `30s`, `5m`, `1h` or `14d`")
		}
		c.diags.Add(d)
		return 0, false
	}
	n, ok := number(m[1])
	n *= map[string]int{"s": 1, "m": minute, "h": hour, "d": day}[m[2]]
	if !ok || n < min || n > max {
		c.diags.Add(diag.Errorf(v.Pos, "`%s` is out of range", v.Text).
			Suggest("%s is between %s and %s", what, formatDuration(min), formatDuration(max)))
		return 0, false
	}
	return n, true
}

// formatDuration writes seconds in the largest unit that divides them.
func formatDuration(s int) string {
	for _, u := range []struct {
		n    int
		unit string
	}{{day, "d"}, {hour, "h"}, {minute, "m"}} {
		if s >= u.n && s%u.n == 0 {
			return fmt.Sprintf("%d%s", s/u.n, u.unit)
		}
	}
	return fmt.Sprintf("%ds", s)
}

// keyword checks that p's values start with the keyword words, as in
// `health check`, `expire after` and `keep messages for`, and returns the
// values after them. Values must follow.
func (c *checker) keyword(p ast.Property, form string, words ...string) ([]ast.Value, bool) {
	vs := p.Values
	for _, w := range words {
		if len(vs) == 0 || !isWord(vs[0], w) {
			at := diag.Span{Start: p.Pos.End, End: p.Pos.End}
			if len(vs) > 0 {
				at = vs[0].Pos
			}
			c.diags.Add(diag.Errorf(at, "expected `%s`", w).Suggest("write `%s`", form))
			return nil, false
		}
		vs = vs[1:]
	}
	if len(vs) == 0 {
		c.diags.Add(diag.Errorf(diag.Span{Start: p.Pos.End, End: p.Pos.End}, "expected a value").Suggest("write `%s`", form))
		return nil, false
	}
	return vs, true
}

// healthCheck parses `health check <path>`. A synonym such as
// `healthcheck /x` has been reported already and has no `check`.
func (c *checker) healthCheck(p ast.Property) (string, bool) {
	const form = "health check /healthz"
	vs := p.Values
	if isWord(vs[0], "check") || strings.EqualFold(p.Key.Text, "health") {
		var ok bool
		if vs, ok = c.keyword(p, form, "check"); !ok {
			return "", false
		}
	}
	if len(vs) > 1 {
		c.diags.Add(diag.Errorf(vs[1].Pos.Join(vs[len(vs)-1].Pos), "a health check takes one path").Suggest("write `%s`", form))
		return "", false
	}
	path := vs[0].Text
	if !strings.HasPrefix(path, "/") || strings.ContainsAny(path, " ?#") {
		c.diags.Add(diag.Errorf(vs[0].Pos, "`%s` is not a path", path).Suggest("paths start with `/`, such as `%s`", form))
		return "", false
	}
	return path, true
}

var storageRE = regexp.MustCompile(`^([0-9]+)gb$`)

// storage bounds, in GB: the smallest disk every cloud offers, and the
// largest.
const (
	minStorage = 20
	maxStorage = 65536
)

// storage parses `<n>gb`.
func (c *checker) storage(v ast.Value) (int, bool) {
	m := storageRE.FindStringSubmatch(strings.ToLower(v.Text))
	if m == nil {
		d := diag.Errorf(v.Pos, "`%s` is not a disk size", v.Text)
		if n, unit := splitUnit(strings.ToLower(v.Text)); n != "" && (unit == "" || unit == "g" || unit == "gib") {
			d = d.Suggest("write `%sgb`", n)
		} else {
			d = d.Suggest("write a whole number of `gb`, such as `storage 50gb`")
		}
		c.diags.Add(d)
		return 0, false
	}
	n, ok := number(m[1])
	if !ok || n < minStorage || n > maxStorage {
		c.diags.Add(diag.Errorf(v.Pos, "`%s` is out of range", v.Text).
			Suggest("storage is between %dgb and %dgb", minStorage, maxStorage))
		return 0, false
	}
	return n, true
}

// backups parses `backups <n> days` or `backups off`, into days (0 for
// off).
func (c *checker) backups(p ast.Property) (int, bool) {
	const form = "backups 14 days"
	vs := p.Values
	if len(vs) == 1 && isWord(vs[0], "off") {
		return 0, true
	}
	if len(vs) != 2 || !(isWord(vs[1], "days") || isWord(vs[1], "day")) {
		c.diags.Add(diag.Errorf(vs[0].Pos.Join(vs[len(vs)-1].Pos), "expected `backups <n> days` or `backups off`").
			Suggest("for example `%s`", form))
		return 0, false
	}
	n, ok := number(vs[0].Text)
	if !ok || n < 1 || n > 365 {
		c.diags.Add(diag.Errorf(vs[0].Pos, "`%s` is not a number of days from 1 to 365", vs[0].Text).
			Suggest("for example `%s`, or `backups off`", form))
		return 0, false
	}
	return n, true
}

// expire parses `expire after <duration>` into whole days: buckets
// expire objects by the day on every cloud.
func (c *checker) expire(p ast.Property) (int, bool) {
	const form = "expire after 30d"
	vs, ok := c.keyword(p, form, "after")
	if !ok {
		return 0, false
	}
	if len(vs) > 1 {
		c.diags.Add(diag.Errorf(vs[1].Pos.Join(vs[len(vs)-1].Pos), "`expire after` takes one duration").Suggest("write `%s`", form))
		return 0, false
	}
	s, ok := c.duration(vs[0], "expiry", day, 100*365*day)
	if !ok {
		return 0, false
	}
	if s%day != 0 {
		c.diags.Add(diag.Errorf(vs[0].Pos, "buckets expire objects in whole days").Suggest("write a number of days, such as `%s`", form))
		return 0, false
	}
	return s / day, true
}

// retry parses `retry <n> times`.
func (c *checker) retry(p ast.Property) (int, bool) {
	const form = "retry 5 times"
	vs := p.Values
	if len(vs) != 2 || !(isWord(vs[1], "times") || isWord(vs[1], "time")) {
		c.diags.Add(diag.Errorf(vs[0].Pos.Join(vs[len(vs)-1].Pos), "expected `retry <n> times`").Suggest("for example `%s`", form))
		return 0, false
	}
	n, ok := number(vs[0].Text)
	if !ok || n < 1 || n > 999 {
		c.diags.Add(diag.Errorf(vs[0].Pos, "`%s` is not a number of retries from 1 to 999", vs[0].Text).Suggest("for example `%s`", form))
		return 0, false
	}
	return n, true
}

// keep parses `keep messages for <duration>` into seconds.
func (c *checker) keep(p ast.Property) (int, bool) {
	const form = "keep messages for 4d"
	vs, ok := c.keyword(p, form, "messages", "for")
	if !ok {
		return 0, false
	}
	if len(vs) > 1 {
		c.diags.Add(diag.Errorf(vs[1].Pos.Join(vs[len(vs)-1].Pos), "`keep messages for` takes one duration").Suggest("write `%s`", form))
		return 0, false
	}
	return c.duration(vs[0], "how long messages are kept", minute, 31*day)
}
