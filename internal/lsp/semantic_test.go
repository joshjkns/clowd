package lsp

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"unicode/utf16"
)

// decode turns semantic tokens back into "word=type" per line, with "*"
// marking a declaration, so tests read like the source they colour.
func decode(text string, data []uint32, enc encoding) string {
	lines := strings.Split(text, "\n")
	var out []string
	line, char := 0, 0
	cur := -1
	for i := 0; i+4 < len(data); i += 5 {
		if data[i] > 0 {
			line += int(data[i])
			char = 0
		}
		char += int(data[i+1])
		word := slice(lines[line], char, int(data[i+2]), enc)
		tok := fmt.Sprintf("%s=%s", word, tokenTypes[data[i+3]])
		if data[i+4]&modDeclaration != 0 {
			tok += "*"
		}
		if line != cur {
			out = append(out, fmt.Sprintf("%d:", line+1))
			cur = line
		}
		out[len(out)-1] += " " + tok
	}
	return strings.Join(out, "\n")
}

// slice returns n units of s from unit start, counting in enc.
func slice(s string, start, n int, enc encoding) string {
	if enc == utf8Encoding {
		return s[start : start+n]
	}
	u := utf16.Encode([]rune(s))
	return string(utf16.Decode(u[start : start+n]))
}

func tokensFor(t *testing.T, src string, enc encoding) string {
	t.Helper()
	s := &server{conn: newConn(nil, io.Discard), enc: enc, docs: map[string]*document{}}
	if rerr := s.open("file:///p/app.clowd", 1, src); rerr != nil {
		t.Fatal(rerr)
	}
	return decode(src, s.semanticTokens(s.docs["file:///p/app.clowd"]).Data, enc)
}

func TestSemanticTokens(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{
			name: "use",
			src:  "use aws in eu-west-2\nuse gcp\n",
			want: "1: use=keyword aws=enumMember in=keyword eu-west-2=string\n2: use=keyword gcp=enumMember",
		},
		{
			name: "declaration with traits",
			src:  "database db is postgres 16, small and protected\n",
			want: "1: database=struct db=variable* is=keyword postgres=type 16=number small=type and=keyword protected=type",
		},
		{
			name: "multi-word kind and network without a name",
			src:  "load balancer web is public\nnetwork is private\n",
			want: "1: load balancer=struct web=variable* is=keyword public=type\n2: network=struct is=keyword private=type",
		},
		{
			name: "properties and value keywords",
			src: "service api\n  image ghcr.io/me/api:latest\n  scale 2 to 10\n  memory 512mb\n" +
				"  env GREETING \"hello there\"\n  health check /healthz\n",
			want: "1: service=struct api=variable*\n" +
				"2: image=property ghcr.io/me/api:latest=string\n" +
				"3: scale=property 2=number to=keyword 10=number\n" +
				"4: memory=property 512mb=number\n" +
				"5: env=property GREETING=string \"hello there\"=string\n" +
				"6: health=property check=property /healthz=string",
		},
		{
			name: "load balancer properties",
			src:  "lb web\n  listen on 443 with tls for api.example.com\n  send / to api\n",
			want: "1: lb=struct web=variable*\n" +
				"2: listen=property on=keyword 443=number with=keyword tls=keyword for=keyword api.example.com=string\n" +
				"3: send=property /=string to=keyword api=string",
		},
		{
			name: "phrase property",
			src:  "queue jobs\n  keep messages for 4d\n",
			want: "1: queue=struct jobs=variable*\n2: keep=property messages=property for=property 4d=number",
		},
		{
			name: "on block inside a resource",
			src:  "service api\n  image x\n  on aws\n    capacity_provider_strategy\n      capacity_provider FARGATE_SPOT\n      weight 1\n",
			want: "1: service=struct api=variable*\n2: image=property x=string\n3: on=keyword aws=enumMember\n" +
				"4: capacity_provider_strategy=decorator\n5: capacity_provider=decorator FARGATE_SPOT=string\n6: weight=decorator 1=number",
		},
		{
			name: "top-level on block",
			src:  "on aws\n  network\n    public_subnets subnet-a, subnet-b\n",
			want: "1: on=keyword aws=enumMember\n2: network=decorator\n3: public_subnets=decorator subnet-a=string subnet-b=string",
		},
		{
			// Verbs are checked before kinds, as in the parser.
			name: "relationships",
			src:  "api connects to db\nqueue reads and writes uploads\n",
			want: "1: api=variable connects to=operator db=variable\n2: queue=variable reads and writes=operator uploads=variable",
		},
		{
			name: "comments",
			src:  "# header\nbucket b # trailing\n",
			want: "1: # header=comment\n2: bucket=struct b=variable* # trailing=comment",
		},
		{
			// The parser recovers, so a file with errors is still coloured.
			name: "errors",
			src:  "service api\n  image x\napi links to db\nbucket\n",
			want: "1: service=struct api=variable*\n2: image=property x=string\n3: api=variable links to=operator db=variable\n4: bucket=struct",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokensFor(t, tt.src, utf8Encoding); got != tt.want {
				t.Errorf("tokens:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

// Lengths and starts count in the client's encoding.
func TestSemanticTokensUTF16(t *testing.T) {
	src := "service api\n  env NOTE \"café 𝄞\" # ok\n"
	want := "1: service=struct api=variable*\n2: env=property NOTE=string \"café 𝄞\"=string # ok=comment"
	for _, enc := range []encoding{utf8Encoding, utf16Encoding} {
		if got := tokensFor(t, src, enc); got != want {
			t.Errorf("%s:\n%s\nwant:\n%s", enc, got, want)
		}
	}
}
