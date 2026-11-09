package native_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native"
	"github.com/bvolpato/mermaid-go-renderer/internal/cytoscape/native/nativetest"
)

var (
	architectureOnce  sync.Once
	architectureCases *nativetest.ArchitectureFile
	architectureErr   error
)

// loadArchitecture reads testdata/fcose.json.gz, which
// TestWriteNativeGoldensFcose of internal/cytoscape writes from the
// JavaScript in the interpreter.
func loadArchitecture(t testing.TB) []*nativetest.ArchitectureCase {
	t.Helper()
	architectureOnce.Do(func() {
		architectureCases = &nativetest.ArchitectureFile{}
		architectureErr = nativetest.ReadGzipJSON("testdata/fcose.json.gz", architectureCases)
	})
	if architectureErr != nil {
		t.Fatal(architectureErr)
	}
	return architectureCases.Cases
}

// TestArchitectureGolden runs each case of the golden file through the port.
// The port lays the case out with the bits of the JavaScript at each trace
// record and in the result, or it reports that it does not cover the case.
func TestArchitectureGolden(t *testing.T) {
	var ta tally
	corpus, corpusExact := 0, 0
	for _, c := range loadArchitecture(t) {
		ta.total++
		isCorpus := strings.HasPrefix(c.Name, "corpus/") || strings.HasPrefix(c.Name, "docs/")
		if isCorpus {
			corpus++
		}
		used := 0
		var calls []nativetest.Measure
		tracer := &nativetest.Tracer{RandomCalls: func() int { return used }}
		in := c.Input
		res, reason := native.Architecture(&native.Env{
			Random:      nativetest.Replay(c.Draws, &used),
			MeasureText: nativetest.MeasureReplay(c.Measure, &calls),
			Trace:       tracer.Trace,
		}, &in)
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
		if len(calls) != len(c.Measure) {
			diffs = append(diffs, fmt.Sprintf("%d MeasureText calls, want %d", len(calls), len(c.Measure)))
		} else {
			for i, m := range c.Measure {
				if calls[i].Font != m.Font || calls[i].Text != m.Text {
					diffs = append(diffs, fmt.Sprintf("MeasureText call %d: %q, want %q", i, calls[i].Text, m.Text))
					break
				}
			}
		}
		if len(res.Nodes) != len(c.Result.Nodes) || len(res.Edges) != len(c.Result.Edges) {
			diffs = append(diffs, "the number of elements differs")
		} else {
			for i, want := range c.Result.Nodes {
				got := res.Nodes[i]
				b := got.BoundingBox
				vals := []float64{got.X, got.Y}
				if got.Type == "group" {
					vals = []float64{b.X1, b.Y1, b.X2, b.Y2, b.W, b.H}
				}
				if len(vals) != len(want) {
					diffs = append(diffs, fmt.Sprintf("node %s: %d values, want %d", got.ID, len(vals), len(want)))
					break
				}
				for k := range vals {
					if !nativetest.SameBits(vals[k], float64(want[k])) {
						diffs = append(diffs, fmt.Sprintf("node %s value %d: got %v, want %v", got.ID, k, vals[k], want[k]))
						break
					}
				}
			}
			for i, want := range c.Result.Edges {
				got := res.Edges[i]
				vals := [6]float64{got.SourceEndpoint.X, got.SourceEndpoint.Y, got.Midpoint.X, got.Midpoint.Y, got.TargetEndpoint.X, got.TargetEndpoint.Y}
				for k := range vals {
					if !nativetest.SameBits(vals[k], float64(want[k])) {
						diffs = append(diffs, fmt.Sprintf("edge %s value %d: got %v, want %v", got.ID, k, vals[k], want[k]))
						break
					}
				}
			}
		}
		if len(diffs) > 0 {
			ta.different++
			if ta.different <= 5 {
				t.Errorf("%s: %s", c.Name, strings.Join(diffs[:min(len(diffs), 3)], "; "))
			}
			continue
		}
		ta.exact++
		ta.traced(c.Trace)
		if isCorpus {
			corpusExact++
		}
	}
	ta.report(t, "fcose")
	t.Logf("fcose: %d of %d layouts of the corpus and of the documentation are native and bit-exact", corpusExact, corpus)
	if corpusExact != corpus {
		t.Errorf("the port does not cover %d layouts of mermaid diagrams", corpus-corpusExact)
	}
}
