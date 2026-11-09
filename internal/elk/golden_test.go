package elk

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// goldenCase is one line of testdata/golden.jsonl.gz, which
// tools/reference/gen-elk.mjs writes. Out is the result of
// JSON.stringify(await new ELK().layout(in)) from the genuine elkjs in node,
// as the first layout of a new module in a new V8 isolate.
type goldenCase struct {
	Name string `json:"name"`
	// Nodes is the number of nodes of the graph, without the root.
	Nodes int `json:"nodes"`
	// Ms is the time of a warm layout in V8, in milliseconds.
	Ms  float64         `json:"ms"`
	In  json.RawMessage `json:"in"`
	Out json.RawMessage `json:"out"`
	// Err is error.message when elkjs rejects the graph.
	Err *string `json:"err"`
	// Native marks an error that is not a Java exception of ELK. The text of
	// such an error comes from the JavaScript engine: V8 and goja word it
	// in different ways.
	Native bool `json:"native"`
	// Hang marks a graph for which elkjs does not terminate.
	Hang bool `json:"hang"`
	// Random is the number of Math.random calls of the layout.
	Random int `json:"random"`
	// KeepHash marks a graph whose nodes have "$H" members in the input.
	KeepHash bool `json:"keepHash"`
}

var (
	goldenOnce  sync.Once
	goldenCases []*goldenCase
	goldenByKey map[string]*goldenCase
	goldenErr   error
)

func loadGolden(t testing.TB) []*goldenCase {
	t.Helper()
	goldenOnce.Do(func() {
		f, err := os.Open("testdata/golden.jsonl.gz")
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
		sc := bufio.NewScanner(zr)
		sc.Buffer(make([]byte, 1<<20), 64<<20)
		goldenByKey = map[string]*goldenCase{}
		for sc.Scan() {
			c := &goldenCase{}
			if err := json.Unmarshal(sc.Bytes(), c); err != nil {
				goldenErr = fmt.Errorf("golden line %d: %w", len(goldenCases)+1, err)
				return
			}
			goldenCases = append(goldenCases, c)
			goldenByKey[c.Name] = c
		}
		goldenErr = sc.Err()
	})
	if goldenErr != nil {
		t.Fatal(goldenErr)
	}
	return goldenCases
}

func goldenCaseByName(t testing.TB, name string) *goldenCase {
	t.Helper()
	loadGolden(t)
	c := goldenByKey[name]
	if c == nil {
		t.Fatalf("no golden case %q", name)
	}
	return c
}

// stackOverflow is the message of V8 when ELK recurses without end.
const stackOverflow = "Maximum call stack size exceeded"

var hashMember = regexp.MustCompile(`,"\$H":\d+`)

// want is the expected output of Layout: the output of node without the
// "$H" members that elkjs adds. elkjs appends "$H" to a node object, so the
// member always follows another one.
func (c *goldenCase) want(fresh bool) []byte {
	if fresh || c.KeepHash {
		return c.Out
	}
	return hashMember.ReplaceAll(c.Out, nil)
}

// firstDiff describes where two outputs differ.
func firstDiff(got, want []byte) string {
	n := min(len(got), len(want))
	i := 0
	for i < n && got[i] == want[i] {
		i++
	}
	lo := max(0, i-80)
	clip := func(b []byte) string {
		hi := min(len(b), i+80)
		if lo >= hi {
			return ""
		}
		return string(b[lo:hi])
	}
	return fmt.Sprintf("lengths %d and %d, first difference at byte %d\n  got:  …%s…\n  want: …%s…", len(got), len(want), i, clip(got), clip(want))
}

// check runs one case and compares the result with node.
func (c *goldenCase) check(e *Engine, fresh bool) error {
	if c.Hang {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := e.Layout(ctx, c.In)
		if !errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("elkjs does not terminate for this graph, but Layout returned %v", err)
		}
		return nil
	}
	got, err := e.Layout(context.Background(), c.In)
	if c.Err != nil {
		if err == nil {
			return fmt.Errorf("no error, node has %q", *c.Err)
		}
		var ee *Error
		if !errors.As(err, &ee) {
			return fmt.Errorf("error %v is not *Error, node has %q", err, *c.Err)
		}
		if (!c.Native || *c.Err == stackOverflow) && ee.Message != *c.Err {
			return fmt.Errorf("error message %q, node has %q", ee.Message, *c.Err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("error %v, node has a layout", err)
	}
	if want := c.want(fresh); !bytes.Equal(got, want) {
		return errors.New(firstDiff(got, want))
	}
	return nil
}

// heavyMs splits the golden cases. A layout that takes longer than this in
// V8 takes from 10 seconds to minutes in goja: these are a few graphs with
// about 100 nodes or more, and the benchmark graphs with 50 and 150 nodes.
const heavyMs = 150

// selectCases picks the golden cases of a test run:
//
//	go test -short                 each fifth case that is fast in V8
//	go test                        each case that takes at most heavyMs in V8
//	ELK_GOLDEN=all go test -timeout 60m   each case
func selectCases(t testing.TB, cases []*goldenCase) []*goldenCase {
	var out []*goldenCase
	switch {
	case os.Getenv("ELK_GOLDEN") == "all":
		return cases
	case testing.Short():
		for i, c := range cases {
			if i%5 == 0 && c.Ms < 20 && !c.Hang {
				out = append(out, c)
			}
		}
	default:
		for _, c := range cases {
			if c.Ms <= heavyMs {
				out = append(out, c)
			}
		}
	}
	t.Logf("%d of %d golden cases. ELK_GOLDEN=all runs each case.", len(out), len(cases))
	return out
}

func testWorkers() int {
	return max(1, min(runtime.GOMAXPROCS(0), 8))
}

// newTestEngine returns an engine with the Math functions of the golden
// file (see fdlibm_ref_test.go), unless the options have their own.
func newTestEngine(o Options) *Engine {
	if o.Math == nil {
		o.Math = fdlibmMath
	}
	// These tests are for elkjs in the interpreter. The default mode is the
	// Go port, which native_check_test.go tests.
	if o.Mode == "" {
		o.Mode = ModeGoja
	}
	return New(o)
}

// mathCounter counts the calls of the Math functions of Options.Math.
type mathCounter struct {
	sin, cos, acos, log, atan2, pow int
	// init counts the calls that class initializers of the engine make:
	// Math.log(2), Math.pow(2, -65) and powers of integers for the tables of
	// the number parser. They do not depend on the graph, and they are exact.
	init int
}

func (c *mathCounter) total() int { return c.sin + c.cos + c.acos + c.log + c.atan2 + c.pow }

func (c *mathCounter) add(o mathCounter) {
	c.sin += o.sin
	c.cos += o.cos
	c.acos += o.acos
	c.log += o.log
	c.atan2 += o.atan2
	c.pow += o.pow
	c.init += o.init
}

// math wraps the functions of m.
func (c *mathCounter) math(m Math) *Math {
	isInt := func(x float64) bool { return x == math.Trunc(x) && math.Abs(x) < 1<<31 }
	return &Math{
		Sin:   func(x float64) float64 { c.sin++; return m.Sin(x) },
		Cos:   func(x float64) float64 { c.cos++; return m.Cos(x) },
		Acos:  func(x float64) float64 { c.acos++; return m.Acos(x) },
		Atan2: func(y, x float64) float64 { c.atan2++; return m.Atan2(y, x) },
		Log: func(x float64) float64 {
			if x == 2 {
				c.init++
			} else {
				c.log++
			}
			return m.Log(x)
		},
		Pow: func(x, y float64) float64 {
			if (isInt(x) && isInt(y) && y >= 0) || (x == 2 && y == -65) {
				c.init++
			} else {
				c.pow++
			}
			return m.Pow(x, y)
		},
	}
}

// mathCategory groups the golden cases by the algorithm that they run.
var mathCategory = regexp.MustCompile(`^(bench|mermaid-layered|mermaid-catalogue|mermaid-option|mermaid-stretch-width|mermaid-(?:TB|BT|LR|RL)-[A-Z]+|mermaid-container|mermaid-elk\.[A-Za-z]+|generic-[A-Za-z]+|special|error)`)

// TestGolden lays out each golden graph and compares the bytes with node.
// The cases run in a random order on several runtimes, so each layout
// follows other layouts on the same runtime: a state of the engine that
// leaks from one layout into the next fails the test.
//
// Each worker has its own runtime and counts the calls of Math.sin, cos,
// acos, atan2, log and pow for each graph. goja and V8 do not agree on the
// last bit of these functions, so the test also shows which layouts depend
// on the functions that the bridge installs. The layered algorithm with the
// options of mermaid must not call them at all.
func TestGolden(t *testing.T) {
	cases := loadGolden(t)
	if len(cases) < 500 {
		t.Fatalf("only %d golden cases", len(cases))
	}
	cases = selectCases(t, cases)
	order := rand.New(rand.NewSource(1)).Perm(len(cases))
	var next atomic.Int64
	var failed, hangs, overflows, errs, randoms atomic.Int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	type result struct {
		c     *goldenCase
		d     time.Duration
		calls mathCounter
	}
	var results []result
	var stats Stats
	start := time.Now()
	for w := 0; w < testWorkers(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var counter mathCounter
			// The time limit is long, so that a busy machine does not fail
			// the test. A graph for which elkjs does not terminate has its
			// own limit.
			e := New(Options{Mode: ModeGoja, MaxRuntimes: 1, Timeout: 20 * time.Minute, Math: counter.math(*fdlibmMath)})
			defer e.Close()
			for {
				i := int(next.Add(1)) - 1
				if i >= len(order) {
					break
				}
				c := cases[order[i]]
				switch {
				case c.Hang:
					hangs.Add(1)
				case c.Err != nil:
					errs.Add(1)
					if *c.Err == stackOverflow {
						overflows.Add(1)
					}
				}
				if c.Random > 0 {
					randoms.Add(1)
				}
				counter = mathCounter{}
				t0 := time.Now()
				err := c.check(e, false)
				mu.Lock()
				results = append(results, result{c, time.Since(t0), counter})
				if err != nil && failed.Add(1) <= 15 {
					t.Errorf("%s: %v", c.Name, err)
				}
				mu.Unlock()
			}
			st := e.Stats()
			mu.Lock()
			stats.Created += st.Created
			stats.Discarded += st.Discarded
			stats.Layouts += st.Layouts
			mu.Unlock()
		}()
	}
	wg.Wait()

	// ELK_GOLDEN_TIMES=1 lists the slowest cases.
	if os.Getenv("ELK_GOLDEN_TIMES") != "" {
		sort.Slice(results, func(i, j int) bool { return results[i].d > results[j].d })
		var total time.Duration
		for _, r := range results {
			total += r.d
		}
		for _, r := range results[:min(25, len(results))] {
			t.Logf("%10v  %s", r.d.Round(time.Millisecond), r.c.Name)
		}
		t.Logf("sum of the layout times: %v", total.Round(time.Second))
	}

	// The calls of the Math functions, by kind of graph.
	type group struct {
		cases, calling int
		calls          mathCounter
	}
	groups := map[string]*group{}
	var initCalls int
	for _, r := range results {
		name := mathCategory.FindString(r.c.Name)
		if name == "" {
			name = "other"
		}
		g := groups[name]
		if g == nil {
			g = &group{}
			groups[name] = g
		}
		g.cases++
		g.calls.add(r.calls)
		initCalls += r.calls.init
		if r.calls.total() > 0 {
			g.calling++
		}
		// mermaid sets no edge routing, so the default applies: ORTHOGONAL.
		if strings.HasPrefix(r.c.Name, "bench-") || strings.HasPrefix(r.c.Name, "mermaid-layered-") || strings.HasSuffix(r.c.Name, "-ORTHOGONAL") {
			if n := r.calls.total(); n > 0 {
				t.Errorf("%s: the layered algorithm with the options of mermaid made %d calls of Math functions that are not exact: %+v", r.c.Name, n, r.calls)
			}
		}
	}
	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		g := groups[name]
		t.Logf("%-26s %3d graphs, %3d call Math: sin %d, cos %d, acos %d, atan2 %d, log %d, pow %d",
			name, g.cases, g.calling, g.calls.sin, g.calls.cos, g.calls.acos, g.calls.atan2, g.calls.log, g.calls.pow)
	}
	t.Logf("class initializers made %d more calls (log(2), pow of integers)", initCalls)

	t.Logf("%d cases (%d errors, %d hangs, %d with Math.random), %d differ from node, %v, %d runtimes created, %d discarded",
		len(cases), errs.Load(), hangs.Load(), randoms.Load(), failed.Load(), time.Since(start).Round(time.Millisecond), stats.Created, stats.Discarded)
	if n := failed.Load(); n > 0 {
		t.Errorf("%d of %d cases differ from node", n, len(cases))
	}
	// Only an interrupted layout or a stack overflow may cost a runtime. An
	// ELK exception must leave the runtime in the pool.
	if want := hangs.Load() + overflows.Load(); stats.Discarded != want {
		t.Errorf("%d runtimes discarded, want %d (one for each interrupted layout and each stack overflow)", stats.Discarded, want)
	}
}

// TestGoldenFresh runs graphs with the option Fresh. The output must be the
// output of node byte for byte, with the "$H" members. A "$H" value is the
// number of objects that the engine hashed before, so equal values show that
// goja runs the same code path as V8, and not only that the result is equal.
func TestGoldenFresh(t *testing.T) {
	cases := loadGolden(t)
	var pick []*goldenCase
	step := 30
	if testing.Short() {
		step = 100
	}
	for i, c := range cases {
		if c.Hang || c.Ms > 40 {
			continue
		}
		if i%step == 0 || c.KeepHash || strings.HasPrefix(c.Name, "special-random-seed-zero") {
			pick = append(pick, c)
		}
	}
	e := newTestEngine(Options{MaxRuntimes: testWorkers(), Fresh: true})
	defer e.Close()
	var wg sync.WaitGroup
	var next, failed, withHash atomic.Int64
	for w := 0; w < testWorkers(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= len(pick) {
					return
				}
				c := pick[i]
				if bytes.Contains(c.Out, []byte(`"$H":`)) {
					withHash.Add(1)
				}
				if err := c.check(e, true); err != nil {
					if failed.Add(1) <= 10 {
						t.Errorf("%s: %v", c.Name, err)
					}
				}
			}
		}()
	}
	wg.Wait()
	t.Logf("%d cases in new runtimes, %d with $H members, %d differ from node", len(pick), withHash.Load(), failed.Load())
	if withHash.Load() == 0 {
		t.Error("no case has $H members")
	}
	if st := e.Stats(); st.Idle != 0 || int(st.Created) != len(pick) {
		t.Errorf("stats %+v: want one new runtime for each of the %d layouts and none kept", st, len(pick))
	}
}

// TestNoStateBetweenLayouts lays out graph A, then B, then A again on one
// runtime, for many pairs. The two results of A must be equal, and equal to
// node.
func TestNoStateBetweenLayouts(t *testing.T) {
	cases := loadGolden(t)
	var small []*goldenCase
	for _, c := range cases {
		if !c.Hang && c.Ms < 8 {
			small = append(small, c)
		}
	}
	pairs := 60
	if testing.Short() {
		pairs = 12
	}
	rng := rand.New(rand.NewSource(7))
	type pair struct{ a, b *goldenCase }
	var work []pair
	for i := 0; i < pairs; i++ {
		work = append(work, pair{small[rng.Intn(len(small))], small[rng.Intn(len(small))]})
	}
	var wg sync.WaitGroup
	var next, failed atomic.Int64
	for w := 0; w < testWorkers(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// One runtime for each worker, so that A, B and A run on the
			// same runtime.
			e := newTestEngine(Options{MaxRuntimes: 1})
			defer e.Close()
			for {
				i := int(next.Add(1)) - 1
				if i >= len(work) {
					return
				}
				p := work[i]
				ctx := context.Background()
				a1, err1 := e.Layout(ctx, p.a.In)
				_, _ = e.Layout(ctx, p.b.In)
				a2, err2 := e.Layout(ctx, p.a.In)
				ok := bytes.Equal(a1, a2) && (err1 == nil) == (err2 == nil)
				if ok && err1 != nil {
					ok = err1.Error() == err2.Error()
				}
				if !ok {
					if failed.Add(1) <= 10 {
						t.Errorf("A=%s, B=%s: the second layout of A differs from the first: %v / %v\n%s", p.a.Name, p.b.Name, err1, err2, firstDiff(a2, a1))
					}
					continue
				}
				if err := p.a.check(e, false); err != nil {
					if failed.Add(1) <= 10 {
						t.Errorf("A=%s after B=%s: %v", p.a.Name, p.b.Name, err)
					}
				}
				if st := e.Stats(); st.Created != 1 {
					t.Errorf("A=%s, B=%s: %d runtimes, want 1", p.a.Name, p.b.Name, st.Created)
				}
			}
		}()
	}
	wg.Wait()
	t.Logf("%d pairs, %d failures", len(work), failed.Load())
}

// TestGoldenCoverage checks that the golden file has the kinds of graphs
// that the package promises to cover.
func TestGoldenCoverage(t *testing.T) {
	cases := loadGolden(t)
	count := map[string]int{}
	maxNodes, minNodes := 0, 1<<30
	for _, c := range cases {
		in := string(c.In)
		maxNodes = max(maxNodes, c.Nodes)
		minNodes = min(minNodes, c.Nodes)
		for _, key := range []string{
			`"elk.direction":"DOWN"`, `"elk.direction":"UP"`, `"elk.direction":"LEFT"`, `"elk.direction":"RIGHT"`,
			`"elk.edgeRouting":"ORTHOGONAL"`, `"elk.edgeRouting":"POLYLINE"`, `"elk.edgeRouting":"SPLINES"`,
			`"elk.hierarchyHandling":"INCLUDE_CHILDREN"`, `"elk.hierarchyHandling":"SEPARATE_CHILDREN"`,
			`"elk.algorithm":"elk.layered"`, `"elk.algorithm":"elk.stress"`, `"elk.algorithm":"elk.force"`, `"elk.algorithm":"elk.mrtree"`,
			`"elk.algorithm":"elk.sporeOverlap"`, `"elk.algorithm":"elk.box"`, `"elk.algorithm":"elk.rectpacking"`, `"elk.algorithm":"elk.radial"`,
			`"elk.layered.nodePlacement.strategy":"BRANDES_KOEPF"`, `"elk.layered.nodePlacement.strategy":"NETWORK_SIMPLEX"`,
			`"elk.layered.nodePlacement.strategy":"LINEAR_SEGMENTS"`, `"elk.layered.nodePlacement.strategy":"SIMPLE"`,
			`"elk.layered.considerModelOrder.strategy":"NODES_AND_EDGES"`, `"elk.layered.considerModelOrder.strategy":"PREFER_EDGES"`,
			`"elk.layered.considerModelOrder.strategy":"PREFER_NODES"`, `"elk.layered.considerModelOrder.strategy":"NONE"`,
			`"elk.padding"`, `"ports"`, `"edgeLabels.placement"`, `"elk.portAlignment.default"`, `"elk.layered.layering.layerConstraint"`,
		} {
			if strings.Contains(in, key) {
				count[key]++
			}
		}
		if c.Hang {
			count["hang"]++
		}
		if c.Err != nil {
			count["error"]++
		}
		if c.Random > 0 {
			count["random"]++
		}
		if c.Nodes >= 100 {
			count["100 nodes or more"]++
		}
	}
	keys := make([]string, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("%4d  %s", count[k], k)
	}
	for _, k := range []string{`"elk.direction":"UP"`, `"elk.direction":"LEFT"`, `"elk.direction":"RIGHT"`, `"elk.edgeRouting":"POLYLINE"`, `"elk.edgeRouting":"SPLINES"`,
		`"elk.algorithm":"elk.stress"`, `"elk.algorithm":"elk.force"`, `"elk.algorithm":"elk.mrtree"`, `"elk.algorithm":"elk.sporeOverlap"`, `"elk.algorithm":"elk.box"`,
		`"elk.algorithm":"elk.rectpacking"`, `"elk.algorithm":"elk.radial"`, `"ports"`, "error", "random", "100 nodes or more"} {
		if count[k] == 0 {
			t.Errorf("no golden case with %s", k)
		}
	}
	if minNodes > 1 || maxNodes < 140 {
		t.Errorf("node counts from %d to %d, want from 1 or less to 140 or more", minNodes, maxNodes)
	}
	t.Logf("%d cases, %d to %d nodes", len(cases), minNodes, maxNodes)
}
