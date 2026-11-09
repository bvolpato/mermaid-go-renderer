package native_test

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native"
	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/nativetest"
)

var (
	coseBilkentOnce  sync.Once
	coseBilkentCases *nativetest.CoseBilkentFile
	coseBilkentErr   error
)

// loadCoseBilkent reads testdata/cosebilkent.json.gz, which
// TestWriteNativeGoldens of internal/cytoscape writes from the JavaScript in
// the interpreter.
func loadCoseBilkent(t testing.TB) []*nativetest.CoseBilkentCase {
	t.Helper()
	coseBilkentOnce.Do(func() {
		coseBilkentCases = &nativetest.CoseBilkentFile{}
		coseBilkentErr = nativetest.ReadGzipJSON("testdata/cosebilkent.json.gz", coseBilkentCases)
	})
	if coseBilkentErr != nil {
		t.Fatal(coseBilkentErr)
	}
	return coseBilkentCases.Cases
}

// tally counts the cases of a golden file by what the port did.
type tally struct {
	total, exact, fallback, different int
	reasons                           map[string]int
	// stages counts the trace records that were compared, by stage.
	stages map[string]int
}

func (ta *tally) fall(reason string) {
	ta.fallback++
	if ta.reasons == nil {
		ta.reasons = map[string]int{}
	}
	if i := strings.IndexByte(reason, '\n'); i >= 0 {
		reason = reason[:i]
	}
	ta.reasons[reason]++
}

func (ta *tally) traced(records []nativetest.TraceRecord) {
	if ta.stages == nil {
		ta.stages = map[string]int{}
	}
	for _, r := range records {
		ta.stages[r.Stage]++
	}
}

func (ta *tally) report(t *testing.T, what string) {
	t.Helper()
	t.Logf("%s: %d cases: %d native and bit-exact, %d not covered, %d different", what, ta.total, ta.exact, ta.fallback, ta.different)
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
	if ta.different > 0 {
		t.Errorf("%s: %d native layouts differ from the JavaScript. An input that the port cannot lay out exactly must not be covered", what, ta.different)
	}
}

// TestCoseBilkentGolden runs each case of the golden file through the port.
// The port lays the case out with the bits of the JavaScript at each trace
// record and in the result, or it reports that it does not cover the case.
func TestCoseBilkentGolden(t *testing.T) {
	var ta tally
	corpus, corpusExact := 0, 0
	for _, c := range loadCoseBilkent(t) {
		ta.total++
		isCorpus := strings.HasPrefix(c.Name, "corpus/") || strings.HasPrefix(c.Name, "docs/")
		if isCorpus {
			corpus++
		}
		nodes, edges := c.Input()
		used := 0
		tracer := &nativetest.Tracer{RandomCalls: func() int { return used }}
		res, reason := native.CoseBilkent(&native.Env{Random: nativetest.Replay(c.Draws, &used), Trace: tracer.Trace}, nodes, edges)
		if reason != "" {
			ta.fall(reason)
			if res != nil {
				t.Errorf("%s: a result with the reason %q", c.Name, reason)
			}
			continue
		}
		if c.Error != nil {
			ta.different++
			t.Errorf("%s: the port has a result, and the JavaScript throws %s: %s", c.Name, c.Error.Name, c.Error.Message)
			continue
		}
		var diffs []string
		if at, text := nativetest.FirstDifference(tracer.Records, c.Trace); at >= 0 {
			diffs = append(diffs, "trace: "+text)
		}
		if used != len(c.Draws) {
			diffs = append(diffs, fmt.Sprintf("%d Math.random calls, want %d", used, len(c.Draws)))
		}
		if len(res.Nodes) != len(c.Result.Nodes) || len(res.Edges) != len(c.Result.Edges) {
			diffs = append(diffs, "the number of elements differs")
		} else {
			for i, want := range c.Result.Nodes {
				got := res.Nodes[i]
				if got.ID != nodes[i].ID || !nativetest.SameBits(got.X, float64(want[0])) || !nativetest.SameBits(got.Y, float64(want[1])) {
					diffs = append(diffs, fmt.Sprintf("node %s: got (%v, %v), want (%v, %v)", got.ID, got.X, got.Y, want[0], want[1]))
					break
				}
			}
			for i, want := range c.Result.Edges {
				got := res.Edges[i]
				vals := [6]float64{got.StartX, got.StartY, got.MidX, got.MidY, got.EndX, got.EndY}
				for k := range vals {
					if !nativetest.SameBits(vals[k], float64(want[k])) {
						diffs = append(diffs, fmt.Sprintf("edge %s value %d: got %v, want %v", got.ID, k, vals[k], want[k]))
						break
					}
				}
				if got.ID != edges[i].ID || got.Source != edges[i].Start || got.Target != edges[i].End {
					diffs = append(diffs, fmt.Sprintf("edge %d: the ids differ", i))
				}
			}
		}
		if len(diffs) > 0 {
			ta.different++
			if ta.different <= 5 {
				t.Errorf("%s (%d nodes): %s", c.Name, len(nodes), strings.Join(diffs[:min(len(diffs), 3)], "; "))
			}
			continue
		}
		ta.exact++
		ta.traced(c.Trace)
		if isCorpus {
			corpusExact++
		}
	}
	ta.report(t, "cose-bilkent")
	t.Logf("cose-bilkent: %d of %d layouts of the corpus and of the documentation are native and bit-exact", corpusExact, corpus)
	if corpusExact != corpus {
		t.Errorf("the port does not cover %d layouts of mermaid diagrams", corpus-corpusExact)
	}
}
