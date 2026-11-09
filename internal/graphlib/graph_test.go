package graphlib

import (
	"compress/gzip"
	"errors"
	"io"
	"os"
	"strconv"
	"testing"
	"unicode/utf16"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden file comes from tools/reference/gen-graphlib.mjs. It holds
// snapshots of the graphlib of dagre-d3-es in Node.js. The tests read it as
// dynamic JavaScript values, so that null and undefined stay distinct and
// objects keep their key order.

func loadGolden(t *testing.T, path string) *js.Obj[any] {
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
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	v, err := js.ParseJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	return js.AsObj(v)
}

func arr(v any) []any {
	a, _ := v.([]any)
	return a
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func strs(v any) []string {
	a := arr(v)
	out := make([]string, len(a))
	for i, x := range a {
		out[i] = str(x)
	}
	return out
}

func optionsOf(v any) Options {
	o := js.AsObj(v)
	opts := Options{}
	if o == nil {
		return opts
	}
	if d, ok := o.Lookup("directed"); ok {
		opts.Undirected = !js.Truthy(d)
	}
	opts.Multigraph = js.Truthy(o.Get("multigraph"))
	opts.Compound = js.Truthy(o.Get("compound"))
	return opts
}

// edgeOf decodes the edge object {v, w, name} of an operation.
func edgeOf(v any) Edge {
	return Edge{V: str(js.Path(v, "v")), W: str(js.Path(v, "w")), Name: str(js.Path(v, "name"))}
}

// apply runs the operations of a golden case. It returns the error message
// of each operation that fails, by operation index.
func apply(t *testing.T, g *Graph, ops []any) map[string]string {
	t.Helper()
	errs := map[string]string{}
	for i, opv := range ops {
		op := arr(opv)
		a := op[1:]
		var err error
		switch str(op[0]) {
		case "setNode":
			if len(a) > 1 {
				g.SetNode(str(a[0]), a[1])
			} else {
				g.SetNode(str(a[0]))
			}
		case "setNodes":
			if len(a) > 1 {
				g.SetNodes(strs(a[0]), a[1])
			} else {
				g.SetNodes(strs(a[0]))
			}
		case "setPath":
			if len(a) > 1 {
				err = g.SetPath(strs(a[0]), a[1])
			} else {
				err = g.SetPath(strs(a[0]))
			}
		case "setEdge":
			switch len(a) {
			case 2:
				err = g.SetEdgeObj(Edge{V: str(a[0]), W: str(a[1])})
			case 3:
				err = g.SetEdge(str(a[0]), str(a[1]), a[2])
			default:
				err = g.SetEdge(str(a[0]), str(a[1]), a[2], str(a[3]))
			}
		case "setEdgeObj":
			if len(a) > 1 {
				err = g.SetEdgeObj(edgeOf(a[0]), a[1])
			} else {
				err = g.SetEdgeObj(edgeOf(a[0]))
			}
		case "removeNode":
			g.RemoveNode(str(a[0]))
		case "removeEdge":
			if len(a) > 2 {
				g.RemoveEdge(str(a[0]), str(a[1]), str(a[2]))
			} else {
				g.RemoveEdge(str(a[0]), str(a[1]))
			}
		case "removeEdgeObj":
			g.RemoveEdgeObj(edgeOf(a[0]))
		case "setParent":
			if len(a) > 1 {
				err = g.SetParent(str(a[0]), str(a[1]))
			} else {
				err = g.SetParent(str(a[0]))
			}
		default:
			t.Fatalf("bad op %v", op[0])
		}
		if err != nil {
			errs[strconv.Itoa(i)] = err.Error()
		}
	}
	return errs
}

func lab(x any) any {
	o := js.NewObj[any]()
	if x == nil {
		o.Set("undefined", true)
	} else {
		o.Set("value", x)
	}
	return o
}

func edgeList(list []Edge) any {
	if list == nil {
		return js.Null
	}
	out := make([]any, len(list))
	for i, e := range list {
		var name any = js.Null
		if e.Name != "" {
			name = e.Name
		}
		out[i] = []any{e.V, e.W, name}
	}
	return out
}

func strList(list []string) any {
	if list == nil {
		return js.Null
	}
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// snapshot builds the same value as snapshot in gen-graphlib.mjs.
func snapshot(g *Graph, pool []string) *js.Obj[any] {
	seen := map[string]bool{}
	probe := []string{}
	add := func(v string) {
		if !seen[v] {
			seen[v] = true
			probe = append(probe, v)
		}
	}
	for _, v := range g.Nodes() {
		add(v)
	}
	for _, v := range pool[:min(4, len(pool))] {
		add(v)
	}
	add("missing")

	snap := js.NewObj[any]()
	snap.Set("directed", g.IsDirected())
	snap.Set("multigraph", g.IsMultigraph())
	snap.Set("compound", g.IsCompound())
	snap.Set("graph", lab(g.Graph()))
	snap.Set("nodeCount", float64(g.NodeCount()))
	snap.Set("edgeCount", float64(g.EdgeCount()))
	snap.Set("nodes", strList(g.Nodes()))
	snap.Set("edges", edgeList(g.Edges()))
	snap.Set("sources", strList(g.Sources()))
	snap.Set("sinks", strList(g.Sinks()))
	snap.Set("rootChildren", strList(g.Children()))
	nodeLabels := []any{}
	for _, v := range g.Nodes() {
		nodeLabels = append(nodeLabels, lab(g.Node(v)))
	}
	snap.Set("nodeLabels", nodeLabels)
	edgeLabels := []any{}
	for _, e := range g.Edges() {
		edgeLabels = append(edgeLabels, lab(g.EdgeObj(e)))
	}
	snap.Set("edgeLabels", edgeLabels)

	perNode := []any{}
	for _, v := range probe {
		o := js.NewObj[any]()
		o.Set("v", v)
		o.Set("has", g.HasNode(v))
		if parent, ok := g.Parent(v); ok {
			o.Set("parent", parent)
		} else {
			o.Set("parent", js.Null)
		}
		o.Set("children", strList(g.Children(v)))
		o.Set("predecessors", strList(g.Predecessors(v)))
		o.Set("successors", strList(g.Successors(v)))
		o.Set("neighbors", strList(g.Neighbors(v)))
		o.Set("inEdges", edgeList(g.InEdges(v)))
		o.Set("outEdges", edgeList(g.OutEdges(v)))
		o.Set("nodeEdges", edgeList(g.NodeEdges(v)))
		if g.HasNode(v) {
			o.Set("isLeaf", g.IsLeaf(v))
		} else {
			o.Set("isLeaf", js.Null)
		}
		perNode = append(perNode, o)
	}
	snap.Set("perNode", perNode)

	pairs := []any{}
	nodes := g.Nodes()
	some := nodes[:min(6, len(nodes))]
	for _, v := range some {
		for _, w := range some {
			o := js.NewObj[any]()
			o.Set("v", v)
			o.Set("w", w)
			o.Set("has", g.HasEdge(v, w))
			o.Set("hasN1", g.HasEdge(v, w, "n1"))
			o.Set("hasObj", g.HasEdgeObj(Edge{V: v, W: w, Name: "x"}))
			o.Set("label", lab(g.Edge(v, w)))
			o.Set("labelN1", lab(g.Edge(v, w, "n1")))
			o.Set("inEdges", edgeList(g.InEdges(v, w)))
			o.Set("outEdges", edgeList(g.OutEdges(v, w)))
			o.Set("nodeEdges", edgeList(g.NodeEdges(v, w)))
			pairs = append(pairs, o)
		}
	}
	snap.Set("pairs", pairs)
	return snap
}

// compareSnap compares a snapshot with the golden snapshot, key by key.
func compareSnap(t *testing.T, what string, got *js.Obj[any], wantV any) bool {
	t.Helper()
	want := js.AsObj(wantV)
	ok := true
	for _, k := range want.Keys() {
		g, w := js.StringifyJSON(got.Get(k)), js.StringifyJSON(want.Get(k))
		if g != w {
			ok = false
			t.Errorf("%s: %s differs\n  got:  %.600s\n  want: %.600s", what, k, g, w)
		}
	}
	if got.Len() != want.Len() {
		ok = false
		t.Errorf("%s: snapshot has %d keys, want %d", what, got.Len(), want.Len())
	}
	return ok
}

// keep is the filter of the filterNodes test in gen-graphlib.mjs. It works
// on UTF-16 code units, as charCodeAt does.
func keep(v string) bool {
	h := int32(7)
	for _, c := range utf16.Encode([]rune(v)) {
		h = h*31 + int32(c)
	}
	return h&3 != 0
}

var idPools = map[string][]string{
	"alpha": {"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"},
	"int":   {"1", "2", "3", "10", "20", "0", "5", "7", "11", "100", "15", "4"},
	"mixed": {"b", "10", "a", "2", "1", "x.y", "007", "1.5", "-1", "4294967295", "4294967294", "é", "漢", "😀", "length", "A B", "3", "z", "0", "1e3", " ", "99"},
}

// TestGraphGolden replays random operation sequences and compares every
// query method with the JavaScript graph. This fixes the enumeration order
// of nodes, edges, children, predecessors and successors. It also compares
// write, read and filterNodes.
func TestGraphGolden(t *testing.T) {
	golden := loadGolden(t, "testdata/graphlib.json.gz")
	cases := arr(golden.Get("cases"))
	failed := 0
	for _, cv := range cases {
		c := js.AsObj(cv)
		name := str(c.Get("name"))
		pool := idPools[str(c.Get("pool"))]

		g := New(optionsOf(c.Get("opts")))
		if label, ok := js.AsObj(c.Get("graphLabel")).Lookup("value"); ok {
			g.SetGraph(label)
		}
		if js.Truthy(c.Get("withDefaults")) {
			g.SetDefaultNodeLabel(func(v string) any { return "node:" + v })
			g.SetDefaultEdgeLabel(func(v, w, name string) any {
				if name == "" {
					return "edge:" + v + ">" + w
				}
				return "edge:" + v + ">" + w + "#" + name
			})
		}
		errs := apply(t, g, arr(c.Get("ops")))

		ok := true
		wantErrs := js.AsObj(c.Get("errors"))
		for _, k := range wantErrs.Keys() {
			if errs[k] != str(wantErrs.Get(k)) {
				ok = false
				t.Errorf("%s: op %s: error %q, want %q", name, k, errs[k], str(wantErrs.Get(k)))
			}
		}
		if len(errs) != wantErrs.Len() {
			ok = false
			t.Errorf("%s: %d operations fail, want %d", name, len(errs), wantErrs.Len())
		}

		ok = compareSnap(t, name, snapshot(g, pool), c.Get("snap")) && ok

		written := js.StringifyJSON(Write(g))
		if want := str(c.Get("written")); written != want {
			ok = false
			t.Errorf("%s: write differs\n  got:  %.600s\n  want: %.600s", name, written, want)
		}

		parsed, err := js.ParseJSON([]byte(str(c.Get("written"))))
		if err != nil {
			t.Fatal(err)
		}
		g2, err := Read(parsed)
		if err != nil {
			t.Fatalf("%s: read: %v", name, err)
		}
		ok = compareSnap(t, name+" after read", snapshot(g2, pool), c.Get("readSnap")) && ok
		if rewritten, want := js.StringifyJSON(Write(g2)), str(c.Get("rewritten")); rewritten != want {
			ok = false
			t.Errorf("%s: write after read differs\n  got:  %.600s\n  want: %.600s", name, rewritten, want)
		}

		ok = compareSnap(t, name+" filtered", snapshot(g.FilterNodes(keep), pool), c.Get("filteredSnap")) && ok

		if !ok {
			failed++
			if failed >= 5 {
				t.Fatalf("stop after %d failed cases", failed)
			}
		}
	}
	t.Logf("%d graph cases, %d failed", len(cases), failed)
	if len(cases) < 100 {
		t.Errorf("only %d cases", len(cases))
	}
}

// TestReadGolden checks read for JSON that write does not produce: ids that
// are numbers, a null name, an empty name and missing options.
func TestReadGolden(t *testing.T) {
	golden := loadGolden(t, "testdata/graphlib.json.gz")
	for i, cv := range arr(golden.Get("readCases")) {
		c := js.AsObj(cv)
		parsed, err := js.ParseJSON([]byte(str(c.Get("input"))))
		if err != nil {
			t.Fatal(err)
		}
		g, err := Read(parsed)
		if err != nil {
			t.Fatalf("read case %d: %v", i, err)
		}
		// The JavaScript graph stores the options as they are. The port
		// converts them to booleans.
		want := js.AsObj(c.Get("snap"))
		for _, k := range []string{"directed", "multigraph", "compound"} {
			want.Set(k, js.Truthy(want.Get(k)))
		}
		compareSnap(t, "read case "+strconv.Itoa(i), snapshot(g, []string{"a", "b", "c", "1"}), want)
		w := Write(g)
		if got, want := js.StringifyJSON(w.Get("nodes")), str(c.Get("nodeJSON")); got != want {
			t.Errorf("read case %d: nodes %s, want %s", i, got, want)
		}
		if got, want := js.StringifyJSON(w.Get("edges")), str(c.Get("edgeJSON")); got != want {
			t.Errorf("read case %d: edges %s, want %s", i, got, want)
		}
	}
}

func TestErrors(t *testing.T) {
	g := New(Options{})
	if err := g.SetParent("a", "b"); !errors.Is(err, ErrNotCompound) {
		t.Errorf("SetParent = %v, want ErrNotCompound", err)
	}
	if err := g.SetEdge("a", "b", nil, "name"); !errors.Is(err, ErrNotMultigraph) {
		t.Errorf("SetEdge = %v, want ErrNotMultigraph", err)
	}
	if g.HasNode("a") || g.HasNode("b") {
		t.Error("a failed call changed the graph")
	}
	c := New(Options{Compound: true})
	if err := c.SetParent("a", "b"); err != nil {
		t.Fatal(err)
	}
	var cycle *CycleError
	if err := c.SetParent("b", "a"); !errors.As(err, &cycle) {
		t.Errorf("SetParent = %v, want a CycleError", err)
	} else if cycle.Error() != "Setting a as parent of b would create a cycle" {
		t.Errorf("message = %q", cycle.Error())
	}
	if _, err := Read(mustParse(t, `{"options":{"compound":true},"nodes":[{"v":"a","parent":"a"}]}`)); err == nil {
		t.Error("Read accepts a node that is its own parent")
	}
}

func mustParse(t *testing.T, s string) any {
	t.Helper()
	v, err := js.ParseJSON([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestEnumerationOrder shows the order rule in a small example: integer-like
// ids come first in numeric order, then the other ids in insertion order.
func TestEnumerationOrder(t *testing.T) {
	g := New(Options{Multigraph: true, Compound: true})
	for _, v := range []string{"b", "10", "a", "2", "007", "1"} {
		g.SetNode(v)
	}
	if got, want := js.StringifyJSON(strList(g.Nodes())), `["1","2","10","b","a","007"]`; got != want {
		t.Errorf("Nodes() = %s, want %s", got, want)
	}
	for _, e := range [][2]string{{"b", "a"}, {"b", "10"}, {"b", "2"}, {"a", "10"}} {
		if err := g.SetEdge(e[0], e[1], nil); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := js.StringifyJSON(strList(g.Successors("b"))), `["2","10","a"]`; got != want {
		t.Errorf("Successors(b) = %s, want %s", got, want)
	}
	// Edges keep insertion order: the edge id is not integer-like.
	if got, want := js.StringifyJSON(edgeList(g.OutEdges("b"))), `[["b","a",null],["b","10",null],["b","2",null]]`; got != want {
		t.Errorf("OutEdges(b) = %s, want %s", got, want)
	}
	// A child that is set again moves to the end.
	for _, v := range []string{"b", "a", "2"} {
		if err := g.SetParent(v, "p"); err != nil {
			t.Fatal(err)
		}
	}
	if err := g.SetParent("b", "p"); err != nil {
		t.Fatal(err)
	}
	if got, want := js.StringifyJSON(strList(g.Children("p"))), `["2","a","b"]`; got != want {
		t.Errorf("Children(p) = %s, want %s", got, want)
	}
}

// TestPrototypeIDs shows a miscount of the JavaScript graph that the port
// keeps. For a node id that is the name of a function of Object.prototype,
// the first edge between two nodes is not counted. After the removal of one
// of two parallel edges, the node is no longer a successor.
func TestPrototypeIDs(t *testing.T) {
	g := New(Options{Multigraph: true})
	for _, name := range []string{"x", "y"} {
		if err := g.SetEdge("a", "constructor", nil, name); err != nil {
			t.Fatal(err)
		}
		if err := g.SetEdge("a", "b", nil, name); err != nil {
			t.Fatal(err)
		}
	}
	g.RemoveEdge("a", "constructor", "x")
	g.RemoveEdge("a", "b", "x")
	if got, want := js.StringifyJSON(strList(g.Successors("a"))), `["b"]`; got != want {
		t.Errorf("Successors(a) = %s, want %s", got, want)
	}
	if got, want := js.StringifyJSON(edgeList(g.OutEdges("a"))), `[["a","constructor","y"],["a","b","y"]]`; got != want {
		t.Errorf("OutEdges(a) = %s, want %s", got, want)
	}
}
