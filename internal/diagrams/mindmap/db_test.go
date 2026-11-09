package mindmap

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
	"sort"
	"strconv"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// The golden file is the layout data that the mindmap renderer of real
// mermaid hands to the layout: tools/reference/gen-mindmap-db.mjs writes it
// with the browser oracle. The test renders the same texts up to the layout
// and compares everything: the nodes and the edges, the node tree, the
// config, and the other properties of the layout data. For a text that
// mermaid rejects it compares the error.

type goldenFile struct {
	BaseConfig map[string]any `json:"baseConfig"`
	Cases      []goldenCase   `json:"cases"`
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
	// Empty is true for a mindmap without a node: mermaid draws nothing and
	// calls no layout.
	Empty       bool  `json:"empty"`
	Nodes       []any `json:"nodes"`
	Edges       []any `json:"edges"`
	ConfigDelta *struct {
		Set [][]any    `json:"set"`
		Del [][]string `json:"del"`
	} `json:"configDelta"`
	RootNode    any    `json:"rootNode"`
	Direction   string `json:"direction"`
	DataType    string `json:"dataType"`
	Layout      string `json:"layout"`
	Markers     []any  `json:"markers"`
	NodeSpacing any    `json:"nodeSpacing"`
	RankSpacing any    `json:"rankSpacing"`
	Shapes      any    `json:"shapes"`
	DiagramID   string `json:"diagramId"`
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
	var me *Error
	if errors.As(err, &me) {
		if me.Name == "" {
			return "Error", me.Message
		}
		return me.Name, me.Message
	}
	return "Error", err.Error()
}

// ---- the layout that keeps the layout data ----

// errCaptured is the error of the capturing layout: it stops the render, as
// the layout of the page script does.
var errCaptured = errors.New("the layout data is captured")

var (
	captureMu sync.Mutex
	captured  *ru.LayoutData
)

// The test binary links no layout package, so the layouts of the test are
// the only ones: each keeps the layout data and stops the render.
func init() {
	keep := func(_ *render.Context, data *ru.LayoutData, _ *dom.Node, _ string) error {
		captured = data
		return errCaptured
	}
	for name, algorithm := range map[string]string{
		"dagre": "", "swimlane": "", "cose-bilkent": "", "elk": "elk.layered",
		"elk.stress": "elk.stress", "elk.force": "elk.force", "elk.mrtree": "elk.mrtree",
		"elk.sporeOverlap": "elk.sporeOverlap", "elk.box": "elk.box", "elk.rectpacking": "elk.rectpacking",
	} {
		ru.RegisterLayout(name, algorithm, keep)
	}
}

// renderToLayout renders text up to the call of the layout. data is nil
// when the draw function calls no layout.
func renderToLayout(text string, site map[string]any) (data *ru.LayoutData, err error) {
	captureMu.Lock()
	defer captureMu.Unlock()
	captured = nil
	// The page of the oracle calls mermaid.initialize({ startOnLoad: false, ...site }).
	config := js.NewObj[any]()
	config.Set("startOnLoad", false)
	if siteObj := toObj(site); siteObj != nil {
		siteObj.Each(func(k string, v any) { config.Set(k, v) })
	}
	_, err = render.Render(text, render.Options{Config: config})
	if errors.Is(err, errCaptured) {
		return captured, nil
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

// treeJSON is JSON.stringify of a MindmapNode.
func treeJSON(n *Node) map[string]any {
	children := make([]any, len(n.Children))
	for i, c := range n.Children {
		children[i] = treeJSON(c)
	}
	out := map[string]any{
		"id":       float64(n.ID),
		"nodeId":   n.NodeID,
		"level":    float64(n.Level),
		"descr":    n.Descr,
		"type":     float64(n.Type),
		"children": children,
		"isRoot":   n.IsRoot,
	}
	if n.Width != nil {
		out["width"] = generic(n.Width)
	}
	if n.Padding != nil {
		out["padding"] = generic(n.Padding)
	}
	if !ru.IsUndef(n.Section) {
		out["section"] = n.Section
	}
	if n.HasIcon {
		out["icon"] = n.Icon
	}
	if n.HasClass {
		out["class"] = n.Class
	}
	return out
}

// treeNodes lists the nodes of a tree by id.
func treeNodes(n *Node, out map[string]*Node) {
	out[strconv.Itoa(n.ID)] = n
	for _, c := range n.Children {
		treeNodes(c, out)
	}
}

// nodeJSON is JSON.stringify of a layout node of the mindmap. tree is the
// node of the tree with the same id: it tells an icon that is the empty
// string from one that is undefined.
func nodeJSON(n *ru.Node, tree *Node) map[string]any {
	out := map[string]any{
		"id":          n.ID,
		"domId":       n.DomID,
		"label":       n.Label,
		"labelType":   n.LabelType,
		"isGroup":     n.IsGroup,
		"shape":       n.Shape,
		"cssClasses":  n.CSSClasses,
		"cssStyles":   generic(n.CSSStyles),
		"centerLabel": n.CenterLabel,
		"nodeId":      n.NodeID,
	}
	num := func(key string, v float64) {
		if !ru.IsUndef(v) {
			out[key] = v
		}
	}
	num("width", n.Width)
	num("height", n.Height)
	num("padding", n.Padding)
	num("x", n.X)
	num("y", n.Y)
	num("level", n.Level)
	num("type", n.Type)
	num("radius", n.Radius)
	num("taper", n.Taper)
	if n.Look != "" {
		out["look"] = n.Look
	}
	if n.Stroke != "" {
		out["stroke"] = n.Stroke
	}
	if tree != nil && tree.HasIcon {
		out["icon"] = n.Icon
	}
	for k, v := range n.Extra {
		out[k] = generic(v)
	}
	return out
}

// edgeJSON is JSON.stringify of a layout edge of the mindmap.
func edgeJSON(e *ru.Edge) map[string]any {
	out := map[string]any{
		"id":        e.ID,
		"start":     e.Start,
		"end":       e.End,
		"type":      e.Type,
		"curve":     e.Curve,
		"thickness": e.Thickness,
		"classes":   e.Classes,
	}
	if e.Look != "" {
		out["look"] = e.Look
	}
	if !ru.IsUndef(e.Depth) {
		out["depth"] = e.Depth
	}
	for k, v := range e.Extra {
		out[k] = generic(v)
	}
	return out
}

// ---- comparison ----

// diff returns a description of the first difference between two JSON
// values, or "". The order of the elements of a list counts.
func diff(path string, got, want any) string {
	if reflect.DeepEqual(got, want) {
		return ""
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			break
		}
		keys := map[string]bool{}
		for k := range w {
			keys[k] = true
		}
		for k := range g {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			gv, gok := g[k]
			wv, wok := w[k]
			switch {
			case !gok:
				return fmt.Sprintf("%s.%s: missing, want %s", path, k, show(wv))
			case !wok:
				return fmt.Sprintf("%s.%s: got %s, want it absent", path, k, show(gv))
			}
			if d := diff(path+"."+k, gv, wv); d != "" {
				return d
			}
		}
	case []any:
		g, ok := got.([]any)
		if !ok {
			break
		}
		for i := 0; i < len(g) && i < len(w); i++ {
			if d := diff(path+"["+strconv.Itoa(i)+"]", g[i], w[i]); d != "" {
				return d
			}
		}
		if len(g) != len(w) {
			return fmt.Sprintf("%s: length %d, want %d", path, len(g), len(w))
		}
	}
	return fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want))
}

func show(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	if len(b) > 300 {
		return string(b[:300]) + "..."
	}
	return string(b)
}

// flatten is the flatten function of the generator: the leaves of a config
// by path. An object with no property is a leaf.
func flatten(v any, prefix []string, out map[string]any) {
	if obj, ok := v.(map[string]any); ok && len(obj) > 0 {
		if _, isNum := obj["$num"]; !isNum {
			for k, e := range obj {
				flatten(e, append(append([]string{}, prefix...), k), out)
			}
			return
		}
	}
	key, _ := json.Marshal(prefix)
	out[string(key)] = v
}

// wantConfig returns the leaves of the config of a case: the base config
// with the delta of the case.
func wantConfig(base map[string]any, c *goldenCase) map[string]any {
	out := make(map[string]any, len(base))
	for k, v := range base {
		out[k] = v
	}
	for _, path := range c.ConfigDelta.Del {
		key, _ := json.Marshal(path)
		delete(out, string(key))
	}
	for _, entry := range c.ConfigDelta.Set {
		key, _ := json.Marshal(entry[0])
		out[string(key)] = entry[1]
	}
	return out
}

// compareCase compares the layout data of this port with the dump of
// upstream. It returns the first difference, or "".
func compareCase(base map[string]any, c *goldenCase, data *ru.LayoutData) string {
	root, _ := data.Extra["rootNode"].(*Node)
	if root == nil {
		return "the layout data has no rootNode"
	}
	if s := diff("rootNode", treeJSON(root), c.RootNode); s != "" {
		return s
	}
	byID := map[string]*Node{}
	treeNodes(root, byID)

	nodes := make([]any, len(data.Nodes))
	for i, n := range data.Nodes {
		nodes[i] = nodeJSON(n, byID[n.ID])
	}
	if s := diff("nodes", nodes, c.Nodes); s != "" {
		return s
	}
	edges := make([]any, len(data.Edges))
	for i, e := range data.Edges {
		edges[i] = edgeJSON(e)
	}
	if s := diff("edges", edges, c.Edges); s != "" {
		return s
	}

	if s := diff("shapes", generic(data.Extra["shapes"]), c.Shapes); s != "" {
		return s
	}
	if s := diff("markers", generic(data.Markers), c.Markers); s != "" {
		return s
	}
	if s := diff("nodeSpacing", generic(data.NodeSpacing), c.NodeSpacing); s != "" {
		return s
	}
	if s := diff("rankSpacing", generic(data.RankSpacing), c.RankSpacing); s != "" {
		return s
	}
	for _, f := range []struct{ name, got, want string }{
		{"direction", data.Direction, c.Direction},
		{"type", data.Type, c.DataType},
		{"layoutAlgorithm", data.LayoutAlgorithm, c.Layout},
		{"diagramId", data.DiagramID, c.DiagramID},
	} {
		if f.got != f.want {
			return fmt.Sprintf("%s: got %q, want %q", f.name, f.got, f.want)
		}
	}

	if c.ConfigDelta == nil {
		return "the case has no config"
	}
	got := map[string]any{}
	flatten(generic(data.Config), nil, got)
	want := wantConfig(base, c)
	return diff("config", got, want)
}

// TestDBAgainstMermaid compares the parser, the db and the first part of
// the renderer with real mermaid.
func TestDBAgainstMermaid(t *testing.T) {
	g := loadGolden(t)
	base := map[string]any{}
	flatten(g.BaseConfig, nil, base)

	var total, same, errorCases, emptyCases, corpus int
	for i := range g.Cases {
		c := &g.Cases[i]
		total++
		if c.Text == nil {
			corpus++
		}
		text := caseText(t, c)
		data, err := renderToLayout(text, c.Site)
		fail := func(format string, args ...any) {
			t.Errorf("%s: %s\n--- text ---\n%s", c.Name, fmt.Sprintf(format, args...), text)
		}
		switch {
		case c.Error != nil:
			errorCases++
			if err == nil {
				fail("no error, mermaid has %q", *c.Error)
				continue
			}
			name, message := errorMessage(err)
			wantName := c.ErrorName
			if wantName == "" {
				wantName = "Error"
			}
			if message != *c.Error || name != wantName {
				fail("error %s %q, mermaid has %s %q", name, message, wantName, *c.Error)
				continue
			}
		case err != nil:
			fail("error %v, mermaid has none", err)
			continue
		case c.Empty:
			emptyCases++
			if data != nil {
				fail("the draw function called a layout, and mermaid draws nothing")
				continue
			}
		case data == nil:
			fail("the draw function called no layout")
			continue
		default:
			if s := compareCase(base, c, data); s != "" {
				fail("%s", s)
				continue
			}
		}
		same++
	}
	t.Logf("%d cases (%d corpus fixtures, %d errors, %d without a node): %d the same as mermaid", total, corpus, errorCases, emptyCases, same)
	if total == 0 {
		t.Fatal("no cases")
	}
}
