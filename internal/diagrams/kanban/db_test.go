package kanban

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/purify"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file is what the kanban db of real mermaid holds after it
// parsed a text inside a real render, and what getData() hands to the
// renderer: tools/reference/gen-kanban-db.mjs writes it with the browser
// oracle. The test renders the same texts up to the draw function and
// compares everything, order included. For a text that mermaid rejects it
// compares the error.

type goldenFile struct {
	Cases []goldenCase `json:"cases"`
}

type goldenCase struct {
	Name string `json:"name"`
	// Text is absent for a corpus fixture: Name is then its path.
	Text *string `json:"text"`
	// Site is the site config: the argument of mermaid.initialize.
	Site      map[string]any `json:"site"`
	Error     *string        `json:"error"`
	ErrorName string         `json:"errorName"`
	Type      string         `json:"type"`
	Nodes     []any          `json:"nodes"`
	Sections  []int          `json:"sections"`
	Cnt       int            `json:"cnt"`
	Data      *struct {
		Nodes []any    `json:"nodes"`
		Edges []any    `json:"edges"`
		Other any      `json:"other"`
		Keys  []string `json:"keys"`
	} `json:"data"`
	Config map[string]any `json:"config"`
}

func loadGolden(tb testing.TB) *goldenFile {
	tb.Helper()
	f, err := os.Open(filepath.Join("testdata", "db.json.gz"))
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		tb.Fatal(err)
	}
	var g goldenFile
	if err := json.Unmarshal(data, &g); err != nil {
		tb.Fatal(err)
	}
	return &g
}

func caseText(tb testing.TB, c *goldenCase) string {
	tb.Helper()
	if c.Text != nil {
		return *c.Text
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "..", filepath.FromSlash(c.Name)))
	if err != nil {
		tb.Fatal(err)
	}
	return string(data)
}

// errorMessage is the name and the message of the JavaScript exception.
func errorMessage(err error) (name, message string) {
	var ce *config.Error
	if errors.As(err, &ce) {
		return ce.Name, ce.Message
	}
	var je *jison.Error
	if errors.As(err, &je) {
		return je.Name, je.Message
	}
	var ke *Error
	if errors.As(err, &ke) {
		if ke.Name == "" {
			return "Error", ke.Message
		}
		return ke.Name, ke.Message
	}
	var te *purify.TypeError
	if errors.As(err, &te) {
		return "TypeError", te.Message
	}
	return "Error", err.Error()
}

// ---- the draw function that keeps the db ----

// errCaptured is the error of the capturing draw function: it stops the
// render, as the draw function of the page script does.
var errCaptured = errors.New("the db is captured")

// capture is what the capturing draw function keeps.
type capture struct {
	db   *DB
	data *Data
}

var (
	captureOnce sync.Once
	captureMu   sync.Mutex
	// capturing is not nil while renderToDraw runs.
	capturing *capture
)

// installCapture replaces the draw function of the kanban definition with
// one that keeps the db and the result of getData when a capture is
// active, and that draws otherwise.
func installCapture() {
	def, ok := render.Lookup("kanban")
	if !ok {
		panic("the kanban diagram is not registered")
	}
	wrapped := *def
	realDraw := def.Draw
	wrapped.Draw = func(ctx *render.Context, db render.DB, text string) error {
		if capturing == nil {
			return realDraw(ctx, db, text)
		}
		d := db.(*DB)
		data, err := d.GetData()
		if err != nil {
			return err
		}
		capturing.db, capturing.data = d, data
		return errCaptured
	}
	render.Register(&wrapped)
}

// renderToDraw renders text up to the draw function. The result is nil
// when the text is not a kanban diagram: typ is then the type.
func renderToDraw(text string, site map[string]any) (got *capture, err error) {
	captureOnce.Do(installCapture)
	captureMu.Lock()
	defer captureMu.Unlock()
	capturing = &capture{}
	defer func() { capturing = nil }()
	// The page of the oracle calls mermaid.initialize({ startOnLoad: false, ...site }).
	cfg := js.NewObj[any]()
	cfg.Set("startOnLoad", false)
	if siteObj := toObj(site); siteObj != nil {
		siteObj.Each(func(k string, v any) { cfg.Set(k, v) })
	}
	_, err = render.Render(text, render.Options{Config: cfg})
	if errors.Is(err, errCaptured) {
		return capturing, nil
	}
	return nil, err
}

func toObj(m map[string]any) *js.Obj[any] {
	if m == nil {
		return nil
	}
	data, _ := json.Marshal(m)
	v, err := js.ParseJSON(data)
	if err != nil {
		panic(err)
	}
	return js.AsObj(v)
}

// ---- conversion of the Go side to JSON data ----

// generic converts a dynamic JavaScript value to the value that
// encoding/json gives for JSON.stringify of it: an undefined property is
// absent, undefined in an array is null, and a number that is not finite is
// { $num: "NaN" }, as the oracle writes it.
func generic(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case bool, string:
		return x
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return map[string]any{"$num": js.Str(x)}
		}
		return x
	case int:
		return float64(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = generic(e)
		}
		return out
	case []string:
		out := make([]any, len(x))
		for i, s := range x {
			out[i] = s
		}
		return out
	case *js.Obj[any]:
		if x == nil {
			return nil
		}
		out := map[string]any{}
		x.Each(func(k string, e any) {
			if e != nil {
				out[k] = generic(e)
			}
		})
		return out
	}
	if js.IsNullish(v) {
		return nil
	}
	panic(fmt.Sprintf("generic: unexpected %T", v))
}

// dbNodeJSON is JSON.stringify of a node of the db.
func dbNodeJSON(n *Node) map[string]any {
	out := map[string]any{
		"id":      n.ID,
		"level":   float64(n.Level),
		"label":   generic(n.Label),
		"isGroup": n.IsGroup,
	}
	if n.Width != nil {
		out["width"] = generic(n.Width)
	}
	if n.Padding != nil {
		out["padding"] = generic(n.Padding)
	}
	if n.HasShape {
		out["shape"] = n.Shape
	}
	if n.HasIcon {
		out["icon"] = n.Icon
	}
	if n.HasAssigned {
		out["assigned"] = n.Assigned
	}
	if n.HasTicket {
		out["ticket"] = n.Ticket
	}
	if n.Priority != nil {
		out["priority"] = generic(n.Priority)
	}
	if n.HasParentID {
		out["parentId"] = n.ParentID
	}
	if n.HasClasses {
		out["cssClasses"] = n.CSSClasses
	}
	return out
}

// dataNodeJSON is JSON.stringify of a node of getData. item is the node of
// the db that the node comes from: it tells an icon that is the empty
// string from one that is undefined.
func dataNodeJSON(n *ru.Node, item *Node) map[string]any {
	out := map[string]any{
		"id":        n.ID,
		"label":     n.Label,
		"labelType": n.LabelType,
		"isGroup":   n.IsGroup,
		"shape":     n.Shape,
		"level":     n.Level,
	}
	if n.IsGroup {
		if n.Look != "" {
			out["look"] = n.Look
		}
	} else {
		out["parentId"] = n.ParentID
		out["rx"] = n.Rx
		out["ry"] = n.Ry
		out["cssStyles"] = generic(n.CSSStyles)
		if item.HasIcon {
			out["icon"] = n.Icon
		}
	}
	for k, v := range n.Extra {
		out[k] = generic(v)
	}
	return out
}

// dataNodes is the JSON of the nodes of getData. It pairs each node with
// its node of the db in the order of getData.
func dataNodes(db *DB, data *Data) ([]any, error) {
	var out []any
	i := 0
	next := func(item *Node) error {
		if i >= len(data.Nodes) {
			return fmt.Errorf("getData has %d nodes, too few", len(data.Nodes))
		}
		out = append(out, dataNodeJSON(data.Nodes[i], item))
		i++
		return nil
	}
	for _, section := range db.sections {
		if err := next(section); err != nil {
			return nil, err
		}
		for _, item := range db.nodes {
			if item.HasParentID && item.ParentID == section.ID {
				if err := next(item); err != nil {
					return nil, err
				}
			}
		}
	}
	if i != len(data.Nodes) {
		return nil, fmt.Errorf("getData has %d nodes, too many", len(data.Nodes))
	}
	return out, nil
}

// yamlLoaderRejects reports whether the text has shape data that js-yaml
// reads and that the YAML loader of this port (config.LoadYAML) rejects.
// The loader documents that the two parsers can disagree on what is a
// syntax error. The known input is a flow mapping with an empty key:
// js-yaml reads "{ : }" as an object with the key "null".
func yamlLoaderRejects(text string) bool {
	return strings.Contains(text, "@{ : }")
}

func show(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func TestDBAgainstMermaid(t *testing.T) {
	g := loadGolden(t)
	if len(g.Cases) < 300 {
		t.Fatalf("only %d cases", len(g.Cases))
	}
	passed, rejected, kanban, yamlDisagreements := 0, 0, 0, 0
	for i := range g.Cases {
		c := &g.Cases[i]
		text := caseText(t, c)
		fail := func(format string, args ...any) {
			t.Errorf("%s: %s\n--- text ---\n%s", c.Name, fmt.Sprintf(format, args...), text)
		}
		got, err := renderToDraw(text, c.Site)
		if c.Error != nil {
			rejected++
			if err == nil {
				fail("no error, mermaid fails with %s: %q", c.ErrorName, *c.Error)
				continue
			}
			name, message := errorMessage(err)
			if name == "YAMLException" && c.ErrorName == "YAMLException" {
				// The YAML parser of this port has its own messages.
				passed++
				continue
			}
			if name != c.ErrorName || message != *c.Error {
				fail("error %s: %q\n     want %s: %q", name, message, c.ErrorName, *c.Error)
				continue
			}
			passed++
			continue
		}
		if c.Type != "kanban" {
			// The test binary has no other diagram type.
			var unsupported *render.UnsupportedError
			if !errors.As(err, &unsupported) || unsupported.Type != c.Type {
				fail("got %v, mermaid renders the type %s", err, c.Type)
				continue
			}
			passed++
			continue
		}
		kanban++
		if err != nil {
			if name, _ := errorMessage(err); name == "YAMLException" && yamlLoaderRejects(text) {
				yamlDisagreements++
				continue
			}
			fail("error %v, mermaid has no error", err)
			continue
		}
		ok := true
		check := func(what string, got, want any) {
			if !reflect.DeepEqual(got, want) {
				ok = false
				fail("%s:\n got  %s\n want %s", what, show(got), show(want))
			}
		}
		nodes := []any{}
		index := map[*Node]int{}
		for k, n := range got.db.nodes {
			nodes = append(nodes, dbNodeJSON(n))
			index[n] = k
		}
		check("nodes", nodes, c.Nodes)
		sections := []int{}
		for _, s := range got.db.GetSections() {
			sections = append(sections, index[s])
		}
		check("sections", sections, c.Sections)
		check("cnt", got.db.cnt, c.Cnt)
		data, err := dataNodes(got.db, got.data)
		if err != nil {
			fail("%v", err)
			continue
		}
		if data == nil {
			data = []any{}
		}
		check("getData().nodes", data, c.Data.Nodes)
		check("getData() keys", []string{"nodes", "edges", "other", "config"}, c.Data.Keys)
		conf := map[string]any{}
		for _, key := range []string{"look", "theme", "htmlLabels", "securityLevel", "kanban", "mindmap"} {
			if v := got.data.Config.Get(key); v != nil {
				conf[key] = generic(v)
			}
		}
		check("config", conf, c.Config)
		if ok {
			passed++
		}
	}
	t.Logf("%d cases: %d pass, %d kanban diagrams, %d that mermaid rejects, %d that only the YAML loader of this port rejects",
		len(g.Cases), passed, kanban, rejected, yamlDisagreements)
}
