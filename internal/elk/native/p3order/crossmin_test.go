package p3order

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/preserveorder"
)

// crossminProcessors has the processors of the steps of the golden
// testdata/crossmin.jsonl.gz, by step name.
var crossminProcessors = map[string]lgraph.Factory{
	"LayerSweepCrossingMinimizer": func() lgraph.Processor {
		return NewLayerSweepCrossingMinimizer(CrossMinTypeBarycenter)
	},
	"LayerSweepCrossingMinimizer:ONE_SIDED_GREEDY_SWITCH": func() lgraph.Processor {
		return NewLayerSweepCrossingMinimizer(CrossMinTypeOneSidedGreedySwitch)
	},
	"LayerSweepCrossingMinimizer:TWO_SIDED_GREEDY_SWITCH": func() lgraph.Processor {
		return NewLayerSweepCrossingMinimizer(CrossMinTypeTwoSidedGreedySwitch)
	},
	"SortByInputModelProcessor": func() lgraph.Processor {
		return preserveorder.NewSortByInputModelProcessor()
	},
}

// TestCrossminGolden runs the crossing minimization processors against the
// states of elkjs 0.9.3 for the graphs of tools/reference/gen-elk-crossmin.mjs:
// graphs with many crossings, with ports of each kind and with hierarchy,
// each with option sets that the phase goldens do not have. The environment
// variables ELK_PHASE_CASE and ELK_PHASE_DIFFS work as in the phase tests.
func TestCrossminGolden(t *testing.T) {
	f, err := os.Open("testdata/crossmin.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	filter := os.Getenv("ELK_PHASE_CASE")
	maxDiffs := 12
	if n, err := strconv.Atoi(os.Getenv("ELK_PHASE_DIFFS")); err == nil && n > 0 {
		maxDiffs = n
	}
	type count struct{ steps, changed, failed int }
	counts := map[string]*count{}
	cases, printed := 0, 0
	r := bufio.NewReaderSize(zr, 1<<20)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 1 {
			c := &nativetest.Case{}
			if err := json.Unmarshal(line, c); err != nil {
				t.Fatal(err)
			}
			if filter != "" && !strings.Contains(c.Name, filter) {
				continue
			}
			cases++
			for ri, run := range c.Runs {
				for si, step := range run.Steps {
					newProcessor := crossminProcessors[step.Proc]
					if newProcessor == nil {
						t.Fatalf("%s: no processor for the step %s", c.Name, step.Proc)
					}
					key := step.Proc + " (" + step.Kind + ")"
					if counts[key] == nil {
						counts[key] = &count{}
					}
					counts[key].steps++
					if *step.Before != *step.After {
						counts[key].changed++
					}
					diffs, err := checkStepInTime(run, step, newProcessor(), maxDiffs)
					if err == nil && len(diffs) == 0 {
						continue
					}
					counts[key].failed++
					if printed++; printed > 5 {
						continue
					}
					where := fmt.Sprintf("%s, run %d, step %d (%s, %s, graph %d)", c.Name, ri, si, step.Proc, step.Kind, step.Graph)
					if err != nil {
						t.Errorf("%s: %v", where, err)
					} else {
						t.Errorf("%s: the state after the processor differs from elkjs:\n  %s", where, strings.Join(diffs, "\n  "))
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
	if cases == 0 {
		t.Fatalf("the golden has no case (ELK_PHASE_CASE=%q)", filter)
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c := counts[k]
		t.Logf("%s: %d steps (%d steps change the state), %d steps differ from elkjs", k, c.steps, c.changed, c.failed)
		if c.failed > 0 {
			t.Errorf("%s: %d of %d steps differ from elkjs", k, c.failed, c.steps)
		}
	}
	t.Logf("%d graphs", cases)
}

// checkStepInTime is checkStep with a time limit, so that a processor that
// does not end fails the test with the name of the step.
func checkStepInTime(run *nativetest.Run, step *nativetest.Step, p lgraph.Processor, maxDiffs int) ([]string, error) {
	type result struct {
		diffs []string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		diffs, err := checkStep(run, step, p, maxDiffs)
		done <- result{diffs, err}
	}()
	select {
	case r := <-done:
		return r.diffs, r.err
	case <-time.After(20 * time.Second):
		return nil, fmt.Errorf("the processor did not end in 20 s")
	}
}
