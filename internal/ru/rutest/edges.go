package rutest

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/fonttest"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	"github.com/bvolpato/mermaid-go-renderer/internal/sanitize"
)

// This file replays the cases of tools/reference/probes/edges.js: the edge
// functions of the unified renderer, recorded from upstream mermaid in
// Chromium by tools/reference/gen-edges.mjs.

// EdgeGolden is a golden file of gen-edges.mjs. The effective configs and
// the style sheets are stored once; a case holds their indexes.
type EdgeGolden struct {
	Configs []json.RawMessage `json:"configs"`
	Styles  []string          `json:"styles"`
	Cases   []json.RawMessage `json:"cases"`
}

// LoadEdgeGolden reads a golden file (JSON, or gzip-compressed JSON when
// the name ends in .gz). It skips the test when the file does not exist.
func LoadEdgeGolden(t testing.TB, path string) *EdgeGolden {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("no edge golden file: %v", err)
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
	var g EdgeGolden
	if err := json.NewDecoder(r).Decode(&g); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return &g
}

// EdgeResult is the outcome of one case.
type EdgeResult struct {
	Name   string
	Status string // "exact", "equivalent", "different", "error"
	Detail string
}

// edgeNum reads a number of a case. The strings "#NaN", "#Infinity" and
// "#-Infinity" stand for those values, and "#undefined" and a missing
// value are undefined (NaN).
func edgeNum(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		switch x {
		case "#Infinity":
			return math.Inf(1)
		case "#-Infinity":
			return math.Inf(-1)
		}
	}
	return math.NaN()
}

func edgeStr(v any) string {
	if s, ok := v.(string); ok && s != "#undefined" {
		return s
	}
	return ""
}

func edgeIsUndefined(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == "#undefined"
}

// edgePoints reads points given as [[x, y], ...] or [{x, y}, ...].
func edgePoints(v any) []ru.Point {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]ru.Point, 0, len(arr))
	for _, e := range arr {
		switch p := e.(type) {
		case []any:
			if len(p) == 2 {
				out = append(out, ru.Point{X: edgeNum(p[0]), Y: edgeNum(p[1])})
			}
		case *js.Obj[any]:
			out = append(out, ru.Point{X: edgeNum(p.Get("x")), Y: edgeNum(p.Get("y"))})
		}
	}
	return out
}

func edgeStrings(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, len(arr))
	for i, e := range arr {
		out[i] = js.ToString(e)
	}
	return out
}

// EdgeFromJS builds an Edge from a JavaScript object with the property
// names of types.ts and of edges.js. Tests use it to feed the same edge to
// the Go code and to upstream. Properties that have no field go to Extra.
func EdgeFromJS(o *js.Obj[any]) *ru.Edge {
	e := ru.NewEdge("")
	// A missing style is undefined.
	e.StyleUndefined = true
	if o == nil {
		return e
	}
	o.Each(func(k string, v any) {
		if edgeIsUndefined(v) {
			return
		}
		switch k {
		case "id":
			e.ID = edgeStr(v)
		case "label":
			e.Label, e.HasLabel = edgeStr(v), true
		case "classes":
			e.Classes = edgeStr(v)
		case "style":
			e.StyleUndefined = false
			if s, ok := v.(string); ok {
				// A string is the one element of the array.
				e.Style = []string{s}
			} else {
				e.Style = edgeStrings(v)
			}
		case "animate":
			e.Animate, e.HasAnimate = js.Truthy(v), true
		case "animation":
			e.Animation = edgeStr(v)
		case "metadata":
			e.Metadata = js.AsObj(v)
		case "arrowhead":
			e.Arrowhead = edgeStr(v)
		case "arrowheadStyle":
			e.ArrowheadStyle = edgeStr(v)
		case "arrowTypeEnd":
			e.ArrowTypeEnd = edgeStr(v)
		case "arrowTypeStart":
			e.ArrowTypeStart = edgeStr(v)
		case "cssCompiledStyles":
			e.CSSCompiledStyles = edgeStrings(v)
		case "cssStyles":
			e.CSSStyles = edgeStrings(v)
		case "defaultInterpolate":
			e.DefaultInterpolate = edgeStr(v)
		case "end":
			e.End = edgeStr(v)
		case "interpolate":
			e.Interpolate = edgeStr(v)
		case "labelType":
			e.LabelType = edgeStr(v)
		case "length":
			e.Length = edgeNum(v)
		case "start":
			e.Start = edgeStr(v)
		case "stroke":
			e.Stroke = edgeStr(v)
		case "text":
			e.Text = edgeStr(v)
		case "type":
			e.Type = edgeStr(v)
		case "startLabelLeft":
			e.StartLabelLeft = edgeStr(v)
		case "startLabelRight":
			e.StartLabelRight = edgeStr(v)
		case "endLabelLeft":
			e.EndLabelLeft = edgeStr(v)
		case "endLabelRight":
			e.EndLabelRight = edgeStr(v)
		case "terminalLabelSizes":
			if sizes := js.AsObj(v); sizes != nil {
				e.TerminalLabelSizes = map[string]ru.TerminalLabelSize{}
				sizes.Each(func(key string, size any) {
					if s := js.AsObj(size); s != nil {
						e.TerminalLabelSizes[key] = ru.TerminalLabelSize{Width: edgeNum(s.Get("width")), Height: edgeNum(s.Get("height"))}
					}
				})
			}
		case "terminalLabelCenters":
			if centers := js.AsObj(v); centers != nil {
				e.TerminalLabelCenters = map[string]ru.Point{}
				centers.Each(func(key string, c any) {
					if p := js.AsObj(c); p != nil {
						e.TerminalLabelCenters[key] = ru.Point{X: edgeNum(p.Get("x")), Y: edgeNum(p.Get("y"))}
					}
				})
			}
		case "curve":
			// Only a string counts as a curve.
			e.Curve = edgeStr(v)
		case "labelpos":
			e.Labelpos = edgeStr(v)
		case "labelStyle":
			if s, ok := v.(string); ok {
				e.LabelStyleText, e.LabelStyleIsText = s, true
			} else {
				e.LabelStyle = edgeStrings(v)
			}
		case "minlen":
			e.Minlen = edgeNum(v)
		case "pattern":
			e.Pattern = edgeStr(v)
		case "thickness":
			e.Thickness = edgeStr(v)
		case "look":
			e.Look = edgeStr(v)
		case "isUserDefinedId":
			e.IsUserDefinedID = js.Truthy(v)
		case "showPoints":
			e.ShowPoints = js.Truthy(v)
		case "width":
			e.Width = edgeNum(v)
		case "height":
			e.Height = edgeNum(v)
		case "x":
			e.X = edgeNum(v)
		case "y":
			e.Y = edgeNum(v)
		case "points":
			e.Points = edgePoints(v)
		case "parentId":
			e.ParentID = edgeStr(v)
		case "dir":
			e.Dir = edgeStr(v)
		case "source":
			e.Source = edgeStr(v)
		case "target":
			e.Target = edgeStr(v)
		case "depth":
			e.Depth = edgeNum(v)
		case "isLabelEdge":
			e.IsLabelEdge = js.Truthy(v)
		case "labelNodeId":
			e.LabelNodeID = edgeStr(v)
		case "isLayoutOnly":
			e.IsLayoutOnly = js.Truthy(v)
		case "toCluster":
			e.ToCluster = edgeStr(v)
		case "fromCluster":
			e.FromCluster = edgeStr(v)
		default:
			e.Set(k, v)
		}
	})
	return e
}

// edgeCompare collects the differences of one case. diffs are the
// differences that make the case different. notes say why a case is only
// equivalent.
type edgeCompare struct {
	exact bool
	diffs []string
	notes []string
}

// inexact records a difference within the tolerance.
func (c *edgeCompare) inexact(format string, args ...any) {
	c.exact = false
	if len(c.notes) < 3 {
		c.notes = append(c.notes, fmt.Sprintf(format, args...))
	}
}

func (c *edgeCompare) fail(format string, args ...any) {
	c.diffs = append(c.diffs, fmt.Sprintf(format, args...))
}

// num compares a number with the recorded value. A difference in the last
// digits makes the case equivalent and not exact.
func (c *edgeCompare) num(what string, got float64, want any) {
	if edgeIsUndefined(want) || want == "#NaN" {
		if got == got {
			c.fail("%s: got %v, want %v", what, got, want)
		}
		return
	}
	w := edgeNum(want)
	if math.Float64bits(got) == math.Float64bits(w) || (got == 0 && w == 0) {
		return
	}
	c.inexact("%s: got %v, want %v", what, js.Str(got), js.Str(w))
	if !(math.Abs(got-w) <= 0.01) {
		c.fail("%s: got %v, want %v", what, js.Str(got), js.Str(w))
	}
}

func (c *edgeCompare) points(what string, got []ru.Point, want any) {
	arr, ok := want.([]any)
	if !ok {
		c.fail("%s: want %v", what, want)
		return
	}
	if len(arr) != len(got) {
		c.fail("%s: got %d points, want %d", what, len(got), len(arr))
		return
	}
	for i, w := range arr {
		p, _ := w.([]any)
		if len(p) != 2 {
			c.fail("%s[%d]: bad golden point", what, i)
			return
		}
		c.num(fmt.Sprintf("%s[%d].x", what, i), got[i].X, p[0])
		c.num(fmt.Sprintf("%s[%d].y", what, i), got[i].Y, p[1])
	}
}

func (c *edgeCompare) text(what, got, want string) {
	if got != want {
		c.fail("%s: got %q, want %q", what, got, want)
	}
}

func (c *edgeCompare) result(name string) EdgeResult {
	if len(c.diffs) > 0 {
		return EdgeResult{name, "different", strings.Join(c.diffs, "\n    ")}
	}
	if c.exact {
		return EdgeResult{name, "exact", ""}
	}
	return EdgeResult{name, "equivalent", strings.Join(c.notes, "\n    ")}
}

func edgeGeoms(v any) []edges.EdgeGeom {
	arr, _ := v.([]any)
	out := make([]edges.EdgeGeom, 0, len(arr))
	for _, e := range arr {
		o := js.AsObj(e)
		if o == nil {
			continue
		}
		out = append(out, edges.EdgeGeom{
			ID:             edgeStr(o.Get("id")),
			Points:         edgePoints(o.Get("points")),
			Curve:          edgeStr(o.Get("curve")),
			ArrowTypeStart: edgeStr(o.Get("arrowTypeStart")),
			ArrowTypeEnd:   edgeStr(o.Get("arrowTypeEnd")),
		})
	}
	return out
}

func edgeJumpConfig(v any) edges.LineJumpConfig {
	o := js.AsObj(v)
	if o == nil {
		return edges.LineJumpConfig{}
	}
	return edges.LineJumpConfig{
		Enabled:    js.Truthy(o.Get("enabled")),
		JumpRadius: edgeNum(o.Get("jumpRadius")),
		JumpStyle:  edgeStr(o.Get("jumpStyle")),
	}
}

func edgePaths(v any) *edges.Paths {
	o := js.AsObj(v)
	if o == nil {
		return nil
	}
	paths := &edges.Paths{}
	if up := o.Get("updatedPath"); up != nil {
		paths.UpdatedPath, paths.HasUpdatedPath = edgePoints(up), true
	}
	if op := o.Get("originalPath"); op != nil {
		paths.OriginalPath, paths.HasOriginalPath = edgePoints(op), true
	}
	return paths
}

// runPureEdgeCase replays a case of one pure function.
func runPureEdgeCase(name string, c *js.Obj[any]) EdgeResult {
	pure := js.AsObj(c.Get("pure"))
	fn := edgeStr(pure.Get("fn"))
	args, _ := pure.Get("args").([]any)
	arg := func(i int) any {
		if i < len(args) {
			return args[i]
		}
		return nil
	}
	want := c.Get("result")
	wantErr := edgeStr(c.Get("error"))
	cmp := &edgeCompare{exact: true}
	switch fn {
	case "computeLabelTransform":
		o := js.AsObj(arg(0))
		bbox := dom.Rect{Width: edgeNum(o.Get("width")), Height: edgeNum(o.Get("height"))}
		if !edgeIsUndefined(o.Get("x")) {
			bbox.X = edgeNum(o.Get("x"))
		}
		if !edgeIsUndefined(o.Get("y")) {
			bbox.Y = edgeNum(o.Get("y"))
		}
		cmp.text("result", edges.ComputeLabelTransform(bbox, js.Truthy(arg(1))), edgeStr(want))
	case "getLabelStyles":
		var got string
		switch a := arg(0).(type) {
		case []any:
			got = edges.GetLabelStyles(edgeStrings(a))
		case string:
			if a != "#undefined" {
				// For a string, upstream returns the string.
				got = a
			} else {
				got = edges.GetLabelStyles(nil)
			}
		}
		cmp.text("result", got, edgeStr(want))
	case "hasEdgeLabel":
		got := edges.HasEdgeLabel(EdgeFromJS(js.AsObj(arg(0))))
		if got != js.Truthy(want) {
			cmp.fail("result: got %v, want %v", got, want)
		}
	case "intersection":
		o := js.AsObj(arg(0))
		node := ru.Bounds{X: edgeNum(o.Get("x")), Y: edgeNum(o.Get("y")), Width: edgeNum(o.Get("width")), Height: edgeNum(o.Get("height"))}
		pt := func(v any) ru.Point {
			p := js.AsObj(v)
			return ru.Point{X: edgeNum(p.Get("x")), Y: edgeNum(p.Get("y"))}
		}
		got := edges.Intersection(node, pt(arg(1)), pt(arg(2)))
		cmp.points("result", []ru.Point{got}, []any{want})
	case "generateRoundedPath":
		cmp.text("result", edges.GenerateRoundedPath(edgePoints(arg(0)), edgeNum(arg(1))), edgeStr(want))
	case "applyMarkerOffsetsToPoints":
		o := js.AsObj(arg(1))
		got := edges.ApplyMarkerOffsetsToPoints(edgePoints(arg(0)), edgeStr(o.Get("arrowTypeStart")), edgeStr(o.Get("arrowTypeEnd")))
		cmp.points("result", got, want)
	case "resolveEdgeLabelPosition":
		got, err := edges.ResolveEdgeLabelPosition(EdgeFromJS(js.AsObj(arg(0))), edgePaths(arg(1)))
		if wantErr != "" {
			if err == nil {
				cmp.fail("upstream fails: %s", wantErr)
			}
			break
		}
		if err != nil {
			cmp.fail("error: %v", err)
			break
		}
		cmp.points("result", []ru.Point{got}, []any{want})
	case "findEdgeIntersections":
		got := edges.FindEdgeIntersections(edgeGeoms(arg(0)))
		arr, _ := want.([]any)
		if len(arr) != len(got) {
			cmp.fail("result: got %d crossings, want %d", len(got), len(arr))
			break
		}
		for i, w := range arr {
			o := js.AsObj(w)
			cmp.text(fmt.Sprintf("crossing %d jumpEdgeId", i), got[i].JumpEdgeID, edgeStr(o.Get("jumpEdgeId")))
			cmp.text(fmt.Sprintf("crossing %d otherEdgeId", i), got[i].OtherEdgeID, edgeStr(o.Get("otherEdgeId")))
			cmp.num(fmt.Sprintf("crossing %d segIndex", i), float64(got[i].SegIndex), o.Get("segIndex"))
			cmp.num(fmt.Sprintf("crossing %d t", i), got[i].T, o.Get("t"))
			cmp.points(fmt.Sprintf("crossing %d point", i), []ru.Point{got[i].Point}, []any{o.Get("point")})
		}
	case "processEdgesWithJumps":
		got := edges.ProcessEdgesWithJumps(edgeGeoms(arg(0)), edgeJumpConfig(arg(1)))
		arr, _ := want.([]any)
		keys := got.Keys()
		if len(arr) != len(keys) {
			cmp.fail("result: got %d paths, want %d", len(keys), len(arr))
			break
		}
		for i, w := range arr {
			pair, _ := w.([]any)
			if len(pair) != 2 {
				cmp.fail("bad golden entry %d", i)
				break
			}
			cmp.text(fmt.Sprintf("entry %d id", i), keys[i], edgeStr(pair[0]))
			cmp.text(fmt.Sprintf("entry %d path", i), got.Get(keys[i]), edgeStr(pair[1]))
		}
	case "isStraightPath":
		if got := edges.IsStraightPath(edgeStr(arg(0))); got != js.Truthy(want) {
			cmp.fail("result: got %v, want %v", got, want)
		}
	case "curveSupportsLineHops":
		if got := edges.CurveSupportsLineHops(edgeStr(arg(0))); got != js.Truthy(want) {
			cmp.fail("result: got %v, want %v", got, want)
		}
	default:
		return EdgeResult{name, "error", "unknown pure function " + fn}
	}
	return cmp.result(name)
}

// edgeNode builds the node of an edge op: nil for a missing spec, a node
// whose intersect returns a fixed point, or a rectangle.
func edgeNode(spec any) *ru.Node {
	o := js.AsObj(spec)
	if o == nil {
		return nil
	}
	node := ru.NewNode("")
	if fixed, ok := o.Get("fixed").([]any); ok && len(fixed) == 2 {
		p := ru.Point{X: edgeNum(fixed[0]), Y: edgeNum(fixed[1])}
		node.Intersect = func(ru.Point) ru.Point { return p }
		return node
	}
	rect := js.AsObj(o.Get("rect"))
	if rect == nil {
		return nil
	}
	bounds := ru.Bounds{X: edgeNum(rect.Get("x")), Y: edgeNum(rect.Get("y")), Width: edgeNum(rect.Get("width")), Height: edgeNum(rect.Get("height"))}
	node.X, node.Y, node.Width, node.Height = bounds.X, bounds.Y, bounds.Width, bounds.Height
	node.Intersect = func(point ru.Point) ru.Point { return ru.IntersectRect(bounds, point) }
	return node
}

// firstDifference shows the text around the first byte where two strings
// differ.
func firstDifference(got, want string) string {
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	clip := func(s string) string {
		start := max(i-40, 0)
		end := min(i+60, len(s))
		return s[start:end]
	}
	return fmt.Sprintf("at byte %d: got ...%s..., want ...%s...", i, clip(got), clip(want))
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// RunEdgeCase replays one case of a golden file: a pure function call, or
// a scenario that builds the same page, runs the same calls of
// InsertMarkers, InsertEdgeLabel, InsertEdge, PositionEdgeLabel and
// ApplyLineJumpsToSvg, and compares the markup and the returned values
// with upstream.
func RunEdgeCase(g *EdgeGolden, raw json.RawMessage) (res EdgeResult) {
	val, err := js.ParseJSON(raw)
	c := js.AsObj(val)
	if err != nil || c == nil {
		return EdgeResult{"?", "error", fmt.Sprintf("bad case in golden: %v", err)}
	}
	name := edgeStr(c.Get("name"))
	res.Name = name
	defer func() {
		if p := recover(); p != nil {
			res = EdgeResult{name, "error", fmt.Sprintf("panic: %v", p)}
		}
	}()
	if c.Get("pure") != nil {
		return runPureEdgeCase(name, c)
	}
	// A case that failed as a whole in the oracle is one whose markers
	// upstream does not know: insertMarkers throws.
	caseErr := edgeStr(c.Get("error"))
	ci, ok1 := c.Get("config").(float64)
	si, ok2 := c.Get("css").(float64)
	if !ok1 || !ok2 || int(ci) >= len(g.Configs) || int(si) >= len(g.Styles) {
		return EdgeResult{name, "error", "bad config or css index in golden"}
	}
	cfgVal, err := js.ParseJSON(g.Configs[int(ci)])
	cfg := js.AsObj(cfgVal)
	if err != nil || cfg == nil {
		return EdgeResult{name, "error", fmt.Sprintf("bad config in golden: %v", err)}
	}
	typ := edgeStr(c.Get("type"))
	if typ == "" {
		typ = "flowchart-v2"
	}
	id := edgeStr(c.Get("id"))
	if id == "" {
		id = "my-svg"
	}
	ops, _ := c.Get("ops").([]any)
	// randomAt of an edge op is the number of Math.random draws before the
	// call in the real render. The probe puts the random stream there
	// before the first such op, and the following edges go on from it.
	randomSkip := 0
	for _, o := range ops {
		if at, ok := js.AsObj(o).Get("randomAt").(float64); ok {
			randomSkip = int(at)
			break
		}
	}
	ctx := render.NewContext(render.ContextOptions{Config: cfg, ID: id, RandomSkip: randomSkip})
	ctx.Type = typ
	if loc := js.AsObj(c.Get("location")); loc != nil {
		edges.SetPageLocation(ctx, sanitize.Location{
			Protocol: edgeStr(loc.Get("protocol")),
			Host:     edgeStr(loc.Get("host")),
			Pathname: edgeStr(loc.Get("pathname")),
			Search:   edgeStr(loc.Get("search")),
		})
	}
	// The probe builds its own svg content: a style element and one group.
	ctx.SVG.RemoveChildren()
	style := ctx.Doc.CreateElement("style")
	style.Text(g.Styles[int(si)])
	ctx.SVG.AppendChild(style)
	root := ctx.SVG.Append("g")
	edges.Clear(ctx)

	cmp := &edgeCompare{exact: true}
	if markers, ok := c.Get("markers").([]any); ok {
		err := edges.InsertMarkers(ctx, root, edgeStrings(markers), typ, id)
		if caseErr != "" {
			if err == nil {
				return EdgeResult{name, "different", "insertMarkers: no error, upstream fails: " + caseErr}
			}
			return EdgeResult{name, "exact", ""}
		}
		if err != nil {
			return EdgeResult{name, "error", "insertMarkers: " + err.Error()}
		}
	}
	if caseErr != "" {
		return EdgeResult{name, "error", "the golden case failed in the oracle: " + caseErr}
	}
	edgePathsGroup := root.Insert("g").Attr("class", "edgePaths")
	edgeLabelsGroup := root.Insert("g").Attr("class", "edgeLabels")

	var edgeList []*ru.Edge
	if arr, ok := c.Get("edges").([]any); ok {
		for _, e := range arr {
			edgeList = append(edgeList, EdgeFromJS(js.AsObj(e)))
		}
	}
	results, _ := c.Get("results").([]any)
	if len(results) != len(ops) {
		return EdgeResult{name, "error", "the golden has no result for every op"}
	}
	pathsOf := map[int]*edges.Paths{}
	edgeAt := func(op *js.Obj[any]) *ru.Edge {
		i, _ := op.Get("edge").(float64)
		return edgeList[int(i)]
	}
	for i, o := range ops {
		op := js.AsObj(o)
		want := js.AsObj(results[i])
		wantErr := edgeStr(want.Get("error"))
		kind := edgeStr(op.Get("op"))
		what := fmt.Sprintf("op %d (%s)", i, kind)
		var opErr error
		switch kind {
		case "label":
			var el *dom.Node
			el, opErr = edges.InsertEdgeLabel(ctx, edgeLabelsGroup, edgeAt(op))
			if opErr == nil && wantErr == "" {
				cmp.text(what+" tag", el.Tag, edgeStr(want.Get("tag")))
			}
		case "labels":
			var batch []*ru.Edge
			idx, _ := op.Get("edges").([]any)
			for _, k := range idx {
				n, _ := k.(float64)
				batch = append(batch, edgeList[int(n)])
			}
			opErr = edges.InsertEdgeLabels(ctx, edgeLabelsGroup, batch)
		case "edge":
			var clusterDb edges.ClusterDb
			if db := js.AsObj(op.Get("clusterDb")); db != nil {
				clusterDb = func(id string) (ru.Bounds, bool) {
					n := js.AsObj(db.Get(id))
					if n == nil {
						return ru.Bounds{}, false
					}
					return ru.Bounds{X: edgeNum(n.Get("x")), Y: edgeNum(n.Get("y")), Width: edgeNum(n.Get("width")), Height: edgeNum(n.Get("height"))}, true
				}
			}
			diagramType := edgeStr(op.Get("diagramType"))
			if diagramType == "" {
				diagramType = typ
			}
			diagramID := id
			if js.Truthy(op.Get("noDiagramId")) {
				diagramID = ""
			}
			var paths *edges.Paths
			paths, opErr = edges.InsertEdge(ctx, edgePathsGroup, edgeAt(op), clusterDb, diagramType,
				edgeNode(op.Get("startNode")), edgeNode(op.Get("endNode")), diagramID,
				js.Truthy(op.Get("skipIntersect")), edgeStr(op.Get("layoutAlgorithm")))
			if opErr == nil && wantErr == "" {
				pathsOf[i] = paths
				wp := js.AsObj(want.Get("paths"))
				if up := wp.Get("updatedPath"); up != nil {
					if !paths.HasUpdatedPath {
						cmp.fail("%s: no updatedPath, upstream has one", what)
					} else {
						cmp.points(what+" updatedPath", paths.UpdatedPath, up)
					}
				} else if paths.HasUpdatedPath {
					cmp.fail("%s: updatedPath, upstream has none", what)
				}
				if op := wp.Get("originalPath"); op != nil {
					cmp.points(what+" originalPath", paths.OriginalPath, op)
				}
			}
		case "position":
			var paths *edges.Paths
			if po := js.AsObj(op.Get("paths")); po != nil {
				if from, ok := po.Get("fromOp").(float64); ok {
					paths = pathsOf[int(from)]
				} else {
					paths = edgePaths(po)
				}
			}
			opErr = edges.PositionEdgeLabel(ctx, edgeAt(op), paths)
		case "curve":
			got := edges.ResolveEdgeCurveType(ctx, edgeAt(op).Curve)
			cmp.text(what, got, edgeStr(want.Get("curve")))
		case "clear":
			edges.Clear(ctx)
		case "lineJumps":
			edges.ApplyLineJumpsToSvg(edgePathsGroup, edgeGeoms(op.Get("edges")), edgeJumpConfig(op.Get("config")))
		default:
			return EdgeResult{name, "error", "unknown op " + kind}
		}
		if (opErr != nil) != (wantErr != "") {
			if opErr != nil {
				cmp.fail("%s: error %v, upstream has none", what, opErr)
			} else {
				cmp.fail("%s: no error, upstream fails: %s", what, wantErr)
			}
		}
	}

	got := root.InnerHTML()
	wantHTML := edgeStr(c.Get("html"))
	if got != wantHTML {
		cmp.inexact("markup: %s", firstDifference(got, wantHTML))
		verdict, diff := parity.CompareSVG("<svg>"+got+"</svg>", "<svg>"+wantHTML+"</svg>", 0.01)
		if verdict == parity.Different {
			cmp.fail("markup: %s", diff.String())
		}
	}

	// The module maps.
	wantLabelIDs := edgeStrings(c.Get("edgeLabelIds"))
	sort.Strings(wantLabelIDs)
	if gotIDs := sortedKeys(edges.EdgeLabels(ctx)); strings.Join(gotIDs, "\x00") != strings.Join(wantLabelIDs, "\x00") {
		cmp.fail("edgeLabels: got %q, want %q", gotIDs, wantLabelIDs)
	}
	terminalLabels := edges.TerminalLabels(ctx)
	wantTerminals, _ := c.Get("terminalLabelIds").([]any)
	if len(wantTerminals) != len(terminalLabels) {
		cmp.fail("terminalLabels: got %d entries, want %d", len(terminalLabels), len(wantTerminals))
	}
	for _, entry := range wantTerminals {
		pair, _ := entry.([]any)
		if len(pair) != 2 {
			continue
		}
		elems := terminalLabels[edgeStr(pair[0])]
		for _, key := range edgeStrings(pair[1]) {
			if elems.Get(key) == nil {
				cmp.fail("terminalLabels[%q] has no %s", edgeStr(pair[0]), key)
			}
		}
	}

	// The edges after the calls.
	after, _ := c.Get("edgesAfter").([]any)
	for i, a := range after {
		if i >= len(edgeList) {
			break
		}
		o := js.AsObj(a)
		e := edgeList[i]
		what := fmt.Sprintf("edge %d", i)
		cmp.num(what+" width", e.Width, o.Get("width"))
		cmp.num(what+" height", e.Height, o.Get("height"))
		if e.LabelStyleIsText {
			if s, ok := o.Get("labelStyle").(string); !ok || s != e.LabelStyleText {
				cmp.fail("%s labelStyle: got %q, want %v", what, e.LabelStyleText, o.Get("labelStyle"))
			}
		} else if _, ok := o.Get("labelStyle").(string); ok && !edgeIsUndefined(o.Get("labelStyle")) {
			cmp.fail("%s labelStyle: got an array, want %v", what, o.Get("labelStyle"))
		}
		sizes := js.AsObj(o.Get("terminalLabelSizes"))
		wantCount := 0
		if sizes != nil {
			wantCount = sizes.Len()
			sizes.Each(func(key string, v any) {
				s := js.AsObj(v)
				size, ok := e.TerminalLabelSizes[key]
				if !ok || s == nil {
					cmp.fail("%s terminalLabelSizes has no %s", what, key)
					return
				}
				cmp.num(what+" terminalLabelSizes."+key+".width", size.Width, s.Get("width"))
				cmp.num(what+" terminalLabelSizes."+key+".height", size.Height, s.Get("height"))
			})
		}
		if len(e.TerminalLabelSizes) != wantCount {
			cmp.fail("%s terminalLabelSizes: got %d entries, want %d", what, len(e.TerminalLabelSizes), wantCount)
		}
		if pts := o.Get("points"); !edgeIsUndefined(pts) {
			cmp.points(what+" points", e.Points, pts)
		}
	}
	return cmp.result(name)
}

// edgeCaseHasLabel reports whether a case inserts an edge label. The size
// of a label is the size of its text in Chromium on the macOS reference
// machine, and the markup and the positions of the case depend on it.
func edgeCaseHasLabel(raw json.RawMessage) bool {
	var c struct {
		Ops []struct {
			Op string `json:"op"`
		} `json:"ops"`
	}
	if json.Unmarshal(raw, &c) != nil {
		return false
	}
	for _, o := range c.Ops {
		if o.Op == "label" || o.Op == "labels" {
			return true
		}
	}
	return false
}

// RunEdgeGolden replays every case of a golden file and fails the test for
// cases that differ. It logs a summary and returns the counts by status.
//
// On a machine without the system fonts of the macOS reference machine, a
// case with an edge label fails the test only for an error, and the test is
// marked as skipped after the last case. A case with no label does not
// depend on the fonts, and every machine compares it.
func RunEdgeGolden(t *testing.T, path string) map[string]int {
	t.Helper()
	g := LoadEdgeGolden(t, path)
	counts := map[string]int{}
	reference := fonttest.IsMacOSReference()
	skipped := 0
	for _, raw := range g.Cases {
		r := RunEdgeCase(g, raw)
		counts[r.Status]++
		skip := !reference && edgeCaseHasLabel(raw)
		if skip {
			skipped++
		}
		if r.Status == "error" || r.Status == "different" && !skip {
			t.Errorf("%s: %s\n    %s", r.Name, r.Status, r.Detail)
		} else if r.Status == "equivalent" && testing.Verbose() {
			t.Logf("%s: equivalent\n    %s", r.Name, r.Detail)
		}
	}
	t.Logf("%s: %d cases: exact=%d equivalent=%d different=%d error=%d",
		path, len(g.Cases), counts["exact"], counts["equivalent"], counts["different"], counts["error"])
	RequireFonts(t, skipped)
	return counts
}
