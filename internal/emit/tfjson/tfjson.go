// Package tfjson writes OpenTofu/Terraform configurations in JSON syntax
// (.tf.json), which both tools read.
//
// Backends build a Config of resources whose bodies are plain Go values.
// The output is meant to be read by people who eject from Clowd, so it is
// deterministic and keeps the order the backend added things in: resources
// appear in the order they were added (grouped by type, as the syntax
// requires), and attributes are sorted by name.
//
// In .tf.json every string is a template, so `${` in a user's value would
// be evaluated. Plain Go strings are therefore always escaped, and only an
// Expr is emitted as written: references can't be injected by accident.
package tfjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Expr is a template string emitted as written, e.g. "${aws_vpc.main.id}".
type Expr string

// Ref returns an expression that evaluates addr ("aws_vpc.main.id").
// A template that is exactly one interpolation keeps the value's type,
// so a Ref can stand for a list or a number as well as a string.
func Ref(addr string) Expr { return Expr("${" + addr + "}") }

// Escape makes s a template that evaluates to s.
func Escape(s string) string {
	s = strings.ReplaceAll(s, "${", "$${")
	return strings.ReplaceAll(s, "%{", "%%{")
}

// Body is a block's arguments and nested blocks. Values may be string,
// Expr, bool, int, Body, []Body (repeated nested blocks), []string,
// []Expr, []any or map[string]string.
type Body map[string]any

// Block is a resource or data source.
type Block struct {
	Type, Name string
	Body       Body
}

// Addr is the block's address within its kind: "aws_vpc.main".
func (b Block) Addr() string { return b.Type + "." + b.Name }

// Provider is a required provider.
type Provider struct {
	// Name is the local name: "aws".
	Name string
	// Source is the registry address: "hashicorp/aws".
	Source string
	// Version is a version constraint: "~> 6.0" or "= 6.14.1".
	Version string
	// Config is the provider block, or nil for none.
	Config Body
}

// Output is an output value.
type Output struct {
	Name        string
	Description string
	Value       any
	Sensitive   bool
}

// Config is one root module.
type Config struct {
	// Comment is written as a top-level "//" property, which both tools
	// ignore, to say where the file came from.
	Comment   string
	Providers []Provider
	Data      []Block
	Resources []Block
	Outputs   []Output
}

// Resource adds a resource and returns its address.
func (c *Config) Resource(typ, name string, body Body) string {
	c.Resources = append(c.Resources, Block{Type: typ, Name: name, Body: body})
	return typ + "." + name
}

// DataSource adds a data source and returns its address ("data.x.y").
func (c *Config) DataSource(typ, name string, body Body) string {
	c.Data = append(c.Data, Block{Type: typ, Name: name, Body: body})
	return "data." + typ + "." + name
}

// Find returns the resource at addr ("aws_ecs_service.api"), or nil.
func (c *Config) Find(addr string) *Block {
	i := slices.IndexFunc(c.Resources, func(b Block) bool { return b.Addr() == addr })
	if i < 0 {
		return nil
	}
	return &c.Resources[i]
}

// Marshal returns c as indented JSON ending in a newline.
func Marshal(c *Config) ([]byte, error) {
	var top object
	if c.Comment != "" {
		top.add("//", c.Comment)
	}
	if len(c.Providers) > 0 {
		var req, conf object
		for _, p := range c.Providers {
			req.add(p.Name, Body{"source": p.Source, "version": p.Version})
			if p.Config != nil {
				conf.add(p.Name, p.Config)
			}
		}
		top.add("terraform", object{{"required_providers", req}})
		if len(conf) > 0 {
			top.add("provider", conf)
		}
	}
	if len(c.Data) > 0 {
		top.add("data", blocks(c.Data))
	}
	if len(c.Resources) > 0 {
		top.add("resource", blocks(c.Resources))
	}
	if len(c.Outputs) > 0 {
		var outs object
		for _, o := range c.Outputs {
			body := Body{"value": o.Value}
			if o.Description != "" {
				body["description"] = o.Description
			}
			if o.Sensitive {
				body["sensitive"] = true
			}
			outs.add(o.Name, body)
		}
		top.add("output", outs)
	}
	v, err := convert(top)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// blocks groups bs by type, keeping first-appearance order of types and
// insertion order within a type.
func blocks(bs []Block) object {
	var out object
	for _, b := range bs {
		i := slices.IndexFunc(out, func(kv field) bool { return kv.key == b.Type })
		if i < 0 {
			out = append(out, field{key: b.Type, value: object{}})
			i = len(out) - 1
		}
		out[i].value = append(out[i].value.(object), field{key: b.Name, value: b.Body})
	}
	return out
}

// object is a JSON object that keeps its key order.
type object []field

type field struct {
	key   string
	value any
}

func (o *object) add(key string, value any) { *o = append(*o, field{key, value}) }

func (o object) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, f := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := encode(&buf, f.key); err != nil {
			return nil, err
		}
		buf.WriteByte(':')
		if err := encode(&buf, f.value); err != nil {
			return nil, err
		}
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func encode(buf *bytes.Buffer, v any) error {
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return err
	}
	buf.Truncate(buf.Len() - 1) // Encode's newline
	return nil
}

// convert escapes literal strings and checks every value is one Body
// allows, so a mistake in a backend is an error rather than odd output.
func convert(v any) (any, error) {
	switch v := v.(type) {
	case nil, bool, int, int64, float64:
		return v, nil
	case string:
		return Escape(v), nil
	case Expr:
		return string(v), nil
	case object:
		out := make(object, len(v))
		for i, f := range v {
			c, err := convert(f.value)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", f.key, err)
			}
			out[i] = field{f.key, c}
		}
		return out, nil
	case Body:
		return convertMap(v)
	case map[string]any:
		return convertMap(v)
	case map[string]string:
		out := make(map[string]any, len(v))
		for k, s := range v {
			out[k] = Escape(s)
		}
		return out, nil
	case []Body:
		return convertSlice(v)
	case []any:
		return convertSlice(v)
	case []string:
		return convertSlice(v)
	case []Expr:
		return convertSlice(v)
	}
	return nil, fmt.Errorf("unsupported value of type %T", v)
}

func convertMap[M ~map[string]any](m M) (map[string]any, error) {
	out := make(map[string]any, len(m))
	for k, v := range m {
		c, err := convert(v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		out[k] = c
	}
	return out, nil
}

func convertSlice[T any](s []T) ([]any, error) {
	out := make([]any, len(s))
	for i, v := range s {
		c, err := convert(v)
		if err != nil {
			return nil, fmt.Errorf("[%d]: %w", i, err)
		}
		out[i] = c
	}
	return out, nil
}

// JSON encodes v as a JSON document inside a template, for arguments that
// take JSON text (IAM policies, ECS container definitions) but need
// references. Literal strings in v are escaped and Expr values interpolate,
// so `{"Resource": Ref("aws_s3_bucket.b.arn")}` becomes the bucket's ARN
// when the configuration is applied. References must evaluate to strings
// that need no JSON escaping, which ARNs, names and hostnames don't.
func JSON(v any) (Expr, error) {
	c, err := convert(v)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := encode(&buf, c); err != nil {
		return "", err
	}
	return Expr(buf.String()), nil
}
