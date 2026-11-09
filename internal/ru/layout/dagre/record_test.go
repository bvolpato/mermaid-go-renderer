package dagre

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"

	dagrelib "github.com/bvolpato/mermaid-go-renderer/internal/dagre"
	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru"
	"github.com/bvolpato/mermaid-go-renderer/internal/ru/edges"
	rugraph "github.com/bvolpato/mermaid-go-renderer/internal/ru/graph"
)

// The record comes from tools/reference/gen-dagre-layout.mjs. It holds what
// the dagre layout of mermaid 12.2.0 did during real renders in Chromium:
// the layout data that it got, the graphs that it built, the sizes that the
// DOM functions measured, the graphs before and after each layout of dagre,
// and the values that it handed to positionNode, insertCluster, insertEdge
// and positionEdgeLabel. See tools/reference/probes/dagre-layout-spy.js.
//
// TestLayoutRecord runs the Go layout on the recorded layout data. The DOM
// functions are stubs that return the recorded sizes. Everything else must
// be equal to the record, numbers bit for bit.

type recordFile struct {
	Fixtures []map[string]any
}

func loadJSONGz(t testing.TB, path string, v any) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.NewDecoder(zr).Decode(v); err != nil {
		t.Fatal(err)
	}
}

// ---- the encoding of probes/dagre-layout-spy.js ----

// encNum is num(): a number that JSON cannot hold is { $n: text }.
func encNum(v float64) any {
	if math.IsNaN(v) || math.IsInf(v, 0) || (v == 0 && math.Signbit(v)) {
		text := js.Str(v)
		if v == 0 {
			text = "-0"
		}
		return map[string]any{"$n": text}
	}
	return v
}

// encDyn is dyn() for a number whose NaN stands for undefined.
func encDyn(v float64) any {
	if ru.IsUndef(v) {
		return map[string]any{"$u": 1.0}
	}
	return encNum(v)
}

// encAny is dyn() for a dynamic value.
func encAny(v any) any {
	switch x := v.(type) {
	case nil:
		return map[string]any{"$u": 1.0}
	case float64:
		return encNum(x)
	}
	return v
}

func putStr(o map[string]any, k, v string) {
	if v != "" {
		o[k] = v
	}
}

func putNum(o map[string]any, k string, v float64) {
	if !ru.IsUndef(v) {
		o[k] = encNum(v)
	}
}

// decDyn reads a value that dyn() wrote: undefined is NaN.
func decDyn(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case map[string]any:
		if _, ok := x["$u"]; ok {
			return ru.Undef
		}
		switch x["$n"] {
		case "NaN":
			return math.NaN()
		case "Infinity":
			return math.Inf(1)
		case "-Infinity":
			return math.Inf(-1)
		case "-0":
			return math.Copysign(0, -1)
		}
	case nil:
		return ru.Undef
	}
	panic(fmt.Sprintf("bad number %v", v))
}

// decNum reads an optional number property that put() wrote.
func decNum(m map[string]any, k string) float64 {
	v, ok := m[k]
	if !ok {
		return ru.Undef
	}
	return decDyn(v)
}

func decStr(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func encPos(n *ru.Node) []any {
	return []any{n.ID, encDyn(n.X), encDyn(n.Y), encDyn(n.Width), encDyn(n.Height), encDyn(n.Diff), n.ClusterNode}
}

func encGeom(n *ru.Node) any {
	if n == nil {
		return nil
	}
	return []any{encDyn(n.X), encDyn(n.Y), encDyn(n.Width), encDyn(n.Height)}
}

// projectNode is projectNode(). With attrs, d is what the Go layout hands
// to dagre. Without, it is what dagre would read from the node.
func projectNode(n *Node, deep, attrs bool) map[string]any {
	o := map[string]any{}
	putStr(o, "id", n.ID)
	putStr(o, "parentId", n.ParentID)
	putStr(o, "dir", n.Dir)
	putStr(o, "domId", n.DomID)
	putStr(o, "shape", n.Shape)
	putStr(o, "label", n.Label)
	putStr(o, "labelStyle", n.LabelStyle)
	if style, ok := n.Get("style").(string); ok {
		putStr(o, "style", style)
	}
	putNum(o, "width", n.Width)
	putNum(o, "height", n.Height)
	putNum(o, "x", n.X)
	putNum(o, "y", n.Y)
	putNum(o, "diff", n.Diff)
	putNum(o, "padding", n.Padding)
	if n.IsGroup {
		o["isGroup"] = true
	}
	if n.ClusterNode {
		o["clusterNode"] = true
	}
	if n.ClusterData != nil {
		o["clusterData"] = n.ClusterData.ID
	}
	if attrs {
		o["d"] = []any{encNum(n.attrs.Width), encNum(n.attrs.Height)}
	} else {
		o["d"] = []any{encNum(absentZero(n.Width)), encNum(absentZero(n.Height))}
	}
	if deep && n.Graph != nil {
		o["graph"] = projectGraph(n.Graph, deep, attrs)
	}
	return o
}

func projectRuEdge(e *ru.Edge) map[string]any {
	o := map[string]any{}
	putStr(o, "id", e.ID)
	putStr(o, "start", e.Start)
	putStr(o, "end", e.End)
	putStr(o, "label", e.Label)
	putStr(o, "labelpos", e.Labelpos)
	putStr(o, "fromCluster", e.FromCluster)
	putStr(o, "toCluster", e.ToCluster)
	putStr(o, "arrowTypeStart", e.ArrowTypeStart)
	putStr(o, "arrowTypeEnd", e.ArrowTypeEnd)
	putStr(o, "startLabelLeft", e.StartLabelLeft)
	putStr(o, "startLabelRight", e.StartLabelRight)
	putStr(o, "endLabelLeft", e.EndLabelLeft)
	putStr(o, "endLabelRight", e.EndLabelRight)
	putNum(o, "minlen", e.Minlen)
	putNum(o, "width", e.Width)
	putNum(o, "height", e.Height)
	putNum(o, "x", e.X)
	putNum(o, "y", e.Y)
	if e.Points != nil {
		o["points"] = encPoints(e.Points)
	}
	return o
}

func encPoints(points []ru.Point) []any {
	out := make([]any, len(points))
	for i, p := range points {
		out[i] = []any{encNum(p.X), encNum(p.Y)}
	}
	return out
}

func numOr(n dagrelib.Num, def float64) any {
	if n.Set {
		return encNum(n.V)
	}
	return def
}

// projectEdge is projectEdge(). With attrs, d is what the Go layout hands
// to dagre.
func projectEdge(e *Edge, attrs bool) map[string]any {
	o := projectRuEdge(e.Edge)
	if e.SelfLoop != nil {
		o["selfLoop"] = []any{idOrNull(e.SelfLoop.ID), float64(e.SelfLoop.Order)}
	}
	if e.OriginalEdge != nil {
		o["originalEdge"] = idOrNull(e.OriginalEdge.ID)
	}
	if attrs {
		labelpos := e.attrs.Labelpos
		if labelpos == "" {
			labelpos = "r"
		}
		o["d"] = []any{numOr(e.attrs.Minlen, 1), numOr(e.attrs.Weight, 1), encNum(e.attrs.Width), encNum(e.attrs.Height), numOr(e.attrs.Labeloffset, 10), labelpos}
	} else {
		o["d"] = ruEdgeD(e.Edge)
	}
	return o
}

// ruEdgeD is what dagre would read from the edge.
func ruEdgeD(e *ru.Edge) []any {
	minlen := 1.0
	if !ru.IsUndef(e.Minlen) {
		minlen = e.Minlen
	}
	labelpos := e.Labelpos
	if labelpos == "" {
		labelpos = "r"
	}
	return []any{encNum(minlen), numOr(extraNum(e.Extra, "weight"), 1), encNum(absentZero(e.Width)), encNum(absentZero(e.Height)), numOr(extraNum(e.Extra, "labeloffset"), 10), labelpos}
}

func projectGraphLabel(l *GraphLabel) any {
	if l == nil {
		return nil
	}
	o := map[string]any{}
	putStr(o, "rankdir", l.Rankdir)
	o["nodesep"] = encAny(l.Nodesep)
	o["ranksep"] = encAny(l.Ranksep)
	putNum(o, "marginx", l.Marginx)
	putNum(o, "marginy", l.Marginy)
	putNum(o, "width", l.Width)
	putNum(o, "height", l.Height)
	return o
}

func projectGraph(g *graphlib.Graph, deep, attrs bool) map[string]any {
	nodes := []any{}
	for _, v := range g.Nodes() {
		o := map[string]any{"v": v}
		if parent, ok := g.Parent(v); ok {
			o["parent"] = parent
		}
		if value := nodeOf(g, v); value != nil {
			o["value"] = projectNode(value, deep, attrs)
		}
		nodes = append(nodes, o)
	}
	edgeList := []any{}
	for _, e := range g.Edges() {
		o := map[string]any{"v": e.V, "w": e.W}
		if e.Name != "" {
			o["name"] = e.Name
		}
		if value := edgeOf(g, e); value != nil {
			o["value"] = projectEdge(value, attrs)
		}
		edgeList = append(edgeList, o)
	}
	return map[string]any{"value": projectGraphLabel(graphOf(g)), "nodes": nodes, "edges": edgeList}
}

func projectLayoutResult(g *graphlib.Graph) map[string]any {
	label := graphOf(g)
	nodes := []any{}
	for _, v := range g.Nodes() {
		n := nodeOf(g, v)
		if n == nil {
			nodes = append(nodes, nil)
			continue
		}
		nodes = append(nodes, []any{encDyn(n.X), encDyn(n.Y), encDyn(n.Width), encDyn(n.Height)})
	}
	edgeList := []any{}
	for _, e := range g.Edges() {
		l := edgeOf(g, e)
		edgeList = append(edgeList, []any{encPoints(l.Points), encDyn(l.X), encDyn(l.Y)})
	}
	return map[string]any{"graph": []any{encDyn(label.Width), encDyn(label.Height)}, "nodes": nodes, "edges": edgeList}
}

func projectClusterDb(st *state) []any {
	out := []any{}
	for _, key := range st.clusterDb.Keys() {
		entry := st.clusterDb.Get(key)
		o := map[string]any{"key": key}
		putStr(o, "id", entry.ID)
		if entry.ExternalConnections {
			o["externalConnections"] = true
		}
		if entry.ClusterData != nil {
			o["clusterData"] = entry.ClusterData.ID
		}
		if entry.Node != nil {
			o["node"] = encGeom(entry.Node.Node)
		}
		out = append(out, o)
	}
	return out
}

// ---- the comparison ----

// diffJSON compares two JSON values. Numbers are compared bit for bit.
func diffJSON(path string, got, want any, out *[]string) {
	if len(*out) >= 8 {
		return
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want)))
			return
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
			if !gok || !wok {
				*out = append(*out, fmt.Sprintf("%s.%s: got %s, want %s", path, k, showOpt(gv, gok), showOpt(wv, wok)))
				continue
			}
			diffJSON(path+"."+k, gv, wv, out)
		}
	case []any:
		g, ok := got.([]any)
		if !ok {
			*out = append(*out, fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want)))
			return
		}
		if len(g) != len(w) {
			*out = append(*out, fmt.Sprintf("%s: got %d elements, want %d: got %s, want %s", path, len(g), len(w), show(got), show(want)))
			return
		}
		for i := range w {
			diffJSON(fmt.Sprintf("%s[%d]", path, i), g[i], w[i], out)
		}
	case float64:
		g, ok := got.(float64)
		if !ok || math.Float64bits(g) != math.Float64bits(w) {
			*out = append(*out, fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want)))
		}
	default:
		if got != want {
			*out = append(*out, fmt.Sprintf("%s: got %s, want %s", path, show(got), show(want)))
		}
	}
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

func showOpt(v any, ok bool) string {
	if !ok {
		return "(absent)"
	}
	return show(v)
}

// without returns a copy of a JSON object without some keys.
func without(v any, keys ...string) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	out := make(map[string]any, len(m))
	for k, e := range m {
		out[k] = e
	}
	for _, k := range keys {
		delete(out, k)
	}
	return out
}

// The DOM functions change properties of a node that the layout does not
// read: insertNode replaces the shape "rect", and shapes rewrite labels.
// The stubs do not, so these properties are compared only before the
// measure phase.
var measuredNodeKeys = []string{"shape", "label", "labelStyle", "domId", "style", "padding"}

// stripGraph removes keys from the node and edge values of a graph.
func stripGraph(graph any, nodeKeys, edgeKeys []string) any {
	g, ok := graph.(map[string]any)
	if !ok {
		return graph
	}
	out := map[string]any{"value": g["value"]}
	strip := func(list any, keys []string) []any {
		items, _ := list.([]any)
		res := make([]any, len(items))
		for i, item := range items {
			m := without(item).(map[string]any)
			if value, ok := m["value"]; ok {
				m["value"] = without(value, keys...)
			}
			res[i] = m
		}
		return res
	}
	out["nodes"] = strip(g["nodes"], nodeKeys)
	out["edges"] = strip(g["edges"], edgeKeys)
	return out
}

// ---- the replay ----

// graphGot is what the Go layout did for one graph.
type graphGot struct {
	idCounter float64
	before    any
	after     any
	position  []any
	edges     []any
	nodeCalls int
	edgeCalls int
	layouts   int
}

type replay struct {
	t      *testing.T
	fx     map[string]any
	st     *state
	graphs map[string]*graphGot
	// clusters are the recorded insertCluster calls by node id, in order.
	clusters map[string][]map[string]any

	adjustBefore, adjustAfter any
	clusterDb0                any
	descendants, parents      any
	layoutOrder               []any

	problems []string
}

func (r *replay) problem(format string, args ...any) {
	if len(r.problems) < 12 {
		r.problems = append(r.problems, fmt.Sprintf(format, args...))
	}
}

func (r *replay) got(key string) *graphGot {
	g := r.graphs[key]
	if g == nil {
		g = &graphGot{position: []any{}, edges: []any{}}
		r.graphs[key] = g
	}
	return g
}

// recGraph returns the record of a graph.
func (r *replay) recGraph(key string) map[string]any {
	graphs, _ := r.fx["graphs"].(map[string]any)
	g, _ := graphs[key].(map[string]any)
	return g
}

func entriesOf(m *js.Map[string, []string]) []any {
	out := []any{}
	for _, k := range m.Keys() {
		list := []any{}
		for _, v := range m.Get(k) {
			list = append(list, v)
		}
		out = append(out, []any{k, list})
	}
	return out
}

func (r *replay) ops() *ops {
	return &ops{
		createLayoutElementGroups: func(*scope, *dom.Node, rugraph.CreateLayoutElementGroupsOptions) *rugraph.LayoutElementGroups {
			return &rugraph.LayoutElementGroups{}
		},
		insertMeasuredNode: func(sc *scope, _ *dom.Node, node *ru.Node, renderOptions *ru.ShapeRenderOptions) error {
			g := r.got(sc.parentCluster)
			calls, _ := r.recGraph(sc.parentCluster)["nodes"].([]any)
			i := g.nodeCalls
			g.nodeCalls++
			if i >= len(calls) {
				r.problem("graph %q: insertMeasuredNode call %d for %q was not recorded", sc.parentCluster, i, node.ID)
				return nil
			}
			call := calls[i].(map[string]any)
			if decStr(call, "id") != node.ID {
				r.problem("graph %q: insertMeasuredNode call %d is for %q, want %q", sc.parentCluster, i, node.ID, decStr(call, "id"))
				return nil
			}
			if dir, _ := call["dir"].(string); dir != renderOptions.Dir {
				r.problem("graph %q: insertMeasuredNode(%q) has dir %q, want %q", sc.parentCluster, node.ID, renderOptions.Dir, dir)
			}
			size := call["size"].([]any)
			node.Width = decDyn(size[0])
			node.Height = decDyn(size[1])
			return nil
		},
		insertEdgeLabel: func(sc *scope, _ *dom.Node, edge *ru.Edge) error {
			g := r.got(sc.parentCluster)
			calls, _ := r.recGraph(sc.parentCluster)["edgeLabels"].([]any)
			i := g.edgeCalls
			g.edgeCalls++
			if i >= len(calls) {
				r.problem("graph %q: insertEdgeLabel call %d for %q was not recorded", sc.parentCluster, i, edge.ID)
				return nil
			}
			call := calls[i].(map[string]any)
			if decStr(call, "id") != edge.ID {
				r.problem("graph %q: insertEdgeLabel call %d is for %q, want %q", sc.parentCluster, i, edge.ID, decStr(call, "id"))
				return nil
			}
			// The label and the four terminal labels that are drawn.
			texts := []any{idOrNull(edge.Label), idOrNull(edge.StartLabelLeft), idOrNull(edge.StartLabelRight), idOrNull(edge.EndLabelLeft), idOrNull(edge.EndLabelRight)}
			var out []string
			diffJSON(fmt.Sprintf("graph %q: insertEdgeLabel(%q) texts", sc.parentCluster, edge.ID), texts, call["texts"], &out)
			r.problems = append(r.problems, out...)
			size := call["size"].([]any)
			edge.Width = decDyn(size[0])
			edge.Height = decDyn(size[1])
			// insertEdgeLabel replaces labelStyle with a string.
			edge.LabelStyle = nil
			edge.LabelStyleText = ""
			edge.LabelStyleIsText = true
			return nil
		},
		updateNodeBounds: func(_ *scope, node *ru.Node, _ *dom.Node) {
			bounds, _ := r.fx["bounds"].(map[string]any)
			size, ok := bounds[node.ID].([]any)
			if !ok {
				r.problem("updateNodeBounds(%q) was not recorded", node.ID)
				return
			}
			node.Width = decDyn(size[0])
			node.Height = decDyn(size[1])
		},
		setNodeElem: func(*scope, *dom.Node, *ru.Node) {},
		insertCluster: func(_ *scope, _ *dom.Node, node *ru.Node) error {
			r.insertCluster(node)
			return nil
		},
		positionNode: func(sc *scope, node *ru.Node) {
			g := r.got(sc.parentCluster)
			g.position = append(g.position, encPos(node))
		},
		insertEdge: func(sc *scope, _ *dom.Node, edge *ru.Edge, clusterDb edges.ClusterDb, _ string, startNode, endNode *ru.Node, _ string) (*edges.Paths, error) {
			g := r.got(sc.parentCluster)
			entry := map[string]any{"edge": projectRuEdge(edge), "startNode": nil, "endNode": nil}
			if startNode != nil {
				entry["startNode"] = startNode.ID
			}
			if endNode != nil {
				entry["endNode"] = endNode.ID
			}
			cluster := func(id string) any {
				bounds, ok := clusterDb(id)
				if !ok {
					return nil
				}
				return []any{encDyn(bounds.X), encDyn(bounds.Y), encDyn(bounds.Width), encDyn(bounds.Height)}
			}
			if edge.ToCluster != "" {
				entry["toCluster"] = cluster(edge.ToCluster)
			}
			if edge.FromCluster != "" {
				entry["fromCluster"] = cluster(edge.FromCluster)
			}
			g.edges = append(g.edges, entry)
			if err := clusterLookups(edge, clusterDb); err != nil {
				return nil, err
			}
			return &edges.Paths{OriginalPath: edge.Points, HasOriginalPath: true}, nil
		},
		positionEdgeLabel: func(sc *scope, edge *ru.Edge, _ *edges.Paths) error {
			g := r.got(sc.parentCluster)
			g.edges[len(g.edges)-1].(map[string]any)["label"] = []any{encDyn(edge.X), encDyn(edge.Y)}
			return nil
		},
	}
}

// clusterLookups does what insertEdge does with clusterDb: it reads the
// node of the cluster that the edge is cut at. The error is the TypeError
// of upstream for a cluster that clusterDb does not have (edges.InsertEdge
// returns it too).
func clusterLookups(edge *ru.Edge, clusterDb edges.ClusterDb) error {
	for _, id := range []string{edge.ToCluster, edge.FromCluster} {
		if id == "" {
			continue
		}
		if _, ok := clusterDb(id); !ok {
			return errors.New("Cannot read properties of undefined (reading 'node')")
		}
	}
	return nil
}

// insertCluster is the stub of insertCluster: the node must be the one that
// upstream drew, and it gets what the cluster shape of upstream wrote.
func (r *replay) insertCluster(node *ru.Node) {
	calls := r.clusters[node.ID]
	if len(calls) == 0 {
		r.problem("insertCluster(%q) was not recorded", node.ID)
		return
	}
	call := calls[0]
	r.clusters[node.ID] = calls[1:]
	var out []string
	diffJSON("insertCluster("+node.ID+")", encGeom(node), call["pre"], &out)
	r.problems = append(r.problems, out...)
	post, ok := call["post"].([]any)
	if !ok {
		return
	}
	node.Width = decDyn(post[0])
	node.Height = decDyn(post[1])
	node.Diff = decDyn(post[2])
	node.OffsetX = decDyn(post[3])
	node.OffsetY = decDyn(post[4])
}

func (r *replay) trace() *trace {
	return &trace{
		beforeAdjust: func(g *graphlib.Graph) {
			r.adjustBefore = projectGraph(g, false, false)
		},
		afterAdjust: func(g *graphlib.Graph) {
			r.adjustAfter = projectGraph(g, true, false)
			r.clusterDb0 = projectClusterDb(r.st)
			r.descendants = entriesOf(r.st.descendants)
			parents := []any{}
			for _, k := range r.st.parents.Keys() {
				parents = append(parents, []any{k, r.st.parents.Get(k)})
			}
			r.parents = parents
		},
		beforeLayout: func(sc *scope, g *graphlib.Graph) {
			got := r.got(sc.parentCluster)
			got.layouts++
			got.idCounter = float64(r.st.dagre.IDCounter)
			got.before = projectGraph(g, false, true)
			r.layoutOrder = append(r.layoutOrder, sc.parentCluster)
		},
		afterLayout: func(sc *scope, g *graphlib.Graph) {
			r.got(sc.parentCluster).after = projectLayoutResult(g)
		},
	}
}

// inputData builds the layout data and the config of a recorded fixture.
func inputData(input map[string]any) (*ru.LayoutData, *js.Obj[any]) {
	data := &ru.LayoutData{
		Type:            decStr(input, "type"),
		DiagramID:       decStr(input, "diagramId"),
		LayoutAlgorithm: decStr(input, "layoutAlgorithm"),
	}
	data.Direction, _ = input["direction"].(string)
	// A spacing that is a string: the number goes into the field and the
	// JavaScript value into Extra, as the diagram packages do.
	spacingOf := func(key string) float64 {
		if text, ok := input[key].(string); ok {
			if data.Extra == nil {
				data.Extra = map[string]any{}
			}
			data.Extra[key] = text
			return js.ToNumber(text)
		}
		return decDyn(input[key])
	}
	data.NodeSpacing = spacingOf("nodeSpacing")
	data.RankSpacing = spacingOf("rankSpacing")

	config := js.NewObj[any]()
	flowchart := js.NewObj[any]()
	dataConfig := input["dataConfig"].(map[string]any)
	set := func(o *js.Obj[any], key string, v any) {
		switch x := v.(type) {
		case float64, string, bool:
			o.Set(key, x)
		case map[string]any:
			if _, undef := x["$u"]; !undef {
				o.Set(key, decDyn(x))
			}
		}
	}
	set(config, "nodeSpacing", dataConfig["nodeSpacing"])
	set(config, "rankSpacing", dataConfig["rankSpacing"])
	set(flowchart, "nodeSpacing", dataConfig["flowchartNodeSpacing"])
	set(flowchart, "rankSpacing", dataConfig["flowchartRankSpacing"])
	if margin, ok := input["subGraphTitleMargin"].(map[string]any); ok {
		m := js.NewObj[any]()
		for _, k := range []string{"top", "bottom"} {
			if v, ok := margin[k]; ok {
				m.Set(k, v)
			}
		}
		flowchart.Set("subGraphTitleMargin", m)
	}
	config.Set("flowchart", flowchart)
	data.Config = config

	for _, item := range input["nodes"].([]any) {
		m := item.(map[string]any)
		node := ru.NewNode(decStr(m, "id"))
		node.ParentID = decStr(m, "parentId")
		node.Dir = decStr(m, "dir")
		node.DomID = decStr(m, "domId")
		node.Shape = decStr(m, "shape")
		node.Label = decStr(m, "label")
		_, node.HasLabel = m["hasLabel"]
		node.LabelStyle = decStr(m, "labelStyle")
		if style, ok := m["style"].(string); ok {
			node.Set("style", style)
		}
		node.Width = decNum(m, "width")
		node.Height = decNum(m, "height")
		node.X = decNum(m, "x")
		node.Y = decNum(m, "y")
		node.Diff = decNum(m, "diff")
		node.Padding = decNum(m, "padding")
		_, node.IsGroup = m["isGroup"]
		data.Nodes = append(data.Nodes, node)
	}
	for _, item := range input["edges"].([]any) {
		edge := edgeFromJSON(item.(map[string]any))
		data.Edges = append(data.Edges, edge)
	}
	return data, config
}

// edgeFromJSON builds an edge from the properties that projectEdge of the
// spy writes.
func edgeFromJSON(m map[string]any) *ru.Edge {
	edge := ru.NewEdge(decStr(m, "id"))
	edge.Start = decStr(m, "start")
	edge.End = decStr(m, "end")
	edge.Label = decStr(m, "label")
	if _, ok := m["hasLabel"]; ok {
		edge.HasLabel = true
	} else if _, ok := m["label"]; ok {
		edge.HasLabel = true
	}
	edge.Labelpos = decStr(m, "labelpos")
	edge.FromCluster = decStr(m, "fromCluster")
	edge.ToCluster = decStr(m, "toCluster")
	edge.ArrowTypeStart = decStr(m, "arrowTypeStart")
	edge.ArrowTypeEnd = decStr(m, "arrowTypeEnd")
	edge.StartLabelLeft = decStr(m, "startLabelLeft")
	edge.StartLabelRight = decStr(m, "startLabelRight")
	edge.EndLabelLeft = decStr(m, "endLabelLeft")
	edge.EndLabelRight = decStr(m, "endLabelRight")
	edge.Minlen = decNum(m, "minlen")
	edge.Width = decNum(m, "width")
	edge.Height = decNum(m, "height")
	edge.X = decNum(m, "x")
	edge.Y = decNum(m, "y")
	if points, ok := m["points"].([]any); ok {
		edge.Points = []ru.Point{}
		for _, p := range points {
			xy := p.([]any)
			edge.Points = append(edge.Points, ru.Point{X: decDyn(xy[0]), Y: decDyn(xy[1])})
		}
	}
	for _, k := range []string{"weight", "labeloffset"} {
		if v, ok := m[k]; ok {
			edge.Set(k, decDyn(v))
		}
	}
	return edge
}

// counters of TestLayoutRecord.
type recordStats struct {
	fixtures, graphs, nodes, edgeList, clusters int
	nested, selfLoops, orderDiffers             int
}

// replayFixture runs the Go layout on a recorded fixture. It returns the
// differences from the record, and the error of the Go layout, if any.
// After an error only the phases before it are compared: upstream starts
// the work for all nodes of a graph together, so what it did before it
// failed is not what a sequential run did.
func replayFixture(t *testing.T, fx map[string]any, stats *recordStats) ([]string, error) {
	input := fx["input"].(map[string]any)
	data, config := inputData(input)
	recGraphs := fx["graphs"].(map[string]any)

	r := &replay{t: t, fx: fx, graphs: map[string]*graphGot{}, clusters: map[string][]map[string]any{}}
	for _, item := range fx["clusters"].([]any) {
		call := item.(map[string]any)
		id := decStr(call, "id")
		r.clusters[id] = append(r.clusters[id], call)
	}
	// The page of the recorder rendered other diagrams before this one, so
	// the uniqueId counter of lodash does not start at 0. The first layout
	// of the render has the lowest value.
	idCounter := math.Inf(1)
	for _, g := range recGraphs {
		if c, ok := g.(map[string]any)["idCounter"].(float64); ok {
			idCounter = math.Min(idCounter, c)
		}
	}
	if math.IsInf(idCounter, 1) {
		idCounter = 0
	}
	st := &state{
		clusterDb:   js.NewMap[string, *ClusterEntry](),
		descendants: js.NewMap[string, []string](),
		parents:     js.NewMap[string, string](),
		dagre:       &dagrelib.State{IDCounter: int(idCounter)},
	}
	r.st = st
	st.ops = r.ops()
	st.trace = r.trace()
	ctx := &render.Context{ID: data.DiagramID, Config: config}
	st.ctx = ctx
	ctx.Value(stateKey, func() any { return st })

	check := func(what string, got, want any) {
		var out []string
		diffJSON(what, got, want, &out)
		r.problems = append(r.problems, out...)
	}

	// clearLayoutRenderState, then the phases of createCommonLayoutRenderer.
	Clear(ctx)
	rc := &rugraph.RenderContext{Ctx: ctx}
	prepared, err := PrepareLayoutForDagre(ctx, data)
	if err != nil {
		return r.problems, fmt.Errorf("prepareLayoutForDagre: %w", err)
	}
	rc.PreparedLayout = prepared
	check("adjustBefore", r.adjustBefore, fx["adjustBefore"])
	check("adjustAfter", r.adjustAfter, fx["adjustAfter"])
	check("clusterDb0", r.clusterDb0, fx["clusterDb0"])
	check("descendants", r.descendants, fx["descendants"])
	check("parents", r.parents, fx["parents"])
	if len(r.problems) > 0 {
		return r.problems, nil
	}

	measure, err := MeasureDagreLayout(data, rc)
	if err != nil {
		return r.problems, err
	}
	if _, err := RunDagreLayoutCore(data, rc); err != nil {
		return r.problems, err
	}

	// Every graph: what dagre got, what it returned, and what the paint of
	// a nested graph handed to the DOM functions.
	keys := make([]string, 0, len(recGraphs))
	for key := range recGraphs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		want := recGraphs[key].(map[string]any)
		got := r.graphs[key]
		name := fmt.Sprintf("graph %q", key)
		if got == nil {
			r.problem("%s: the Go layout has no such graph", name)
			continue
		}
		stats.graphs++
		if got.layouts != 1 {
			r.problem("%s: %d layouts, want 1", name, got.layouts)
		}
		if calls, _ := want["nodes"].([]any); got.nodeCalls != len(calls) {
			r.problem("%s: %d insertMeasuredNode calls, want %d", name, got.nodeCalls, len(calls))
		}
		if calls, _ := want["edgeLabels"].([]any); got.edgeCalls != len(calls) {
			r.problem("%s: %d insertEdgeLabel calls, want %d", name, got.edgeCalls, len(calls))
		}
		stats.nodes += got.nodeCalls
		stats.edgeList += got.edgeCalls
		check(name+".before", stripGraph(got.before, measuredNodeKeys, nil), stripGraph(want["before"], measuredNodeKeys, nil))
		check(name+".after", got.after, want["after"])
		check(name+".position", got.position, want["position"])
		// The stub of insertEdge gets the ru.Edge, which has no selfLoop
		// and no originalEdge.
		wantEdges := []any{}
		for _, item := range want["edges"].([]any) {
			entry := without(item).(map[string]any)
			entry["edge"] = without(entry["edge"], "selfLoop", "originalEdge")
			wantEdges = append(wantEdges, entry)
		}
		check(name+".edges", got.edges, wantEdges)
		if got.idCounter != want["idCounter"] {
			stats.orderDiffers++
		}
	}
	for key := range r.graphs {
		if _, ok := recGraphs[key]; !ok {
			r.problem("graph %q: upstream has no such graph", key)
		}
	}
	if len(recGraphs) > 1 {
		stats.nested++
	}

	// The layout data after the layout phase.
	coreNodes := []any{}
	for _, node := range data.Nodes {
		coreNodes = append(coreNodes, encPos(node))
	}
	coreEdges := []any{}
	for _, edge := range data.Edges {
		o := projectRuEdge(edge)
		o["d"] = ruEdgeD(edge)
		coreEdges = append(coreEdges, o)
	}
	core := fx["core"].(map[string]any)
	wantCoreEdges := []any{}
	for _, item := range core["edges"].([]any) {
		if _, ok := item.(map[string]any)["selfLoop"]; ok {
			stats.selfLoops++
		}
		wantCoreEdges = append(wantCoreEdges, without(item, "selfLoop", "originalEdge"))
	}
	check("core.nodes", coreNodes, core["nodes"])
	check("core.edges", coreEdges, wantCoreEdges)

	// paintLayoutData with the paint options of the layout.
	pc := &rugraph.PaintContext{RenderContext: *rc, Measure: measure}
	paintNodes := []any{}
	nodes := paintOptions.GetNodes(data, pc)
	for _, node := range nodes {
		paintNodes = append(paintNodes, append(encPos(node), node.IsGroup, paintOptions.SkipNode(node, pc), paintOptions.IsCluster(node, pc)))
	}
	check("paintNodes", paintNodes, fx["paintNodes"])
	for _, node := range nodes {
		if paintOptions.SkipNode(node, pc) {
			continue
		}
		if !node.ClusterNode && node.IsGroup && paintOptions.IsCluster(node, pc) {
			r.insertCluster(node)
		}
	}
	// The clusterDb argument of insertEdge for the top graph.
	clusterDb := paintOptions.ClusterDb(pc)
	var paintErr error
	edgeNodes := []any{}
	for _, edge := range data.Edges {
		if edge.IsLayoutOnly {
			continue
		}
		for _, id := range []string{edge.Start, edge.End} {
			var p any
			if node := paintOptions.GetEdgeNode(id, edge, pc); node != nil {
				p = encPos(node)
			}
			edgeNodes = append(edgeNodes, []any{edge.ID, id, p})
		}
		if paintErr = clusterLookups(edge, clusterDb); paintErr != nil {
			break
		}
	}
	check("edgeNodes", edgeNodes, fx["edgeNodes"])
	check("clusterDbEnd", projectClusterDb(st), fx["clusterDbEnd"])
	for _, item := range fx["clusterDbEnd"].([]any) {
		entry := item.(map[string]any)
		key := decStr(entry, "key")
		bounds, ok := clusterDb(key)
		var got any
		if ok {
			got = []any{encDyn(bounds.X), encDyn(bounds.Y), encDyn(bounds.Width), encDyn(bounds.Height)}
		}
		check("clusterDb("+key+")", got, entry["node"])
	}
	for id, calls := range r.clusters {
		if len(calls) > 0 {
			r.problem("insertCluster(%q): %d recorded calls were not made", id, len(calls))
		}
	}
	stats.clusters += len(fx["clusters"].([]any))
	return r.problems, paintErr
}

func TestLayoutRecord(t *testing.T) {
	var file recordFile
	loadJSONGz(t, "testdata/layout-record.json.gz", &file)

	stats := &recordStats{}
	failed, throws := 0, 0
	byType := map[string]int{}
	for _, fx := range file.Fixtures {
		name := decStr(fx, "name")
		if _, ok := fx["input"]; !ok {
			// mermaid itself fails on the fixture before the layout.
			continue
		}
		stats.fixtures++
		byType[decStr(fx["input"].(map[string]any), "type")]++
		problems, err := replayFixture(t, fx, stats)
		// Where upstream throws, the Go layout must return the same error.
		wantErr, throwsUpstream := fx["error"].(string)
		switch {
		case throwsUpstream && err == nil:
			problems = append(problems, fmt.Sprintf("no error, upstream throws %q", wantErr))
		case throwsUpstream && err.Error() != wantErr:
			problems = append(problems, fmt.Sprintf("error %q, upstream throws %q", err, wantErr))
		case !throwsUpstream && err != nil:
			problems = append(problems, "error: "+err.Error())
		}
		if throwsUpstream {
			throws++
		}
		if len(problems) > 0 {
			failed++
			if failed <= 15 {
				t.Errorf("%s:\n  %s", name, strings.Join(problems, "\n  "))
			}
		}
	}
	t.Logf("%d fixtures (%v): %d graphs (%d fixtures with nested graphs), %d measured nodes, %d edge labels, %d clusters, %d self-loop segments that are not merged", stats.fixtures, byType, stats.graphs, stats.nested, stats.nodes, stats.edgeList, stats.clusters, stats.selfLoops)
	t.Logf("%d of %d fixtures differ (upstream throws in %d of them, and so does the Go layout)", failed, stats.fixtures, throws)
	if stats.orderDiffers > 0 {
		t.Logf("%d graphs are laid out at another value of the uniqueId counter than upstream: the order of the nested layouts differs", stats.orderDiffers)
	}
}
