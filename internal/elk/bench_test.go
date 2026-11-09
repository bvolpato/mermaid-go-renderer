package elk

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"
)

// benchNames are the benchmark graphs of the golden file: mermaid flowcharts
// with subgraphs for the layered algorithm, with 5, 20, 50 and 150 leaf
// nodes. tools/reference/gen-elk.mjs --bench times the same graphs in V8.
var benchNames = []string{"bench-5", "bench-20", "bench-50", "bench-150"}

// BenchmarkLayout is a warm layout on one runtime.
func BenchmarkLayout(b *testing.B) {
	for _, name := range benchNames {
		c := goldenCaseByName(b, name)
		b.Run(name, func(b *testing.B) {
			e := New(Options{Mode: ModeGoja, MaxRuntimes: 1})
			defer e.Close()
			ctx := context.Background()
			for i := 0; i < 2; i++ {
				if _, err := e.Layout(ctx, c.In); err != nil {
					b.Fatal(err)
				}
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(c.In)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := e.Layout(ctx, c.In); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkLayoutParallel is the throughput of the pool: warm layouts of the
// graph with 20 nodes from GOMAXPROCS goroutines.
func BenchmarkLayoutParallel(b *testing.B) {
	c := goldenCaseByName(b, "bench-20")
	e := New(Options{Mode: ModeGoja, MaxRuntimes: runtime.GOMAXPROCS(0)})
	defer e.Close()
	if err := e.Warm(context.Background(), runtime.GOMAXPROCS(0)); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := e.Layout(context.Background(), c.In); err != nil {
				b.Error(err)
				return
			}
		}
	})
}

// BenchmarkCompile is the cost that a process pays one time: parse and
// compile the script.
func BenchmarkCompile(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, _, err := compileSource(true); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNewRuntime is the cost of one more runtime: run the script and
// register the algorithms.
func BenchmarkNewRuntime(b *testing.B) {
	if _, _, err := programs(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := newRuntime(nil); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFirstLayout is a new runtime and its first layout, which is what
// the option Fresh costs for each layout.
func BenchmarkFirstLayout(b *testing.B) {
	c := goldenCaseByName(b, "bench-5")
	if _, _, err := programs(); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt, err := newRuntime(nil)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := rt.run(context.Background(), string(c.In), true, false); err != nil {
			b.Fatal(err)
		}
	}
}

func heapMB() float64 {
	runtime.GC()
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return float64(ms.HeapAlloc) / (1 << 20)
}

// TestPerformanceReport prints the numbers of the performance table. It
// runs only with ELK_PERF=1, because it needs a quiet machine:
//
//	ELK_PERF=1 go test ./internal/elk -run TestPerformanceReport -v
//
// Each time is the minimum of several runs, which is the best estimate of
// the cost on a machine that also runs other programs.
func TestPerformanceReport(t *testing.T) {
	if os.Getenv("ELK_PERF") == "" {
		t.Skip("set ELK_PERF=1 to measure")
	}
	minOf := func(n int, f func()) time.Duration {
		best := time.Duration(1<<63 - 1)
		for i := 0; i < n; i++ {
			start := time.Now()
			f()
			if d := time.Since(start); d < best {
				best = d
			}
		}
		return best
	}
	median := func(n int, f func()) time.Duration {
		ds := make([]time.Duration, n)
		for i := range ds {
			start := time.Now()
			f()
			ds[i] = time.Since(start)
		}
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		return ds[n/2]
	}
	ctx := context.Background()

	base := heapMB()
	compile := minOf(5, func() {
		if _, _, err := compileSource(true); err != nil {
			t.Fatal(err)
		}
	})
	if _, _, err := programs(); err != nil {
		t.Fatal(err)
	}
	program := heapMB() - base
	t.Logf("compile the script: %v, the compiled program holds %.1f MB", compile.Round(time.Millisecond), program)

	load := minOf(7, func() {
		if _, err := newRuntime(nil); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("new runtime (run the script, register the algorithms): %v", load.Round(100*time.Microsecond))

	small := goldenCaseByName(t, "bench-5")
	first := minOf(7, func() {
		rt, err := newRuntime(nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rt.run(ctx, string(small.In), true, false); err != nil {
			t.Fatal(err)
		}
	})
	t.Logf("new runtime and first layout (bench-5): %v", first.Round(time.Millisecond))
	t.Logf("cold start (compile, new runtime, first layout): %v", (compile + first).Round(time.Millisecond))

	// Memory of warm runtimes: load, then lay out each benchmark graph.
	before := heapMB()
	const n = 4
	rts := make([]*jsRuntime, n)
	for i := range rts {
		rt, err := newRuntime(nil)
		if err != nil {
			t.Fatal(err)
		}
		rts[i] = rt
	}
	loaded := (heapMB() - before) / n
	for _, rt := range rts {
		for _, name := range benchNames[:3] {
			if _, err := rt.run(ctx, string(goldenCaseByName(t, name).In), true, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	warm := (heapMB() - before) / n
	t.Logf("memory of one runtime: %.1f MB after the load, %.1f MB after layouts of 5, 20 and 50 nodes", loaded, warm)
	runtime.KeepAlive(rts)

	for _, name := range benchNames {
		c := goldenCaseByName(t, name)
		rt := rts[0]
		reps := 12
		switch {
		case c.Nodes > 100:
			reps = 2
		case c.Nodes > 40:
			reps = 4
		}
		run := func() {
			if _, err := rt.run(ctx, string(c.In), true, false); err != nil {
				t.Fatal(err)
			}
		}
		run()
		var ms1, ms2 runtime.MemStats
		runtime.ReadMemStats(&ms1)
		cpu := processCPU()
		best := minOf(reps, run)
		cpu = (processCPU() - cpu) / time.Duration(reps)
		runtime.ReadMemStats(&ms2)
		med := median(reps, run)
		alloc := float64(ms2.TotalAlloc-ms1.TotalAlloc) / float64(reps) / (1 << 20)
		t.Logf("%-9s %3d nodes, %6d bytes in: warm layout min %v, median %v, process CPU %v, %.0f MB allocated",
			name, c.Nodes, len(c.In), best.Round(100*time.Microsecond), med.Round(100*time.Microsecond), cpu.Round(100*time.Microsecond), alloc)
	}
	t.Log(fmt.Sprintf("GOMAXPROCS %d, %s/%s", runtime.GOMAXPROCS(0), runtime.GOOS, runtime.GOARCH))
}
