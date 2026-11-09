package elk

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dop251/goja"
)

// TestEngineFile checks that the embedded script is the file of elkjs 0.9.3.
func TestEngineFile(t *testing.T) {
	sum := sha256.Sum256([]byte(workerSource))
	if got := hex.EncodeToString(sum[:]); got != engineSHA256 {
		t.Fatalf("elkjs/elk-worker.min.js has the SHA-256 %s, want %s", got, engineSHA256)
	}
	// The reference tree has the npm package, when it is installed.
	for _, name := range []string{"elk-worker.min.js", "elk.bundled.js"} {
		ref, err := os.ReadFile("../../tools/reference/node_modules/elkjs/lib/" + name)
		if err != nil {
			t.Logf("no reference copy of %s: %v", name, err)
			continue
		}
		if !bytes.Contains(ref, []byte(workerSource)) {
			t.Errorf("node_modules/elkjs/lib/%s does not contain the embedded script", name)
		}
	}
	if _, err := os.Stat("elkjs/LICENSE.md"); err != nil {
		t.Errorf("the license of elkjs is missing: %v", err)
	}
}

// TestEngineMathFunctions checks which Math members the script reads. The
// bridge gives exact results for this set. A new member in a later version
// of elkjs must be checked against V8 before it is added here.
func TestEngineMathFunctions(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range regexp.MustCompile(`\bMath\.([A-Za-z0-9_]+)`).FindAllStringSubmatch(workerSource, -1) {
		seen[m[1]] = true
	}
	var got []string
	for name := range seen {
		got = append(got, name)
	}
	sort.Strings(got)
	// abs, ceil, floor, max, min, round and sqrt are exact in IEEE 754, and
	// goja computes them as V8 does. LOG10E is a constant. acos, atan2, cos,
	// log, pow and sin come from fdlibm.go. random is the bridge's own.
	want := []string{"LOG10E", "abs", "acos", "atan2", "ceil", "cos", "floor", "log", "max", "min", "pow", "random", "round", "sin", "sqrt"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("the script reads Math.{%s}, want {%s}", strings.Join(got, ", "), strings.Join(want, ", "))
	}
}

// TestExactMathOfGoja checks the Math functions that the bridge leaves to
// goja, on the arguments where an implementation can go wrong.
func TestExactMathOfGoja(t *testing.T) {
	vm := goja.New()
	negZero := math.Copysign(0, -1)
	tests := []struct {
		expr string
		want float64
	}{
		{"Math.round(0.5)", 1}, {"Math.round(-0.5)", negZero}, {"Math.round(1.5)", 2}, {"Math.round(2.5)", 3}, {"Math.round(-1.5)", -1},
		{"Math.round(-2.5)", -2}, {"Math.round(0.49999999999999994)", 0}, {"Math.round(-0.2)", negZero}, {"Math.round(4503599627370497)", 4503599627370497},
		{"Math.round(-4503599627370495.5)", -4503599627370495}, {"Math.round(NaN)", math.NaN()}, {"Math.round(-0)", negZero},
		{"Math.max(0, -0)", 0}, {"Math.max(-0, 0)", 0}, {"Math.min(0, -0)", negZero}, {"Math.min(-0, 0)", negZero}, {"Math.max(1, NaN)", math.NaN()},
		{"Math.min(NaN, 1)", math.NaN()}, {"Math.max()", math.Inf(-1)}, {"Math.min()", math.Inf(1)}, {"Math.max(-Infinity, -1e308)", -1e308},
		{"Math.floor(-0.5)", -1}, {"Math.ceil(-0.5)", negZero}, {"Math.floor(-0)", negZero}, {"Math.ceil(0.2)", 1}, {"Math.abs(-0)", 0},
		{"Math.sqrt(2)", math.Sqrt2}, {"Math.sqrt(-0)", negZero}, {"Math.sqrt(1e-320)", math.Sqrt(1e-320)}, {"Math.LOG10E", 0.4342944819032518},
	}
	for _, tt := range tests {
		v, err := vm.RunString(tt.expr)
		if err != nil {
			t.Fatal(err)
		}
		if got := v.ToFloat(); !sameBits(got, tt.want) {
			t.Errorf("%s = %v, want %v", tt.expr, got, tt.want)
		}
	}
}

// TestStringHashPatch compares the Go function that replaces
// java.lang.String.hashCode with the function of the script.
func TestStringHashPatch(t *testing.T) {
	var p *patch
	for i := range patches {
		if patches[i].name == "String.hashCode" {
			p = &patches[i]
		}
	}
	if p == nil {
		t.Fatal("no patch String.hashCode")
	}
	rt := &jsRuntime{vm: goja.New()}
	if err := rt.vm.Set("native", rt.strHash); err != nil {
		t.Fatal(err)
	}
	// BFb is checkCriticalStringElementIndex: it throws for a bad index.
	if _, err := rt.vm.RunString("function BFb(i, n) { if (i < 0 || i >= n) throw new Error('index'); }\n" + p.old); err != nil {
		t.Fatal(err)
	}
	inputs := []string{
		`""`, `"a"`, `"org.eclipse.elk.layered.spacing.nodeNodeBetweenLayers"`, `"Größe"`, `"日本語"`, `"😀"`, `"\ud83d"`, `"\ude00x"`,
		`"\u0000￿"`, `"x".repeat(1000)`, `"é́".repeat(77)`, `String(1/3)`, `"a\nb"`,
	}
	for _, in := range inputs {
		v, err := rt.vm.RunString(fmt.Sprintf("[ohb(%s), native(%s)]", in, in))
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		pair := v.Export().([]any)
		if fmt.Sprint(pair[0]) != fmt.Sprint(pair[1]) {
			t.Errorf("hash of %s: the script has %v, Go has %v", in, pair[0], pair[1])
		}
	}
}

// TestUnpatchedEngine lays out graphs with the script as elkjs ships it,
// without patches. The result must be the result of node too: the patches
// change the speed only.
func TestUnpatchedEngine(t *testing.T) {
	worker, boot, err := compileSource(false)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := newRuntimeWith(worker, boot, fdlibmMath)
	if err != nil {
		t.Fatal(err)
	}
	cases := loadGolden(t)
	step := 24
	if testing.Short() {
		step = 120
	}
	n := 0
	for i, c := range cases {
		if i%step != 0 || c.Hang || c.Err != nil || c.Ms > 40 {
			continue
		}
		got, err := rt.run(context.Background(), string(c.In), true, false)
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		if want := c.want(false); got != string(want) {
			t.Errorf("%s: %s", c.Name, firstDiff([]byte(got), want))
		}
		n++
	}
	t.Logf("%d cases without patches", n)
}

func TestErrors(t *testing.T) {
	e := newTestEngine(Options{MaxRuntimes: 1})
	defer e.Close()
	ctx := context.Background()
	good := goldenCaseByName(t, "special-small")

	tests := []struct {
		name    string
		graph   string
		class   string
		message string
	}{
		{
			"unknown algorithm",
			`{"id":"root","layoutOptions":{"elk.algorithm":"nope"},"children":[{"id":"a","width":1,"height":1}]}`,
			"org.eclipse.elk.core.UnsupportedConfigurationException",
			"org.eclipse.elk.core.UnsupportedConfigurationException: Layout algorithm 'nope' not found for Root Node root",
		},
		{
			"missing id",
			`{"id":"root","children":[{"width":1,"height":1}]}`,
			"org.eclipse.elk.graph.json.JsonImportException",
			"org.eclipse.elk.graph.json.JsonImportException: Every element must have an id.",
		},
		{
			"edge to a missing node",
			`{"id":"root","children":[{"id":"a"}],"edges":[{"id":"e","sources":["a"],"targets":["zz"]}]}`,
			"org.eclipse.elk.graph.json.JsonImportException",
			"org.eclipse.elk.graph.json.JsonImportException: Referenced shape does not exist: zz",
		},
		{"null graph", `null`, "Error", "Missing mandatory parameter 'graph'."},
		{"false graph", `false`, "Error", "Missing mandatory parameter 'graph'."},
	}
	for _, tt := range tests {
		_, err := e.Layout(ctx, []byte(tt.graph))
		var ee *Error
		if !errors.As(err, &ee) {
			t.Errorf("%s: error %v, want *Error", tt.name, err)
			continue
		}
		if ee.Class != tt.class || ee.Message != tt.message {
			t.Errorf("%s: class %q, message %q\nwant class %q, message %q", tt.name, ee.Class, ee.Message, tt.class, tt.message)
		}
		if ee.Error() != "elk: "+tt.message {
			t.Errorf("%s: Error() = %q", tt.name, ee.Error())
		}
		// The same runtime must still give the layout of node.
		if err := good.check(e, false); err != nil {
			t.Errorf("after %s: %v", tt.name, err)
		}
	}
	// A JavaScript error that is not an ELK exception: the importer applies
	// the operator "in" to a string.
	_, err := e.Layout(ctx, []byte(`"x"`))
	var ee *Error
	if !errors.As(err, &ee) || ee.Class != "TypeError" || ee.Message == "" {
		t.Errorf("string graph: error %#v, want a TypeError", err)
	} else {
		t.Logf("string graph: %v", err)
	}
	if err := good.check(e, false); err != nil {
		t.Errorf("after a TypeError: %v", err)
	}
	if st := e.Stats(); st.Created != 1 || st.Discarded != 0 {
		t.Errorf("stats %+v: an ELK exception must not cost a runtime", st)
	}

	for _, bad := range []string{``, `{`, `{"id":"r"`, `{"id":"r"}x`, `{'id':'r'}`, "\xff"} {
		if _, err := e.Layout(ctx, []byte(bad)); !errors.Is(err, ErrInvalidJSON) {
			t.Errorf("input %q: error %v, want ErrInvalidJSON", bad, err)
		}
	}
}

// hangGraph is a graph for which elkjs does not terminate: the layering
// strategy STRETCH_WIDTH, which mermaid offers as elk.layeringStrategy.
func hangGraph(t testing.TB) []byte {
	t.Helper()
	for _, c := range loadGolden(t) {
		if c.Hang {
			return c.In
		}
	}
	t.Fatal("no golden case that does not terminate")
	return nil
}

func TestInterrupt(t *testing.T) {
	hang := hangGraph(t)
	good := goldenCaseByName(t, "special-small")

	t.Run("context deadline", func(t *testing.T) {
		e := newTestEngine(Options{MaxRuntimes: 1})
		defer e.Close()
		// The runtime is ready, so the deadline passes during the layout. On
		// a busy machine the deadline can pass before the layout starts:
		// then no runtime is lost, and the test tries a longer one.
		if err := e.Warm(context.Background(), 1); err != nil {
			t.Fatal(err)
		}
		for _, limit := range []time.Duration{500 * time.Millisecond, 3 * time.Second, 12 * time.Second} {
			ctx, cancel := context.WithTimeout(context.Background(), limit)
			start := time.Now()
			_, err := e.Layout(ctx, hang)
			cancel()
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error %v, want a deadline error", err)
			}
			if d := time.Since(start); d > limit+20*time.Second {
				t.Errorf("the interrupt took %v", d)
			}
			if e.Stats().Discarded == 1 {
				break
			}
		}
		if st := e.Stats(); st.Discarded != 1 || st.Idle != 0 {
			t.Errorf("stats %+v: the interrupted runtime must be dropped", st)
		}
		if err := good.check(e, false); err != nil {
			t.Errorf("layout after the interrupt: %v", err)
		}
		if st := e.Stats(); st.Created != 2 || st.Idle != 1 {
			t.Errorf("stats %+v: want a second runtime in the pool", st)
		}
	})

	t.Run("cancel", func(t *testing.T) {
		e := newTestEngine(Options{MaxRuntimes: 1})
		defer e.Close()
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(200 * time.Millisecond)
			cancel()
		}()
		if _, err := e.Layout(ctx, hang); !errors.Is(err, context.Canceled) {
			t.Fatalf("error %v, want context.Canceled", err)
		}
		if err := good.check(e, false); err != nil {
			t.Errorf("layout after the cancel: %v", err)
		}
	})

	t.Run("timeout option", func(t *testing.T) {
		// The limit is long enough for the first layout of a new runtime.
		e := newTestEngine(Options{MaxRuntimes: 1, Timeout: 4 * time.Second})
		defer e.Close()
		start := time.Now()
		if _, err := e.Layout(context.Background(), hang); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error %v, want a deadline error", err)
		}
		if d := time.Since(start); d < 4*time.Second {
			t.Errorf("the layout ended after %v, before the time limit", d)
		}
		if err := good.check(e, false); err != nil {
			t.Errorf("layout after the timeout: %v", err)
		}
		if st := e.Stats(); st.Discarded != 1 {
			t.Errorf("stats %+v: want one dropped runtime", st)
		}
	})

	t.Run("context done before the start", func(t *testing.T) {
		e := newTestEngine(Options{MaxRuntimes: 1})
		defer e.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := e.Layout(ctx, good.In); !errors.Is(err, context.Canceled) {
			t.Fatalf("error %v, want context.Canceled", err)
		}
		if st := e.Stats(); st.Created != 0 {
			t.Errorf("stats %+v: no runtime is needed", st)
		}
	})

	t.Run("wait for a runtime", func(t *testing.T) {
		e := newTestEngine(Options{MaxRuntimes: 1, Timeout: 2 * time.Second})
		defer e.Close()
		started := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			close(started)
			_, err := e.Layout(context.Background(), hang)
			done <- err
		}()
		<-started
		time.Sleep(100 * time.Millisecond)
		// The only runtime is busy. The second caller gives up first.
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		if _, err := e.Layout(ctx, good.In); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("error %v, want a deadline error", err)
		}
		if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("first layout: error %v, want a deadline error", err)
		}
		if err := good.check(e, false); err != nil {
			t.Errorf("layout after both: %v", err)
		}
	})
}

// TestStackOverflow checks the result of a recursion without end in the
// engine: the error of V8, and a runtime that is not used again. The test
// replaces the layout function, because no graph of the golden file makes
// ELK recurse without end.
func TestStackOverflow(t *testing.T) {
	rt, err := newRuntime(nil)
	if err != nil {
		t.Fatal(err)
	}
	fn, err := rt.vm.RunString("(function f(text) { return f(text) + 1; })")
	if err != nil {
		t.Fatal(err)
	}
	rt.layout, _ = goja.AssertFunction(fn)
	_, err = rt.run(context.Background(), `{"id":"r"}`, true, false)
	var ee *Error
	if !errors.As(err, &ee) || ee.Class != "RangeError" || ee.Message != stackOverflow {
		t.Fatalf("error %#v, want the RangeError of V8", err)
	}
	if !rt.broken {
		t.Error("the runtime is not marked as broken")
	}
	if _, err := rt.run(context.Background(), `{"id":"r"}`, true, false); err == nil {
		t.Error("a broken runtime ran a layout")
	}

	// A Go panic in the engine is an error too, not a crash.
	rt, err = newRuntime(nil)
	if err != nil {
		t.Fatal(err)
	}
	rt.layout = func(goja.Value, ...goja.Value) (goja.Value, error) { panic("boom") }
	if _, err := rt.run(context.Background(), `{"id":"r"}`, true, false); err == nil || !rt.broken {
		t.Errorf("error %v, broken %v: want an error and a broken runtime", err, rt.broken)
	}
}

func TestConcurrentLayouts(t *testing.T) {
	cases := loadGolden(t)
	var small []*goldenCase
	for i, c := range cases {
		if i%9 == 0 && !c.Hang && c.Ms < 8 {
			small = append(small, c)
		}
	}
	const maxRuntimes = 3
	e := newTestEngine(Options{MaxRuntimes: maxRuntimes})
	defer e.Close()
	var wg sync.WaitGroup
	var failed atomic.Int64
	callers := 16
	rounds := 3
	if testing.Short() {
		rounds = 1
	}
	for g := 0; g < callers; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < rounds; i++ {
				c := small[(g*7+i*13)%len(small)]
				if err := c.check(e, false); err != nil {
					failed.Add(1)
					t.Errorf("%s: %v", c.Name, err)
				}
			}
		}()
	}
	wg.Wait()
	st := e.Stats()
	if st.Created > maxRuntimes || st.Idle > maxRuntimes {
		t.Errorf("stats %+v: more than %d runtimes", st, maxRuntimes)
	}
	if st.Layouts != int64(callers*rounds) {
		t.Errorf("stats %+v: want %d layouts", st, callers*rounds)
	}
}

func TestWarmCloseAndIdle(t *testing.T) {
	good := goldenCaseByName(t, "special-small")
	e := newTestEngine(Options{MaxRuntimes: 2})
	if err := e.Warm(context.Background(), 5); err != nil {
		t.Fatal(err)
	}
	if st := e.Stats(); st.Idle != 2 || st.Created != 2 {
		t.Fatalf("stats after Warm %+v: want 2 warm runtimes", st)
	}
	if err := good.check(e, false); err != nil {
		t.Errorf("layout on a warmed runtime: %v", err)
	}
	// Warm again does nothing when the runtimes are there.
	if err := e.Warm(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if st := e.Stats(); st.Created != 2 || st.Idle != 2 {
		t.Errorf("stats %+v: the layout and the second Warm must use the warm runtimes", st)
	}
	e.Close()
	if _, err := e.Layout(context.Background(), good.In); !errors.Is(err, ErrClosed) {
		t.Errorf("error %v after Close, want ErrClosed", err)
	}
	if st := e.Stats(); st.Idle != 0 {
		t.Errorf("stats %+v after Close", st)
	}

	// IdleTimeout drops a runtime that no layout uses.
	e = newTestEngine(Options{MaxRuntimes: 2, IdleTimeout: 100 * time.Millisecond})
	defer e.Close()
	if err := good.check(e, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for e.Stats().Idle != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if st := e.Stats(); st.Idle != 0 {
		t.Errorf("stats %+v: the idle runtime must go after IdleTimeout", st)
	}
	if err := good.check(e, false); err != nil {
		t.Errorf("layout after the idle timeout: %v", err)
	}
	if st := e.Stats(); st.Created != 2 {
		t.Errorf("stats %+v: want a second runtime after the idle timeout", st)
	}
}

// TestHashMembers checks the handling of "$H": Layout removes the members
// that the engine adds, and keeps a member of the input.
func TestHashMembers(t *testing.T) {
	e := newTestEngine(Options{MaxRuntimes: 1})
	defer e.Close()
	ctx := context.Background()
	out, err := e.Layout(ctx, []byte(`{"id":"r","children":[{"id":"a","width":10,"height":10},{"id":"b","width":10,"height":10,"children":[{"id":"c","width":5,"height":5}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("$H")) {
		t.Errorf("the output has a $H member: %s", out)
	}
	// The key of the second node is written with an escape.
	out, err = e.Layout(ctx, []byte(`{"id":"r","children":[{"id":"a","$H":77,"width":10,"height":10},{"id":"b","$H":78,"width":10,"height":10},{"id":"c","width":10,"height":10}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(out, []byte(`"$H":`)); n != 2 || !bytes.Contains(out, []byte(`"$H":77`)) || !bytes.Contains(out, []byte(`"$H":78`)) {
		t.Errorf("want the two $H members of the input: %s", out)
	}
}

// TestDefaultMath lays out graphs that call Math.sin, cos, acos, atan2, log
// or pow with the functions that a program gets, the functions of
// internal/js for Chromium, and with the Go math package that goja has by
// default. It reports how many layouts change against the golden file, which
// was computed with fdlibm. The test fails only when a layout fails: the
// three sets of functions are allowed to give other bytes.
func TestDefaultMath(t *testing.T) {
	cases := loadGolden(t)
	perCategory := 5
	if testing.Short() {
		perCategory = 1
	}
	var counter mathCounter
	fdlibm := New(Options{Mode: ModeGoja, MaxRuntimes: 1, Math: counter.math(*fdlibmMath)})
	defer fdlibm.Close()
	chromium := New(Options{Mode: ModeGoja, MaxRuntimes: 1})
	defer chromium.Close()
	goMath := New(Options{Mode: ModeGoja, MaxRuntimes: 1, Math: &Math{Sin: math.Sin, Cos: math.Cos, Acos: math.Acos, Log: math.Log, Atan2: math.Atan2, Pow: math.Pow}})
	defer goMath.Close()

	sorted := append([]*goldenCase(nil), cases...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Ms < sorted[j].Ms })
	taken := map[string]int{}
	calling, chromiumDiffers, goDiffers := 0, 0, 0
	ctx := context.Background()
	for _, c := range sorted {
		name := mathCategory.FindString(c.Name)
		// These kinds are known to call the functions (see TestGolden).
		if !regexp.MustCompile(`radial|mrtree|box|rectpacking|disco|container|SPLINES|generic-force`).MatchString(name) {
			continue
		}
		if c.Hang || c.Err != nil || c.Ms > 20 || taken[name] >= perCategory {
			continue
		}
		counter = mathCounter{}
		if err := c.check(fdlibm, false); err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		if counter.total() == 0 {
			continue
		}
		taken[name]++
		calling++
		want := c.want(false)
		if got, err := chromium.Layout(ctx, c.In); err != nil {
			t.Errorf("%s with the Math of internal/js: %v", c.Name, err)
		} else if !bytes.Equal(got, want) {
			chromiumDiffers++
			t.Logf("%s: other bytes with the Math of internal/js: %s", c.Name, firstDiff(got, want))
		}
		if got, err := goMath.Layout(ctx, c.In); err != nil {
			t.Errorf("%s with the Go math package: %v", c.Name, err)
		} else if !bytes.Equal(got, want) {
			goDiffers++
		}
	}
	t.Logf("%d graphs that call these functions: %d have other bytes with the Math of internal/js than with fdlibm, %d with the Go math package", calling, chromiumDiffers, goDiffers)
	if calling == 0 {
		t.Error("no graph calls sin, cos, acos, atan2, log or pow")
	}
}

// TestNativeSort checks the two facts that make Array.prototype.sort safe.
// The script calls it in one place, a helper that sorts arrays of double with
// Double.compare. That order is total, so each correct sort has the same
// result. Each other sort of ELK is the merge sort of the Java runtime, which
// is part of the script. The sort of goja is stable in any case, as in V8.
func TestNativeSort(t *testing.T) {
	if n := strings.Count(workerSource, ".sort("); n != 1 {
		t.Errorf("the script calls .sort( in %d places, want 1", n)
	}
	if !strings.Contains(workerSource, "function bFb(a,b){a.sort(b)}") {
		t.Error("the helper that calls Array.prototype.sort has changed")
	}
	// Its two callers pass Double.compare: the sorted method of DoubleStream,
	// and the balanced layout of the Brandes-Koepf node placement, which
	// takes the two middle values of four candidates.
	if n := strings.Count(workerSource, "bFb("); n != 3 {
		t.Errorf("the helper has %d references, want its definition and 2 calls", n)
	}
	vm := goja.New()
	v, err := vm.RunString(`
		var a = [];
		for (var i = 0; i < 200; i++) a.push({ key: (i * 7919) % 5, pos: i });
		a.sort(function (x, y) { return x.key - y.key; });
		var ok = true;
		for (var i = 1; i < a.length; i++) {
			if (a[i - 1].key > a[i].key || (a[i - 1].key === a[i].key && a[i - 1].pos > a[i].pos)) ok = false;
		}
		// Double.compare of the script, on an array with both zeros and NaN.
		function cmp(x, y) { if (x < y) return -1; if (x > y) return 1; if (x == y) return x == 0 ? cmp(1 / x, 1 / y) : 0; return isNaN(x) ? (isNaN(y) ? 0 : 1) : -1; }
		var d = [3, 0, -0, NaN, -1, 0, -Infinity, 1e-300, -0, 2.5];
		d.sort(cmp);
		ok && d.map(function (x) { return Object.is(x, -0) ? '-0' : String(x); }).join(' ');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := v.String(), "-Infinity -1 -0 -0 0 0 1e-300 2.5 3 NaN"; got != want {
		t.Errorf("sort in goja: %q, want %q", got, want)
	}
}

func TestDecode(t *testing.T) {
	c := goldenCaseByName(t, "special-small")
	root, err := Decode(c.want(false))
	if err != nil {
		t.Fatal(err)
	}
	if root.ID != "root" || root.Width != 195 || root.Height != 431 || len(root.Children) != 3 || len(root.Edges) != 2 {
		t.Fatalf("root %+v", root)
	}
	a := root.Children[0]
	if a.ID != "A" || a.X != 21.5 || a.Y != 12 || a.Width != 152 || a.Height != 45 {
		t.Errorf("node A %+v", a)
	}
	sg := root.Children[2]
	if sg.ID != "sg" || len(sg.Children) != 1 || sg.Children[0].ID != "E" || sg.Children[0].X != 20 || sg.Children[0].Y != 40 {
		t.Errorf("subgraph %+v", sg)
	}
	e2 := root.Edges[1]
	if e2.ID != "e2" || e2.Container != "root" || len(e2.Sources) != 1 || e2.Sources[0] != "B" || e2.Targets[0] != "E" || len(e2.Sections) != 1 {
		t.Fatalf("edge e2 %+v", e2)
	}
	s := e2.Sections[0]
	if s.ID != "e2_s0" || s.StartPoint != (Point{97.5, 248}) || s.EndPoint != (Point{97.5, 354}) || s.IncomingShape != "B" || s.OutgoingShape != "E" {
		t.Errorf("section %+v", s)
	}
	if l := e2.Labels[0]; l.Text != "Yes" || l.X != 72.5 || l.Y != 268 || l.Width != 23 || l.Height != 21 {
		t.Errorf("label %+v", l)
	}

	// Integer ids, bend points and junction points.
	n, err := Decode([]byte(`{"id":0,"children":[{"id":1,"x":1,"y":2,"ports":[{"id":"p","x":3,"y":4,"labels":[{"text":"t","x":5}]}]}],"edges":[{"id":3,"sources":[1],"targets":[2],"sections":[{"id":"3_s0","startPoint":{"x":1,"y":2},"endPoint":{"x":3,"y":4},"bendPoints":[{"x":5,"y":6},{"x":7,"y":8}],"incomingShape":1,"outgoingShape":2}],"junctionPoints":[{"x":9,"y":10}],"container":0}]}`))
	if err != nil {
		t.Fatal(err)
	}
	ed := n.Edges[0]
	if n.ID != "0" || n.Children[0].ID != "1" || ed.ID != "3" || ed.Sources[0] != "1" || ed.Container != "0" || ed.Sections[0].IncomingShape != "1" {
		t.Errorf("integer ids: %+v %+v", n, ed)
	}
	if len(ed.Sections[0].BendPoints) != 2 || ed.Sections[0].BendPoints[1] != (Point{7, 8}) || ed.JunctionPoints[0] != (Point{9, 10}) {
		t.Errorf("points: %+v", ed)
	}
	if p := n.Children[0].Ports[0]; p.ID != "p" || p.X != 3 || p.Labels[0].Text != "t" || p.Labels[0].X != 5 {
		t.Errorf("port %+v", p)
	}
	if _, err := Decode([]byte(`{"id":{}}`)); err == nil {
		t.Error("no error for an object as id")
	}

	// LayoutGraph on the shared engine.
	g, err := LayoutGraph(context.Background(), c.In)
	if err != nil {
		t.Fatal(err)
	}
	if g.Width != 195 || g.Children[1].ID != "B" || g.Children[1].X != 12 || g.Children[1].Y != 77 {
		t.Errorf("LayoutGraph: %+v", g)
	}
	out, err := Layout(context.Background(), c.In)
	if err != nil || !bytes.Equal(out, c.want(false)) {
		t.Errorf("Layout on the shared engine: %v", err)
	}
}
