package cytoscape

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// engineFile is testdata/engine.json.gz, which
// tools/reference/gen-cytoscape-engine.mjs writes from Chromium.
type engineFile struct {
	Sort []struct {
		Cmp    string `json:"cmp"`
		Input  []any  `json:"input"`
		Output []any  `json:"output"`
		Calls  int    `json:"calls"`
	} `json:"sort"`
	Numbers [][2]string `json:"numbers"`
	Keys    [][2][]string
	Parse   [][3]string `json:"parse"`
}

func loadEngine(t testing.TB) *engineFile {
	t.Helper()
	f, err := os.Open("testdata/engine.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	e := &engineFile{}
	if err := json.NewDecoder(zr).Decode(e); err != nil {
		t.Fatal(err)
	}
	return e
}

func testRuntime(t testing.TB, opts runtimeOptions) *jsRuntime {
	t.Helper()
	rt, err := newRuntime(Env{Now: time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)}, opts)
	if err != nil {
		t.Fatal(err)
	}
	return rt
}

// sortHarness runs one sort case in JavaScript. It returns the array as
// text, with u for undefined and h for a hole, and the number of comparator
// calls.
const sortHarness = `(function (cmpSource, input) {
  var cmp = eval(cmpSource);
  var arr = [];
  arr.length = input.length;
  for (var i = 0; i < input.length; i++) {
    if (input[i] === 'u') { arr[i] = undefined; } else if (input[i] !== 'h') { arr[i] = input[i]; }
  }
  var calls = 0;
  var ret = arr.sort(function (a, b) { calls++; return cmp(a, b); });
  if (ret !== arr) { throw new Error('sort did not return the array'); }
  var out = [];
  for (var j = 0; j < input.length; j++) {
    out.push(!(j in arr) ? 'h' : arr[j] === undefined ? 'u' : String(arr[j]));
  }
  return out.join(',') + '|' + calls;
})`

func sortCaseText(vals []any, calls int) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		switch x := v.(type) {
		case string:
			parts[i] = x
		case float64:
			parts[i] = strconv.FormatFloat(x, 'f', -1, 64)
		}
	}
	return strings.Join(parts, ",") + "|" + strconv.Itoa(calls)
}

// TestSortIsV8 compares Array.prototype.sort of the runtime with the sort of
// Chromium, for comparators that are not consistent. The result and the
// number of comparator calls must be the same. It also counts the cases in
// which the sort of goja gives another result: the reason for the
// replacement.
func TestSortIsV8(t *testing.T) {
	e := loadEngine(t)
	run := func(opts runtimeOptions) (wrong int, first string) {
		rt := testRuntime(t, opts)
		v, err := rt.vm.RunString(sortHarness)
		if err != nil {
			t.Fatal(err)
		}
		harness, _ := goja.AssertFunction(v)
		// cytoscape sorts one array when it loads.
		before := rt.sortCalls
		for i, c := range e.Sort {
			got, err := harness(goja.Undefined(), rt.vm.ToValue(c.Cmp), rt.vm.ToValue(c.Input))
			if err != nil {
				t.Fatalf("case %d: %v", i, err)
			}
			if want := sortCaseText(c.Output, c.Calls); got.String() != want {
				wrong++
				if first == "" {
					first = fmt.Sprintf("case %d, %s, %d elements:\n got  %s\n want %s", i, c.Cmp, len(c.Input), got, want)
				}
			}
		}
		if !opts.gojaSort && rt.sortCalls-before != len(e.Sort) {
			t.Errorf("the host sort ran %d times for %d sorts", rt.sortCalls-before, len(e.Sort))
		}
		return wrong, first
	}
	if wrong, first := run(runtimeOptions{}); wrong != 0 {
		t.Errorf("%d of %d sorts differ from Chromium. %s", wrong, len(e.Sort), first)
	}
	wrong, _ := run(runtimeOptions{gojaSort: true})
	t.Logf("%d sorts are the sorts of Chromium. The sort of goja differs in %d of them (the result or the comparator calls).", len(e.Sort), wrong)
	if wrong == 0 {
		t.Error("the sort of goja agrees with Chromium in every case: the cases do not show the difference")
	}
}

// TestNumbersAndKeys checks the parts of goja that the layouts depend on and
// that the package does not replace: number to string, parseInt and
// parseFloat, and the order of object keys.
func TestNumbersAndKeys(t *testing.T) {
	e := loadEngine(t)
	rt := testRuntime(t, runtimeOptions{})
	vm := rt.vm
	fn := func(src string) goja.Callable {
		v, err := vm.RunString(src)
		if err != nil {
			t.Fatal(err)
		}
		f, _ := goja.AssertFunction(v)
		return f
	}
	bits := func(hex string) float64 {
		u, err := strconv.ParseUint(hex, 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		return math.Float64frombits(u)
	}
	hexOf := fn(`(function (x) {
	  var v = new DataView(new ArrayBuffer(8)); v.setFloat64(0, x);
	  var hi = v.getUint32(0).toString(16), lo = v.getUint32(4).toString(16);
	  return ('00000000' + hi).slice(-8) + ('00000000' + lo).slice(-8);
	})`)

	toString := fn(`(function (x) { return String(x); })`)
	for _, c := range e.Numbers {
		x := bits(c[0])
		got, err := toString(goja.Undefined(), vm.ToValue(x))
		if err != nil {
			t.Fatal(err)
		}
		if got.String() != c[1] {
			t.Errorf("String(%s): got %q, Chromium has %q", c[0], got, c[1])
		}
		if s := js.Str(x); s != c[1] {
			t.Errorf("js.Str(%s): got %q, Chromium has %q", c[0], s, c[1])
		}
	}

	keys := fn(`(function (order) { var o = {}; for (var i = 0; i < order.length; i++) { o[order[i]] = i; } return Object.keys(o).join('|'); })`)
	forIn := fn(`(function (order) { var o = {}; for (var i = 0; i < order.length; i++) { o[order[i]] = i; } var out = []; for (var k in o) { out.push(k); } return out.join('|'); })`)
	for i, c := range e.Keys {
		want := strings.Join(c[1], "|")
		for name, f := range map[string]goja.Callable{"Object.keys": keys, "for in": forIn} {
			got, err := f(goja.Undefined(), vm.ToValue(c[0]))
			if err != nil {
				t.Fatal(err)
			}
			if got.String() != want {
				t.Errorf("%s, case %d %v: got %s, Chromium has %s", name, i, c[0], got, want)
			}
		}
	}

	// A text that is the text of a number is parsed as the layouts do it:
	// parseInt of the number.
	parseInt := fn(`(function (text) { var n = Number(text); return parseInt(String(n) === text ? n : text); })`)
	parseFloat := fn(`(function (text) { return parseFloat(text); })`)
	for _, c := range e.Parse {
		for name, f := range map[string]struct {
			fn   goja.Callable
			want string
		}{"parseInt": {parseInt, c[1]}, "parseFloat": {parseFloat, c[2]}} {
			v, err := f.fn(goja.Undefined(), vm.ToValue(c[0]))
			if err != nil {
				t.Fatal(err)
			}
			got, err := hexOf(goja.Undefined(), v)
			if err != nil {
				t.Fatal(err)
			}
			want := f.want
			if math.IsNaN(bits(want)) {
				if !math.IsNaN(v.ToFloat()) {
					t.Errorf("%s(%q): got %v, Chromium has NaN", name, c[0], v)
				}
				continue
			}
			if got.String() != want {
				t.Errorf("%s(%q): got bits %s, Chromium has %s", name, c[0], got, want)
			}
		}
	}
	t.Logf("%d numbers, %d key orders, %d parses", len(e.Numbers), len(e.Keys), len(e.Parse))
}

// TestMathIsPackageJS checks that every Math function that is not exact by
// definition is the function of the package js inside the runtime, and that
// the exact functions of goja follow JavaScript.
func TestMathIsPackageJS(t *testing.T) {
	rt := testRuntime(t, runtimeOptions{})
	vm := rt.vm
	args := []float64{0, math.Copysign(0, -1), 1, -1, 0.5, 2, 3, 10, 100, 1e-9, 1e9, 0.1, 0.7, 1.5, 2.5, -2.5, 123.456, 1e300, math.Pi, math.E, 6.5, 37, 1000, 4.35, math.NaN(), math.Inf(1), math.Inf(-1)}
	// The arguments of RandomSeed.nextDouble of layout-base: sin(1), sin(2), ...
	for i := 1; i <= 400; i++ {
		args = append(args, float64(i))
	}
	same := func(a, b float64) bool {
		return math.Float64bits(a) == math.Float64bits(b) || (math.IsNaN(a) && math.IsNaN(b))
	}
	unary := map[string]func(float64) float64{
		"sin": js.Sin, "cos": js.Cos, "tan": js.Tan, "asin": js.Asin, "acos": js.Acos, "atan": js.Atan,
		"exp": js.Exp, "expm1": js.Expm1, "log": js.Log, "log1p": js.Log1p, "log2": js.Log2, "log10": js.Log10,
		"cbrt": js.Cbrt, "sinh": js.Sinh, "cosh": js.Cosh, "tanh": js.Tanh, "asinh": js.Asinh, "acosh": js.Acosh, "atanh": js.Atanh,
		// Exact by definition: goja computes them.
		"sqrt": math.Sqrt, "abs": math.Abs, "floor": math.Floor, "ceil": math.Ceil, "round": js.Round,
		"trunc": js.Trunc, "sign": js.Sign, "fround": js.Fround,
	}
	for name, want := range unary {
		f, ok := goja.AssertFunction(vm.Get("Math").ToObject(vm).Get(name))
		if !ok {
			t.Fatalf("Math.%s is not a function", name)
		}
		for _, x := range args {
			v, err := f(goja.Undefined(), vm.ToValue(x))
			if err != nil {
				t.Fatal(err)
			}
			if got := v.ToFloat(); !same(got, want(x)) {
				t.Errorf("Math.%s(%v): got %v (%#x), want %v (%#x)", name, x, got, math.Float64bits(got), want(x), math.Float64bits(want(x)))
				break
			}
		}
	}
	binary := map[string]func(a, b float64) float64{
		"atan2": js.Atan2, "pow": js.Pow,
		"hypot": func(a, b float64) float64 { return js.Hypot(a, b) },
		"max":   func(a, b float64) float64 { return js.Max(a, b) },
		"min":   func(a, b float64) float64 { return js.Min(a, b) },
	}
	for name, want := range binary {
		f, _ := goja.AssertFunction(vm.Get("Math").ToObject(vm).Get(name))
		for _, x := range args[:40] {
			for _, y := range args[:40] {
				v, err := f(goja.Undefined(), vm.ToValue(x), vm.ToValue(y))
				if err != nil {
					t.Fatal(err)
				}
				if got := v.ToFloat(); !same(got, want(x, y)) {
					t.Errorf("Math.%s(%v, %v): got %v (%#x), want %v (%#x)", name, x, y, got, math.Float64bits(got), want(x, y), math.Float64bits(want(x, y)))
				}
			}
		}
	}
	// The clock does not move.
	v, err := vm.RunString(`[Date.now(), new Date().getTime(), performance.now(), typeof window, window === globalThis].join()`)
	if err != nil {
		t.Fatal(err)
	}
	if v.String() != "1768478400000,1768478400000,0,object,true" {
		t.Errorf("the environment: %s", v)
	}
}

func triangle() ([]Node, []Edge) {
	return []Node{{ID: "a", Width: 60, Height: 30}, {ID: "b", Width: 40, Height: 30}, {ID: "c", Width: 50, Height: 40}},
		[]Edge{{ID: "ab", Start: "a", End: "b"}, {ID: "ac", Start: "a", End: "c"}}
}

// TestInputsThatCytoscapeRejects feeds graphs that are not valid. cytoscape
// throws for an element that it cannot add, as it does in mermaid, and the
// call returns that error. No call panics.
func TestInputsThatCytoscapeRejects(t *testing.T) {
	var seed uint32 = 1
	env := Env{Now: time.Now(), Random: func() float64 {
		seed = seed*1664525 + 1013904223
		return float64(seed) / 4294967296
	}}
	ctx := context.Background()
	wantError := func(what string, err error, text string) {
		t.Helper()
		var jsErr *Error
		if !errors.As(err, &jsErr) || !strings.Contains(jsErr.Message, text) {
			t.Errorf("%s: got %v, want an error with %q", what, err, text)
		}
	}

	res, err := CoseBilkent(ctx, env, nil, nil)
	if err != nil || len(res.Nodes) != 0 || len(res.Edges) != 0 {
		t.Errorf("empty graph: %v %v", res, err)
	}

	// A second node with the same id.
	nodes, edges := triangle()
	_, err = CoseBilkent(ctx, env, append(nodes, Node{ID: "a", Width: 10, Height: 10}), edges)
	wantError("a second node with the same id", err, "Can not create second element with ID `a`")
	// An edge to a node that does not exist.
	_, err = CoseBilkent(ctx, env, nodes, append(edges, Edge{ID: "ax", Start: "a", End: "x"}))
	wantError("an edge to a missing node", err, "nonexistent target `x`")
	// A second edge with the same id.
	_, err = CoseBilkent(ctx, env, nodes, append(edges, Edge{ID: "ab", Start: "b", End: "c"}))
	wantError("a second edge with the same id", err, "Can not create second element with ID `ab`")
	// An empty id.
	_, err = CoseBilkent(ctx, env, append(nodes, Node{ID: "", Width: 10, Height: 10}), edges)
	wantError("an empty id", err, "invalid string ID")

	// A self loop, an edge between two nodes in both directions, a node
	// without edges, and sizes that are not numbers or are zero.
	nodes, edges = triangle()
	nodes = append(nodes, Node{ID: "alone", Width: 0, Height: 0})
	edges = append(edges, Edge{ID: "aa", Start: "a", End: "a"}, Edge{ID: "ba", Start: "b", End: "a"})
	res, err = CoseBilkent(ctx, env, nodes, edges)
	if err != nil || len(res.Nodes) != 4 || len(res.Edges) != 4 {
		t.Errorf("a self loop and a node without edges: %+v %v", res, err)
	}
	nodes[1].Width, nodes[2].Height = math.NaN(), math.Inf(1)
	if res, err = CoseBilkent(ctx, env, nodes, edges); err != nil {
		t.Logf("sizes that are not numbers: %v", err)
	} else if len(res.Nodes) != 4 {
		t.Errorf("sizes that are not numbers: %+v", res)
	}

	// An architecture edge to a node that does not exist, and a group
	// inside itself.
	in := &ArchitectureInput{
		Config:   ArchitectureConfig{IconSize: 80, FontSize: 16, Padding: 40, IdealEdgeLengthMultiplier: 1.5, EdgeElasticity: 0.45, NodeSeparation: 75, NumIter: 2500, Seed: 1},
		Groups:   []ArchitectureNode{{ID: "g", In: "g"}},
		Services: []ArchitectureNode{{ID: "s1", In: "g"}, {ID: "s2"}},
		Edges:    []ArchitectureEdge{{LhsID: "s1", RhsID: "s2", LhsDir: "R", RhsDir: "L"}, {LhsID: "s1", RhsID: "nope", LhsDir: "R", RhsDir: "L"}, {LhsID: "s1", RhsID: "s2", LhsDir: "B", RhsDir: "T"}},
	}
	_, err = Architecture(ctx, env, in)
	wantError("an architecture edge to a missing node", err, "nonexistent target `nope`")
	in.Edges = in.Edges[:1]
	in.Edges = append(in.Edges, ArchitectureEdge{LhsID: "s1", RhsID: "s2", LhsDir: "B", RhsDir: "T"})
	_, err = Architecture(ctx, env, in)
	wantError("two architecture edges between the same nodes", err, "Can not create second element with ID `s1-s2`")
	in.Edges = in.Edges[:1]
	ares, err := Architecture(ctx, env, in)
	if err != nil || len(ares.Edges) != 1 || ares.Edges[0].Index != 0 || len(ares.Nodes) != 3 {
		t.Errorf("a group inside itself: %+v %v", ares, err)
	}

	// A label needs MeasureText.
	in.Services[0].Title = "Service"
	_, err = Architecture(ctx, env, in)
	if err == nil || !strings.Contains(err.Error(), "Env.MeasureText is nil") {
		t.Errorf("a label without MeasureText: %v", err)
	}

	// A layout that does not exist is a JavaScript error.
	opts := js.NewObj[any]()
	opts.Set("name", "no-such-layout")
	_, err = Run(ctx, env, &Graph{Layouts: []*js.Obj[any]{opts}})
	var jsErr *Error
	if !errors.As(err, &jsErr) {
		t.Errorf("unknown layout: %v", err)
	}

	// A value that JavaScript does not have.
	bad := js.NewObj[any]()
	bad.Set("name", struct{}{})
	if _, err = Run(ctx, env, &Graph{Layouts: []*js.Obj[any]{bad}}); err == nil {
		t.Error("a struct as an option value: no error")
	}
}

// TestRandomIsEnv checks that Math.random of a layout is Env.Random, and
// that a layout that needs it fails without it. The spread of nodes that
// have the same position draws from Math.random in fcose with randomize.
func TestRandomIsEnv(t *testing.T) {
	g := loadGolden(t)
	for _, rec := range g.Records {
		if rec.Layout != "fcose" || !rec.Architecture.Config.Randomize {
			continue
		}
		in := architectureInput(t, rec)
		// Seed 0: withSeededRandom keeps Math.random of the page.
		in.Config.Seed = 0
		page := newPageEnv(t, rec)
		if _, err := Architecture(context.Background(), page.env(), in); err != nil {
			t.Fatal(err)
		}
		if page.randomCalls == 0 {
			t.Skip("the layout does not draw random numbers")
		}
		t.Logf("%s with seed 0: %d Math.random calls", rec.Name, page.randomCalls)
		env := page.env()
		env.Random = nil
		_, err := Architecture(context.Background(), env, in)
		if err == nil || !strings.Contains(err.Error(), "Env.Random is nil") {
			t.Errorf("a random layout without Random: %v", err)
		}
		return
	}
	t.Skip("no fcose record with randomize")
}

// TestTimeout checks that a call ends when its context ends.
func TestTimeout(t *testing.T) {
	g := loadGolden(t)
	var rec *goldenRecord
	for _, r := range g.Records {
		if r.Layout == "cose-bilkent" && (rec == nil || len(r.Adds) > len(rec.Adds)) {
			rec = r
		}
	}
	nodes, edges := coseBilkentInput(t, rec)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	// The Go port ends before the time limit: the test is about the
	// interpreter.
	env := newPageEnv(t, rec).env()
	env.Mode = ModeGoja
	_, err := CoseBilkent(ctx, env, nodes, edges)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want a deadline error", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("the call ended after %v", d)
	}

	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if _, err := CoseBilkent(ctx, Env{}, nodes, edges); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled context: %v", err)
	}
}

// TestConcurrent runs layouts at the same time. Each one must give the
// result of the browser: no state is shared.
func TestConcurrent(t *testing.T) {
	g := loadGolden(t)
	var recs []*goldenRecord
	for _, r := range g.Records {
		if len(r.Adds) <= 12 {
			recs = append(recs, r)
		}
	}
	if len(recs) > 16 {
		recs = recs[:16]
	}
	var wg sync.WaitGroup
	errs := make(chan string, 4*len(recs))
	for round := 0; round < 4; round++ {
		for _, rec := range recs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				page := newPageEnv(t, rec)
				d := &diff{}
				switch rec.Layout {
				case "cose-bilkent":
					nodes, edges := coseBilkentInput(t, rec)
					res, err := CoseBilkent(context.Background(), page.env(), nodes, edges)
					if err != nil {
						errs <- rec.Name + ": " + err.Error()
						return
					}
					checkCoseBilkent(rec, res, d)
				case "fcose":
					res, err := Architecture(context.Background(), page.env(), architectureInput(t, rec))
					if err != nil {
						errs <- rec.Name + ": " + err.Error()
						return
					}
					checkArchitecture(rec, res, d)
				}
				if len(d.list) > 0 {
					errs <- rec.Name + ": " + d.list[0]
				}
			}()
		}
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// TestSettle runs the timers and frames that wait after a layout. In the
// browser they run after mermaid has read the result, and they do not change
// it: the renderer is destroyed, so its loops end.
func TestSettle(t *testing.T) {
	nodes, edges := triangle()
	graph := func(settle int) *Graph {
		// The style of the cose-bilkent setup of mermaid. With the default
		// style an edge is a haystack edge, which has no start and end point
		// in the renderer scratch.
		edgeStyle := js.NewObj[any]()
		edgeStyle.Set("curve-style", "bezier")
		rule := js.NewObj[any]()
		rule.Set("selector", "edge")
		rule.Set("style", edgeStyle)
		g := &Graph{Style: []any{rule}, DataDimensions: true, Settle: settle}
		for _, n := range nodes {
			data := js.NewObj[any]()
			data.Set("id", n.ID)
			data.Set("width", n.Width)
			data.Set("height", n.Height)
			g.Elements = append(g.Elements, Element{Group: "nodes", Data: data, Position: &Point{}})
		}
		for _, e := range edges {
			data := js.NewObj[any]()
			data.Set("id", e.ID)
			data.Set("source", e.Start)
			data.Set("target", e.End)
			g.Elements = append(g.Elements, Element{Group: "edges", Data: data})
		}
		opts := js.NewObj[any]()
		opts.Set("name", "cose-bilkent")
		opts.Set("quality", "proof")
		opts.Set("animate", false)
		g.Layouts = []*js.Obj[any]{opts}
		return g
	}
	env := Env{Now: time.Now(), Random: func() float64 { return 0.25 }}
	plain, err := Run(context.Background(), env, graph(0))
	if err != nil {
		t.Fatal(err)
	}
	if plain.PendingTasks == 0 {
		t.Error("no timer or frame waits after the layout")
	}
	settled, err := Run(context.Background(), env, graph(1000))
	if err != nil {
		t.Fatal(err)
	}
	if settled.PendingTasks != 0 {
		t.Errorf("%d tasks wait after Settle", settled.PendingTasks)
	}
	for i := range plain.Nodes {
		if plain.Nodes[i] != settled.Nodes[i] {
			t.Errorf("node %d: %+v, after Settle %+v", i, plain.Nodes[i], settled.Nodes[i])
		}
	}
	for i := range plain.Edges {
		if plain.Edges[i] != settled.Edges[i] {
			t.Errorf("edge %d: %+v, after Settle %+v", i, plain.Edges[i], settled.Edges[i])
		}
		if math.IsNaN(plain.Edges[i].RScratch.Start.X) || plain.Edges[i].RScratch.Start != plain.Edges[i].SourceEndpoint {
			t.Errorf("edge %d: start %+v, source endpoint %+v", i, plain.Edges[i].RScratch.Start, plain.Edges[i].SourceEndpoint)
		}
	}
	t.Logf("%d tasks wait after the layout", plain.PendingTasks)
}

// TestBundleIsUnchanged checks that cytoscape.bundle.js is the file that
// tools/reference/gen-cytoscape-bundle.mjs wrote, with the versions that
// the package names.
func TestBundleIsUnchanged(t *testing.T) {
	data, err := os.ReadFile("cytoscape.bundle.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Mermaid string `json:"mermaid"`
		Bytes   int    `json:"bytes"`
		SHA256  string `json:"sha256"`
		Modules []struct {
			Package string `json:"package"`
			Version string `json:"version"`
		} `json:"modules"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(bundleSource))
	if got := hex.EncodeToString(sum[:]); got != manifest.SHA256 || len(bundleSource) != manifest.Bytes {
		t.Errorf("cytoscape.bundle.js has %d bytes and the hash %s, the manifest has %d and %s", len(bundleSource), got, manifest.Bytes, manifest.SHA256)
	}
	versions := map[string][]string{}
	for _, m := range manifest.Modules {
		versions[m.Package] = append(versions[m.Package], m.Version)
	}
	for pkg, want := range map[string]string{"cytoscape": CytoscapeVersion, "cytoscape-cose-bilkent": CoseBilkentVersion, "cytoscape-fcose": FcoseVersion} {
		if got := versions[pkg]; len(got) != 1 || got[0] != want {
			t.Errorf("the bundle has %s %v, the package names %s", pkg, got, want)
		}
	}
	// The version that the script reports.
	rt := testRuntime(t, runtimeOptions{})
	if got := rt.api.Get("version").String(); got != CytoscapeVersion {
		t.Errorf("cytoscape.version is %s", got)
	}
}
