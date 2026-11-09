package p1cycles

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

// TestMorePhases runs the processors of the package against
// testdata/phases.json.gz: states of elkjs for graphs that reach the branches
// which the phase goldens of nativetest do not reach. See
// tools/reference/gen-elk-groupa-phases.mjs.
func TestMorePhases(t *testing.T) {
	factories := map[string]lgraph.Factory{
		"DepthFirstCycleBreaker":       func() lgraph.Processor { return NewDepthFirstCycleBreaker() },
		"GreedyCycleBreaker":           func() lgraph.Processor { return NewGreedyCycleBreaker() },
		"GreedyModelOrderCycleBreaker": func() lgraph.Processor { return NewGreedyModelOrderCycleBreaker() },
		"ModelOrderCycleBreaker":       func() lgraph.Processor { return NewModelOrderCycleBreaker() },
	}
	f, err := os.Open("testdata/phases.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []*nativetest.Case
	if err := json.NewDecoder(zr).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 80 {
		t.Fatalf("the golden file has %d cases", len(cases))
	}
	steps := map[string]int{}
	failed := map[string]int{}
	reported := 0
	for _, c := range cases {
		for ri, run := range c.Runs {
			for si, step := range run.Steps {
				factory := factories[step.Proc]
				if factory == nil {
					t.Fatalf("%s: a step of the processor %q, which is not in this package", c.Name, step.Proc)
				}
				steps[step.Proc]++
				res := nativetest.CheckStep(run, step, factory())
				if res.OK() {
					continue
				}
				failed[step.Proc]++
				if reported++; reported > 5 {
					continue
				}
				where := fmt.Sprintf("%s, run %d, step %d (%s, %s)", c.Name, ri, si, step.Proc, step.Kind)
				if res.Err != nil {
					t.Errorf("%s: %v", where, res.Err)
				} else {
					t.Errorf("%s: the state after the processor differs from elkjs:\n  %s", where, strings.Join(res.Diffs, "\n  "))
				}
			}
		}
	}
	for name := range factories {
		if steps[name] == 0 {
			t.Errorf("%s: the golden file has no step", name)
		}
		t.Logf("%s: %d steps, %d steps differ from elkjs", name, steps[name], failed[name])
		if failed[name] > 0 {
			t.Errorf("%s: %d of %d steps differ from elkjs", name, failed[name], steps[name])
		}
	}
}
