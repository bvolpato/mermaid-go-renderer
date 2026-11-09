package compoundtest

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/nativetest"
)

// LoadCases reads a file of extra phase goldens that
// tools/reference/gen-elk-hierarchy.mjs wrote: a gzip file with a JSON list
// of cases in the format of nativetest.
func LoadCases(path string) ([]*nativetest.Case, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	var cases []*nativetest.Case
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cases, nil
}

// CaseFactory makes the processor for one step of a case.
type CaseFactory func(c *nativetest.Case) lgraph.Processor

// Factory returns the CaseFactory of a processor that does not depend on
// the case.
func Factory(f lgraph.Factory) CaseFactory {
	return func(*nativetest.Case) lgraph.Processor { return f() }
}

// RunCases checks each step of the cases against elkjs, with the processor
// that the map has for the name of the step. It fails for a processor of the
// map without a step, and for one whose steps never change the state: such
// goldens do not test the processor.
//
// A step without a state after it is a step in which elkjs failed with an
// exception. The processor must then stop with lgraph.Bail for the state
// before the step, so that the layout falls back to elkjs.
//
// The environment variable ELK_PHASE_CASE limits the run to the cases whose
// name contains its value.
func RunCases(t *testing.T, cases []*nativetest.Case, processors map[string]CaseFactory) {
	t.Helper()
	type tally struct{ steps, changed, failing, failed int }
	tallies := map[string]*tally{}
	for name := range processors {
		tallies[name] = &tally{}
	}
	filter := os.Getenv("ELK_PHASE_CASE")
	printed := 0
	for _, c := range cases {
		if filter != "" && !strings.Contains(c.Name, filter) {
			continue
		}
		for ri, run := range c.Runs {
			for si, step := range run.Steps {
				factory := processors[step.Proc]
				if factory == nil {
					continue
				}
				count := tallies[step.Proc]
				where := fmt.Sprintf("%s, run %d, step %d (%s %s, slot %d, graph %d)", c.Name, ri, si, step.Kind, step.Proc, step.Slot, step.Graph)
				if step.After == nil {
					count.failing++
					if err := checkStops(run, step, factory(c)); err != nil {
						count.failed++
						if printed++; printed <= 6 {
							t.Errorf("%s: elkjs fails in this step: %v", where, err)
						}
					}
					continue
				}
				count.steps++
				if step.Before != nil && *step.Before != *step.After {
					count.changed++
				}
				res := nativetest.CheckStep(run, step, factory(c))
				if res.OK() {
					continue
				}
				count.failed++
				if printed++; printed > 6 {
					continue
				}
				if res.Err != nil {
					t.Errorf("%s: %v", where, res.Err)
				} else {
					t.Errorf("%s: the state after the processor differs from elkjs:\n  %s", where, strings.Join(res.Diffs, "\n  "))
				}
			}
		}
	}
	names := make([]string, 0, len(tallies))
	for name := range tallies {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		count := tallies[name]
		t.Logf("%s: %d steps (%d steps change the state) and %d steps that fail in elkjs, %d steps differ from elkjs", name, count.steps, count.changed, count.failing, count.failed)
		if count.failed > 0 {
			t.Errorf("%s: %d of %d steps differ from elkjs", name, count.failed, count.steps+count.failing)
		}
		if filter == "" && (count.steps == 0 || count.changed == 0) {
			t.Errorf("%s: the goldens have %d steps of the processor, and %d of them change the state", name, count.steps, count.changed)
		}
	}
}

// checkStops builds the graph of the state before a step in which elkjs
// failed, and runs the processor on it. It returns an error when the
// processor does not stop with lgraph.Bail.
func checkStops(run *nativetest.Run, step *nativetest.Step, p lgraph.Processor) error {
	if step.Before == nil {
		return fmt.Errorf("the step has no state before it")
	}
	before, err := run.State(*step.Before)
	if err != nil {
		return err
	}
	built, err := nativetest.Build(before)
	if err != nil {
		return err
	}
	if step.Graph < 0 || step.Graph >= len(built.Graphs) {
		return fmt.Errorf("the step names the graph %d, the state has %d graphs", step.Graph, len(built.Graphs))
	}
	err = nativetest.Process(p, built.Graphs[step.Graph], lgraph.NewContext())
	switch {
	case err == nil:
		return fmt.Errorf("the processor gives a result")
	case !strings.HasPrefix(err.Error(), "the processor stopped"):
		return fmt.Errorf("the processor does not stop with lgraph.Bail: %v", err)
	}
	return nil
}
