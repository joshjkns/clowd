package check

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/joshjkns/clowd/internal/diag"
	"github.com/joshjkns/clowd/internal/ir"
	"github.com/joshjkns/clowd/internal/syntax/ast"
)

// Value words are interpreted here, from the property they appear in
// (decision D8: the lexer only knows words).

// number parses a non-negative decimal integer without sign or separators.
func number(s string) (int, bool) {
	if s == "" || len(s) > 9 {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

func (c *checker) port(v ast.Value, what string) (int, bool) {
	n, ok := number(v.Text)
	if !ok || n < 1 || n > 65535 {
		c.diags.Add(diag.Errorf(v.Pos, "`%s` is not a port", v.Text).
			Suggest("a %s is a number from 1 to 65535", what))
		return 0, false
	}
	return n, true
}

// maxInstances bounds `scale` to catch typos like `scale 1 to 1000000`.
const maxInstances = 1000

// scale parses `scale <n>` or `scale <min> to <max>`.
func (c *checker) scale(p ast.Property) (ir.Scale, bool) {
	vs := p.Values
	all := vs[0].Pos.Join(vs[len(vs)-1].Pos)
	bad := func(sp diag.Span, format string, args ...any) (ir.Scale, bool) {
		c.diags.Add(diag.Errorf(sp, format, args...).
			Suggest("write `scale <count>` or `scale <min> to <max>`, such as `scale 2 to 10`"))
		return ir.Scale{}, false
	}
	count := func(v ast.Value) (int, bool) {
		n, ok := number(v.Text)
		if !ok {
			c.diags.Add(diag.Errorf(v.Pos, "`%s` is not an instance count", v.Text).
				Suggest("write `scale <count>` or `scale <min> to <max>`, such as `scale 2 to 10`"))
		}
		return n, ok
	}

	var sc ir.Scale
	switch {
	case len(vs) == 1:
		// `scale 2-10` and `scale 2..10` are common guesses.
		for _, sep := range []string{"..", "-"} {
			if lo, hi, ok := strings.Cut(vs[0].Text, sep); ok {
				if _, ok1 := number(lo); ok1 {
					if _, ok2 := number(hi); ok2 {
						return bad(vs[0].Pos, "ranges are written with `to`")
					}
				}
			}
		}
		n, ok := count(vs[0])
		if !ok {
			return ir.Scale{}, false
		}
		sc = ir.Scale{Min: n, Max: n}
	case len(vs) == 3 && isWord(vs[1], "to"):
		lo, ok1 := count(vs[0])
		hi, ok2 := count(vs[2])
		if !ok1 || !ok2 {
			return ir.Scale{}, false
		}
		sc = ir.Scale{Min: lo, Max: hi}
		if lo > hi {
			c.diags.Add(diag.Errorf(all, "the minimum (%d) is larger than the maximum (%d)", lo, hi).
				Suggest("write `scale %d to %d`", hi, lo))
			return ir.Scale{}, false
		}
		if lo == hi {
			c.diags.Add(diag.Warnf(all, "`%d to %d` is a fixed count", lo, hi).
				Suggest("write `scale %d`", lo))
		}
	default:
		return bad(all, "can't read `%s` as a scale", joinValues(vs))
	}
	if sc.Max == 0 {
		c.diags.Add(diag.Errorf(all, "a service needs at least one instance").
			Suggest("to scale to zero when idle, write `scale 0 to <max>`"))
		return ir.Scale{}, false
	}
	if sc.Max > maxInstances {
		c.diags.Add(diag.Errorf(all, "%d instances is more than the limit of %d", sc.Max, maxInstances))
		return ir.Scale{}, false
	}
	return sc, true
}

// cpuSizes are the vCPU amounts a service may ask for, in millicores. They
// are the sizes both Fargate and Cloud Run offer; capability narrows them
// per cloud.
var cpuSizes = []int{250, 500, 1000, 2000, 4000, 8000}

const cpuHelp = "`cpu` is in vCPUs: 0.25, 0.5, 1, 2, 4 or 8"

func (c *checker) cpu(v ast.Value) (int, bool) {
	text := strings.ToLower(v.Text)
	if m, ok := strings.CutSuffix(text, "m"); ok {
		if n, ok := number(m); ok {
			c.diags.Add(diag.Errorf(v.Pos, "`cpu` is not written in millicores").
				Suggest("write `cpu %s`", strconv.FormatFloat(float64(n)/1000, 'f', -1, 64)))
			return 0, false
		}
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsNaN(f) || strings.ContainsAny(text, "eEx+-") {
		c.diags.Add(diag.Errorf(v.Pos, "`%s` is not a CPU size", v.Text).Suggest(cpuHelp))
		return 0, false
	}
	milli := int(math.Round(f * 1000))
	for _, s := range cpuSizes {
		if s == milli {
			return milli, true
		}
	}
	c.diags.Add(diag.Errorf(v.Pos, "%s vCPUs is not an available size", v.Text).Suggest(cpuHelp))
	return 0, false
}

var memoryRE = regexp.MustCompile(`^([0-9]+)(mb|gb)$`)

// maxMemory bounds `memory`, in MiB, to catch unit mistakes.
const maxMemory = 128 * 1024

// memory parses `512mb` or `2gb` into MiB. Units are binary (1gb = 1024mb),
// matching how both Fargate and Cloud Run size memory.
func (c *checker) memory(v ast.Value) (int, bool) {
	text := strings.ToLower(v.Text)
	m := memoryRE.FindStringSubmatch(text)
	if m == nil {
		d := diag.Errorf(v.Pos, "`%s` is not an amount of memory", v.Text)
		switch n, unit := splitUnit(text); {
		case n != "" && unit == "":
			d = d.Suggest("add a unit: `%smb` or `%sgb`", n, n)
		case n != "" && (unit == "m" || unit == "mi" || unit == "mib"):
			d = d.Suggest("write `%smb`", n)
		case n != "" && (unit == "g" || unit == "gi" || unit == "gib"):
			d = d.Suggest("write `%sgb`", n)
		default:
			d = d.Suggest("write a whole number of `mb` or `gb`, such as `memory 512mb`")
		}
		c.diags.Add(d)
		return 0, false
	}
	n, ok := number(m[1])
	if m[2] == "gb" {
		n *= 1024
	}
	if !ok || n == 0 || n > maxMemory {
		c.diags.Add(diag.Errorf(v.Pos, "`%s` is out of range", v.Text).
			Suggest("memory is between 1mb and 128gb"))
		return 0, false
	}
	return n, true
}

// splitUnit splits "512mib" into "512" and "mib".
func splitUnit(s string) (num, unit string) {
	i := strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' })
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i:]
}

var envNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// env parses `env <NAME> <value>`.
func (c *checker) env(p ast.Property) (ir.EnvVar, bool) {
	vs := p.Values
	name := vs[0]
	if k, v, ok := strings.Cut(name.Text, "="); ok && len(vs) == 1 && !name.Quoted {
		c.diags.Add(diag.Errorf(name.Pos, "`env` takes a name and a value separated by a space").
			Suggest("write `env %s %s`", k, v))
		return ir.EnvVar{}, false
	}
	if !envNameRE.MatchString(name.Text) {
		c.diags.Add(diag.Errorf(name.Pos, "`%s` is not a valid environment variable name", name.Text).
			Suggest("use letters, digits and `_`, not starting with a digit, such as `LOG_LEVEL`"))
		return ir.EnvVar{}, false
	}
	switch {
	case len(vs) == 1:
		c.diags.Add(diag.Errorf(diag.Span{Start: name.Pos.End, End: name.Pos.End}, "expected a value after `%s`", name.Text).
			Suggest("write `env %s <value>`; use `\"\"` for an empty value", name.Text))
		return ir.EnvVar{}, false
	case len(vs) > 2:
		c.diags.Add(diag.Errorf(vs[1].Pos.Join(vs[len(vs)-1].Pos), "`env` takes one value").
			Suggest("quote values that contain spaces: `env %s \"%s\"`", name.Text, joinValues(vs[1:])))
		return ir.EnvVar{}, false
	}
	return ir.EnvVar{Name: name.Text, Value: vs[1].Text, Pos: p.Pos}, true
}

// listen parses `listen on <port> [with tls for <domain>]`.
func (c *checker) listen(p ast.Property) (ir.Listener, bool) {
	vs := p.Values
	const form = "write `listen on <port>` or `listen on <port> with tls for <domain>`"
	if !isWord(vs[0], "on") {
		if _, ok := number(vs[0].Text); ok {
			c.diags.Add(diag.Errorf(vs[0].Pos, "expected `on` before the port").
				Suggest("write `listen on %s`", joinValues(vs)))
		} else {
			c.diags.Add(diag.Errorf(vs[0].Pos, "expected `on`").Suggest(form))
		}
		return ir.Listener{}, false
	}
	if len(vs) < 2 {
		c.diags.Add(diag.Errorf(diag.Span{Start: vs[0].Pos.End, End: vs[0].Pos.End}, "expected a port after `on`").Suggest(form))
		return ir.Listener{}, false
	}
	port, ok := c.port(vs[1], "listener port")
	if !ok {
		return ir.Listener{}, false
	}
	l := ir.Listener{Port: port, Pos: p.Pos}
	rest := vs[2:]
	switch {
	case len(rest) == 0:
		return l, true
	case len(rest) == 4 && isWord(rest[0], "with") && isWord(rest[1], "tls") && isWord(rest[2], "for"):
		domain, ok := c.domain(rest[3])
		if !ok {
			return ir.Listener{}, false
		}
		l.Domain = domain
		return l, true
	case len(rest) == 2 && isWord(rest[0], "with") && isWord(rest[1], "tls"):
		c.diags.Add(diag.Errorf(rest[1].Pos, "TLS needs a domain for its certificate").
			Suggest("write `listen on %d with tls for api.example.com`", port))
		return ir.Listener{}, false
	case len(rest) == 3 && isWord(rest[0], "with") && isWord(rest[1], "tls") && !isWord(rest[2], "for"):
		c.diags.Add(diag.Errorf(rest[2].Pos, "expected `for` before the domain").
			Suggest("write `listen on %d with tls for %s`", port, rest[2].Text))
		return ir.Listener{}, false
	}
	c.diags.Add(diag.Errorf(rest[0].Pos.Join(rest[len(rest)-1].Pos), "unexpected `%s`", joinValues(rest)).Suggest(form))
	return ir.Listener{}, false
}

var domainRE = regexp.MustCompile(`^(\*\.)?([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,63}$`)

func (c *checker) domain(v ast.Value) (string, bool) {
	text := strings.ToLower(v.Text)
	for _, scheme := range []string{"https://", "http://"} {
		if rest, ok := strings.CutPrefix(text, scheme); ok {
			c.diags.Add(diag.Errorf(v.Pos, "a domain doesn't include `%s`", scheme).
				Suggest("write `%s`", strings.TrimSuffix(rest, "/")))
			return "", false
		}
	}
	if !domainRE.MatchString(text) {
		c.diags.Add(diag.Errorf(v.Pos, "`%s` is not a domain name", v.Text).
			Suggest("for example `api.example.com`"))
		return "", false
	}
	return text, true
}

var pathRE = regexp.MustCompile(`^/[A-Za-z0-9._~!$&'()+,;=:@%/-]*$`)

// route parses `send <path> to <service>`. resolve has already resolved the
// target of the well-formed shape.
func (c *checker) route(p ast.Property) (ir.Route, bool) {
	vs := p.Values
	if len(vs) == 2 && !isWord(vs[0], "to") && !isWord(vs[1], "to") {
		c.diags.Add(diag.Errorf(vs[1].Pos, "expected `to` before the service").
			Suggest("write `send %s to %s`", vs[0].Text, vs[1].Text))
		return ir.Route{}, false
	}
	if len(vs) != 3 || !isWord(vs[1], "to") {
		c.diags.Add(diag.Errorf(vs[0].Pos.Join(vs[len(vs)-1].Pos), "can't read `%s` as a route", joinValues(vs)).
			Suggest("write `send <path> to <service>`, such as `send / to %s`", c.someService()))
		return ir.Route{}, false
	}
	path, target := vs[0], vs[2]
	if strings.HasSuffix(path.Text, "*") {
		trimmed := strings.TrimRight(strings.TrimSuffix(path.Text, "*"), "/")
		if trimmed == "" {
			trimmed = "/"
		}
		c.diags.Add(diag.Errorf(path.Pos, "paths don't take wildcards").
			Suggest("paths match by prefix, so write `send %s to %s`", trimmed, target.Text))
		return ir.Route{}, false
	}
	if !pathRE.MatchString(path.Text) {
		d := diag.Errorf(path.Pos, "`%s` is not a path", path.Text)
		if !strings.HasPrefix(path.Text, "/") {
			d = d.Suggest("paths start with `/`: `send /%s to %s`", path.Text, target.Text)
		}
		c.diags.Add(d)
		return ir.Route{}, false
	}
	decl := c.table.Lookup(target.Text)
	if decl == nil {
		return ir.Route{}, false // resolve has reported it
	}
	if decl.Kind != ast.Service {
		c.diags.Add(diag.Errorf(target.Pos, "`%s` is a %s; a load balancer can only send to a service", decl.Name.Name, decl.Kind))
		return ir.Route{}, false
	}
	return ir.Route{Path: path.Text, Service: decl.Name.Name, Pos: p.Pos}, true
}

var versionRE = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)

func validVersion(s string) bool { return versionRE.MatchString(s) }

func isWord(v ast.Value, w string) bool { return !v.Quoted && strings.EqualFold(v.Text, w) }

func joinValues(vs []ast.Value) string {
	words := make([]string, len(vs))
	for i, v := range vs {
		words[i] = v.Text
	}
	return strings.Join(words, " ")
}
