package lsp

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/joshjkns/clowd/internal/diag"
)

// cursor marks the cursor in test sources; it never appears in Clowd.
const cursor = "|"

// openAt opens src, with the cursor marker removed, in a server that
// discards what it sends, and returns the document and the cursor's
// offset.
func openAt(t *testing.T, src string) (*server, *document, int) {
	t.Helper()
	offset := strings.Index(src, cursor)
	if offset < 0 {
		t.Fatalf("no cursor in %q", src)
	}
	s := &server{conn: newConn(nil, io.Discard), enc: utf8Encoding, docs: map[string]*document{}}
	if rerr := s.open("file:///p/app.clowd", 1, strings.Replace(src, cursor, "", 1)); rerr != nil {
		t.Fatal(rerr)
	}
	return s, s.docs["file:///p/app.clowd"], offset
}

func TestPositions(t *testing.T) {
	// é is 2 bytes and 1 UTF-16 unit; 𝄞 is 4 bytes and 2 UTF-16 units.
	f := diag.NewFile("t", "ab\r\né𝄞x\n\nlast")
	tests := []struct {
		offset int
		enc    encoding
		want   position
	}{
		{0, utf16Encoding, position{0, 0}},
		{2, utf16Encoding, position{0, 2}},
		{3, utf16Encoding, position{0, 2}}, // inside \r\n: end of line
		{4, utf16Encoding, position{1, 0}},
		{6, utf16Encoding, position{1, 1}},
		{10, utf16Encoding, position{1, 3}},
		{10, utf8Encoding, position{1, 6}},
		{11, utf16Encoding, position{1, 4}},
		{12, utf16Encoding, position{2, 0}},
		{17, utf16Encoding, position{3, 4}},
	}
	for _, tt := range tests {
		got := toPosition(f, tt.offset, tt.enc)
		if got != tt.want {
			t.Errorf("toPosition(%d, %s) = %v, want %v", tt.offset, tt.enc, got, tt.want)
		}
		if tt.offset != 3 { // \r is not a position an editor can ask for
			if back := toOffset(f, got, tt.enc); back != tt.offset {
				t.Errorf("toOffset(%v, %s) = %d, want %d", got, tt.enc, back, tt.offset)
			}
		}
	}
	// Past the end of a line or the file clamps.
	clamps := []struct {
		p    position
		want int
	}{
		{position{0, 99}, 2},
		{position{3, 99}, 17},
		{position{9, 0}, 17},
		{position{-1, 0}, 0},
	}
	for _, tt := range clamps {
		if got := toOffset(f, tt.p, utf16Encoding); got != tt.want {
			t.Errorf("toOffset(%v) = %d, want %d", tt.p, got, tt.want)
		}
	}
}

func TestDocsCoverVocabulary(t *testing.T) {
	for _, k := range kinds {
		if kindDocs[k] == "" {
			t.Errorf("no docs for kind %s", k)
		}
		v := vocabulary(k)
		for _, g := range v.Traits {
			for _, w := range g.Words {
				if traitDocs[k][w] == "" {
					t.Errorf("no docs for %s trait %q", k, w)
				}
			}
		}
		for _, p := range v.Props {
			if propDocs[k][p.Key] == "" {
				t.Errorf("no docs for %s property %q", k, p.Key)
			}
		}
	}
	for _, v := range verbs {
		if verbDocs[v] == "" {
			t.Errorf("no docs for verb %s", v)
		}
	}
	// And nothing documented that check doesn't accept.
	for k, props := range propDocs {
		for key := range props {
			found := false
			for _, p := range vocabulary(k).Props {
				found = found || p.Key == key
			}
			if !found {
				t.Errorf("docs for unknown %s property %q", k, key)
			}
		}
	}
}

func TestDiagnostics(t *testing.T) {
	s, doc, _ := openAt(t, "use aws\nuse gcp\nqueue jobs is fifo|\nbucket b is shiny\n")
	got := s.diagnostics(doc)
	if len(got) != 1 {
		t.Fatalf("got %d diagnostics, want 1 (errors hide capability findings): %+v", len(got), got)
	}
	want := diagnostic{
		Range:    lspRange{Start: position{3, 12}, End: position{3, 17}},
		Severity: severityError,
		Source:   "clowd",
		Message:  "unknown bucket trait `shiny`\nhelp: a bucket's traits are `private`, `public`, `versioned` and `protected`",
	}
	if got[0] != want {
		t.Errorf("diagnostic = %+v\nwant %+v", got[0], want)
	}

	s, doc, _ = openAt(t, "use aws\nuse gcp\nqueue jobs is fifo|\n")
	got = s.diagnostics(doc)
	if len(got) != 1 || got[0].Severity != severityInformation || !strings.HasPrefix(got[0].Message, "on gcp: Pub/Sub has no FIFO queues") {
		t.Errorf("capability finding = %+v", got)
	}
	if r := got[0].Range; r.Start != (position{2, 14}) || r.End != (position{2, 18}) {
		t.Errorf("capability finding range = %+v, want the fifo trait", r)
	}
}

func TestFormat(t *testing.T) {
	s, doc, _ := openAt(t, "USE aws|\nservice api\n    image a\n")
	edits := s.format(doc)
	if len(edits) != 1 || edits[0].NewText != "use aws\n\nservice api\n  image a\n" {
		t.Fatalf("edits = %+v", edits)
	}
	if r := edits[0].Range; r.Start != (position{}) || r.End != (position{3, 0}) {
		t.Errorf("edit range = %+v, want the whole file", r)
	}
	s, doc, _ = openAt(t, "use aws|\n")
	if edits := s.format(doc); len(edits) != 0 {
		t.Errorf("formatted file: edits = %+v, want none", edits)
	}
	s, doc, _ = openAt(t, "use aws|\nservice\n")
	if edits := s.format(doc); len(edits) != 0 {
		t.Errorf("broken file: edits = %+v, want none (D11)", edits)
	}
}

const hoverSrc = `use aws in eu-west-2
use gcp

database db is postgres 16, small
  storage 50gb
  on aws
    backup_retention_period 14
    blue_green_update
      enabled true

queue jobs is fifo

service api
  image ghcr.io/me/api:latest
  health check /healthz

load balancer web
  listen on 80
  send / to api

api connects to db
api sends to jobs

on aws
  network
    vpc vpc-1

bucket files
  on gcp
    storage_class NEARLINE

on gcp
  project shop-prod-1
`

func TestHover(t *testing.T) {
	tests := []struct {
		name string
		at   string // the text the cursor is put just inside of
		want string // "" for no hover
	}{
		{"use keyword", "use aws", "makes a cloud a deploy target"},
		{"cloud", "aws in", "**aws**: Amazon Web Services"},
		{"kind", "database db", "**database**\n\nA managed relational database. Its traits choose the engine and version, size, availability and protection.\n\nAWS: RDS · GCP: Cloud SQL"},
		{"resource name", "db is", "**database db**\n\nAWS: RDS · GCP: Cloud SQL"},
		{"versioned trait", "16,", "**database** trait `postgres 16`\n\nPostgreSQL."},
		{"trait", "small\n", "**database** trait `small`"},
		{"property", "storage", "**database** `storage`\n\nThe initial disk size"},
		{"phrase property", "health", "**service** `health check`"},
		{"emulated trait note", "jobs is", "**queue jobs**\n\nAWS: SQS · GCP: Pub/Sub\n\n- ⚠ **gcp**: Pub/Sub has no FIFO queues"},
		{"reference", "\napi connects", "**service api**\n\nAWS: ECS Fargate · GCP: Cloud Run\n\n- ⚠ **gcp**: Cloud Run pulls only from Artifact Registry and Docker Hub, so this image comes through an Artifact Registry proxy, which needs credentials if the image is private\n\nDeclared on line 13."},
		{"send target", " api\n\napi", "**service api**"},
		{"verb", "connects", "**connects to**\n\nGives the service a network path"},
		{"verb note", "sends to", "**sends to**"},
		{"on keyword", "on aws\n    b", "holds settings for one cloud"},
		{"override", "backup_retention_period", "`backup_retention_period`: an argument of `aws_db_instance`"},
		{"nested override", "enabled", "the `blue_green_update` block of `aws_db_instance`"},
		{"cloud setting", "network\n", "Uses a network that already exists"},
		{"nested cloud setting", "vpc vpc", "The ID of the existing VPC."},
		{"gcp override", "storage_class", "`storage_class`: an argument of `google_storage_bucket`, passed to the provider as written and applied only when deploying to GCP.\n\n[google_storage_bucket documentation](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/storage_bucket)"},
		{"gcp project", "project shop", "The Google Cloud project to deploy into"},
		{"value", "50gb", ""},
		{"blank line", "\n\nqueue", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := strings.Index(hoverSrc, tt.at)
			if i < 0 {
				t.Fatalf("%q not in source", tt.at)
			}
			src := hoverSrc[:i+1] + cursor + hoverSrc[i+1:]
			s, doc, offset := openAt(t, src)
			h := s.hover(doc, offset)
			switch {
			case tt.want == "" && h != nil:
				t.Errorf("hover = %q, want none", h.Contents.Value)
			case tt.want != "" && h == nil:
				t.Errorf("no hover, want %q", tt.want)
			case tt.want != "" && !strings.Contains(h.Contents.Value, tt.want):
				t.Errorf("hover = %q\nwant it to contain %q", h.Contents.Value, tt.want)
			}
		})
	}
}

func TestHoverWithoutTargets(t *testing.T) {
	s, doc, offset := openAt(t, "service a|pi\n  image a\n")
	h := s.hover(doc, offset)
	if h == nil || !strings.Contains(h.Contents.Value, "AWS: ECS Fargate · GCP: Cloud Run") {
		t.Errorf("hover = %+v, want every cloud's product", h)
	}
}

func TestComplete(t *testing.T) {
	decls := "use aws\ndatabase db is postgres\nbucket files\nqueue jobs\nservice api\n  image a\nservice worker\n  image b\n"
	tests := []struct {
		name string
		src  string
		want string // labels, space-separated
	}{
		{"line start", "|", "service database bucket load balancer queue project network use on"},
		{"line start offers services", decls + "w|", "service database bucket load balancer queue project network use on api worker"},
		{"use", "use |", "aws gcp azure"},
		{"use region", "use aws |", "in"},
		{"top-level on", "on |", "aws gcp azure"},
		{"is", "bucket b |", "is"},
		{"no is for a service", "service api |", ""},
		{"traits", "database db is |", "postgres mysql small medium large highly available protected"},
		{"traits after a comma", "database db is postgres 16, |", "small medium large highly available protected"},
		{"traits after and", "bucket b is public and |", "versioned protected"},
		{"no traits mid-trait", "database db is postgres |", ""},
		{"network traits", "network is |", "public private"},
		{"verbs", decls + "api |", "connects to reads writes reads and writes sends to"},
		{"rest of a verb", decls + "api connects |", "to"},
		{"rest of a verb, and its targets", decls + "api reads |", "files jobs and writes"},
		{"connects to targets", decls + "api connects to |", "db worker"},
		{"sends to targets", decls + "api sends to |", "jobs"},
		{"partial name", decls + "api connects to d|", "db worker"},
		{"no verbs for a data store", decls + "db |", ""},
		{"properties", "service api\n  |", "command cpu env health check image memory port scale on"},
		{"properties after a blank line", "queue q\n  retry 3 times\n\n  |", "keep messages for retry timeout on"},
		{"project properties", "project p\n  |", "edition state tag"},
		{"property on", "bucket b\n  on |", "aws gcp azure"},
		{"scale to", "service api\n  scale 2 |", "to"},
		{"listen", "lb web\n  listen |", "on"},
		{"listen tls", "lb web\n  listen on 443 |", "with tls for"},
		{"send to", decls + "lb web\n  send / |", "to"},
		{"send target", decls + "lb web\n  send / to |", "api worker"},
		{"overrides", "bucket b\n  on aws\n    |", "force_destroy object_lock_enabled"},
		{"nested overrides", "service api\n  on aws\n    capacity_provider_strategy\n      |", "base capacity_provider weight"},
		{"gcp overrides", "bucket b\n  on gcp\n    |", "cors default_event_based_hold enable_object_retention logging requester_pays rpo soft_delete_policy storage_class"},
		{"gcp nested overrides", "database d\n  on gcp\n    settings\n      maintenance_window\n        |", "day hour update_track"},
		{"azure overrides", "bucket b\n  on azure\n    |", ""},
		{"gcp cloud settings", "on gcp\n  |", "project"},
		{"cloud settings", "on aws\n  |", "network"},
		{"network settings", "on aws\n  network\n    |", "vpc public_subnets private_subnets"},
		{"in a comment", "# service |", ""},
		{"in a trailing comment", "service api  # port |", ""},
		{"after a # inside a word", "use aws#1 |", "in"},
		{"in a string", "service api\n  env A \"# x\"\n  |", "command cpu env health check image memory port scale on"},
		{"after a value", "service api\n  image a |", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, doc, offset := openAt(t, tt.src)
			var got []string
			for _, item := range s.complete(doc, offset).Items {
				got = append(got, item.Label)
			}
			if g := strings.Join(got, " "); g != tt.want {
				t.Errorf("labels = %q\nwant     %q", g, tt.want)
			}
		})
	}
}

func TestCompleteReplacesPartialWord(t *testing.T) {
	s, doc, offset := openAt(t, "bucket b is pub|")
	items := s.complete(doc, offset).Items
	if len(items) == 0 {
		t.Fatal("no items")
	}
	want := lspRange{Start: position{0, 12}, End: position{0, 15}}
	if r := items[0].TextEdit.Range; r != want {
		t.Errorf("replace range = %+v, want %+v", r, want)
	}
}

// TestSession drives the server over its stream, as an editor does.
func TestSession(t *testing.T) {
	c := startClient(t)
	var init initializeResult
	c.call("initialize", map[string]any{"capabilities": map[string]any{}}, &init)
	if init.Capabilities.PositionEncoding != "utf-16" || !init.Capabilities.HoverProvider || init.ServerInfo.Name != "clowd" {
		t.Errorf("initialize = %+v", init)
	}
	c.notify("initialized", map[string]any{})
	c.notify("textDocument/didOpen", didOpenParams{TextDocument: textDocumentItem{URI: "file:///app.clowd", Version: 1, Text: "use aws\nbucket b is shiny\n"}})
	pub := c.next("textDocument/publishDiagnostics")
	var p publishDiagnosticsParams
	_ = json.Unmarshal(pub.Params, &p)
	if len(p.Diagnostics) != 1 || *p.Version != 1 {
		t.Errorf("published %+v", p)
	}
	c.notify("textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": "file:///app.clowd", "version": 2},
		"contentChanges": []map[string]any{{"text": "use aws\nbucket b is public\n"}},
	})
	_ = json.Unmarshal(c.next("textDocument/publishDiagnostics").Params, &p)
	if len(p.Diagnostics) != 0 || *p.Version != 2 {
		t.Errorf("after the fix, published %+v", p)
	}
	var h *hover
	c.call("textDocument/hover", textDocumentPositionParams{TextDocument: textDocumentIdentifier{URI: "file:///app.clowd"}, Position: position{1, 1}}, &h)
	if h == nil || !strings.HasPrefix(h.Contents.Value, "**bucket**") {
		t.Errorf("hover = %+v", h)
	}
	c.call("textDocument/hover", textDocumentPositionParams{TextDocument: textDocumentIdentifier{URI: "file:///other.clowd"}, Position: position{0, 0}}, &h)
	if h != nil {
		t.Errorf("hover on an unopened file = %+v, want null", h)
	}
	if rerr := c.callErr("workspace/symbol", map[string]any{}); rerr == nil || rerr.Code != codeMethodNotFound {
		t.Errorf("unknown method: error = %+v", rerr)
	}
	c.notify("textDocument/didClose", didCloseParams{TextDocument: textDocumentIdentifier{URI: "file:///app.clowd"}})
	_ = json.Unmarshal(c.next("textDocument/publishDiagnostics").Params, &p)
	if len(p.Diagnostics) != 0 {
		t.Errorf("on close, published %+v", p)
	}
	c.call("shutdown", nil, nil)
	c.notify("exit", nil)
	if err := c.wait(); err != nil {
		t.Errorf("Serve = %v, want nil after shutdown and exit", err)
	}
}

func TestSessionErrors(t *testing.T) {
	c := startClient(t)
	if rerr := c.callErr("textDocument/hover", map[string]any{}); rerr == nil || rerr.Code != codeServerNotInitialized {
		t.Errorf("before initialize: error = %+v", rerr)
	}
	var init initializeResult
	c.call("initialize", map[string]any{"capabilities": map[string]any{"general": map[string]any{"positionEncodings": []string{"utf-16", "utf-8"}}}}, &init)
	if init.Capabilities.PositionEncoding != "utf-8" {
		t.Errorf("positionEncoding = %q, want utf-8 when offered", init.Capabilities.PositionEncoding)
	}
	c.notify("exit", nil)
	if err := c.wait(); err == nil {
		t.Error("Serve = nil, want an error for exit without shutdown")
	}
}

// client is the editor's end of a session.
type client struct {
	t    *testing.T
	conn *conn
	in   io.WriteCloser
	id   int
	// pending are notifications read while waiting for a response.
	pending []*message
	// last is the result of the latest call.
	last json.RawMessage
	done chan error
}

func startClient(t *testing.T) *client {
	t.Helper()
	toServer, clientOut := io.Pipe()
	clientIn, fromServer := io.Pipe()
	c := &client{t: t, conn: newConn(clientIn, clientOut), in: clientOut, done: make(chan error, 1)}
	go func() {
		err := Serve(toServer, fromServer, "test")
		fromServer.Close()
		c.done <- err
	}()
	t.Cleanup(func() { clientOut.Close() })
	return c
}

func (c *client) notify(method string, params any) {
	c.t.Helper()
	if err := c.conn.notify(method, params); err != nil {
		c.t.Fatal(err)
	}
}

// callErr sends a request and returns its error.
func (c *client) callErr(method string, params any) *rpcError {
	c.t.Helper()
	c.id++
	id := json.RawMessage(strings.TrimSpace(string(must(json.Marshal(c.id)))))
	b, _ := json.Marshal(params)
	if err := c.conn.write(&message{ID: id, Method: method, Params: b}); err != nil {
		c.t.Fatal(err)
	}
	for {
		m, err := c.conn.read()
		if err != nil {
			c.t.Fatalf("reading the %s response: %v", method, err)
		}
		if m.Method != "" {
			c.pending = append(c.pending, m)
			continue
		}
		if string(m.ID) != string(id) {
			c.t.Fatalf("response to %s has id %s", method, m.ID)
		}
		if m.Error != nil {
			return m.Error
		}
		c.last = m.Result
		return nil
	}
}

func (c *client) call(method string, params, result any) {
	c.t.Helper()
	if rerr := c.callErr(method, params); rerr != nil {
		c.t.Fatalf("%s: %v", method, rerr)
	}
	if result != nil {
		if err := json.Unmarshal(c.last, result); err != nil {
			c.t.Fatalf("%s result: %v", method, err)
		}
	}
}

// next returns the next notification with method.
func (c *client) next(method string) *message {
	c.t.Helper()
	for {
		var m *message
		if len(c.pending) > 0 {
			m, c.pending = c.pending[0], c.pending[1:]
		} else {
			var err error
			if m, err = c.conn.read(); err != nil {
				c.t.Fatalf("waiting for %s: %v", method, err)
			}
		}
		if m.Method == method {
			return m
		}
	}
}

func (c *client) wait() error {
	c.in.Close()
	return <-c.done
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
