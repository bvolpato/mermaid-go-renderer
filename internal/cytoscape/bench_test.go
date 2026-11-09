package cytoscape

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/dop251/goja"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/nativetest"
)

// BenchmarkCompile is the cost that a process pays one time: parse and
// compile the three scripts.
func BenchmarkCompile(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, s := range []struct{ name, src string }{{"cytoscape.bundle.js", bundleSource}, {"env.js", envSource}, {"glue.js", glueSource}} {
			if _, err := goja.Compile(s.name, s.src, false); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkNewRuntime is the cost that each call pays before the layout: a
// new realm with the environment and cytoscape loaded.
func BenchmarkNewRuntime(b *testing.B) {
	if err := compile(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := newRuntime(Env{}, runtimeOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

// benchRecords are layouts of the corpus, from small to large.
var benchRecords = []string{
	"mindmap/pr-check/simple-mindmap",
	"mindmap/nightly-check/branches-with-shapes-and-labels",
	"mindmap/nightly-check/level-2-nodes-exceeding-11",
	"architecture/pr-check/should-render-a-simple-architecture-diagram-with-groups",
	"architecture/pr-check/should-render-a-deterministic-layout-for-a-complex-deeply-nested-diagram",
}

func runRecord(t testing.TB, rec *goldenRecord, mode Mode) {
	page := newPageEnv(t, rec)
	env := page.env()
	env.Mode = mode
	var err error
	if rec.Layout == "cose-bilkent" {
		nodes, edges := coseBilkentInput(t, rec)
		_, err = CoseBilkent(context.Background(), env, nodes, edges)
	} else {
		_, err = Architecture(context.Background(), env, architectureInput(t, rec))
	}
	if err != nil {
		t.Fatal(err)
	}
}

// BenchmarkLayout is one call: the Go port, and the JavaScript in a new
// runtime.
func BenchmarkLayout(b *testing.B) {
	g := loadGolden(b)
	for _, name := range benchRecords {
		for _, rec := range g.Records {
			if rec.Name != name {
				continue
			}
			for _, mode := range []Mode{ModeNative, ModeGoja} {
				b.Run(name+"/"+string(mode), func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						runRecord(b, rec, mode)
					}
				})
			}
		}
	}
}

// TestTimes reports the CPU time of the compile step, of a new runtime and
// of every layout of the corpus, in the Go port and in the interpreter, and
// the same for larger random inputs. It runs with CYTOSCAPE_TIMES=1. CPU
// time is the measure, because the wall clock depends on what else the
// machine does.
func TestTimes(t *testing.T) {
	if os.Getenv("CYTOSCAPE_TIMES") == "" {
		t.Skip("set CYTOSCAPE_TIMES=1 to measure")
	}
	g := loadGolden(t)
	cpu := func(fn func()) time.Duration {
		start := processCPU()
		fn()
		return processCPU() - start
	}
	t.Logf("compile: %v", cpu(func() {
		if err := compile(); err != nil {
			t.Fatal(err)
		}
	}))
	const n = 20
	t.Logf("new runtime: %v", cpu(func() {
		for i := 0; i < n; i++ {
			if _, err := newRuntime(Env{}, runtimeOptions{}); err != nil {
				t.Fatal(err)
			}
		}
	})/n)
	type timed struct {
		name         string
		n            int
		goja, native time.Duration
	}
	report := func(what string, list []timed) {
		if len(list) == 0 {
			return
		}
		byGoja := append([]timed(nil), list...)
		sort.Slice(byGoja, func(i, j int) bool { return byGoja[i].goja < byGoja[j].goja })
		byNative := append([]timed(nil), list...)
		sort.Slice(byNative, func(i, j int) bool { return byNative[i].native < byNative[j].native })
		var totalGoja, totalNative time.Duration
		for _, x := range list {
			totalGoja += x.goja
			totalNative += x.native
		}
		last := len(list) - 1
		t.Logf("%s: %d layouts. interpreter: CPU total %v, median %v, max %v. native: CPU total %v, median %v, max %v. ratio of the totals: %.0f",
			what, len(list),
			totalGoja.Round(time.Millisecond), byGoja[len(list)/2].goja.Round(time.Millisecond), byGoja[last].goja.Round(time.Millisecond),
			totalNative.Round(time.Microsecond), byNative[len(list)/2].native.Round(time.Microsecond), byNative[last].native.Round(time.Microsecond),
			float64(totalGoja)/float64(max(totalNative, 1)))
		for _, x := range byGoja {
			t.Logf("  interpreter %8v  native %10v  %3d elements  %s", x.goja.Round(time.Millisecond), x.native.Round(time.Microsecond), x.n, x.name)
		}
	}
	// repeat is the number of native runs that give one time.
	const repeat = 20
	byLayout := map[string][]timed{}
	for _, rec := range g.Records {
		x := timed{name: rec.Name, n: len(rec.Adds)}
		x.goja = cpu(func() { runRecord(t, rec, ModeGoja) })
		before := ReadStats().Native
		x.native = cpu(func() {
			for i := 0; i < repeat; i++ {
				runRecord(t, rec, ModeNative)
			}
		}) / repeat
		if ReadStats().Native-before != repeat {
			t.Errorf("%s: the Go port did not lay the record out", rec.Name)
		}
		byLayout[rec.Layout] = append(byLayout[rec.Layout], x)
	}
	report("cose-bilkent, corpus", byLayout["cose-bilkent"])
	report("fcose, corpus", byLayout["fcose"])

	// Larger inputs: trees of 50 to 200 nodes, and architecture inputs of
	// 20 to 44 nodes.
	var large []timed
	for seed := uint32(1); len(large) < 9 && seed < 5000; seed++ {
		m := nativetest.RandomMindmap(seed)
		want := []int{50, 100, 200}[len(large)/3]
		if m.Kind != "tree" || len(m.Nodes) < want*8/10 || len(m.Nodes) > want {
			continue
		}
		nodes := make([]Node, len(m.Nodes))
		for k, x := range m.Nodes {
			nodes[k] = Node{ID: x.ID, Width: x.Width, Height: x.Height}
		}
		edges := make([]Edge, len(m.Edges))
		for k, x := range m.Edges {
			edges[k] = Edge{ID: x.ID, Start: x.Start, End: x.End}
		}
		x := timed{name: fmt.Sprintf("random tree, seed %d", seed), n: len(nodes) + len(edges)}
		x.goja = cpu(func() {
			if _, err := CoseBilkent(context.Background(), Env{Mode: ModeGoja}, nodes, edges); err != nil {
				t.Fatal(err)
			}
		})
		x.native = cpu(func() {
			for i := 0; i < repeat; i++ {
				if _, reason := nativeCoseBilkent(newNativeRun(context.Background()), Env{}, nodes, edges); reason != "" {
					t.Fatal(reason)
				}
			}
		}) / repeat
		large = append(large, x)
	}
	report("cose-bilkent, larger inputs", large)
	large = nil
	for seed := uint32(1); len(large) < 8 && seed < 5000; seed++ {
		a := nativetest.RandomArchitecture(seed)
		in := fromNativeInput(&a.Input)
		nodes := len(in.Groups) + len(in.Services) + len(in.Junctions)
		if a.Kind != "grid" || nodes < 20 || in.Config.Seed == 0 {
			continue
		}
		env := Env{MeasureText: nativetest.FakeMeasure}
		if _, reason := nativeArchitecture(newNativeRun(context.Background()), env, in); reason != "" {
			continue
		}
		x := timed{name: fmt.Sprintf("random architecture, seed %d", seed), n: nodes + len(in.Edges)}
		x.goja = cpu(func() {
			gojaEnv := env
			gojaEnv.Mode = ModeGoja
			if _, err := Architecture(context.Background(), gojaEnv, in); err != nil {
				t.Fatal(err)
			}
		})
		x.native = cpu(func() {
			for i := 0; i < repeat; i++ {
				nativeArchitecture(newNativeRun(context.Background()), env, in)
			}
		}) / repeat
		large = append(large, x)
	}
	report("fcose, larger inputs", large)
}

// TestWhatTheReplacementsAreFor runs every layout of the corpus with the
// Math functions of goja, and with the sort of goja, and counts the layouts
// that then differ from the browser. It runs with CYTOSCAPE_SENSITIVITY=1.
func TestWhatTheReplacementsAreFor(t *testing.T) {
	if os.Getenv("CYTOSCAPE_SENSITIVITY") == "" {
		t.Skip("set CYTOSCAPE_SENSITIVITY=1 to measure")
	}
	g := loadGolden(t)
	for _, c := range []struct {
		name string
		opts runtimeOptions
	}{
		{"the Math functions of the package js and the sort of V8", runtimeOptions{}},
		{"the Math functions of goja", runtimeOptions{gojaMath: true}},
		{"the sort of goja", runtimeOptions{gojaSort: true}},
	} {
		differ := map[string]int{}
		var names []string
		for _, rec := range g.Records {
			page := newPageEnv(t, rec)
			d := &diff{}
			if rec.Layout == "cose-bilkent" {
				nodes, edges := coseBilkentInput(t, rec)
				res, err := coseBilkent(context.Background(), page.env(), c.opts, nodes, edges)
				if err != nil {
					t.Fatal(err)
				}
				checkCoseBilkent(rec, res, d)
			} else {
				res, err := architecture(context.Background(), page.env(), c.opts, architectureInput(t, rec))
				if err != nil {
					t.Fatal(err)
				}
				checkArchitecture(rec, res, d)
			}
			if len(d.list) > 0 {
				differ[rec.Layout]++
				names = append(names, rec.Name)
			}
		}
		t.Logf("with %s: %d of 33 cose-bilkent layouts and %d of 20 fcose layouts differ from the browser %v", c.name, differ["cose-bilkent"], differ["fcose"], names)
	}
}
