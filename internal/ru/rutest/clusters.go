package rutest

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
)

// ClusterFuncs are the functions of internal/ru/clusters that a cluster
// golden file tests. The test passes them in, so that this package does not
// import the package under test.
type ClusterFuncs struct {
	// InsertCluster is clusters.InsertCluster.
	InsertCluster func(ctx *render.Context, elem *dom.Node, node *ru.Node) (*ru.ClusterResult, error)
	// Geometry is clusters.GetUsecaseSystemBoundaryGeometry, with the result
	// as a map from the upstream property name to its value: a string for
	// boundaryType, a float64 for the others.
	Geometry func(node *ru.Node, labelBBox dom.Rect) map[string]any
	// PaintsTitle is clusters.ClusterPaintsTitle.
	PaintsTitle func(shape string) bool
}

// GoldenBox is a box in a golden file. A number that is undefined or not
// finite is a string there, see GoldenNum.
type GoldenBox struct {
	X      any `json:"x"`
	Y      any `json:"y"`
	Width  any `json:"width"`
	Height any `json:"height"`
}

// GoldenNum converts a number of a golden file. The probes write a number
// that JSON cannot hold as a string: "undefined", "NaN", "Infinity" or
// "-Infinity". undefined and NaN both give NaN, as in the ru data model.
func GoldenNum(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		switch x {
		case "Infinity":
			return math.Inf(1)
		case "-Infinity":
			return math.Inf(-1)
		}
	}
	return math.NaN()
}

// ClusterCase is one case of tools/reference/probes/clusters.js with the
// result that upstream produced.
type ClusterCase struct {
	Name string          `json:"name"`
	Type string          `json:"type"`
	Node json.RawMessage `json:"node"`
	// Config and CSS are indexes into the lists of the golden file.
	Config             *int          `json:"config"`
	CSS                *int          `json:"css"`
	HTML               string        `json:"html"`
	Random             int           `json:"randomCalls"`
	ClusterIsLastChild bool          `json:"clusterIsLastChild"`
	LabelBBox          *GoldenBox    `json:"labelBBox"`
	After              *ClusterAfter `json:"after"`
	Intersect          []any         `json:"intersects"`
	Error              string        `json:"error"`
}

// ClusterAfter are the fields that a cluster shape wrote into its node.
type ClusterAfter struct {
	Width     any        `json:"width"`
	Height    any        `json:"height"`
	Diff      any        `json:"diff"`
	OffsetX   any        `json:"offsetX"`
	OffsetY   any        `json:"offsetY"`
	LabelBBox *GoldenBox `json:"labelBBox"`
}

// ClusterAllCase is a case of probes/clusters.js with several clusters
// that upstream inserted together:
//
//	await Promise.all(nodes.map(async (node) => { await insertCluster(g, node); ... }))
type ClusterAllCase struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Nodes  []json.RawMessage `json:"nodes"`
	Config *int              `json:"config"`
	CSS    *int              `json:"css"`
	HTML   string            `json:"html"`
	Random int               `json:"randomCalls"`
	// Order has the indexes of the nodes in the order in which their
	// calls finished.
	Order []int          `json:"order"`
	After []ClusterAfter `json:"after"`
	Error string         `json:"error"`
}

// ClusterGolden is the golden file of tools/reference/gen-clusters.mjs.
type ClusterGolden struct {
	// BaseConfig is an effective config of upstream. Configs are patches
	// of it, see PatchedConfig.
	BaseConfig json.RawMessage   `json:"baseConfig"`
	Configs    []json.RawMessage `json:"configs"`
	CSS        []string          `json:"css"`
	Cases      []ClusterCase     `json:"cases"`
	// All are the cases with several clusters inserted together.
	All      []ClusterAllCase `json:"all"`
	Geometry []struct {
		Node      json.RawMessage `json:"node"`
		LabelBBox struct {
			Width  float64 `json:"width"`
			Height float64 `json:"height"`
		} `json:"labelBBox"`
		Result map[string]any `json:"result"`
	} `json:"geometry"`
	PaintsTitle []struct {
		Shape  *string `json:"shape"`
		Result bool    `json:"result"`
	} `json:"paintsTitle"`
}

// LoadGolden reads a golden file (JSON, or gzip-compressed JSON when the
// name ends in .gz) into v. It skips the test when the file is missing.
func LoadGolden(t testing.TB, path string, v any) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("no golden file: %v", err)
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(path, ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		defer gz.Close()
		r = gz
	}
	if err := json.NewDecoder(r).Decode(v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// RandomAfter returns the value that ctx.Random gives after n earlier draws
// of a new context. A test compares it with the next draw of its context to
// check that the code under test drew n numbers.
func RandomAfter(n int) float64 {
	// The generator of render.Context.Random.
	rng := render.DefaultRandomSeed
	var v float64
	for i := 0; i <= n; i++ {
		rng += 0x6d2b79f5
		t := rng
		t = (t ^ (t >> 15)) * (t | 1)
		t ^= t + (t^(t>>7))*(t|61)
		v = float64(t^(t>>14)) / 4294967296
	}
	return v
}

// LayoutNodeFromJS is ru.NodeFromJS for a node of a layout: it also reads
// the properties that the cluster shapes and the dagre layout write (diff,
// offsetX, offsetY, clusterNode and labelBBox).
func LayoutNodeFromJS(o *js.Obj[any]) *ru.Node {
	n := ru.NodeFromJS(o)
	num := func(key string, dst *float64) {
		if v, ok := n.Extra[key]; ok {
			if !js.IsNullish(v) {
				*dst = js.ToNumber(v)
			}
			delete(n.Extra, key)
		}
	}
	num("diff", &n.Diff)
	num("offsetX", &n.OffsetX)
	num("offsetY", &n.OffsetY)
	if v, ok := n.Extra["clusterNode"]; ok {
		n.ClusterNode = js.Truthy(v)
		delete(n.Extra, "clusterNode")
	}
	if b := js.AsObj(n.Extra["labelBBox"]); b != nil {
		box := &ru.Bounds{X: ru.Undef, Y: ru.Undef, Width: ru.Undef, Height: ru.Undef}
		set := func(key string, dst *float64) {
			if v := b.Get(key); !js.IsNullish(v) {
				*dst = js.ToNumber(v)
			}
		}
		set("x", &box.X)
		set("y", &box.Y)
		set("width", &box.Width)
		set("height", &box.Height)
		n.LabelBBox = box
		delete(n.Extra, "labelBBox")
	}
	return n
}

// NumCheck compares numbers with upstream and keeps track of whether every
// one was bit-exact. Start with Exact set to true.
type NumCheck struct {
	Exact bool
}

// Num compares got with a number of a golden file. It returns a description
// of the difference when they differ by more than 0.01, or when one of them
// is undefined and the other is not. A smaller difference clears Exact.
func (c *NumCheck) Num(what string, got float64, want any) string {
	w := GoldenNum(want)
	if w != w || got != got {
		if (w != w) != (got != got) {
			return fmt.Sprintf("%s: got %v, want %v", what, got, want)
		}
		return ""
	}
	if got != w {
		c.Exact = false
		if math.Abs(got-w) > 0.01 || math.IsInf(got, 0) || math.IsInf(w, 0) {
			return fmt.Sprintf("%s: got %v, want %v", what, got, w)
		}
	}
	return ""
}

// deletedMark is the value of a property in a config patch that the
// base config has and the patched config does not.
const deletedMark = "__mmdg_deleted__"

// PatchedConfig returns an effective config of a golden file. The
// generators store one config in full and the others as patches of it: the
// properties that differ, recursively. An empty or null patch gives the
// base config.
func PatchedConfig(base, patch json.RawMessage) (*js.Obj[any], error) {
	baseVal, err := js.ParseJSON(base)
	if err != nil || js.AsObj(baseVal) == nil {
		return nil, fmt.Errorf("bad base config in golden: %v", err)
	}
	if len(patch) == 0 {
		return js.AsObj(baseVal), nil
	}
	patchVal, err := js.ParseJSON(patch)
	if err != nil {
		return nil, fmt.Errorf("bad config patch in golden: %v", err)
	}
	if js.AsObj(patchVal) == nil {
		return js.AsObj(baseVal), nil
	}
	return js.AsObj(applyConfigPatch(baseVal, patchVal)), nil
}

func applyConfigPatch(base, patch any) any {
	bo, po := js.AsObj(base), js.AsObj(patch)
	if bo == nil || po == nil {
		return patch
	}
	out := js.NewObj[any]()
	bo.Each(func(k string, v any) {
		pv, ok := po.Lookup(k)
		if !ok {
			out.Set(k, v)
			return
		}
		if s, isString := pv.(string); isString && s == deletedMark {
			return
		}
		out.Set(k, applyConfigPatch(v, pv))
	})
	po.Each(func(k string, v any) {
		if !bo.Has(k) {
			out.Set(k, v)
		}
	})
	return out
}

// NewProbeContext builds the page of the probes: the svg has a style
// element with the style sheet of upstream and nothing else. diagramType
// defaults to "flowchart-v2".
func NewProbeContext(config *js.Obj[any], css, diagramType string) *render.Context {
	ctx := render.NewContext(render.ContextOptions{Config: config})
	ctx.Type = diagramType
	if ctx.Type == "" {
		ctx.Type = "flowchart-v2"
	}
	// The probe removes the <g> that appendDivSvgG adds and builds its own
	// svg content.
	ctx.SVG.RemoveChildren()
	style := ctx.Doc.CreateElement("style")
	style.Text(css)
	ctx.SVG.AppendChild(style)
	return ctx
}

// CompareMarkup compares markup with the markup of upstream. The result is
// "exact", "equivalent" (the same tree, numbers within 0.01) or "different"
// with the first difference.
func CompareMarkup(got, want string) (status, detail string) {
	if got == want {
		return "exact", ""
	}
	verdict, diff := parity.CompareSVG("<svg>"+got+"</svg>", "<svg>"+want+"</svg>", 0.01)
	if verdict == parity.Different {
		return "different", "markup: " + diff.String()
	}
	return "equivalent", ""
}

// RunClusterCase replays one case: it builds the same page, calls
// InsertCluster with the same node and config, and compares the markup, the
// returned label box, the fields that the shape wrote into the node, the
// number of random draws and the intersections with upstream.
func RunClusterCase(g *ClusterGolden, c ClusterCase, funcs ClusterFuncs) (res ShapeResult) {
	res.Name = c.Name
	defer func() {
		if p := recover(); p != nil {
			res.Status, res.Detail = "error", fmt.Sprintf("panic: %v", p)
		}
	}()
	nodeVal, err := js.ParseJSON(c.Node)
	if err != nil {
		return ShapeResult{c.Name, "error", fmt.Sprintf("bad node in golden: %v", err)}
	}
	if c.Config == nil || c.CSS == nil {
		if c.Error == "" {
			return ShapeResult{c.Name, "error", "the golden case has no config"}
		}
		// Upstream failed before it made the config: nothing to replay.
		return ShapeResult{c.Name, "exact", ""}
	}
	cfg, err := PatchedConfig(g.BaseConfig, g.Configs[*c.Config])
	if err != nil {
		return ShapeResult{c.Name, "error", err.Error()}
	}
	ctx := NewProbeContext(cfg, g.CSS[*c.CSS], c.Type)
	group := ctx.SVG.Append("g").Attr("class", "clusters")

	node := LayoutNodeFromJS(js.AsObj(nodeVal))
	result, err := funcs.InsertCluster(ctx, group, node)
	if c.Error != "" {
		if err != nil {
			return ShapeResult{c.Name, "exact", ""}
		}
		return ShapeResult{c.Name, "different", "upstream fails: " + c.Error}
	}
	if err != nil {
		if _, ok := err.(*ru.NotPortedError); ok {
			return ShapeResult{c.Name, "not-ported", err.Error()}
		}
		return ShapeResult{c.Name, "error", err.Error()}
	}

	check := &NumCheck{Exact: true}
	status, detail := CompareMarkup(group.InnerHTML(), c.HTML)
	if status == "different" {
		return ShapeResult{c.Name, status, detail}
	}
	if status != "exact" {
		check.Exact = false
	}
	if next, want := ctx.Random(), RandomAfter(c.Random); next != want {
		return ShapeResult{c.Name, "different", fmt.Sprintf("the number of random draws is not %d", c.Random)}
	}
	if isLast := result.Cluster != nil && result.Cluster == lastElementChild(group); isLast != c.ClusterIsLastChild {
		return ShapeResult{c.Name, "different", fmt.Sprintf("cluster is the last child: got %v, want %v", isLast, c.ClusterIsLastChild)}
	}

	// The returned labelBBox. The divider returns an empty object.
	if c.LabelBBox != nil {
		wantHas := c.LabelBBox.Width != "undefined"
		if result.HasLabelBBox != wantHas {
			return ShapeResult{c.Name, "different", fmt.Sprintf("labelBBox present: got %v, want %v", result.HasLabelBBox, wantHas)}
		}
		if wantHas {
			if d := check.Num("labelBBox.width", result.LabelBBox.Width, c.LabelBBox.Width); d != "" {
				return ShapeResult{c.Name, "different", d}
			}
			if d := check.Num("labelBBox.height", result.LabelBBox.Height, c.LabelBBox.Height); d != "" {
				return ShapeResult{c.Name, "different", d}
			}
		}
	}

	if c.After != nil {
		if d := compareClusterAfter(check, "node", node, c.After); d != "" {
			return ShapeResult{c.Name, "different", d}
		}
	}

	for i, want := range c.Intersect {
		pair, ok := want.([]any)
		if !ok || len(pair) != 2 {
			continue
		}
		if node.Intersect == nil {
			return ShapeResult{c.Name, "different", "node.intersect is not set"}
		}
		p := node.Intersect(ru.Point{X: shapeRays[i][0], Y: shapeRays[i][1]})
		if d := check.Num(fmt.Sprintf("intersect ray %d x", i), p.X, pair[0]); d != "" {
			return ShapeResult{c.Name, "different", d}
		}
		if d := check.Num(fmt.Sprintf("intersect ray %d y", i), p.Y, pair[1]); d != "" {
			return ShapeResult{c.Name, "different", d}
		}
	}
	if check.Exact {
		return ShapeResult{c.Name, "exact", ""}
	}
	return ShapeResult{c.Name, "equivalent", ""}
}

// compareClusterAfter compares the fields that a cluster shape wrote into
// a node with upstream.
func compareClusterAfter(check *NumCheck, what string, node *ru.Node, want *ClusterAfter) string {
	for _, f := range []struct {
		what string
		got  float64
		want any
	}{
		{what + ".width", node.Width, want.Width},
		{what + ".height", node.Height, want.Height},
		{what + ".diff", node.Diff, want.Diff},
		{what + ".offsetX", node.OffsetX, want.OffsetX},
		{what + ".offsetY", node.OffsetY, want.OffsetY},
	} {
		if d := check.Num(f.what, f.got, f.want); d != "" {
			return d
		}
	}
	if (node.LabelBBox != nil) != (want.LabelBBox != nil) {
		return fmt.Sprintf("%s.labelBBox present: got %v, want %v", what, node.LabelBBox != nil, want.LabelBBox != nil)
	}
	if node.LabelBBox != nil {
		if d := check.Num(what+".labelBBox.width", node.LabelBBox.Width, want.LabelBBox.Width); d != "" {
			return d
		}
		if d := check.Num(what+".labelBBox.height", node.LabelBBox.Height, want.LabelBBox.Height); d != "" {
			return d
		}
	}
	return ""
}

// RunClusterAllCase replays a case with several clusters: it inserts them
// together with ctx.All, which stands for the Promise.all of upstream, and
// compares the markup, the order in which the calls finished, the number of
// random draws and the fields that the shapes wrote into the nodes.
func RunClusterAllCase(g *ClusterGolden, c ClusterAllCase, funcs ClusterFuncs) (res ShapeResult) {
	res.Name = c.Name
	defer func() {
		if p := recover(); p != nil {
			res.Status, res.Detail = "error", fmt.Sprintf("panic: %v", p)
		}
	}()
	if c.Config == nil || c.CSS == nil {
		return ShapeResult{c.Name, "error", "the golden case has no config"}
	}
	cfg, err := PatchedConfig(g.BaseConfig, g.Configs[*c.Config])
	if err != nil {
		return ShapeResult{c.Name, "error", err.Error()}
	}
	ctx := NewProbeContext(cfg, g.CSS[*c.CSS], c.Type)
	group := ctx.SVG.Append("g").Attr("class", "clusters")
	nodes := make([]*ru.Node, len(c.Nodes))
	for i, raw := range c.Nodes {
		nodeVal, err := js.ParseJSON(raw)
		if err != nil {
			return ShapeResult{c.Name, "error", fmt.Sprintf("bad node in golden: %v", err)}
		}
		nodes[i] = LayoutNodeFromJS(js.AsObj(nodeVal))
	}
	order := []int{}
	err = ctx.All(len(nodes), func(i int) error {
		if _, err := funcs.InsertCluster(ctx, group, nodes[i]); err != nil {
			return err
		}
		order = append(order, i)
		return nil
	})
	if c.Error != "" {
		if err != nil {
			return ShapeResult{c.Name, "exact", ""}
		}
		return ShapeResult{c.Name, "different", "upstream fails: " + c.Error}
	}
	if err != nil {
		if _, ok := err.(*ru.NotPortedError); ok {
			return ShapeResult{c.Name, "not-ported", err.Error()}
		}
		return ShapeResult{c.Name, "error", err.Error()}
	}
	check := &NumCheck{Exact: true}
	if fmt.Sprint(order) != fmt.Sprint(c.Order) {
		return ShapeResult{c.Name, "different", fmt.Sprintf("order in which the calls finished: got %v, want %v", order, c.Order)}
	}
	status, detail := CompareMarkup(group.InnerHTML(), c.HTML)
	if status == "different" {
		return ShapeResult{c.Name, status, detail}
	}
	if status != "exact" {
		check.Exact = false
	}
	if next, want := ctx.Random(), RandomAfter(c.Random); next != want {
		return ShapeResult{c.Name, "different", fmt.Sprintf("the number of random draws is not %d", c.Random)}
	}
	for i := range c.After {
		if d := compareClusterAfter(check, fmt.Sprintf("node %d", i), nodes[i], &c.After[i]); d != "" {
			return ShapeResult{c.Name, "different", d}
		}
	}
	if check.Exact {
		return ShapeResult{c.Name, "exact", ""}
	}
	return ShapeResult{c.Name, "equivalent", ""}
}

func lastElementChild(n *dom.Node) *dom.Node {
	for c := n.LastChild; c != nil; c = c.Prev {
		if c.Type == dom.ElementNode {
			return c
		}
	}
	return nil
}

// RunClusterGolden replays every case of a cluster golden file and fails
// the test for cases that differ. It logs a summary. Cases of shapes that
// are not ported yet are counted and do not fail the test.
//
// On a machine without the system fonts of the macOS reference machine, a
// case of insertCluster with text fails the test only for an error, and the
// test is marked as skipped after all the other cases. See SkipComparison.
func RunClusterGolden(t *testing.T, path string, funcs ClusterFuncs) {
	t.Helper()
	var g ClusterGolden
	LoadGolden(t, path, &g)
	skipped := 0
	counts := map[string]int{}
	for _, c := range g.Cases {
		r := RunClusterCase(&g, c, funcs)
		counts[r.Status]++
		skip := SkipComparison(c.HTML)
		if skip {
			skipped++
		}
		if r.Status == "error" || r.Status == "different" && !skip {
			t.Errorf("%s: %s\n    %s", r.Name, r.Status, r.Detail)
		}
	}
	t.Logf("%s: insertCluster: %d cases: exact=%d equivalent=%d different=%d error=%d not-ported=%d",
		path, len(g.Cases), counts["exact"], counts["equivalent"], counts["different"], counts["error"], counts["not-ported"])

	counts = map[string]int{}
	for _, c := range g.All {
		r := RunClusterAllCase(&g, c, funcs)
		counts[r.Status]++
		skip := SkipComparison(c.HTML)
		if skip {
			skipped++
		}
		if r.Status == "error" || r.Status == "different" && !skip {
			t.Errorf("%s: %s\n    %s", r.Name, r.Status, r.Detail)
		}
	}
	t.Logf("%s: insertCluster with Promise.all: %d cases: exact=%d equivalent=%d different=%d error=%d not-ported=%d",
		path, len(g.All), counts["exact"], counts["equivalent"], counts["different"], counts["error"], counts["not-ported"])

	// getUsecaseSystemBoundaryGeometry: every number must be bit-exact.
	bad := 0
	for i, c := range g.Geometry {
		nodeVal, err := js.ParseJSON(c.Node)
		if err != nil {
			t.Fatalf("geometry %d: bad node in golden: %v", i, err)
		}
		got := funcs.Geometry(ru.NodeFromJS(js.AsObj(nodeVal)), dom.Rect{Width: c.LabelBBox.Width, Height: c.LabelBBox.Height})
		for key, want := range c.Result {
			if key == "boundaryType" {
				if got[key] != want {
					bad++
					t.Errorf("geometry %d: %s: got %v, want %v", i, key, got[key], want)
				}
				continue
			}
			gv, _ := got[key].(float64)
			wv := GoldenNum(want)
			if math.Float64bits(gv) != math.Float64bits(wv) && !(gv != gv && wv != wv) {
				bad++
				t.Errorf("geometry %d: %s: got %v, want %v", i, key, gv, want)
			}
		}
	}
	t.Logf("%s: getUsecaseSystemBoundaryGeometry: %d cases, %d differences", path, len(g.Geometry), bad)

	bad = 0
	for _, c := range g.PaintsTitle {
		shape := ""
		if c.Shape != nil {
			shape = *c.Shape
		}
		if got := funcs.PaintsTitle(shape); got != c.Result {
			bad++
			t.Errorf("clusterPaintsTitle(%q): got %v, want %v", shape, got, c.Result)
		}
	}
	t.Logf("%s: clusterPaintsTitle: %d cases, %d differences", path, len(g.PaintsTitle), bad)
	RequireFonts(t, skipped)
}
