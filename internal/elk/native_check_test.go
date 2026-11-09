package elk

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native"
)

// nativeTally counts how the Go port handles a set of graphs against the
// recorded results of elkjs.
type nativeTally struct {
	total, identical, fallback, different int
	reasons                               map[string]int
	firstDiffs                            []string
	// nativeTime is the time of the native layouts that are identical, and
	// v8Ms the time that elkjs needs for the same graphs in V8, when the
	// golden file has it.
	nativeTime time.Duration
	v8Ms       float64
	// large and largeIdentical count the graphs with 40 nodes or more: the
	// network simplex algorithm and the greedy switch change their path at
	// that size.
	large, largeIdentical int
}

func (n *nativeTally) add(name string, in, want []byte, v8Ms float64, nodes int) {
	n.total++
	if nodes >= 40 {
		n.large++
	}
	start := time.Now()
	out, reason := native.LayoutReason(in)
	elapsed := time.Since(start)
	if reason == "" && string(out) == string(want) {
		n.nativeTime += elapsed
		n.v8Ms += v8Ms
		if nodes >= 40 {
			n.largeIdentical++
		}
	}
	switch {
	case reason != "":
		n.fallback++
		if n.reasons == nil {
			n.reasons = map[string]int{}
		}
		if i := strings.IndexByte(reason, '\n'); i >= 0 {
			reason = reason[:i]
		}
		n.reasons[reason]++
	case string(out) == string(want):
		n.identical++
	default:
		n.different++
		if len(n.firstDiffs) < 5 {
			n.firstDiffs = append(n.firstDiffs, name+": "+firstDiff(out, want))
		}
	}
}

func (n *nativeTally) report(t *testing.T, what string) {
	t.Helper()
	t.Logf("%s: %d graphs: %d native and identical to elkjs, %d fall back to elkjs, %d different", what, n.total, n.identical, n.fallback, n.different)
	if n.large > 0 {
		t.Logf("%s: %d graphs have 40 nodes or more, %d of them are native and identical", what, n.large, n.largeIdentical)
	}
	if n.identical > 0 {
		line := fmt.Sprintf("%s: the %d native layouts take %v in total, %v on average", what, n.identical, n.nativeTime.Round(time.Microsecond), (n.nativeTime / time.Duration(n.identical)).Round(time.Microsecond))
		if n.v8Ms > 0 {
			line += fmt.Sprintf("; elkjs in V8 takes %.0f ms for the same graphs", n.v8Ms)
		}
		t.Log(line)
	}
	type row struct {
		reason string
		count  int
	}
	var rows []row
	for r, c := range n.reasons {
		rows = append(rows, row{r, c})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].count > rows[j].count || rows[i].count == rows[j].count && rows[i].reason < rows[j].reason
	})
	for i, r := range rows {
		if i >= 12 {
			t.Logf("      ... and %d more reasons", len(rows)-i)
			break
		}
		t.Logf("%6d  %s", r.count, r.reason)
	}
	for _, d := range n.firstDiffs {
		t.Errorf("%s: %s", what, d)
	}
	if n.different > 0 {
		t.Errorf("%s: %d native layouts differ from elkjs. A graph that the port cannot lay out exactly must fall back", what, n.different)
	}
}

// TestNativeAgainstGolden is the measure of the Go port over the golden
// graphs of this package: each graph is laid out by the port and equal to
// elkjs byte for byte, or the port reports that it does not cover the graph.
// No layout may differ.
func TestNativeAgainstGolden(t *testing.T) {
	var tally nativeTally
	for _, c := range loadGolden(t) {
		if c.Hang || c.Err != nil {
			// elkjs has no layout. The port must not have one either.
			if _, reason := native.LayoutReason(c.In); reason == "" {
				t.Errorf("%s: the port laid out a graph that elkjs rejects or does not finish", c.Name)
			}
			continue
		}
		tally.add(c.Name, c.In, c.want(false), c.Ms, c.Nodes)
	}
	tally.report(t, "golden graphs")
}

// TestNativeAgainstMermaid is the same measure over the ELK graphs that
// mermaid produced for the corpus (the fixtures of internal/ru/layout/elk).
func TestNativeAgainstMermaid(t *testing.T) {
	const file = "../ru/layout/elk/testdata/fixtures.jsonl.gz"
	f, err := os.Open(file)
	if err != nil {
		t.Skipf("no fixtures of the ELK glue: %v", err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 256<<20)
	var tally nativeTally
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var rec struct {
			Name    string `json:"name"`
			Renders []struct {
				Elk []struct {
					In  *string `json:"in"`
					Out *string `json:"out"`
				} `json:"elk"`
			} `json:"renders"`
		}
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatal(err)
		}
		for _, r := range rec.Renders {
			for _, call := range r.Elk {
				if call.In == nil || call.Out == nil {
					continue
				}
				tally.add(rec.Name, []byte(*call.In), hashMember.ReplaceAll([]byte(*call.Out), nil), 0, 0)
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if tally.total < 500 {
		t.Fatalf("only %d graphs", tally.total)
	}
	tally.report(t, "mermaid graphs")
}

// largeCase is one line of testdata/mermaid-large.jsonl.gz: an ELK graph that
// the Go renderer produced for a diagram of testdata/corpus/mermaid-large,
// with the result of elkjs in goja. TestRecordLargeCorpus writes the file.
type largeCase struct {
	Name  string          `json:"name"`
	Nodes int             `json:"nodes"`
	In    json.RawMessage `json:"in"`
	Out   json.RawMessage `json:"out"`
}

// TestNativeAgainstMermaidLarge is the measure over the ELK graphs of large
// real mermaid diagrams (up to 150 nodes). The recorded mermaid population
// has at most 22 nodes in a graph, so only these graphs cross the sizes
// where the layered algorithm changes its path.
func TestNativeAgainstMermaidLarge(t *testing.T) {
	f, err := os.Open("testdata/mermaid-large.jsonl.gz")
	if err != nil {
		t.Skipf("no recorded graphs of the large corpus: %v. Record them with TestRecordLargeCorpus", err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(zr)
	sc.Buffer(make([]byte, 1<<20), 256<<20)
	var tally nativeTally
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var c largeCase
		if err := json.Unmarshal(sc.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		tally.add(c.Name, c.In, c.Out, 0, c.Nodes)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	tally.report(t, "large mermaid graphs")
	if tally.large == 0 {
		t.Error("no recorded graph has 40 nodes or more")
	}
}

// TestModeCheck runs the engine in ModeCheck and ModeNative on a few graphs:
// the result is the result of elkjs, and a fallback is not an error.
func TestModeCheck(t *testing.T) {
	cases := loadGolden(t)
	var pick []*goldenCase
	for _, c := range cases {
		if !c.Hang && c.Err == nil && c.Ms < 10 && strings.HasPrefix(c.Name, "mermaid-layered-") && len(pick) < 6 {
			pick = append(pick, c)
		}
	}
	for _, mode := range []Mode{ModeCheck, ModeNative} {
		e := newTestEngine(Options{MaxRuntimes: 1, Mode: mode})
		for _, c := range pick {
			got, err := e.Layout(context.Background(), c.In)
			var ce *CheckError
			if errors.As(err, &ce) {
				t.Errorf("%s in mode %s: %v", c.Name, mode, err)
				continue
			}
			if err != nil {
				t.Errorf("%s in mode %s: %v", c.Name, mode, err)
				continue
			}
			if string(got) != string(c.want(false)) {
				t.Errorf("%s in mode %s: %s", c.Name, mode, firstDiff(got, c.want(false)))
			}
		}
		st := e.Stats()
		if st.Native+st.Fallback != int64(len(pick)) {
			t.Errorf("mode %s: %d native and %d fallback layouts, want %d in total", mode, st.Native, st.Fallback, len(pick))
		}
		e.Close()
	}
}

// BenchmarkNativeLayout times the Go port on the benchmark graphs. A graph
// that the port does not cover is skipped.
func BenchmarkNativeLayout(b *testing.B) {
	for _, name := range benchNames {
		c := goldenCaseByName(b, name)
		b.Run(name, func(b *testing.B) {
			if _, reason := native.LayoutReason(c.In); reason != "" {
				b.Skipf("the port does not cover the graph: %s", strings.SplitN(reason, "\n", 2)[0])
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(c.In)))
			for i := 0; i < b.N; i++ {
				if _, ok, _ := native.Layout(c.In); !ok {
					b.Fatal("fallback")
				}
			}
		})
	}
}

// TestNativeConcurrent lays out graphs on several goroutines at the same
// time. Each result must be the result of a single layout, so a layout must
// not share state with another one. Run it with -race.
func TestNativeConcurrent(t *testing.T) {
	var pick []*goldenCase
	for _, c := range loadGolden(t) {
		if c.Hang || c.Err != nil || c.Ms > 20 {
			continue
		}
		if _, reason := native.LayoutReason(c.In); reason == "" {
			pick = append(pick, c)
		}
		if len(pick) == 40 {
			break
		}
	}
	if len(pick) == 0 {
		t.Skip("the port covers no golden graph yet")
	}
	done := make(chan string, 8)
	for w := 0; w < 8; w++ {
		go func(w int) {
			for round := 0; round < 4; round++ {
				for i := range pick {
					c := pick[(i+w*5)%len(pick)]
					out, ok, _ := native.Layout(c.In)
					if !ok || string(out) != string(c.want(false)) {
						done <- c.Name
						return
					}
				}
			}
			done <- ""
		}(w)
	}
	for w := 0; w < 8; w++ {
		if name := <-done; name != "" {
			t.Errorf("%s: a concurrent layout differs from elkjs", name)
		}
	}
	t.Logf("%d graphs on 8 goroutines", len(pick))
}

// TestNativeSpeed compares the Go port with elkjs in goja on the benchmark
// graphs, by the CPU time of the process and by the best wall time. It runs
// with ELK_PERF=1.
func TestNativeSpeed(t *testing.T) {
	if os.Getenv("ELK_PERF") == "" {
		t.Skip("set ELK_PERF=1 to measure")
	}
	e := newTestEngine(Options{MaxRuntimes: 1, Mode: ModeGoja, Timeout: 30 * time.Minute})
	defer e.Close()
	ctx := context.Background()
	for _, name := range benchNames {
		c := goldenCaseByName(t, name)
		if _, reason := native.LayoutReason(c.In); reason != "" {
			t.Logf("%s: the port does not cover the graph: %s", name, strings.SplitN(reason, "\n", 2)[0])
			continue
		}
		measure := func(n int, f func()) (cpu, best time.Duration) {
			best = time.Duration(1<<63 - 1)
			start := processCPU()
			for i := 0; i < n; i++ {
				t0 := time.Now()
				f()
				if d := time.Since(t0); d < best {
					best = d
				}
			}
			return (processCPU() - start) / time.Duration(n), best
		}
		reps := 200
		if c.Nodes > 40 {
			reps = 30
		}
		nativeCPU, nativeBest := measure(reps, func() {
			if _, ok, _ := native.Layout(c.In); !ok {
				t.Fatal("fallback")
			}
		})
		// One warm layout, then the measured ones.
		if _, err := e.Layout(ctx, c.In); err != nil {
			t.Fatal(err)
		}
		gojaReps := 3
		if c.Nodes > 40 {
			gojaReps = 1
		}
		gojaCPU, gojaBest := measure(gojaReps, func() {
			if _, err := e.Layout(ctx, c.In); err != nil {
				t.Fatal(err)
			}
		})
		// The CPU time of the process includes the garbage collector on
		// other threads, so the best wall time of the native layout is the
		// lower bound and its CPU time the upper bound.
		t.Logf("%-9s %3d nodes: native %8v best wall, %8v cpu; goja %8v best wall, %8v cpu; V8 %6.1f ms warm; goja cpu / native cpu = %.0f, V8 / native wall = %.1f",
			name, c.Nodes, nativeBest.Round(time.Microsecond), nativeCPU.Round(time.Microsecond), gojaBest.Round(time.Millisecond), gojaCPU.Round(time.Millisecond), c.Ms,
			float64(gojaCPU)/float64(nativeCPU), c.Ms*1e6/float64(nativeBest))
	}
}

// TestNativeRandomSeedZero lays out the golden graphs with elk.randomSeed 0
// natively. elkjs seeds java.util.Random from the clock there, and the port
// seeds it as the bridge does: the k-th generator of a layout has the seed
// fixedNow + k. The test fails when such a layout differs, and it logs
// whether each graph is covered, because a fallback proves nothing about the
// seed.
func TestNativeRandomSeedZero(t *testing.T) {
	covered := 0
	for _, c := range loadGolden(t) {
		if !strings.HasPrefix(c.Name, "special-random-seed-zero") {
			continue
		}
		out, reason := native.LayoutReason(c.In)
		switch {
		case reason != "":
			t.Logf("%s: falls back: %s", c.Name, strings.SplitN(reason, "\n", 2)[0])
		case string(out) == string(c.want(false)):
			covered++
			t.Logf("%s: native and identical to elkjs", c.Name)
		default:
			t.Errorf("%s: %s", c.Name, firstDiff(out, c.want(false)))
		}
	}
	if covered == 0 {
		t.Log("no graph with elk.randomSeed 0 is laid out natively: the clock seed of the port is not verified end to end")
	}
}
