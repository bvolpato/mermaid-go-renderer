package dagre

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/graphlib"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The golden file comes from tools/reference/gen-dagre.mjs. It holds the
// results of layout() of dagre-d3-es in Node.js.

type goldenFile struct {
	V8               string
	CounterSensitive []string
	Cases            []goldenCase
}

type goldenCase struct {
	Name string
	Opts struct {
		Directed   *bool
		Multigraph bool
		Compound   bool
	}
	Graph   map[string]any
	DefEdge bool
	Ops     [][]any
	IDStart int `json:"idStart"`
	Out     *struct {
		Nodes  [][]any
		Edges  [][]any
		Width  any
		Height any
	}
	Err *string
}

var (
	goldenOnce sync.Once
	goldenData *goldenFile
	goldenErr  error
)

func loadGolden(t testing.TB) *goldenFile {
	t.Helper()
	goldenOnce.Do(func() {
		// DAGRE_GOLDEN selects another file from gen-dagre.mjs --out.
		path := os.Getenv("DAGRE_GOLDEN")
		if path == "" {
			path = "testdata/layout.json.gz"
		}
		f, err := os.Open(path)
		if err != nil {
			goldenErr = err
			return
		}
		defer f.Close()
		zr, err := gzip.NewReader(f)
		if err != nil {
			goldenErr = err
			return
		}
		data, err := io.ReadAll(zr)
		if err != nil {
			goldenErr = err
			return
		}
		goldenData = &goldenFile{}
		dec := json.NewDecoder(bytes.NewReader(data))
		goldenErr = dec.Decode(goldenData)
	})
	if goldenErr != nil {
		t.Fatal(goldenErr)
	}
	return goldenData
}

// The label types of the tests embed the dagre label structs, as the label
// types of a caller do.
type testGraph struct {
	GraphLabel
	title string
}

type testNode struct {
	id string
	NodeLabel
}

type testEdge struct {
	EdgeLabel
	text string
}

// inNum decodes an input number of the golden file. ok is false for an
// absent property. The strings "undefined" and "NaN" are properties that
// exist: Number() makes NaN of both.
func inNum(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		return x, true
	case string:
		if x == "undefined" || x == "NaN" {
			return math.NaN(), true
		}
	}
	panic(fmt.Sprintf("bad number %v for %s", v, key))
}

func inOpt(m map[string]any, key string) Num {
	v, ok := inNum(m, key)
	if !ok {
		return Num{}
	}
	return N(v)
}

func inStr(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// build replays the graph operations of the case. It can run on any
// goroutine.
func (c *goldenCase) build() (g *graphlib.Graph, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: bad golden case: %v", c.Name, r)
		}
	}()
	opts := graphlib.Options{Multigraph: c.Opts.Multigraph, Compound: c.Opts.Compound}
	if c.Opts.Directed != nil {
		opts.Undirected = !*c.Opts.Directed
	}
	g = graphlib.New(opts)
	if c.Graph != nil {
		label := &testGraph{title: c.Name}
		label.Rankdir = inStr(c.Graph, "rankdir")
		label.Nodesep = inOpt(c.Graph, "nodesep")
		label.Edgesep = inOpt(c.Graph, "edgesep")
		label.Ranksep = inOpt(c.Graph, "ranksep")
		label.Marginx, _ = inNum(c.Graph, "marginx")
		label.Marginy, _ = inNum(c.Graph, "marginy")
		label.Ranker = inStr(c.Graph, "ranker")
		label.Acyclicer = inStr(c.Graph, "acyclicer")
		label.Align = inStr(c.Graph, "align")
		g.SetGraph(label)
	}
	if c.DefEdge {
		g.SetDefaultEdgeLabel(func(v, w, name string) any { return &testEdge{} })
	}
	edgeLabel := func(m map[string]any) *testEdge {
		label := &testEdge{}
		label.Minlen = inOpt(m, "minlen")
		label.Weight = inOpt(m, "weight")
		label.Width, _ = inNum(m, "width")
		label.Height, _ = inNum(m, "height")
		label.Labeloffset = inOpt(m, "labeloffset")
		label.Labelpos = inStr(m, "labelpos")
		return label
	}
	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	for _, op := range c.Ops {
		switch op[0].(string) {
		case "n":
			v := op[1].(string)
			if len(op) > 2 {
				m := op[2].(map[string]any)
				label := &testNode{id: v}
				label.Width, _ = inNum(m, "width")
				label.Height, _ = inNum(m, "height")
				g.SetNode(v, label)
			} else {
				g.SetNode(v)
			}
		case "p":
			if len(op) > 2 {
				must(g.SetParent(op[1].(string), op[2].(string)))
			} else {
				must(g.SetParent(op[1].(string)))
			}
		case "e":
			v, w := op[1].(string), op[2].(string)
			name, hasName := op[3].(string)
			label, hasLabel := op[4].(map[string]any)
			switch {
			case !hasLabel && !hasName:
				must(g.SetEdgeObj(graphlib.Edge{V: v, W: w}))
			case !hasLabel:
				must(g.SetEdgeObj(graphlib.Edge{V: v, W: w, Name: name}))
			case !hasName:
				must(g.SetEdge(v, w, edgeLabel(label)))
			default:
				must(g.SetEdge(v, w, edgeLabel(label), name))
			}
		case "rn":
			g.RemoveNode(op[1].(string))
		case "re":
			if name, ok := op[3].(string); ok {
				g.RemoveEdge(op[1].(string), op[2].(string), name)
			} else {
				g.RemoveEdge(op[1].(string), op[2].(string))
			}
		default:
			panic(fmt.Sprintf("bad op %v", op))
		}
	}
	return g, nil
}

// outNum decodes an output number of the golden file. ok is false for
// undefined.
func outNum(v any) (float64, bool) {
	switch x := v.(type) {
	case nil:
		return 0, false
	case float64:
		return x, true
	case string:
		switch x {
		case "NaN":
			return math.NaN(), true
		case "Infinity":
			return math.Inf(1), true
		case "-Infinity":
			return math.Inf(-1), true
		case "-0":
			return math.Copysign(0, -1), true
		}
	}
	panic(fmt.Sprintf("bad output number %v", v))
}

// sameBits compares two numbers by bits. JavaScript has one NaN, so every
// NaN is equal to every other NaN.
func sameBits(a, b float64) bool {
	if a != a && b != b {
		return true
	}
	return math.Float64bits(a) == math.Float64bits(b)
}

// compare returns the differences between the graph after Layout and the
// expected result. It also checks the order of g.Nodes() and g.Edges().
func (c *goldenCase) compare(g *graphlib.Graph) []string {
	var diffs []string
	diff := func(format string, args ...any) {
		if len(diffs) < 20 {
			diffs = append(diffs, fmt.Sprintf(format, args...))
		}
	}
	num := func(what string, got float64, gotDefined bool, want any) {
		w, wantDefined := outNum(want)
		switch {
		case gotDefined != wantDefined:
			diff("%s: defined is %v, want %v", what, gotDefined, wantDefined)
		case gotDefined && !sameBits(got, w):
			diff("%s = %s (%016x), want %s (%016x)", what, fmtNum(got), math.Float64bits(got), fmtNum(w), math.Float64bits(w))
		}
	}

	nodes := g.Nodes()
	if len(nodes) != len(c.Out.Nodes) {
		diff("%d nodes, want %d", len(nodes), len(c.Out.Nodes))
		return diffs
	}
	for i, v := range nodes {
		want := c.Out.Nodes[i]
		if want[0].(string) != v {
			diff("node %d is %q, want %q", i, v, want[0])
			continue
		}
		label, _ := g.Node(v).(*testNode)
		if len(want) == 1 {
			if label != nil {
				diff("node %q has a label, want none", v)
			}
			continue
		}
		if label == nil {
			diff("node %q has no label", v)
			continue
		}
		num(fmt.Sprintf("node %q x", v), label.X, true, want[1])
		num(fmt.Sprintf("node %q y", v), label.Y, true, want[2])
		if len(want) > 3 {
			num(fmt.Sprintf("node %q width", v), label.Width, true, want[3])
			num(fmt.Sprintf("node %q height", v), label.Height, true, want[4])
		}
	}

	edges := g.Edges()
	if len(edges) != len(c.Out.Edges) {
		diff("%d edges, want %d", len(edges), len(c.Out.Edges))
		return diffs
	}
	for i, e := range edges {
		want := c.Out.Edges[i]
		wantName, _ := want[2].(string)
		if want[0].(string) != e.V || want[1].(string) != e.W || wantName != e.Name {
			diff("edge %d is %v, want %v", i, e, want[:3])
			continue
		}
		label, _ := g.EdgeObj(e).(*testEdge)
		if label == nil {
			diff("edge %v has no label", e)
			continue
		}
		what := fmt.Sprintf("edge %q->%q %q", e.V, e.W, e.Name)
		wantPoints, _ := want[3].([]any)
		if len(wantPoints) != 2*len(label.Points) {
			diff("%s has %d points, want %d", what, len(label.Points), len(wantPoints)/2)
		} else {
			for j, p := range label.Points {
				num(fmt.Sprintf("%s point %d x", what, j), p.X, true, wantPoints[2*j])
				num(fmt.Sprintf("%s point %d y", what, j), p.Y, true, wantPoints[2*j+1])
			}
		}
		num(what+" label x", label.X.V, label.X.Set, want[4])
		num(what+" label y", label.Y.V, label.Y.Set, want[5])
	}

	graph := g.Graph().(*testGraph)
	num("graph width", graph.Width, true, c.Out.Width)
	num("graph height", graph.Height, true, c.Out.Height)
	return diffs
}

// run builds the graph of the case, runs Layout and returns the differences
// to the expected result.
func (c *goldenCase) run(state *State) []string {
	g, err := c.build()
	if err != nil {
		return []string{err.Error()}
	}
	err = Layout(g, state)
	if c.Err != nil {
		if err == nil {
			return []string{fmt.Sprintf("no error, JavaScript throws %q", *c.Err)}
		}
		return nil
	}
	if err != nil {
		return []string{fmt.Sprintf("error %v, JavaScript does not throw", err)}
	}
	return c.compare(g)
}

// goldenSort is the sort implementation of the V8 that made the golden
// file.
func goldenSort(t testing.TB, golden *goldenFile) js.V8Sort {
	var major, minor int
	if _, err := fmt.Sscanf(golden.V8, "%d.%d", &major, &minor); err != nil {
		t.Fatalf("bad V8 version %q", golden.V8)
	}
	if major > 14 || (major == 14 && minor >= 9) {
		return js.V8PowerSort
	}
	return js.V8TimSort
}

// forEachCase runs fn for every golden case on several goroutines. It
// returns the differences that fn reports, by case index. Every call of fn
// must use its own State.
func forEachCase(cases []goldenCase, fn func(c *goldenCase) []string) [][]string {
	results := make([][]string, len(cases))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < runtime.GOMAXPROCS(0); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				results[i] = fn(&cases[i])
			}
		}()
	}
	for i := range cases {
		next <- i
	}
	close(next)
	wg.Wait()
	return results
}

// caseGroup is the name of a case without its number.
func caseGroup(name string) string {
	for _, prefix := range []string{"fixed", "exotic", "proto", "size"} {
		if strings.HasPrefix(name, prefix+"-") {
			return prefix
		}
	}
	return name[:strings.LastIndex(name, "-")]
}

// TestLayoutGolden runs every golden case and compares every number by
// bits. The layouts run on several goroutines, each with its own state.
func TestLayoutGolden(t *testing.T) {
	golden := loadGolden(t)
	sortMode := goldenSort(t, golden)
	results := forEachCase(golden.Cases, func(c *goldenCase) []string {
		return c.run(&State{IDCounter: c.IDStart, Sort: sortMode})
	})
	failed, throws := 0, 0
	groups := map[string][2]int{}
	for i := range golden.Cases {
		c := &golden.Cases[i]
		if c.Err != nil {
			throws++
		}
		group := caseGroup(c.Name)
		n := groups[group]
		n[0]++
		if diffs := results[i]; len(diffs) > 0 {
			n[1]++
			failed++
			if failed <= 15 {
				t.Errorf("%s (%d ops): %d differences:\n  %s", c.Name, len(c.Ops), len(diffs), strings.Join(diffs[:min(len(diffs), 5)], "\n  "))
			}
		}
		groups[group] = n
	}
	t.Logf("%d golden cases in %d groups, %d failed (%d cases expect an error; golden from V8 %s)", len(golden.Cases), len(groups), failed, throws, golden.V8)
	if failed > 0 {
		for group, n := range groups {
			if n[1] > 0 {
				t.Logf("  group %s: %d of %d failed", group, n[1], n[0])
			}
		}
		t.Fail()
	}
	if len(golden.Cases) < 400 {
		t.Errorf("only %d golden cases", len(golden.Cases))
	}
}

// TestLayoutCounterIndependent checks that the start value of the uniqueId
// counter does not change the result. The generator checks the same for the
// JavaScript code and records the cases that differ in counterSensitive.
func TestLayoutCounterIndependent(t *testing.T) {
	golden := loadGolden(t)
	if len(golden.CounterSensitive) != 0 {
		t.Logf("JavaScript results depend on the counter for: %v", golden.CounterSensitive)
	}
	sensitive := map[string]bool{}
	for _, name := range golden.CounterSensitive {
		sensitive[name] = true
	}
	sortMode := goldenSort(t, golden)
	starts := []int{0, 8, 999999, 123456789}
	if testing.Short() {
		starts = []int{0}
	}
	results := forEachCase(golden.Cases, func(c *goldenCase) []string {
		if sensitive[c.Name] {
			return nil
		}
		for _, start := range append([]int{c.IDStart + 1}, starts...) {
			if diffs := c.run(&State{IDCounter: start, Sort: sortMode}); len(diffs) > 0 {
				return []string{fmt.Sprintf("counter %d: %s", start, diffs[0])}
			}
		}
		return nil
	})
	failed := 0
	for i, diffs := range results {
		if len(diffs) > 0 {
			failed++
			if failed <= 10 {
				t.Errorf("%s: %s", golden.Cases[i].Name, diffs[0])
			}
		}
	}
	t.Logf("%d cases, each with %d other counter values: %d depend on the counter", len(golden.Cases), len(starts)+1, failed)
}

// TestLayoutCounterCollision shows the one known way for the uniqueId
// counter to change a result: dagre names a reversed edge 'rev' + id, and
// the graph already has an edge with that name between the same nodes. The
// JavaScript layout then throws, and it does not throw at another counter
// value. The generator records this case as the only one in
// counterSensitive.
func TestLayoutCounterCollision(t *testing.T) {
	golden := loadGolden(t)
	if len(golden.CounterSensitive) != 1 || golden.CounterSensitive[0] != "counter-rev-collision" {
		t.Fatalf("counterSensitive = %v, want [counter-rev-collision]", golden.CounterSensitive)
	}
	sortMode := goldenSort(t, golden)
	for i := range golden.Cases {
		c := &golden.Cases[i]
		if c.Name != "counter-rev-collision" {
			continue
		}
		if c.Err == nil {
			t.Fatal("the golden case does not throw")
		}
		g, err := c.build()
		if err != nil {
			t.Fatal(err)
		}
		if err := Layout(g, &State{IDCounter: c.IDStart, Sort: sortMode}); err == nil {
			t.Errorf("counter %d: no error, JavaScript throws %q", c.IDStart, *c.Err)
		}
		g, err = c.build()
		if err != nil {
			t.Fatal(err)
		}
		if err := Layout(g, &State{IDCounter: c.IDStart + 10, Sort: sortMode}); err != nil {
			t.Errorf("counter %d: error %v, want none", c.IDStart+10, err)
		}
		return
	}
	t.Fatal("no golden case counter-rev-collision")
}

// TestLayoutSortSensitivity reports how many golden cases depend on the
// Array.prototype.sort implementation. A case depends on it only when a
// comparator is not consistent. The test fails when a case outside of the
// groups with an edge weight of 0 depends on it.
func TestLayoutSortSensitivity(t *testing.T) {
	golden := loadGolden(t)
	other := js.V8PowerSort
	if goldenSort(t, golden) == js.V8PowerSort {
		other = js.V8TimSort
	}
	results := forEachCase(golden.Cases, func(c *goldenCase) []string {
		return c.run(&State{IDCounter: c.IDStart, Sort: other})
	})
	var names []string
	for i, diffs := range results {
		if len(diffs) > 0 {
			name := golden.Cases[i].Name
			names = append(names, name)
			if group := caseGroup(name); group != "weight0" && group != "exotic" {
				t.Errorf("%s depends on the sort implementation: %s", name, diffs[0])
			}
		}
	}
	t.Logf("%d of %d cases give another result with the other V8 sort implementation: %v", len(names), len(golden.Cases), names)
}

// TestLayoutStateCounter checks that one state carries the counter from one
// layout to the next, as the module global does in JavaScript.
func TestLayoutStateCounter(t *testing.T) {
	golden := loadGolden(t)
	sortMode := goldenSort(t, golden)
	// The generator runs each case five times. Between the runs it moves
	// the counter forward, so the counter after a case is not the start of
	// the next case. The counter must only ever grow.
	state := &State{Sort: sortMode}
	for i := range golden.Cases[:40] {
		c := &golden.Cases[i]
		if c.Name == "counter-rev-collision" {
			// The result of this case depends on the counter.
			continue
		}
		before := state.IDCounter
		g, err := c.build()
		if err != nil {
			t.Fatal(err)
		}
		err = Layout(g, state)
		if (err != nil) != (c.Err != nil) {
			t.Fatalf("%s: error %v, want error: %v", c.Name, err, c.Err != nil)
		}
		if state.IDCounter < before {
			t.Fatalf("%s: counter went from %d to %d", c.Name, before, state.IDCounter)
		}
		if err == nil && len(g.Nodes()) > 0 && state.IDCounter == before {
			t.Fatalf("%s: layout made no id", c.Name)
		}
	}
	if got := (&State{IDCounter: 41}).UniqueID("_d"); got != "_d42" {
		t.Errorf("UniqueID = %q, want _d42", got)
	}
}

func TestLayoutLabelErrors(t *testing.T) {
	g := graphlib.New(graphlib.Options{})
	g.SetNode("a", &testNode{})
	if err := Layout(g, nil); err == nil {
		t.Error("no error for a graph without a label")
	}
	g.SetGraph(&testGraph{})
	if err := Layout(g, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	g.SetNode("b", "not a label")
	if err := Layout(g, nil); err == nil {
		t.Error("no error for a node label of a wrong type")
	}
	g.SetNode("b", &testNode{})
	if err := g.SetEdge("a", "b", nil); err != nil {
		t.Fatal(err)
	}
	if err := Layout(g, nil); err == nil {
		t.Error("no error for an edge without a label")
	}
	if err := g.SetEdge("a", "b", &testEdge{}); err != nil {
		t.Fatal(err)
	}
	if err := Layout(g, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// A plain label struct also implements the interfaces.
	g2 := graphlib.New(graphlib.Options{Multigraph: true, Compound: true}).SetGraph(&GraphLabel{Rankdir: "LR"})
	g2.SetNode("a", &NodeLabel{Width: 10, Height: 10})
	g2.SetNode("b", &NodeLabel{Width: 10, Height: 10})
	if err := g2.SetEdge("a", "b", &EdgeLabel{Width: 5, Height: 5, Labelpos: "c"}, "x"); err != nil {
		t.Fatal(err)
	}
	if err := Layout(g2, &State{}); err != nil {
		t.Fatal(err)
	}
	e := g2.Edge("a", "b", "x").(*EdgeLabel)
	if !e.X.Set || len(e.Points) != 3 {
		t.Errorf("edge label = %+v", e)
	}
}

// TestLayoutPrototypeIDs checks the node ids that are property names of
// Object.prototype. The JavaScript layout throws for such a leaf node in a
// graph with more than one rank. The expected results come from a run of
// dagre-d3-es with these ids.
func TestLayoutPrototypeIDs(t *testing.T) {
	for _, id := range []string{"constructor", "toString", "valueOf", "hasOwnProperty", "isPrototypeOf", "toLocaleString", "__defineGetter__"} {
		for _, compound := range []bool{false, true} {
			g := graphlib.New(graphlib.Options{Multigraph: true, Compound: compound}).SetGraph(&testGraph{})
			g.SetNode("a", &testNode{NodeLabel: NodeLabel{Width: 50, Height: 30}})
			g.SetNode(id, &testNode{NodeLabel: NodeLabel{Width: 60, Height: 40}})
			g.SetNode("b", &testNode{NodeLabel: NodeLabel{Width: 50, Height: 30}})
			for _, e := range [][3]string{{"a", id, "e1"}, {id, "b", "e2"}, {"a", "b", "e3"}} {
				if err := g.SetEdge(e[0], e[1], &testEdge{}, e[2]); err != nil {
					t.Fatal(err)
				}
			}
			if compound {
				g.SetNode("sg", &testNode{})
				if err := g.SetParent(id, "sg"); err != nil {
					t.Fatal(err)
				}
				if err := g.SetParent("b", "sg"); err != nil {
					t.Fatal(err)
				}
			}
			if err := Layout(g, nil); err == nil {
				t.Errorf("id %q, compound %v: no error, JavaScript throws", id, compound)
			}
		}
	}
	// An id that is not a property of Object.prototype works.
	g := graphlib.New(graphlib.Options{}).SetGraph(&testGraph{})
	g.SetNode("prototype", &testNode{})
	g.SetNode("length", &testNode{})
	if err := g.SetEdge("prototype", "length", &testEdge{}); err != nil {
		t.Fatal(err)
	}
	if err := Layout(g, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
