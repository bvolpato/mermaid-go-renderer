package cytoscape

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native"
	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/nativetest"
)

// traceSource is the debug build of the glue: hooks that report the state of
// a layout of the JavaScript at stage boundaries. The test reads the file,
// because the test data is not in the module (see testdata/go.mod).
func traceSource() (string, error) {
	data, err := os.ReadFile(filepath.Join("testdata", "trace.js"))
	return string(data), err
}

// tracePatch returns the runtime option that loads the hooks. emit receives
// each record.
func tracePatch(emit func(stage string, values []float64)) func(rt *jsRuntime, modules goja.Value) error {
	return func(rt *jsRuntime, modules goja.Value) error {
		source, err := traceSource()
		if err != nil {
			return err
		}
		v, err := rt.vm.RunScript("trace.js", source)
		if err != nil {
			return fmt.Errorf("trace.js: %w", err)
		}
		fn, ok := goja.AssertFunction(v)
		if !ok {
			return fmt.Errorf("trace.js is not a function")
		}
		host := func(call goja.FunctionCall) goja.Value {
			stage := call.Argument(0).String()
			arr := call.Argument(1).ToObject(rt.vm)
			n := int(arr.Get("length").ToInteger())
			values := make([]float64, n)
			for i := range values {
				values[i] = arr.Get(strconv.Itoa(i)).ToFloat()
			}
			emit(stage, values)
			return goja.Undefined()
		}
		_, err = fn(goja.Undefined(), modules, rt.vm.ToValue(host))
		return err
	}
}

// coseBilkentResultValues is the record cy.result of a result.
func coseBilkentResultValues(res *CoseBilkentResult) []float64 {
	var values []float64
	for _, n := range res.Nodes {
		values = append(values, n.X, n.Y)
	}
	for _, e := range res.Edges {
		values = append(values, e.StartX, e.StartY, e.MidX, e.MidY, e.EndX, e.EndY)
	}
	return values
}

// sideBySide is the outcome of one input in the two implementations.
type sideBySide struct {
	// skip tells that the JavaScript did not end within its time limit:
	// the input has no oracle, and the outcome says nothing about the port.
	skip string
	// reason is the reason of the port when it does not cover the input.
	reason string
	// diff is the first difference, or "".
	diff string
	// records is the number of trace records that were compared.
	records []nativetest.TraceRecord
	// nativeTime and gojaTime are the times of the two runs. The cases of a
	// test run at the same time, so the times are for the ratio only:
	// TestNativeTimes measures CPU time.
	nativeTime, gojaTime time.Duration
}

// coseBilkentSideBySide runs the JavaScript with the debug hooks and the Go
// port with its trace, and compares each trace record, the result and the
// number of Math.random calls.
func coseBilkentSideBySide(nodes []Node, edges []Edge, seed uint32) sideBySide {
	var out sideBySide
	jsRandom := nativetest.NewMulberry32(seed, 0)
	jsTrace := &nativetest.Tracer{RandomCalls: func() int { return jsRandom.Calls }}
	start := time.Now()
	want, err := coseBilkent(context.Background(), Env{Random: jsRandom.Random, Now: time.Unix(0, 0)}, runtimeOptions{patch: tracePatch(jsTrace.Trace)}, nodes, edges)
	out.gojaTime = time.Since(start)
	if noOracle(err) {
		out.skip = err.Error()
		return out
	}
	if err == nil {
		jsTrace.Trace("cy.result", coseBilkentResultValues(want))
	}

	goRandom := nativetest.NewMulberry32(seed, 0)
	goTrace := &nativetest.Tracer{RandomCalls: func() int { return goRandom.Calls }}
	start = time.Now()
	got, reason := nativeCoseBilkent(newNativeRun(context.Background()), Env{Random: goRandom.Random, trace: goTrace.Trace}, nodes, edges)
	out.nativeTime = time.Since(start)
	if reason != "" {
		out.reason = reason
		return out
	}
	if err != nil {
		out.diff = "the port has a result, and the JavaScript fails: " + err.Error()
		return out
	}
	if at, text := nativetest.FirstDifference(goTrace.Records, jsTrace.Records); at >= 0 {
		out.diff = "trace: " + text
		return out
	}
	if d := diffCoseBilkent(got, want); d != "" {
		out.diff = d
		return out
	}
	if goRandom.Calls != jsRandom.Calls {
		out.diff = fmt.Sprintf("%d Math.random calls, the JavaScript has %d", goRandom.Calls, jsRandom.Calls)
		return out
	}
	out.records = jsTrace.Records
	return out
}

// noOracle tells whether err is the end of the time limit of the
// interpreter, and not a result of the JavaScript. On a slow machine a large
// layout can take the interpreter longer than DefaultTimeout.
func noOracle(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		strings.Contains(err.Error(), "layout interrupted") || strings.Contains(err.Error(), "layout not started")
}

// sideTally counts the outcomes of a set of inputs.
type sideTally struct {
	total, exact, fallback, different int
	// skipped counts the inputs without an oracle.
	skipped              int
	reasons              map[string]int
	stages               map[string]int
	nativeTime, gojaTime time.Duration
}

func (ta *sideTally) add(t *testing.T, name string, r sideBySide) {
	ta.total++
	switch {
	case r.skip != "":
		ta.skipped++
		t.Logf("%s: no oracle, the JavaScript did not end: %s", name, r.skip)
	case r.reason != "":
		ta.fallback++
		if ta.reasons == nil {
			ta.reasons = map[string]int{}
		}
		ta.reasons[r.reason]++
	case r.diff != "":
		ta.different++
		if ta.different <= 5 {
			t.Errorf("%s: %s", name, r.diff)
		}
	default:
		ta.exact++
		ta.nativeTime += r.nativeTime
		ta.gojaTime += r.gojaTime
		if ta.stages == nil {
			ta.stages = map[string]int{}
		}
		for _, rec := range r.records {
			ta.stages[rec.Stage]++
		}
	}
}

func (ta *sideTally) report(t *testing.T, what string) {
	t.Helper()
	t.Logf("%s: %d inputs: %d native and bit-exact at every record, %d not covered, %d different, %d without an oracle", what, ta.total, ta.exact, ta.fallback, ta.different, ta.skipped)
	var reasons []string
	for r, n := range ta.reasons {
		reasons = append(reasons, fmt.Sprintf("%6d  %s", n, r))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(reasons)))
	for _, r := range reasons {
		t.Log(r)
	}
	var stages []string
	for s, n := range ta.stages {
		stages = append(stages, fmt.Sprintf("%s %d", s, n))
	}
	sort.Strings(stages)
	t.Logf("%s: trace records compared, by stage: %s", what, strings.Join(stages, ", "))
	if ta.exact > 0 {
		t.Logf("%s: time of the %d exact layouts, with the trace: native %v (%v each), JavaScript %v (%v each)", what, ta.exact,
			ta.nativeTime.Round(time.Microsecond), (ta.nativeTime / time.Duration(ta.exact)).Round(time.Microsecond),
			ta.gojaTime.Round(time.Millisecond), (ta.gojaTime / time.Duration(ta.exact)).Round(time.Millisecond))
	}
	if ta.different > 0 {
		t.Errorf("%s: %d native layouts differ from the JavaScript", what, ta.different)
	}
}

// The side by side tests run the interpreter, which is slow: a layout of 60
// elements takes it seconds, and more on a slow machine. So a default run
// has a small fixed set: the layouts of the corpus up to smallCorpus
// elements, and smallRandom random inputs up to a small size. The golden
// files of internal/cytoscape/native have the other sizes. A full run has
// every layout of the corpus and random inputs of every size: set
// CYTOSCAPE_RANDOM to the number of random inputs, or CYTOSCAPE_FULL=1.
const (
	smallRandom = 12
	// smallMindmap and smallArchitecture are the largest numbers of nodes
	// of a random input of a default run.
	smallMindmap      = 20
	smallArchitecture = 12
)

// fullRun tells whether the tests run their long variants.
func fullRun() bool {
	return os.Getenv("CYTOSCAPE_FULL") != "" || os.Getenv("CYTOSCAPE_RANDOM") != ""
}

// randomCount is the number of random inputs of a side by side test: the
// environment variable CYTOSCAPE_RANDOM, or smallRandom.
func randomCount() int {
	if s := os.Getenv("CYTOSCAPE_RANDOM"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	}
	return smallRandom
}

// randomSeedBase is the first seed of the random inputs: the environment
// variable CYTOSCAPE_SEED, or 1.
func randomSeedBase() uint32 {
	if s := os.Getenv("CYTOSCAPE_SEED"); s != "" {
		if n, err := strconv.ParseUint(s, 10, 32); err == nil {
			return uint32(n)
		}
	}
	return 1
}

// sideBySideAll runs fn for the indexes 0 to n-1 on all processors, and adds
// the outcomes to the tally in the order of the indexes.
func sideBySideAll(t *testing.T, ta *sideTally, n int, fn func(i int) (string, sideBySide)) {
	names := make([]string, n)
	results := make([]sideBySide, n)
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for i := 0; i < n; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			names[i], results[i] = fn(i)
		}()
	}
	wg.Wait()
	for i := range results {
		ta.add(t, names[i], results[i])
	}
}

// TestCoseBilkentSideBySide runs the Go port and the JavaScript on recorded
// layouts of the corpus and on new random inputs, and compares the state at
// every stage boundary bit for bit. A default run has the layouts of the
// corpus up to 20 elements and 12 random inputs up to 20 nodes. With
// CYTOSCAPE_RANDOM=N it has every layout of the corpus and N random inputs
// up to 200 nodes. CYTOSCAPE_SEED sets the first seed.
func TestCoseBilkentSideBySide(t *testing.T) {
	full := fullRun()
	var corpus sideTally
	var records []*goldenRecord
	for _, rec := range loadGolden(t).Records {
		if rec.Layout != "cose-bilkent" {
			continue
		}
		if !full && len(rec.Adds) > 20 {
			continue
		}
		records = append(records, rec)
	}
	sideBySideAll(t, &corpus, len(records), func(i int) (string, sideBySide) {
		nodes, edges := coseBilkentInput(t, records[i])
		return records[i].Name, coseBilkentSideBySide(nodes, edges, nativetest.PageSeed)
	})
	corpus.report(t, "cose-bilkent, corpus")
	if corpus.fallback > 0 {
		t.Errorf("the port does not cover %d mindmap layouts of the corpus", corpus.fallback)
	}

	var random sideTally
	n := randomCount()
	base := randomSeedBase()
	sideBySideAll(t, &random, n, func(i int) (string, sideBySide) {
		seed := base + uint32(i)
		in := nativetest.RandomMindmap(seed)
		if !full && len(in.Nodes) > smallMindmap {
			in.Nodes, in.Edges = truncateMindmap(in, smallMindmap)
		}
		nodes := make([]Node, len(in.Nodes))
		for k, x := range in.Nodes {
			nodes[k] = Node{ID: x.ID, Width: x.Width, Height: x.Height, X: x.X, Y: x.Y}
		}
		edges := make([]Edge, len(in.Edges))
		for k, x := range in.Edges {
			edges[k] = Edge{ID: x.ID, Start: x.Start, End: x.End}
		}
		return fmt.Sprintf("seed %d (%s, %d nodes)", seed, in.Kind, len(nodes)), coseBilkentSideBySide(nodes, edges, seed)
	})
	random.report(t, "cose-bilkent, random")
}

// architectureResultValues is the record cy.result of a result.
func architectureResultValues(res *ArchitectureResult) []float64 {
	var values []float64
	for _, n := range res.Nodes {
		if n.Type == "group" {
			b := n.BoundingBox
			values = append(values, b.X1, b.Y1, b.X2, b.Y2, b.W, b.H)
		} else {
			values = append(values, n.X, n.Y)
		}
	}
	for _, e := range res.Edges {
		values = append(values, e.SourceEndpoint.X, e.SourceEndpoint.Y, e.Midpoint.X, e.Midpoint.Y, e.TargetEndpoint.X, e.TargetEndpoint.Y)
	}
	return values
}

// measureLog is a MeasureText that records its calls.
type measureLog struct {
	measure func(font, text string) float64
	calls   []nativetest.Measure
}

func (m *measureLog) fn(font, text string) float64 {
	w := m.measure(font, text)
	m.calls = append(m.calls, nativetest.Measure{Font: font, Text: text, Width: nativetest.Num(w)})
	return w
}

// architectureSideBySide runs the JavaScript with the debug hooks and the Go
// port with its trace, and compares each trace record, the result, the
// Math.random calls and the calls of MeasureText.
func architectureSideBySide(in *ArchitectureInput, seed uint32, skip int, measure func(font, text string) float64) sideBySide {
	var out sideBySide
	jsRandom := nativetest.NewMulberry32(seed, skip)
	jsTrace := &nativetest.Tracer{RandomCalls: func() int { return jsRandom.Calls }}
	jsMeasure := &measureLog{measure: measure}
	start := time.Now()
	want, err := architecture(context.Background(), Env{Random: jsRandom.Random, Now: time.Unix(0, 0), MeasureText: jsMeasure.fn}, runtimeOptions{patch: tracePatch(jsTrace.Trace)}, in)
	out.gojaTime = time.Since(start)
	if noOracle(err) {
		out.skip = err.Error()
		return out
	}
	if err == nil {
		jsTrace.Trace("cy.result", architectureResultValues(want))
	}

	goRandom := nativetest.NewMulberry32(seed, skip)
	goTrace := &nativetest.Tracer{RandomCalls: func() int { return goRandom.Calls }}
	goMeasure := &measureLog{measure: measure}
	start = time.Now()
	got, reason := nativeArchitecture(newNativeRun(context.Background()), Env{Random: goRandom.Random, MeasureText: goMeasure.fn, trace: goTrace.Trace}, in)
	out.nativeTime = time.Since(start)
	if reason != "" {
		out.reason = reason
		return out
	}
	if err != nil {
		out.diff = "the port has a result, and the JavaScript fails: " + err.Error()
		return out
	}
	if at, text := nativetest.FirstDifference(goTrace.Records, jsTrace.Records); at >= 0 {
		out.diff = "trace: " + text
		return out
	}
	if d := diffArchitecture(got, want); d != "" {
		out.diff = d
		return out
	}
	if goRandom.Calls != jsRandom.Calls {
		out.diff = fmt.Sprintf("%d Math.random calls, the JavaScript has %d", goRandom.Calls, jsRandom.Calls)
		return out
	}
	if len(goMeasure.calls) != len(jsMeasure.calls) {
		out.diff = fmt.Sprintf("%d MeasureText calls, the JavaScript has %d", len(goMeasure.calls), len(jsMeasure.calls))
		return out
	}
	for i, c := range jsMeasure.calls {
		if g := goMeasure.calls[i]; g.Font != c.Font || g.Text != c.Text {
			out.diff = fmt.Sprintf("MeasureText call %d: %q with %q, the JavaScript has %q with %q", i, g.Text, g.Font, c.Text, c.Font)
			return out
		}
	}
	out.records = jsTrace.Records
	return out
}

// fromNativeInput converts an input of the Go port to the input of
// Architecture.
func fromNativeInput(in *native.ArchitectureInput) *ArchitectureInput {
	out := &ArchitectureInput{
		Config:    ArchitectureConfig(in.Config),
		Alignment: AlignmentConstraint(in.Alignment),
	}
	for _, n := range in.Groups {
		out.Groups = append(out.Groups, ArchitectureNode(n))
	}
	for _, n := range in.Services {
		out.Services = append(out.Services, ArchitectureNode(n))
	}
	for _, n := range in.Junctions {
		out.Junctions = append(out.Junctions, ArchitectureNode(n))
	}
	for _, e := range in.Edges {
		out.Edges = append(out.Edges, ArchitectureEdge(e))
	}
	for _, r := range in.RelativePlacement {
		out.RelativePlacement = append(out.RelativePlacement, RelativePlacement(r))
	}
	return out
}

// TestArchitectureSideBySide runs the Go port and the JavaScript on recorded
// layouts of the corpus and on new random inputs, and compares the state at
// every stage boundary bit for bit. A default run has the layouts of the
// corpus up to 15 elements and 12 random inputs up to 12 nodes. With
// CYTOSCAPE_RANDOM=N it has every layout of the corpus and N random inputs
// up to 44 nodes. CYTOSCAPE_SEED sets the first seed.
func TestArchitectureSideBySide(t *testing.T) {
	full := fullRun()
	var corpus sideTally
	var records []*goldenRecord
	for _, rec := range loadGolden(t).Records {
		if rec.Layout != "fcose" {
			continue
		}
		if !full && len(rec.Adds) > 15 {
			continue
		}
		records = append(records, rec)
	}
	sideBySideAll(t, &corpus, len(records), func(i int) (string, sideBySide) {
		rec := records[i]
		page := newPageEnv(t, rec)
		return rec.Name, architectureSideBySide(architectureInput(t, rec), nativetest.PageSeed, rec.RandomBefore, page.env().MeasureText)
	})
	corpus.report(t, "fcose, corpus")
	if corpus.fallback > 0 {
		t.Errorf("the port does not cover %d architecture layouts of the corpus", corpus.fallback)
	}

	// The random inputs: in a default run the first inputs that are small.
	var inputs []nativetest.Architecture
	var seeds []uint32
	n := randomCount()
	for seed := randomSeedBase(); len(inputs) < n; seed++ {
		a := nativetest.RandomArchitecture(seed)
		nodes := len(a.Input.Groups) + len(a.Input.Services) + len(a.Input.Junctions)
		if !full && nodes > smallArchitecture {
			continue
		}
		inputs = append(inputs, a)
		seeds = append(seeds, seed)
	}
	var random sideTally
	sideBySideAll(t, &random, n, func(i int) (string, sideBySide) {
		a := inputs[i]
		in := fromNativeInput(&a.Input)
		nodes := len(in.Groups) + len(in.Services) + len(in.Junctions)
		return fmt.Sprintf("seed %d (%s, %d nodes, %d edges)", seeds[i], a.Kind, nodes, len(in.Edges)), architectureSideBySide(in, seeds[i], 0, nativetest.FakeMeasure)
	})
	random.report(t, "fcose, random")
}
