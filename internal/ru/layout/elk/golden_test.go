package elk

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	elkjs "github.com/bvolpato/mermaid-go-renderer/internal/elk"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	rugraph "github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
)

// The golden file testdata/fixtures.jsonl.gz is written by
// tools/reference/gen-ru-elk.mjs. Each line is one corpus fixture that the
// real mermaid rendered through an ELK layout in Chromium, with what
// tools/reference/probes/ru-elk.js recorded.
//
// The golden file testdata/random.jsonl.gz is written by
// tools/reference/gen-ru-elk-random.mjs. Each line is random layout data
// that went through the real prepareLayoutForElk and runElkLayoutCore of
// mermaid in node, with the same fields. It has the hash of the output of
// ELK in place of the output, the kinds of the intersect functions in place
// of their calls, and the error message for a graph that elkjs rejects.

// elkCall is one call of elk.layout of a fixture.
type elkCall struct {
	fixture string
	// layout is the name of the layout, and algorithm is its ELK algorithm.
	layout    string
	algorithm string
	// entry and prepared are the edges at the start of the layout render
	// and after the prepare phase.
	entry              []*js.Obj[any]
	prepared           []*js.Obj[any]
	preparedCurveTypes []string
	// nodes and edges are data4Layout when elk.layout is called.
	nodes        []*js.Obj[any]
	edges        []*js.Obj[any]
	hasIntersect []bool
	direction    any
	// dataConfig is data4Layout.config, and getConfig is getConfig(),
	// reduced to what the layout reads.
	dataConfig *js.Obj[any]
	getConfig  *js.Obj[any]
	// in and out are the JSON of the graph before and after elk.layout.
	// A random case has outHash, the SHA-256 of out without the "$H"
	// members, and no out.
	in      string
	out     string
	outHash string
	// err is the message of the error of elk.layout, for a graph that
	// elkjs rejects.
	err string
	// intersect has the calls of node.intersect in call order: the index of
	// the node in the ELK graph, the point and the result.
	intersect []intersectCall
	// kinds has, for a random case, the kind of the intersect function of
	// each node, and intersectCount the number of calls.
	kinds          []string
	intersectCount int
	// resultNodes and resultEdges are data4Layout after runElkLayoutCore.
	resultNodes []*js.Obj[any]
	resultEdges []*js.Obj[any]
}

type intersectCall struct {
	index  int
	p      P
	res    P
	hasRes bool
}

var (
	callsOnce sync.Once
	callsList []*elkCall
	callsErr  error
)

// decodeNums replaces every { $num: "NaN" } of the capture by its number.
func decodeNums(v any) any {
	switch x := v.(type) {
	case []any:
		for i, e := range x {
			x[i] = decodeNums(e)
		}
		return x
	case *js.Obj[any]:
		if x.Len() == 1 && x.Has("$num") {
			switch x.Get("$num") {
			case "NaN":
				return math.NaN()
			case "Infinity":
				return math.Inf(1)
			case "-Infinity":
				return math.Inf(-1)
			}
		}
		for _, k := range x.Keys() {
			x.Set(k, decodeNums(x.Get(k)))
		}
		return x
	}
	return v
}

func objs(v any) []*js.Obj[any] {
	arr, _ := v.([]any)
	out := make([]*js.Obj[any], len(arr))
	for i, e := range arr {
		out[i] = js.AsObj(e)
	}
	return out
}

func loadCalls(t testing.TB) []*elkCall {
	t.Helper()
	callsOnce.Do(func() {
		for _, file := range []string{"testdata/fixtures.jsonl.gz", "testdata/random.jsonl.gz"} {
			if callsErr = loadCallsFile(file); callsErr != nil {
				return
			}
		}
	})
	if callsErr != nil {
		t.Fatal(callsErr)
	}
	if len(callsList) == 0 {
		t.Fatal("no fixtures")
	}
	return callsList
}

func loadCallsFile(file string) error {
	{
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()
		zr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		sc := bufio.NewScanner(zr)
		sc.Buffer(make([]byte, 1<<20), 256<<20)
		for sc.Scan() {
			if len(sc.Bytes()) == 0 {
				continue
			}
			parsed, err := js.ParseJSON(sc.Bytes())
			if err != nil {
				return fmt.Errorf("%s line %d: %w", file, len(callsList)+1, err)
			}
			rec := decodeNums(parsed)
			name, _ := prop(rec, "name").(string)
			renders, _ := prop(rec, "renders").([]any)
			for _, r := range renders {
				calls, _ := prop(r, "elk").([]any)
				results, _ := prop(r, "results").([]any)
				for ci, c := range calls {
					call := &elkCall{fixture: name}
					call.layout, _ = prop(r, "layout").(string)
					call.algorithm, _ = prop(r, "algorithm").(string)
					call.entry = objs(prop(r, "entry"))
					call.prepared = objs(prop(r, "prepared"))
					for _, ct := range prop(r, "preparedCurveTypes").([]any) {
						call.preparedCurveTypes = append(call.preparedCurveTypes, ct.(string))
					}
					data := prop(c, "data")
					call.nodes = objs(prop(data, "nodes"))
					call.edges = objs(prop(data, "edges"))
					for _, flag := range prop(data, "intersect").([]any) {
						call.hasIntersect = append(call.hasIntersect, flag == 1.0)
					}
					call.direction = prop(data, "direction")
					call.dataConfig = js.AsObj(prop(data, "config"))
					call.getConfig = js.AsObj(prop(data, "getConfig"))
					call.in, _ = prop(c, "in").(string)
					call.out, _ = prop(c, "out").(string)
					call.outHash, _ = prop(c, "outHash").(string)
					call.err, _ = prop(c, "error").(string)
					if kinds, ok := prop(data, "kinds").([]any); ok {
						for _, k := range kinds {
							kind, _ := k.(string)
							call.kinds = append(call.kinds, kind)
						}
						count, _ := prop(c, "intersectCount").(float64)
						call.intersectCount = int(count)
					}
					recorded, _ := prop(c, "intersect").([]any)
					for _, ic := range recorded {
						a := ic.([]any)
						entry := intersectCall{index: int(a[0].(float64)), p: P{X: a[1].(float64), Y: a[2].(float64)}}
						if rx, ok := a[3].(float64); ok {
							entry.res = P{X: rx, Y: a[4].(float64)}
							entry.hasRes = true
						}
						call.intersect = append(call.intersect, entry)
					}
					if ci < len(results) {
						call.resultNodes = objs(prop(results[ci], "nodes"))
						call.resultEdges = objs(prop(results[ci], "edges"))
					}
					if call.in == "" || (call.out == "" && call.outHash == "" && call.err == "") {
						return fmt.Errorf("%s: no ELK graph recorded", name)
					}
					callsList = append(callsList, call)
				}
			}
		}
		return sc.Err()
	}
}

// isRandom reports whether the call is a random case.
func (c *elkCall) isRandom() bool { return c.kinds != nil }

func (c *elkCall) elkContext() *ElkLayoutContext {
	return &ElkLayoutContext{
		Algorithm: c.algorithm,
		GetConfig: func() *js.Obj[any] { return c.getConfig },
	}
}

// objects returns the recorded layout data as JavaScript objects, with the
// property order of upstream.
func (c *elkCall) objects() *layoutObjects {
	return &layoutObjects{nodes: c.nodes, edges: c.edges, config: c.dataConfig, direction: c.direction}
}

// layoutData returns the recorded layout data as the structs of
// internal/ru: what the Go renderer hands to the layout. The nodes that had
// an intersect function upstream get intersect.
func (c *elkCall) layoutData(intersect func(index int, node *ru.Node) func(point P) P) *ru.LayoutData {
	data := &ru.LayoutData{Config: c.dataConfig}
	if s, ok := c.direction.(string); ok {
		data.Direction = s
	}
	for i, o := range c.nodes {
		n := ru.NodeFromJS(o)
		if c.hasIntersect[i] && intersect != nil {
			n.Intersect = intersect(i, n)
		}
		data.Nodes = append(data.Nodes, n)
	}
	for _, o := range c.edges {
		data.Edges = append(data.Edges, edgeFromJS(o))
	}
	return data
}

// edgeFromJS builds an Edge from a JavaScript object with the property
// names of types.ts.
func edgeFromJS(o *js.Obj[any]) *ru.Edge {
	e := ru.NewEdge("")
	e.StyleUndefined = true
	e.ClassesUndefined = true
	str := func(v any) string {
		if js.IsNullish(v) {
			return ""
		}
		return js.ToString(v)
	}
	numOf := func(v any) float64 {
		if js.IsNullish(v) {
			return ru.Undef
		}
		return js.ToNumber(v)
	}
	strList := func(v any) []string {
		arr, ok := v.([]any)
		if !ok {
			return nil
		}
		out := make([]string, len(arr))
		for i, s := range arr {
			out[i] = js.ToString(s)
		}
		return out
	}
	o.Each(func(k string, v any) {
		switch k {
		case "id":
			e.ID = str(v)
		case "label":
			e.Label, e.HasLabel = str(v), !js.IsNullish(v)
		case "classes":
			e.Classes, e.ClassesUndefined = str(v), js.IsNullish(v)
		case "style":
			switch s := v.(type) {
			case string:
				e.Style, e.StyleUndefined = []string{s}, false
			case []any:
				e.Style, e.StyleUndefined = strList(s), false
			}
		case "animate":
			e.Animate, e.HasAnimate = js.Truthy(v), true
		case "animation":
			e.Animation = str(v)
		case "metadata":
			e.Metadata = js.AsObj(v)
		case "arrowhead":
			e.Arrowhead = str(v)
		case "arrowheadStyle":
			e.ArrowheadStyle = str(v)
		case "arrowTypeEnd":
			e.ArrowTypeEnd = str(v)
		case "arrowTypeStart":
			e.ArrowTypeStart = str(v)
		case "cssCompiledStyles":
			e.CSSCompiledStyles = strList(v)
		case "defaultInterpolate":
			e.DefaultInterpolate = str(v)
		case "end":
			e.End = str(v)
		case "interpolate":
			e.Interpolate = str(v)
		case "labelType":
			e.LabelType = str(v)
		case "length":
			e.Length = numOf(v)
		case "start":
			e.Start = str(v)
		case "stroke":
			e.Stroke = str(v)
		case "text":
			e.Text = str(v)
		case "type":
			e.Type = str(v)
		case "startLabelRight":
			e.StartLabelRight = str(v)
		case "endLabelLeft":
			e.EndLabelLeft = str(v)
		case "startLabelLeft":
			e.StartLabelLeft = str(v)
		case "endLabelRight":
			e.EndLabelRight = str(v)
		case "terminalLabelSizes":
			if sizes := js.AsObj(v); sizes != nil {
				e.TerminalLabelSizes = map[string]ru.TerminalLabelSize{}
				sizes.Each(func(key string, size any) {
					e.TerminalLabelSizes[key] = ru.TerminalLabelSize{Width: numOf(prop(size, "width")), Height: numOf(prop(size, "height"))}
				})
			}
		case "terminalLabelCenters":
			if centers := js.AsObj(v); centers != nil {
				e.TerminalLabelCenters = map[string]ru.Point{}
				centers.Each(func(key string, center any) {
					e.TerminalLabelCenters[key] = ru.Point{X: numOf(prop(center, "x")), Y: numOf(prop(center, "y"))}
				})
			}
		case "curve":
			e.Curve, _ = v.(string)
		case "labelpos":
			e.Labelpos = str(v)
		case "labelStyle":
			switch s := v.(type) {
			case string:
				e.LabelStyleText, e.LabelStyleIsText = s, true
			case []any:
				e.LabelStyle = strList(s)
			}
		case "minlen":
			e.Minlen = numOf(v)
		case "pattern":
			e.Pattern = str(v)
		case "thickness":
			e.Thickness = str(v)
		case "look":
			e.Look = str(v)
		case "isUserDefinedId":
			e.IsUserDefinedID = js.Truthy(v)
		case "showPoints":
			e.ShowPoints = js.Truthy(v)
		case "width":
			e.Width = numOf(v)
		case "height":
			e.Height = numOf(v)
		case "x":
			e.X = numOf(v)
		case "y":
			e.Y = numOf(v)
		case "points":
			if arr, ok := v.([]any); ok {
				e.Points = []ru.Point{}
				for _, p := range arr {
					e.Points = append(e.Points, ru.Point{X: numOf(prop(p, "x")), Y: numOf(prop(p, "y"))})
				}
			}
		case "parentId":
			e.ParentID = str(v)
		case "dir":
			e.Dir = str(v)
		case "source":
			e.Source = str(v)
		case "target":
			e.Target = str(v)
		case "depth":
			e.Depth = numOf(v)
		case "isLabelEdge":
			e.IsLabelEdge = js.Truthy(v)
		case "labelNodeId":
			e.LabelNodeID = str(v)
		case "isLayoutOnly":
			e.IsLayoutOnly = js.Truthy(v)
		case "cssStyles":
			e.CSSStyles = strList(v)
		default:
			e.Set(k, v)
		}
	})
	return e
}

// sameNum compares two numbers by their bits. JSON has no negative zero, so
// the two zeros are the same.
func sameNum(a, b float64) bool {
	if a == 0 && b == 0 {
		return true
	}
	return math.Float64bits(a) == math.Float64bits(b) || (a != a && b != b)
}

func numStr(v float64) string {
	if v != v {
		return "undefined"
	}
	return js.Str(v)
}

// firstDiff returns a window around the first byte where two strings differ.
func firstDiff(got, want string) string {
	i := 0
	for i < len(got) && i < len(want) && got[i] == want[i] {
		i++
	}
	window := func(s string) string {
		lo, hi := max(0, i-80), min(len(s), i+80)
		return s[lo:hi]
	}
	return fmt.Sprintf("at byte %d\n got: …%s…\nwant: …%s…", i, window(got), window(want))
}

// TestBuildElkGraph is layer 1, the first half: the recorded measured
// layout data, as objects with the property order of upstream, must give
// the JSON that upstream passed to elk.layout, byte for byte.
func TestBuildElkGraph(t *testing.T) {
	calls := loadCalls(t)
	failed := 0
	for _, c := range calls {
		state := buildElkGraphFromLayoutData(c.objects(), c.elkContext())
		got := js.StringifyJSON(state.elkGraph)
		if got != c.in {
			failed++
			if failed <= 5 {
				t.Errorf("%s: ELK input differs %s", c.fixture, firstDiff(got, c.in))
			}
		}
	}
	t.Logf("%d ELK graphs, %d differ", len(calls), failed)
	if failed > 0 {
		t.Errorf("%d of %d ELK graphs differ", failed, len(calls))
	}
}

// elkKeys are the properties that ELK reads from a graph element, and the
// properties that applyElkLayoutResult reads from the result.
var elkKeys = map[string]bool{
	// The ELK JSON format.
	"id": true, "x": true, "y": true, "width": true, "height": true,
	"children": true, "edges": true, "ports": true, "labels": true, "text": true,
	"layoutOptions": true, "properties": true,
	"sources": true, "targets": true, "source": true, "target": true,
	"sourcePort": true, "targetPort": true, "sections": true, "junctionPoints": true,
	// What the layout reads back.
	"isGroup": true, "shape": true, "parentId": true, "padding": true, "labelData": true,
	"start": true, "end": true, "sourceId": true, "targetId": true, "layoutReversed": true,
	"terminal": true, "orgWidth": true, "orgHeight": true,
}

// project keeps the properties of elkKeys in the elements of an ELK graph.
// A node of the graph keeps "text" only in its labels: mermaid nodes have a
// text property of their own, which ELK does not read.
func project(v any, kind string) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = project(e, kind)
		}
		return out
	case *js.Obj[any]:
		out := js.NewObj[any]()
		x.Each(func(k string, e any) {
			switch {
			case k == "layoutOptions" || k == "labelData" || k == "sources" || k == "targets":
				out.Set(k, e)
			case k == "children":
				out.Set(k, project(e, "node"))
			case k == "edges":
				out.Set(k, project(e, "edge"))
			case k == "labels":
				out.Set(k, project(e, "label"))
			case k == "text" && kind != "label":
			case k == "isGroup" && !js.Truthy(e):
				// A node without isGroup is not a group.
			case k == "source" || k == "target":
				if kind == "edge" {
					out.Set(k, e)
				}
			case elkKeys[k]:
				out.Set(k, e)
			}
		})
		return out
	}
	return v
}

// canonical sorts the properties of the elements of a projected graph, and
// keeps the order of arrays and of layout options.
func canonical(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = canonical(e)
		}
		return out
	case *js.Obj[any]:
		keys := x.Keys()
		js.SortStrings(keys)
		out := js.NewObj[any]()
		for _, k := range keys {
			if k == "layoutOptions" {
				out.Set(k, x.Get(k))
			} else {
				out.Set(k, canonical(x.Get(k)))
			}
		}
		return out
	}
	return v
}

func projectedJSON(t *testing.T, graphJSON string) string {
	t.Helper()
	parsed, err := js.ParseJSON([]byte(graphJSON))
	if err != nil {
		t.Fatal(err)
	}
	return js.StringifyJSON(canonical(project(parsed, "root")))
}

// TestBuildElkGraphFromStructs is layer 1 for the structs of internal/ru:
// the graph that the layout builds from ru.Node and ru.Edge values must
// have the same elements as the graph of upstream, with the same values of
// every property that ELK or the layout reads, in the same order of
// children, edges, labels and layout options. The other properties of the
// nodes and edges go to ELK too, in the order of the struct fields.
func TestBuildElkGraphFromStructs(t *testing.T) {
	calls := loadCalls(t)
	failed := 0
	for _, c := range calls {
		data := c.layoutData(nil)
		state := buildElkGraphFromLayoutData(newLayoutObjects(data), c.elkContext())
		got := projectedJSON(t, js.StringifyJSON(state.elkGraph))
		want := projectedJSON(t, c.in)
		if got != want {
			failed++
			if failed <= 5 {
				t.Errorf("%s: ELK input differs %s", c.fixture, firstDiff(got, want))
			}
		}
	}
	t.Logf("%d ELK graphs, %d differ", len(calls), failed)
	if failed > 0 {
		t.Errorf("%d of %d ELK graphs differ", failed, len(calls))
	}
}

// replay stands for the intersect functions of the nodes of a call. For a
// fixture it plays the recorded calls of node.intersect back: each call of
// the layout must be the next recorded call, for the same node and point.
// For a random case it has the intersect functions of
// tools/reference/gen-ru-elk-random.mjs, and counts the calls.
type replay struct {
	call *elkCall
	// dfs has the index in the ELK graph of each layout node.
	dfs   map[int]int
	next  int
	errs  []string
	nulls int
}

func newReplay(c *elkCall, state *ElkLayoutState) *replay {
	r := &replay{call: c, dfs: map[int]int{}}
	index := 0
	var walk func(children any)
	walk = func(children any) {
		arr, _ := children.([]any)
		for _, child := range arr {
			o := js.AsObj(child)
			i := index
			index++
			if layoutIndex, ok := state.nodeIndex[o]; ok {
				r.dfs[layoutIndex] = i
			}
			walk(prop(o, "children"))
		}
	}
	walk(prop(state.elkGraph, "children"))
	return r
}

func (r *replay) intersect(layoutIndex int, node *ru.Node) func(point P) P {
	if r.call.isRandom() {
		fn := kindIntersect(r.call.kinds[layoutIndex], node)
		return func(point P) P {
			r.next++
			return fn(point)
		}
	}
	return func(point P) P {
		if r.next >= len(r.call.intersect) {
			r.errs = append(r.errs, fmt.Sprintf("intersect call %d: upstream made %d calls", r.next+1, len(r.call.intersect)))
			r.next++
			return P{X: math.NaN(), Y: math.NaN()}
		}
		rec := r.call.intersect[r.next]
		r.next++
		if rec.index != r.dfs[layoutIndex] || !sameNum(rec.p.X, point.X) || !sameNum(rec.p.Y, point.Y) {
			r.errs = append(r.errs, fmt.Sprintf("intersect call %d: node %d point (%s, %s), upstream node %d point (%s, %s)",
				r.next, r.dfs[layoutIndex], js.Str(point.X), js.Str(point.Y), rec.index, js.Str(rec.p.X), js.Str(rec.p.Y)))
		}
		if !rec.hasRes {
			r.nulls++
		}
		return rec.res
	}
}

// errors returns what the calls of the layout did wrong.
func (r *replay) errors() []string {
	errs := r.errs
	want := len(r.call.intersect)
	if r.call.isRandom() {
		want = r.call.intersectCount
	}
	if r.next != want && len(errs) == 0 {
		errs = append(errs, fmt.Sprintf("%d intersect calls, upstream %d", r.next, want))
	}
	if r.nulls > 0 {
		errs = append(errs, fmt.Sprintf("%d intersect calls of upstream returned no point", r.nulls))
	}
	return errs
}

// kindIntersect returns the intersect function of a node of a random case:
// the Go form of INTERSECT of tools/reference/gen-ru-elk-random.mjs. The
// function reads the position and the size of the node when it is called,
// as the closures of the shapes of mermaid do.
func kindIntersect(kind string, node *ru.Node) func(point P) P {
	ellipse := func(rx, ry float64, point P) P {
		cx := node.X
		cy := node.Y
		px := cx - point.X
		py := cy - point.Y
		det := math.Sqrt(float64(rx*rx*py*py) + float64(ry*ry*px*px))
		dx := math.Abs((rx * ry * px) / det)
		if point.X < cx {
			dx = -dx
		}
		dy := math.Abs((rx * ry * py) / det)
		if point.Y < cy {
			dy = -dy
		}
		return P{X: cx + dx, Y: cy + dy}
	}
	switch kind {
	case "circle":
		return func(point P) P { return ellipse(float64(node.Width/2), float64(node.Width/2), point) }
	case "ellipse":
		return func(point P) P { return ellipse(float64(node.Width/2), float64(node.Height/2), point) }
	case "diamond":
		return func(point P) P {
			dx := point.X - node.X
			dy := point.Y - node.Y
			s := 1 / (math.Abs(dx)/float64(node.Width/2) + math.Abs(dy)/float64(node.Height/2))
			return P{X: node.X + float64(dx*s), Y: node.Y + float64(dy*s)}
		}
	case "wrongSide":
		return func(point P) P {
			return P{X: float64(2*node.X) - point.X, Y: float64(2*node.Y) - point.Y}
		}
	case "self":
		return func(point P) P { return point }
	}
	// rect
	return func(point P) P {
		x := node.X
		y := node.Y
		dx := point.X - x
		dy := point.Y - y
		w := float64(node.Width / 2)
		h := float64(node.Height / 2)
		var sx, sy float64
		if math.Abs(dy)*w > math.Abs(dx)*h {
			if dy < 0 {
				h = -h
			}
			if dy == 0 {
				sx = 0
			} else {
				sx = (h * dx) / dy
			}
			sy = h
		} else {
			if dx < 0 {
				w = -w
			}
			sx = w
			if dx == 0 {
				sy = 0
			} else {
				sy = (w * dy) / dx
			}
		}
		return P{X: x + sx, Y: y + sy}
	}
}

// compareResult compares the layout data after the layout with what
// upstream had when runElkLayoutCore ended.
func compareResult(c *elkCall, data *ru.LayoutData) []string {
	var errs []string
	add := func(format string, args ...any) {
		if len(errs) < 8 {
			errs = append(errs, fmt.Sprintf(format, args...))
		}
	}
	if len(data.Nodes) != len(c.resultNodes) {
		add("%d nodes, upstream %d", len(data.Nodes), len(c.resultNodes))
		return errs
	}
	optNumOf := func(o *js.Obj[any], key string) float64 { return optNum(o.Get(key)) }
	for i, want := range c.resultNodes {
		got := data.Nodes[i]
		if id, _ := want.Get("id").(string); got.ID != id {
			add("node %d is %q, upstream %q", i, got.ID, id)
			continue
		}
		for _, f := range []struct {
			name string
			got  float64
		}{{"x", got.X}, {"y", got.Y}, {"width", got.Width}, {"height", got.Height}} {
			if w := optNumOf(want, f.name); !sameNum(f.got, w) {
				add("node %s %s = %s, upstream %s", got.ID, f.name, numStr(f.got), numStr(w))
			}
		}
		for _, key := range []string{"labelData", "labels"} {
			gotJSON, wantJSON := "undefined", "undefined"
			if v := got.Get(key); v != nil {
				gotJSON = js.StringifyJSON(v)
			}
			if v := want.Get(key); v != nil {
				wantJSON = js.StringifyJSON(v)
			}
			if gotJSON != wantJSON {
				add("node %s %s = %s, upstream %s", got.ID, key, gotJSON, wantJSON)
			}
		}
	}
	if len(data.Edges) != len(c.resultEdges) {
		add("%d edges, upstream %d", len(data.Edges), len(c.resultEdges))
		return errs
	}
	for i, want := range c.resultEdges {
		got := data.Edges[i]
		if id, _ := want.Get("id").(string); got.ID != id {
			add("edge %d is %q, upstream %q", i, got.ID, id)
			continue
		}
		if w := optNumOf(want, "x"); !sameNum(got.X, w) {
			add("edge %s x = %s, upstream %s", got.ID, numStr(got.X), numStr(w))
		}
		if w := optNumOf(want, "y"); !sameNum(got.Y, w) {
			add("edge %s y = %s, upstream %s", got.ID, numStr(got.Y), numStr(w))
		}
		// A curve that is not a string is a function upstream, and "" here.
		wantCurve, _ := want.Get("curve").(string)
		if got.Curve != wantCurve {
			add("edge %s curve = %q, upstream %q", got.ID, got.Curve, wantCurve)
		}
		wantPoints, hasPoints := want.Get("points").([]any)
		if !hasPoints {
			if got.Points != nil {
				add("edge %s has %d points, upstream none", got.ID, len(got.Points))
			}
		} else if len(got.Points) != len(wantPoints) {
			add("edge %s has %d points, upstream %d: %v", got.ID, len(got.Points), len(wantPoints), got.Points)
		} else {
			for j, wp := range wantPoints {
				wx, wy := optNum(prop(wp, "x")), optNum(prop(wp, "y"))
				if !sameNum(got.Points[j].X, wx) || !sameNum(got.Points[j].Y, wy) {
					add("edge %s point %d = (%s, %s), upstream (%s, %s)", got.ID, j, numStr(got.Points[j].X), numStr(got.Points[j].Y), numStr(wx), numStr(wy))
				}
			}
		}
		wantCenters := js.AsObj(want.Get("terminalLabelCenters"))
		if wantCenters == nil {
			if got.TerminalLabelCenters != nil {
				add("edge %s has terminal label centers, upstream none", got.ID)
			}
			continue
		}
		gotKeys := terminalKeys(got.TerminalLabelCenters)
		if strings.Join(gotKeys, ",") != strings.Join(wantCenters.Keys(), ",") {
			add("edge %s terminal labels %v, upstream %v", got.ID, gotKeys, wantCenters.Keys())
			continue
		}
		for _, key := range gotKeys {
			g := got.TerminalLabelCenters[key]
			wx, wy := optNum(prop(wantCenters.Get(key), "x")), optNum(prop(wantCenters.Get(key), "y"))
			if !sameNum(g.X, wx) || !sameNum(g.Y, wy) {
				add("edge %s terminal %s = (%s, %s), upstream (%s, %s)", got.ID, key, numStr(g.X), numStr(g.Y), numStr(wx), numStr(wy))
			}
		}
	}
	return errs
}

// applyRecorded decodes an output of ELK for the graph of the call, applies
// it to the recorded layout data and returns the differences from what
// upstream had after the layout.
func applyRecorded(c *elkCall, out []byte) []string {
	state := buildElkGraphFromLayoutData(c.objects(), c.elkContext())
	rp := newReplay(c, state)
	data := c.layoutData(rp.intersect)
	var failure []string
	func() {
		defer func() {
			if r := recover(); r != nil {
				failure = append(failure, fmt.Sprintf("panic: %v", r))
			}
		}()
		graph, err := decodeElkResult(out, state, func(index int) func(point P) P { return data.Nodes[index].Intersect })
		if err != nil {
			failure = append(failure, err.Error())
			return
		}
		applyElkLayoutResult(data, graph, state)
		orderNodesForElkPaint(data)
	}()
	errs := append(failure, rp.errors()...)
	return append(errs, compareResult(c, data)...)
}

// TestApplyElkLayoutResult is layer 1, the second half: the recorded output
// of ELK, applied to the recorded layout data, must give the node positions
// and the edge points that upstream had after the layout, bit for bit. The
// intersect functions of the nodes are the recorded calls of upstream: the
// layout must call them with the same points in the same order.
func TestApplyElkLayoutResult(t *testing.T) {
	calls := loadCalls(t)
	failed, intersects, layouts := 0, 0, 0
	for _, c := range calls {
		if c.out == "" {
			// A random case has no recorded output: TestElkEngine applies
			// the output of the Go engine.
			continue
		}
		layouts++
		intersects += len(c.intersect)
		if errs := applyRecorded(c, []byte(c.out)); len(errs) > 0 {
			failed++
			if failed <= 5 {
				t.Errorf("%s:\n  %s", c.fixture, strings.Join(errs[:min(len(errs), 6)], "\n  "))
			}
		}
	}
	t.Logf("%d layouts, %d intersect calls, %d differ", layouts, intersects, failed)
	if failed > 0 {
		t.Errorf("%d of %d layouts differ", failed, layouts)
	}
}

// TestPrepareLayoutForElk checks buildEdgeData: the edges at the start of
// the layout render, through applyElkEdgeRenderData, must have the values
// that upstream had after the prepare phase.
func TestPrepareLayoutForElk(t *testing.T) {
	calls := loadCalls(t)
	failed, edges := 0, 0
	for _, c := range calls {
		data := &ru.LayoutData{}
		for _, o := range c.entry {
			data.Edges = append(data.Edges, edgeFromJS(o))
		}
		applyElkEdgeRenderData(data, c.elkContext())
		var errs []string
		for i, want := range c.prepared {
			edges++
			got := data.Edges[i]
			check := func(name, gotValue string, gotDefined bool) {
				w := want.Get(name)
				wantValue, wantDefined := "", !js.IsNullish(w)
				if wantDefined {
					wantValue = js.ToString(w)
				}
				if gotValue != wantValue || gotDefined != wantDefined {
					errs = append(errs, fmt.Sprintf("edge %s %s = %q (defined %v), upstream %q (defined %v)", got.ID, name, gotValue, gotDefined, wantValue, wantDefined))
				}
			}
			// The structs cannot tell a string property that is the empty
			// string from one that is undefined, for the properties below.
			// See buildEdgeData. emptyIs checks such a property: the value
			// that the Go edge has where upstream has the empty string.
			emptyIs := func(name, gotValue, forEmpty string) {
				w, isString := want.Get(name).(string)
				if isString && w == "" {
					if gotValue != forEmpty {
						errs = append(errs, fmt.Sprintf("edge %s %s = %q, upstream the empty string", got.ID, name, gotValue))
					}
					return
				}
				check(name, gotValue, gotValue != "")
			}
			check("minlen", js.Str(got.Minlen), true)
			// Nothing reads text.
			emptyIs("text", got.Text, "")
			check("arrowhead", got.Arrowhead, true)
			// An arrow type that is the empty string is "none" in the
			// structs when the edge type has no arrow at that end. Both
			// mean no marker.
			wantStart, _ := want.Get("arrowTypeStart").(string)
			wantEnd, _ := want.Get("arrowTypeEnd").(string)
			if !(wantStart == "" && got.ArrowTypeStart != "" && isArrowMapStart(got.ArrowTypeStart)) {
				check("arrowTypeStart", got.ArrowTypeStart, true)
			}
			if !(wantEnd == "" && got.ArrowTypeEnd != "" && isArrowMapEnd(got.ArrowTypeEnd)) {
				check("arrowTypeEnd", got.ArrowTypeEnd, true)
			}
			check("thickness", got.Thickness, true)
			check("pattern", got.Pattern, true)
			check("classes", got.Classes, !got.ClassesUndefined)
			check("label", got.Label, got.HasLabel)
			check("labelpos", got.Labelpos, got.Labelpos != "")
			// Nothing reads arrowheadStyle. Where upstream has the empty
			// string, the structs have undefined, and the default applies.
			if w, isString := want.Get("arrowheadStyle").(string); !(isString && w == "") {
				check("arrowheadStyle", got.ArrowheadStyle, got.ArrowheadStyle != "")
			}
			// style is a string or an array upstream.
			switch w := want.Get("style").(type) {
			case string:
				if got.StyleUndefined || len(got.Style) != 1 || got.Style[0] != w {
					errs = append(errs, fmt.Sprintf("edge %s style = %q, upstream %q", got.ID, got.Style, w))
				}
			case []any:
				if got.StyleUndefined || js.StringifyJSON(strs(got.Style)) != js.StringifyJSON(w) {
					errs = append(errs, fmt.Sprintf("edge %s style = %q, upstream %s", got.ID, got.Style, js.StringifyJSON(w)))
				}
			default:
				errs = append(errs, fmt.Sprintf("edge %s: upstream style is %v", got.ID, w))
			}
			switch w := want.Get("labelStyle").(type) {
			case string:
				if !got.LabelStyleIsText || got.LabelStyleText != w {
					errs = append(errs, fmt.Sprintf("edge %s labelStyle = %q, upstream %q", got.ID, got.LabelStyle, w))
				}
			case []any:
				if got.LabelStyleIsText || got.LabelStyle == nil || js.StringifyJSON(strs(got.LabelStyle)) != js.StringifyJSON(w) {
					errs = append(errs, fmt.Sprintf("edge %s labelStyle = %q, upstream %s", got.ID, got.LabelStyle, js.StringifyJSON(w)))
				}
			default:
				errs = append(errs, fmt.Sprintf("edge %s: upstream labelStyle is %v", got.ID, w))
			}
			// The curve is a function upstream after the prepare phase.
			if c.preparedCurveTypes[i] != "function" || got.Curve != "" {
				errs = append(errs, fmt.Sprintf("edge %s curve = %q, upstream a %s", got.ID, got.Curve, c.preparedCurveTypes[i]))
			}
		}
		if len(errs) > 0 {
			failed++
			if failed <= 5 {
				t.Errorf("%s:\n  %s", c.fixture, strings.Join(errs[:min(len(errs), 6)], "\n  "))
			}
		}
	}
	t.Logf("%d layouts, %d edges, %d layouts differ", len(calls), edges, failed)
	if failed > 0 {
		t.Errorf("%d of %d layouts differ", failed, len(calls))
	}
}

// isArrowMapStart reports whether the arrow type is a start value of
// ARROW_MAP, and isArrowMapEnd whether it is an end value.
func isArrowMapStart(arrowType string) bool {
	for _, pair := range ARROW_MAP {
		if pair[0] == arrowType {
			return true
		}
	}
	return false
}

func isArrowMapEnd(arrowType string) bool {
	for _, pair := range ARROW_MAP {
		if pair[1] == arrowType {
			return true
		}
	}
	return false
}

// reHash matches the member "$H" that elkjs adds to each node object: a
// Java identity hash code, which depends on the layouts that the page ran
// before. internal/elk removes it.
var reHash = regexp.MustCompile(`,"\$H":-?\d+`)

// fullRun reports whether the tests that run the ELK engine take every
// case. A layout in the engine takes about 100 ms, so by default they take
// every eighth corpus fixture. Set MMDG_ELK_FULL=1 for all of them.
func fullRun() bool { return os.Getenv("MMDG_ELK_FULL") != "" }

// sample returns the calls that a test with the ELK engine runs: every
// eighth corpus fixture, and every eighth random case when thinRandom is
// set, or all of them for a full run.
func sample(calls []*elkCall, thinRandom bool) []*elkCall {
	if fullRun() {
		return calls
	}
	var out []*elkCall
	fixtures, random := 0, 0
	for _, c := range calls {
		if c.isRandom() {
			if !thinRandom || random%8 == 0 {
				out = append(out, c)
			}
			random++
		} else {
			if fixtures%8 == 0 {
				out = append(out, c)
			}
			fixtures++
		}
	}
	return out
}

// forEachParallel runs fn for each call on several goroutines. The ELK
// engine keeps a pool of JavaScript runtimes.
func forEachParallel(calls []*elkCall, fn func(c *elkCall)) {
	var wg sync.WaitGroup
	work := make(chan *elkCall)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range work {
				fn(c)
			}
		}()
	}
	for _, c := range calls {
		work <- c
	}
	close(work)
	wg.Wait()
}

// testEngine is the ELK engine of the tests. Its time limit is longer than
// the 30 s of the default engine: a layout of 100 ms can take seconds when
// the machine runs other work.
var testEngine = elkjs.New(elkjs.Options{Timeout: 10 * time.Minute})

// elkErrorMessage returns the message of an error of the ELK engine.
func elkErrorMessage(err error) string {
	var e *elkjs.Error
	if errors.As(err, &e) {
		return e.Message
	}
	return err.Error()
}

// TestElkEngine is layer 2: for the recorded inputs, the ELK engine of
// internal/elk must return the output that elkjs returned in Chromium (for
// a corpus fixture) or in node (for a random case), or fail with the same
// message. For a random case, which has no recorded output, the output of
// the engine is then applied to the layout data, and the result must be
// the result of upstream.
func TestElkEngine(t *testing.T) {
	calls := sample(loadCalls(t), false)
	var failed, rejected atomic.Int64
	var mu sync.Mutex
	report := func(c *elkCall, format string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		if failed.Add(1) <= 5 {
			t.Errorf("%s: %s", c.fixture, fmt.Sprintf(format, args...))
		}
	}
	forEachParallel(calls, func(c *elkCall) {
		got, err := testEngine.Layout(context.Background(), []byte(c.in))
		switch {
		case c.err != "":
			rejected.Add(1)
			if err == nil {
				report(c, "no error, upstream %q", c.err)
			} else if msg := elkErrorMessage(err); msg != c.err {
				report(c, "error %q, upstream %q", msg, c.err)
			}
		case err != nil:
			report(c, "%v", err)
		case c.out != "":
			if want := reHash.ReplaceAllString(c.out, ""); string(got) != want {
				report(c, "ELK output differs %s", firstDiff(string(got), want))
			}
		default:
			if sum := fmt.Sprintf("%x", sha256.Sum256(got)); sum != c.outHash {
				report(c, "ELK output has the hash %s, upstream %s", sum, c.outHash)
			} else if errs := applyRecorded(c, got); len(errs) > 0 {
				report(c, "\n  %s", strings.Join(errs[:min(len(errs), 6)], "\n  "))
			}
		}
	})
	t.Logf("%d layouts (%d that elkjs rejects), %d differ", len(calls), rejected.Load(), failed.Load())
	if failed.Load() > 0 {
		t.Errorf("%d of %d layouts differ", failed.Load(), len(calls))
	}
}

// TestRunElkLayoutCore runs the layout as the Go renderer does, without a
// DOM: the recorded measured layout data as structs of internal/ru, through
// runElkLayoutCore with the ELK engine of internal/elk. The node positions
// and the edge points must be the ones of upstream, bit for bit.
func TestRunElkLayoutCore(t *testing.T) {
	calls := sample(loadCalls(t), true)
	defer func(orig func([]byte) ([]byte, error)) { elkLayout = orig }(elkLayout)
	elkLayout = func(graphJSON []byte) ([]byte, error) {
		return testEngine.Layout(context.Background(), graphJSON)
	}
	var failed atomic.Int64
	var mu sync.Mutex
	forEachParallel(calls, func(c *elkCall) {
		// The index of each layout node in the ELK graph is the same for
		// the objects and for the structs.
		rp := newReplay(c, buildElkGraphFromLayoutData(c.objects(), c.elkContext()))
		data := c.layoutData(rp.intersect)
		rc := &rugraph.RenderContext{
			Ctx:     &render.Context{Config: c.getConfig},
			Options: rugraph.RenderOptions{Algorithm: c.algorithm},
		}
		_, err := runElkLayoutCore(data, rc)
		var errs []string
		switch {
		case c.err != "":
			if err == nil {
				errs = append(errs, fmt.Sprintf("no error, upstream %q", c.err))
			} else if msg := elkErrorMessage(err); msg != c.err {
				errs = append(errs, fmt.Sprintf("error %q, upstream %q", msg, c.err))
			}
		case err != nil:
			errs = append(errs, err.Error())
		default:
			errs = append(rp.errors(), compareResult(c, data)...)
		}
		if len(errs) == 0 {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if failed.Add(1) <= 5 {
			t.Errorf("%s:\n  %s", c.fixture, strings.Join(errs[:min(len(errs), 6)], "\n  "))
		}
	})
	t.Logf("%d layouts, %d differ", len(calls), failed.Load())
	if failed.Load() > 0 {
		t.Errorf("%d of %d layouts differ", failed.Load(), len(calls))
	}
}
